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
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv"
)

type PyEnvSrv struct {
	sc *sigmaclnt.SigmaClnt
	pm *pyenv.PyMgr
}

func newPyEnvSrv(sc *sigmaclnt.SigmaClnt) *PyEnvSrv {
	return &PyEnvSrv{
		sc: sc,
		pm: pyenv.NewPyMgr(),
	}
}

// InstallWheel installs a Python wheel and returns the installation path
func (ps *PyEnvSrv) InstallWheel(ctx fs.CtxI, req proto.InstallWheelReq, res *proto.InstallWheelRep) error {
	// Convert proto wheel to pylock.Wheel
	wheel := req.GetWheel()
	if wheel == nil {
		return fmt.Errorf("InstallWheel: nil wheel in request")
	}

	// Create pylock.Wheel from proto
	size := wheel.Size
	pyLockWheel := &pylock.Wheel{
		Name:   wheel.Name,
		URL:    wheel.Url,
		Path:   wheel.Path,
		Size:   &size,
		Hashes: wheel.Hashes,
	}

	// Look up PythonVersion locally from version string
	pythonVersion := req.GetPythonVersion()
	if pythonVersion == "" {
		return fmt.Errorf("InstallWheel: empty python_version in request")
	}

	pyVersion := pyenv.GetPythonVersion(pythonVersion)
	if pyVersion == nil {
		res.InstallPath = ""
		res.Error = fmt.Sprintf("unsupported Python version: %s", pythonVersion)
		return nil
	}

	path, err := ps.pm.InstallWheel(pyLockWheel, pyVersion)
	if err != nil {
		res.InstallPath = ""
		res.Error = err.Error()
		return nil
	}

	res.InstallPath = path
	res.Error = ""
	return nil
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

	db.DPrintf(db.ALWAYS, "pyenvd starting RPC server")
	ssrv.RunServer()
}
