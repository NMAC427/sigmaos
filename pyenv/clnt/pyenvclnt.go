package clnt

import (
	"fmt"
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/pyenv"
	"sigmaos/pyenv/proto"
	"sigmaos/pyenv/pylock"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

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

// InstallWheel installs a Python wheel via the pysrvd service
// This call is blocking and waits for the wheel to be installed
// pythonVersion should be a version string like "python3.11"
func (pc *PyEnvClnt) InstallWheel(wheel *pylock.Wheel, pythonVersion *pyenv.PythonVersion) (string, error) {
	if wheel == nil {
		return "", fmt.Errorf("InstallWheel: nil wheel")
	}
	if pythonVersion == nil {
		return "", fmt.Errorf("InstallWheel: nil pythonVersion")
	}

	// Convert wheel to proto
	protoWheel := &proto.Wheel{
		Name:   wheel.Name,
		Url:    wheel.URL,
		Path:   wheel.Path,
		Size:   0,
		Hashes: wheel.Hashes,
	}
	if wheel.Size != nil {
		protoWheel.Size = *wheel.Size
	}

	req := &proto.InstallWheelReq{
		Wheel:         protoWheel,
		PythonVersion: pythonVersion.Version(),
	}

	res := &proto.InstallWheelRep{}
	if err := pc.rpcc.RPC("PyEnvSrv.InstallWheel", req, res); err != nil {
		db.DPrintf(db.ALWAYS, "PyEnvClnt.InstallWheel RPC error: %v", err)
		return "", err
	}

	if res.Error != "" {
		return "", fmt.Errorf("PyEnvClnt.InstallWheel: %s", res.Error)
	}

	return res.InstallPath, nil
}
