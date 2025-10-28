package python

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
	"syscall"

	db "sigmaos/debug"
	"sigmaos/scontainer/python/pylock"

	"github.com/google/uuid"
)

const (
	PYTHON_PACKAGE_CACHE_DIR = "/tmp/python-package-cache"
	PYTHON_TMP_INSTALL_DIR   = PYTHON_PACKAGE_CACHE_DIR + "/tmp"
)

type PythonVersion struct {
	Version    string
	PythonPath string

	sysTags        []string
	envMarkers     map[string]string
	dcontainerPath string
}

var sysTagsCache map[string][]string = make(map[string][]string)
var envMarkersCache map[string]map[string]string = make(map[string]map[string]string)

func getSysTagsCached(path string) []string {
	if tags, found := sysTagsCache[path]; found {
		return tags
	}

	file, err := os.Open(path)
	if err != nil {
		db.DFatalf("Failed to get python system compatibility tags: %v", err)
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
	sysTagsCache[path] = tags
	return tags
}

func getEnvMarkersCached(path string) map[string]string {
	if markers, found := envMarkersCache[path]; found {
		return markers
	}

	markers, err := pylock.LoadPythonEnvironmentMarkers(path)
	if err != nil {
		db.DFatalf("Failed to get python environment markers: %v", err)
		return map[string]string{}
	}
	envMarkersCache[path] = markers
	return markers
}

func IsSupportedPythonVersion(version string) *PythonVersion {
	if !strings.HasPrefix(version, "python") {
		return nil
	}

	switch version {
	case "python3.11":
		return &PythonVersion{
			Version:        "cpython3.11",
			PythonPath:     "/tmp/python/python/build/lib.linux-x86_64-3.11:/tmp/python/python/Lib:/tmp/python/python/sigmaos/user/site-packages",
			sysTags:        getSysTagsCached("/home/sigmaos/bin/kernel/cpython3.11/sigmaos/sys_tags"),
			envMarkers:     getEnvMarkersCached("/home/sigmaos/bin/kernel/cpython3.11/sigmaos/env_markers.json"),
			dcontainerPath: "/home/sigmaos/bin/kernel/cpython3.11",
		}
	default:
		return nil
	}
}

// Returns the wheel that best matches the compatibility tags supported by sigmaos.
// Compatibility tags (e.g. cp311-cp311-manylinux_2_39_x86_64) are ordered from
// most preferred to least preferred.
func getBestWheel(pkg pylock.Package, compatibilityTags []string) (*pylock.Wheel, error) {
	if len(pkg.Wheels) == 0 {
		return nil, fmt.Errorf("package %q has no wheels", pkg.Name)
	}

	tagRank := make(map[string]int, len(compatibilityTags))
	for i, tag := range compatibilityTags {
		tagRank[tag] = i
	}

	var best *pylock.Wheel
	bestRank := len(compatibilityTags)

	for i := range pkg.Wheels {
		w := &pkg.Wheels[i]

		base := strings.TrimSuffix(w.Name, ".whl")
		parts := strings.Split(base, "-")
		if len(parts) < 5 {
			continue
		}

		// Expand any compressed tag triples
		pytags := strings.Split(parts[len(parts)-3], ".")
		abitags := strings.Split(parts[len(parts)-2], ".")
		platformtags := strings.Split(parts[len(parts)-1], ".")

		for _, py := range pytags {
			for _, abi := range abitags {
				for _, plat := range platformtags {
					tagTriple := fmt.Sprintf("%s-%s-%s", py, abi, plat)
					if rank, ok := tagRank[tagTriple]; ok && rank < bestRank {
						best = w
						bestRank = rank
					}
				}
			}
		}
	}

	if best == nil {
		return nil, fmt.Errorf("no compatible wheel found for %q", pkg.Name)
	}
	return best, nil
}

func getRequiredWheels(lock *pylock.Pylock, pyVersion *PythonVersion) ([]pylock.Wheel, error) {
	var wheels []pylock.Wheel
	for _, pkg := range lock.Packages {
		is_required, err := pylock.EvaluateMarker(pkg.Marker, pyVersion.envMarkers)
		if err != nil {
			return nil, err
		}

		db.DPrintf(db.CONTAINER, "Python package %v (%v) required: %v (%v)", pkg.Name, pkg.Version, is_required, pkg.Marker)
		if !is_required {
			continue
		}

		wheel, err := getBestWheel(pkg, pyVersion.sysTags)
		if err != nil {
			return nil, err
		}

		wheels = append(wheels, *wheel)
	}

	return wheels, nil
}

func downloadWheel(wheel pylock.Wheel) (string, error) {
	db.DPrintf(db.CONTAINER, "downloading python wheel: %v", wheel.Name)

	sha256, found := wheel.Hashes["sha256"]
	if !found {
		return "", fmt.Errorf("wheel %q has no sha256 hash", wheel.Name)
	}

	outPath := filepath.Join("/tmp/python-wheels", sha256, wheel.Name)
	if _, err := os.Stat(outPath); err == nil {
		hashMatch, err := verifyWheelHash(outPath, &wheel)
		if err != nil {
			return "", err
		}
		if hashMatch {
			// File already exists, skip download
			return outPath, nil
		}

		if err := os.Remove(outPath); err != nil {
			return "", err
		}
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
		return false, fmt.Errorf("wheel %q has no sha256 hash", wheel.Name)
	}

	actualSha256, err := computeSHA256(path)
	if err != nil {
		return false, err
	}

	return actualSha256 == expectedSha256, nil
}

func getWheelInstallPath(wheel *pylock.Wheel, pyVersion *PythonVersion) (string, error) {
	base := strings.TrimSuffix(wheel.Name, ".whl")
	sha256, found := wheel.Hashes["sha256"]
	if !found {
		return "", fmt.Errorf("wheel %q has no sha256 hash", wheel.Name)
	}
	return filepath.Join(PYTHON_PACKAGE_CACHE_DIR, pyVersion.Version, fmt.Sprintf("%s-%s", base, sha256)), nil
}

func installWheel(wheelPath string, installPath string, pyVersion *PythonVersion) error {
	db.DPrintf(db.CONTAINER, "installing python wheel: %v", filepath.Base(wheelPath))
	if err := os.MkdirAll(filepath.Dir(installPath), 0777); err != nil {
		return err
	}

	// Install into temporary directory first, and then move to final location
	// to avoid partially installed wheels if installation fails.
	tmpInstallDir := filepath.Join(PYTHON_TMP_INSTALL_DIR, uuid.New().String())
	if err := os.MkdirAll(tmpInstallDir, 0777); err != nil {
		return err
	}
	defer os.RemoveAll(tmpInstallDir)

	cmd := exec.Command(filepath.Join(pyVersion.dcontainerPath, "python"), filepath.Join(pyVersion.dcontainerPath, "sigmaos/kernel/install_wheel.py"), wheelPath, tmpInstallDir)
	cmd.Env = append(cmd.Env, "PYTHONPATH="+filepath.Join(pyVersion.dcontainerPath, "sigmaos/kernel/site-packages"))
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to install wheel %q: %w", wheelPath, err)
	}

	return os.Rename(tmpInstallDir, installPath)
}

func SetupSitePackages(workingDir string, pyVersion *PythonVersion, pylockPath string) (string, error) {
	lock, err := pylock.ParsePylock(pylockPath)
	if err != nil {
		return "", err
	}

	wheels, err := getRequiredWheels(lock, pyVersion)
	if err != nil {
		return "", err
	}

	totalSize := int64(0)
	for _, wheel := range wheels {
		if wheel.Size != nil {
			totalSize += *wheel.Size
		}
	}
	db.DPrintf(db.CONTAINER, "Total size of required python wheels: %d bytes", totalSize)

	type result struct {
		idx         int
		installPath string
		err         error
	}
	results := make(chan result, len(wheels))
	maxConcurrentInstalls := 4
	sem := make(chan struct{}, maxConcurrentInstalls)

	for i, wheel := range wheels {
		sem <- struct{}{}
		go func(idx int, wheel pylock.Wheel) {
			defer func() { <-sem }()

			installPath, err := getWheelInstallPath(&wheel, pyVersion)
			if err != nil {
				results <- result{idx, "", err}
				return
			}

			if s, err := os.Stat(installPath); err == nil && s.IsDir() {
				// Already installed, skip
				results <- result{idx, installPath, nil}
				return
			}

			wheelPath, err := downloadWheel(wheel)
			if err != nil {
				results <- result{idx, "", err}
				return
			}

			if err := installWheel(wheelPath, installPath, pyVersion); err != nil {
				results <- result{idx, "", err}
				return
			}

			results <- result{idx, installPath, nil}
		}(i, wheel)
	}

	wheelInstallPaths := make([]string, len(wheels))
	for i := 0; i < len(wheels); i++ {
		res := <-results
		if res.err != nil {
			return "", res.err
		}
		wheelInstallPaths[res.idx] = res.installPath
	}

	// Create overlayFS with all the wheels
	overlayDir, err := mountOverlayFS(workingDir, wheelInstallPaths)
	if err != nil {
		return "", err
	}

	return filepath.Join(overlayDir, "site-packages"), nil
}

func mountOverlayFS(workingDir string, lowerdirs []string) (string, error) {
	upperdir := filepath.Join(workingDir, "upper")
	workdir := filepath.Join(workingDir, "work")
	target := filepath.Join(workingDir, "overlay")

	for _, d := range append(lowerdirs, upperdir, workdir, target) {
		if err := os.MkdirAll(d, 0755); err != nil {
			return "", err
		}
	}

	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s",
		strings.Join(lowerdirs, ":"), upperdir, workdir)

	// Use fuse-overlayfs to allow creating an overlayFS inside the docker overlayFS
	cmd := exec.Command("fuse-overlayfs", "-o", opts, target)
	if err := cmd.Run(); err != nil {
		// fuse.overlayfs tends to return non-zero exit code even on success
		// with error: "unknown argument ignored: lazytime"
		// So we double-check with findmnt if the mount was successful.
		findmntCmd := exec.Command("findmnt", "-n", "-t", "fuse.fuse-overlayfs", "-T", target)
		if findmntCmd.Run() != nil {
			return "", fmt.Errorf("setting up python site-packages overlayfs failed (%v): %w", cmd, err)
		}
	}

	return target, nil
}

func CleanSitePackages(workingDir string) error {
	target := filepath.Join(workingDir, "overlay")
	if err := syscall.Unmount(target, 0); err != nil {
		return fmt.Errorf("failed to unmount overlayFS: %w", err)
	}
	return nil
}

func GetPythonFileArg(args []string) (string, int, error) {
	for i, arg := range args {
		if strings.HasSuffix(arg, ".py") && !strings.HasPrefix(arg, "-") {
			return arg, i, nil
		}
	}
	return "", -1, fmt.Errorf("no python file argument found")
}

func GetPylockPath(workingDir string, pythonFile string) (string, error) {
	dir := filepath.Dir(pythonFile)
	pylockFileNames := []string{"pylock.sigmaos.toml", "pylock.toml"}
	for {
		for _, name := range pylockFileNames {
			lockPath := filepath.Join(workingDir, dir, name)
			if _, err := os.Stat(lockPath); err == nil {
				return lockPath, nil
			}
		}

		dir = filepath.Dir(dir)
		if dir == "/" || dir == "." {
			break
		}
	}
	return "", fmt.Errorf("no pylock file found")
}
