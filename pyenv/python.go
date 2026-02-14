package pyenv

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"sigmaos/pyenv/pylock"

	"github.com/google/uuid"
)

const (
	PYTHON_PACKAGE_CACHE_DIR = "/tmp/python/package-cache"
	PYTHON_TMP_INSTALL_DIR   = PYTHON_PACKAGE_CACHE_DIR + "/tmp"
)

// PythonVersion represents an immutable Python runtime configuration.
// All fields are unexported and can only be accessed via getter methods.
type PythonVersion struct {
	version        string
	pythonPath     string
	sysTags        []string
	envMarkers     map[string]string
	dcontainerPath string
}

// Version returns the Python version string (e.g., "cpython3.11")
func (p *PythonVersion) Version() string {
	return p.version
}

// PythonPath returns the PYTHONPATH for this Python version
func (p *PythonVersion) PythonPath() string {
	return p.pythonPath
}

// SysTags returns a copy of the system compatibility tags for this Python version
func (p *PythonVersion) SysTags() []string {
	result := make([]string, len(p.sysTags))
	copy(result, p.sysTags)
	return result
}

// EnvMarkers returns a copy of the environment markers for this Python version
func (p *PythonVersion) EnvMarkers() map[string]string {
	result := make(map[string]string, len(p.envMarkers))
	for k, v := range p.envMarkers {
		result[k] = v
	}
	return result
}

// DcontainerPath returns the path to the dcontainer Python installation
func (p *PythonVersion) DcontainerPath() string {
	return p.dcontainerPath
}

var (
	pyOnce     sync.Once
	pyVersions map[string]*PythonVersion
)

func initPythonVersions() {
	pyOnce.Do(func() {
		pyVersions = make(map[string]*PythonVersion)

		// Python 3.11 configuration
		py311 := &PythonVersion{
			version:        "cpython3.11",
			pythonPath:     "/tmp/python/python/build/lib.linux-x86_64-3.11:/tmp/python/python/Lib:/tmp/python/python/sigmaos/user/site-packages",
			sysTags:        loadSysTags("/home/sigmaos/bin/kernel/cpython3.11/sigmaos/sys_tags"),
			envMarkers:     loadEnvMarkers("/home/sigmaos/bin/kernel/cpython3.11/sigmaos/env_markers.json"),
			dcontainerPath: "/home/sigmaos/bin/kernel/cpython3.11",
		}
		pyVersions[py311.Version()] = py311
	})
}

// GetPythonVersion returns the PythonVersion for a given version string (e.g., "python3.11")
func GetPythonVersion(version string) *PythonVersion {
	initPythonVersions()
	return pyVersions[version]
}

// IsSupportedPythonVersion checks if the given version string is supported.
// Returns the PythonVersion if supported, nil otherwise.
// This is a convenience wrapper around GetPythonVersion for backward compatibility.
func IsSupportedPythonVersion(version string) *PythonVersion {
	return GetPythonVersion(version)
}

func loadSysTags(path string) []string {
	file, err := os.Open(path)
	if err != nil {
		return []string{}
	}
	defer file.Close()

	var tags []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			tags = append(tags, line)
		}
	}
	return tags
}

func loadEnvMarkers(path string) map[string]string {
	markers, err := pylock.LoadPythonEnvironmentMarkers(path)
	if err != nil {
		return map[string]string{}
	}
	return markers
}

// DownloadWheel downloads a wheel from its URL and verifies its hash.
// Returns the path to the downloaded wheel.
func DownloadWheel(wheel pylock.Wheel) (string, error) {
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

	hashMatch, err := VerifyWheelHash(outPath, &wheel)
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

// VerifyWheelHash verifies that a wheel file matches its expected SHA256 hash.
func VerifyWheelHash(path string, wheel *pylock.Wheel) (bool, error) {
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

// GetWheelInstallPath returns the installation path for a wheel based on its hash and Python version.
func GetWheelInstallPath(wheel *pylock.Wheel, pyVersion *PythonVersion) (string, error) {
	sha256, found := wheel.Hashes["sha256"]
	if !found || sha256 == "" {
		return "", fmt.Errorf("wheel %q has no sha256 hash", wheel.Name)
	}
	h0h1 := sha256[:2]
	h2h3 := sha256[2:4]

	// This path MUST match the pattern specified in the sigmaos-uproc AppArmor profile.
	// Otherwise, we would leak the entire cache directory to all procs.
	return filepath.Join(PYTHON_PACKAGE_CACHE_DIR, pyVersion.Version(), h0h1, h2h3, sha256), nil
}

// InstallWheel installs a wheel to a temporary directory using the specified Python version.
// Returns the path to the temporary installation directory.
func InstallWheel(wheelPath string, pyVersion *PythonVersion) (string, error) {
	// Install into temporary directory first, and then move to final location
	// to avoid partially installed wheels if installation fails.
	tmpInstallDir := filepath.Join(PYTHON_TMP_INSTALL_DIR, uuid.New().String())
	if err := os.MkdirAll(tmpInstallDir, 0777); err != nil {
		return "", err
	}

	cmd := exec.Command(
		filepath.Join(pyVersion.DcontainerPath(), "python"),
		filepath.Join(pyVersion.DcontainerPath(), "sigmaos/kernel/install_wheel.py"),
		wheelPath,
		tmpInstallDir,
	)
	cmd.Env = append(cmd.Env, "PYTHONPATH="+filepath.Join(pyVersion.DcontainerPath(), "sigmaos/kernel/site-packages"))
	err := cmd.Run()
	if err != nil {
		os.RemoveAll(tmpInstallDir)
		return "", fmt.Errorf("failed to install wheel %q: %w", wheelPath, err)
	}

	return tmpInstallDir, nil
}
