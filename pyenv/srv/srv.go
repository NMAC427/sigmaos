package srv

import (
	"fmt"
	"path/filepath"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/pyenv"
	proto "sigmaos/pyenv/proto"
	"sigmaos/pyenv/pylock"
	sessp "sigmaos/session/proto"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

type PyEnvSrv struct {
	sc   *sigmaclnt.SigmaClnt
	pm   *pyenv.PyMgr
	ssrv *sigmasrv.SigmaSrv
}

func newPyEnvSrv(sc *sigmaclnt.SigmaClnt) *PyEnvSrv {
	return &PyEnvSrv{
		sc: sc,
		pm: pyenv.NewPyMgr(),
	}
}

// InstallWheels installs multiple Python wheels atomically and acquires locks on all of them.
func (ps *PyEnvSrv) InstallWheels(ctx fs.CtxI, req proto.InstallWheelsReq, res *proto.InstallWheelsRep) error {
	wheels := req.GetWheels()
	if len(wheels) == 0 {
		return fmt.Errorf("InstallWheels: no wheels in request")
	}

	pythonVersion := req.GetPythonVersion()
	if pythonVersion == "" {
		return fmt.Errorf("InstallWheels: empty python_version in request")
	}

	pyVersion := pyenv.GetPythonVersion(pythonVersion)
	if pyVersion == nil {
		res.InstallPaths = nil
		res.LockHandle = 0
		res.Error = fmt.Sprintf("unsupported Python version: %s", pythonVersion)
		return nil
	}

	// Convert proto wheels to pylock.Wheel
	pylockWheels := make([]*pylock.Wheel, len(wheels))
	for i, wheel := range wheels {
		if wheel == nil {
			return fmt.Errorf("InstallWheels: nil wheel at index %d", i)
		}
		size := wheel.Size
		pylockWheels[i] = &pylock.Wheel{
			Name:   wheel.Name,
			URL:    wheel.Url,
			Path:   wheel.Path,
			Size:   &size,
			Hashes: wheel.Hashes,
		}
	}

	sessionID := ctx.SessionId()
	handle, installPaths, err := ps.pm.InstallWheels(pylockWheels, pyVersion, sessionID)
	if err != nil {
		res.InstallPaths = nil
		res.LockHandle = 0
		res.Error = err.Error()
		return nil
	}

	res.InstallPaths = installPaths
	res.LockHandle = handle.HandleID
	res.Error = ""
	return nil
}

// ReleaseLocks releases all locks associated with a lock handle.
func (ps *PyEnvSrv) ReleaseLocks(ctx fs.CtxI, req proto.ReleaseLocksReq, res *proto.ReleaseLocksRep) error {
	handleID := req.GetLockHandle()
	if handleID == 0 {
		return fmt.Errorf("ReleaseLocks: empty lock_handle in request")
	}

	sessionID := ctx.SessionId()

	// Find the handle in the session's locks
	ps.pm.SessionLocksMu().Lock()
	sessionHandles, ok := ps.pm.SessionLocks()[sessionID]
	if !ok {
		ps.pm.SessionLocksMu().Unlock()
		res.Error = "no locks found for session"
		return nil
	}
	handle, ok := sessionHandles[handleID]
	if !ok {
		ps.pm.SessionLocksMu().Unlock()
		res.Error = "lock handle not found"
		return nil
	}
	delete(sessionHandles, handleID)
	if len(sessionHandles) == 0 {
		delete(ps.pm.SessionLocks(), sessionID)
	}
	ps.pm.SessionLocksMu().Unlock()

	// Release the locks
	if err := ps.pm.ReleaseLocks(handle); err != nil {
		res.Error = err.Error()
		return nil
	}

	res.Error = ""
	return nil
}

// onSessionDetach is called when a session disconnects.
// It releases all locks held by that session.
func (ps *PyEnvSrv) onSessionDetach(sid sessp.Tsession) {
	db.DPrintf(db.PYENV, "Session %v disconnected, releasing all locks", sid)
	ps.pm.ReleaseAllSessionLocks(sid)
}

func Run(kernelId string) {
	pe := proc.GetProcEnv()
	db.DPrintf(db.ALWAYS, "pyenvd starting with kernelId: %v", kernelId)

	sc, err := sigmaclnt.NewSigmaClnt(pe)
	if err != nil {
		db.DFatalf("Error NewSigmaClnt: %v", err)
	}

	psrv := newPyEnvSrv(sc)

	ssrv, err := sigmasrv.NewSigmaSrvClnt(
		filepath.Join(sp.PYENV, sc.ProcEnv().GetKernelID()),
		sc,
		psrv,
	)
	if err != nil {
		db.DFatalf("Error NewSigmaSrv: %v", err)
	}

	psrv.ssrv = ssrv

	db.DPrintf(db.ALWAYS, "pyenvd registering session detach handler")
	psrv.ssrv.RegisterDetachSess(psrv.onSessionDetach)

	db.DPrintf(db.ALWAYS, "pyenvd starting RPC server")
	ssrv.RunServer()
}
