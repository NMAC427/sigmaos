package clnt

import (
	"fmt"
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/pyenv/proto"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/scontainer/python/pylock"
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
func (pc *PyEnvClnt) InstallWheel(wheel *pylock.Wheel, pyVersion *PyVersion) (string, error) {
	if wheel == nil {
		return "", fmt.Errorf("InstallWheel: nil wheel")
	}
	if pyVersion == nil {
		return "", fmt.Errorf("InstallWheel: nil pyVersion")
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

	// Convert PythonVersion to proto
	protoPyVersion := &proto.PythonVersion{
		Version:        pyVersion.Version,
		PythonPath:     pyVersion.PythonPath,
		SysTags:        pyVersion.SysTags,
		EnvMarkers:     pyVersion.EnvMarkers,
		DcontainerPath: pyVersion.DcontainerPath,
	}

	req := &proto.InstallWheelReq{
		Wheel:     protoWheel,
		PyVersion: protoPyVersion,
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

// PyVersion is a client-side representation of Python version metadata
// This mirrors pyenv.PythonVersion but uses public fields for easier conversion from proto
type PyVersion struct {
	Version        string
	PythonPath     string
	SysTags        []string
	EnvMarkers     map[string]string
	DcontainerPath string
}

// ConvertPyVersion converts from scontainer's PythonVersion-like type
// This is a helper to work with the existing Python code in scontainer
func NewPyVersionFromScontainer(version, pythonPath, dcontainerPath string, sysTags []string, envMarkers map[string]string) *PyVersion {
	return &PyVersion{
		Version:        version,
		PythonPath:     pythonPath,
		SysTags:        sysTags,
		EnvMarkers:     envMarkers,
		DcontainerPath: dcontainerPath,
	}
}
