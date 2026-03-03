package clnt

import (
	"fmt"
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/pyenv"
	proto "sigmaos/pyenv/proto"
	"sigmaos/pyenv/pylock"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type LockHandle uint64

type PyEnvClnt struct {
	fsl  *fslib.FsLib
	rpcc *rpcclnt.RPCClnt
}

func NewPyEnvClnt(fsl *fslib.FsLib, kernelId string) (*PyEnvClnt, error) {
	rpcc, err := sprpcclnt.NewRPCClnt(fsl, filepath.Join(sp.PYENV, kernelId))
	if err != nil {
		db.DPrintf(db.ALWAYS, "NewPyEnvClnt: failed to create RPC client: %v", err)
		return nil, err
	}
	return &PyEnvClnt{fsl, rpcc}, nil
}

// InstallWheels installs multiple wheels atomically and acquires locks on all of them.
// Returns the installation paths and a lock handle that must be passed to ReleaseLocks.
// All-or-nothing semantics: either all wheels are installed and locked, or none are.
func (pc *PyEnvClnt) InstallWheels(wheels []*pylock.Wheel, pythonVersion *pyenv.PythonVersion) ([]string, LockHandle, error) {
	if len(wheels) == 0 {
		return nil, 0, fmt.Errorf("InstallWheels: no wheels to install")
	}
	if pythonVersion == nil {
		return nil, 0, fmt.Errorf("InstallWheels: nil pythonVersion")
	}

	// Convert wheels to proto
	protoWheels := make([]*proto.Wheel, len(wheels))
	for i, wheel := range wheels {
		if wheel == nil {
			return nil, 0, fmt.Errorf("InstallWheels: nil wheel at index %d", i)
		}
		protoWheels[i] = &proto.Wheel{
			Name: wheel.Name,
			Url:  wheel.Url,
			Path: wheel.Path,
			Size: 0,
			Hashes: &proto.Hashes{
				Sha256: wheel.Hashes.Sha256,
			},
		}
		if wheel.Size != nil {
			protoWheels[i].Size = *wheel.Size
		}
	}

	req := &proto.InstallWheelsReq{
		Wheels:        protoWheels,
		PythonVersion: pythonVersion.Version(),
	}

	res := &proto.InstallWheelsRep{}
	if err := pc.rpcc.RPC("PyEnvSrv.InstallWheels", req, res); err != nil {
		db.DPrintf(db.PYENV_ERR, "PyEnvClnt.InstallWheels RPC error: %v", err)
		return nil, 0, err
	}

	if res.Error != "" {
		return nil, 0, fmt.Errorf("PyEnvClnt.InstallWheels: %s", res.Error)
	}

	if res.LockHandle == 0 {
		return nil, 0, fmt.Errorf("PyEnvClnt.InstallWheels: server returned empty lock handle")
	}

	handle := LockHandle(res.LockHandle)
	return res.InstallPaths, handle, nil
}

// ReleaseLocks releases all locks associated with a lock handle.
// This should be called when the process no longer needs the packages (e.g., on cleanup).
func (pc *PyEnvClnt) ReleaseLocks(handle LockHandle) error {
	if handle == 0 {
		return fmt.Errorf("ReleaseLocks: empty handle")
	}
	req := &proto.ReleaseLocksReq{
		LockHandle: uint64(handle),
	}

	res := &proto.ReleaseLocksRep{}
	if err := pc.rpcc.RPC("PyEnvSrv.ReleaseLocks", req, res); err != nil {
		db.DPrintf(db.PYENV_ERR, "PyEnvClnt.ReleaseLocks RPC error: %v", err)
		return err
	}

	if res.Error != "" {
		return fmt.Errorf("PyEnvClnt.ReleaseLocks: %s", res.Error)
	}

	return nil
}

// InstallWheel installs a single Python wheel and acquires a lock on it.
// This is a convenience wrapper around InstallWheels.
// Returns the installation path and a lock handle.
func (pc *PyEnvClnt) InstallWheel(wheel *pylock.Wheel, pythonVersion *pyenv.PythonVersion) (string, LockHandle, error) {
	paths, handle, err := pc.InstallWheels([]*pylock.Wheel{wheel}, pythonVersion)
	if err != nil {
		return "", 0, err
	}
	if len(paths) != 1 {
		return "", 0, fmt.Errorf("InstallWheel: expected 1 path, got %d", len(paths))
	}
	return paths[0], handle, nil
}
