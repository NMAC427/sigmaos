package pyenv

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"

	"sigmaos/scontainer/python/pylock"

	"github.com/google/uuid"
)

const (
	PYTHON_PACKAGE_CACHE_DIR = "/tmp/python/package-cache"
	PYTHON_TMP_INSTALL_DIR   = PYTHON_PACKAGE_CACHE_DIR + "/tmp"
)

type PythonVersion struct {
	Version        string
	PythonPath     string
	SysTags        []string
	EnvMarkers     map[string]string
	DcontainerPath string
}

func (p *PythonVersion) VersionStr() string {
	return p.Version
}

func (p *PythonVersion) PythonPathStr() string {
	return p.PythonPath
}

func downloadWheel(wheel pylock.Wheel) (string, error) {
	sha256, found := wheel.Hashes["sha256"]
	if !found {
		return "", fmt.Errorf("Wheel %q has no sha256 hash", wheel.Name)
	}

	outPath := filepath.Join("/tmp/python-wheels", sha256, wheel.Name)
	if _, err := os.Stat(outPath); err == nil {
		// File already exists, skip download
		return outPath, nil
	}

	err := os.MkdirAll(filepath.Dir(outPath), 0777)
	if err != nil {
		return "", err
	}
	out, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer out.Close()

	resp, err := http.Get(wheel.URL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return "", err
	}

	hashMatch, err := verifyWheelHash(outPath, &wheel)
	if err != nil {
		return "", err
	}
	if !hashMatch {
		_ = os.Remove(outPath)
		return "", fmt.Errorf("downloaded wheel %q has invalid hash", wheel.Name)
	}
	return outPath, nil
}

func computeSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func verifyWheelHash(path string, wheel *pylock.Wheel) (bool, error) {
	expectedSha256, found := wheel.Hashes["sha256"]
	if !found {
		return false, fmt.Errorf("Wheel %q has no sha256 hash", wheel.Name)
	}

	actualSha256, err := computeSHA256(path)
	if err != nil {
		return false, err
	}

	if actualSha256 != expectedSha256 {
		return false, fmt.Errorf("Wheel %q hash mismatch: expected %s, got %s", wheel.Name, expectedSha256, actualSha256)
	}

	return true, nil
}

func getWheelInstallPath(wheel *pylock.Wheel, pyVersion *PythonVersion) (string, error) {
	sha256, found := wheel.Hashes["sha256"]
	if !found || sha256 == "" {
		return "", fmt.Errorf("wheel %q has no sha256 hash", wheel.Name)
	}
	h0h1 := sha256[:2]
	h2h3 := sha256[2:4]

	// This path MUST match the pattern specified in the sigmaos-uproc AppArmor profile.
	// Otherwise, we would leak the entire cache directory to all procs.
	return filepath.Join(PYTHON_PACKAGE_CACHE_DIR, pyVersion.Version, h0h1, h2h3, sha256), nil
}

func installWheelImpl(wheelPath string, pyVersion *PythonVersion) (string, error) {
	// Install into temporary directory first, and then move to final location
	// to avoid partially installed wheels if installation fails.
	tmpInstallDir := filepath.Join(PYTHON_TMP_INSTALL_DIR, uuid.New().String())
	if err := os.MkdirAll(tmpInstallDir, 0777); err != nil {
		return "", err
	}

	cmd := exec.Command(
		filepath.Join(pyVersion.DcontainerPath, "python"),
		filepath.Join(pyVersion.DcontainerPath, "sigmaos/kernel/install_wheel.py"),
		wheelPath,
		tmpInstallDir,
	)
	cmd.Env = append(cmd.Env, "PYTHONPATH="+filepath.Join(pyVersion.DcontainerPath, "sigmaos/kernel/site-packages"))
	err := cmd.Run()
	if err != nil {
		os.RemoveAll(tmpInstallDir)
		return "", fmt.Errorf("failed to install wheel %q: %w", wheelPath, err)
	}

	return tmpInstallDir, nil
}
