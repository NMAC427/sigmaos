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
	"sync"
	"syscall"

	db "sigmaos/debug"
	"sigmaos/pyenv/clnt"
	"sigmaos/sigmaclnt"
	"sigmaos/scontainer/python/pylock"

	"github.com/google/uuid"
)

const (
	PYTHON_BASE_PATH         = "/tmp/python"
	PYTHON_PACKAGE_CACHE_DIR = PYTHON_BASE_PATH + "/package-cache" // Shared
	PYTHON_TMP_DIR           = PYTHON_BASE_PATH + "/tmp"           // Not shared
)

type PythonVersion struct {
	version        string
	pythonPath     string
	index          int
	sysTags        []string
	envMarkers     map[string]string
	dcontainerPath string
}

func (p *PythonVersion) Version() string {
	return p.version
}

func (p *PythonVersion) PythonPath() string {
	return p.pythonPath
}

func (p *PythonVersion) SysTags() []string {
	return p.sysTags
}

func (p *PythonVersion) EnvMarkers() map[string]string {
	return p.envMarkers
}

func (p *PythonVersion) DcontainerPath() string {
	return p.dcontainerPath
}

func loadSysTags(path string) []string {
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
	return tags
}

func loadEnvMarkers(path string) map[string]string {
	markers, err := pylock.LoadPythonEnvironmentMarkers(path)
	if err != nil {
		db.DFatalf("Failed to get python environment markers: %v", err)
		return map[string]string{}
	}
	return markers
}

var (
	pyOnce sync.Once

	pyVersions []*PythonVersion
	py311      *PythonVersion
)

func initPython() {
	pyOnce.Do(func() {
		os.MkdirAll(PYTHON_PACKAGE_CACHE_DIR, 0777)
		os.MkdirAll(PYTHON_TMP_DIR, 0777)

		py311 = &PythonVersion{
			version:        "cpython3.11",
			pythonPath:     "/tmp/python/python/build/lib.linux-x86_64-3.11:/tmp/python/python/Lib:/tmp/python/python/sigmaos/user/site-packages",
			sysTags:        loadSysTags("/home/sigmaos/bin/kernel/cpython3.11/sigmaos/sys_tags"),
			envMarkers:     loadEnvMarkers("/home/sigmaos/bin/kernel/cpython3.11/sigmaos/env_markers.json"),
			dcontainerPath: "/home/sigmaos/bin/kernel/cpython3.11",
		}

		pyVersions = []*PythonVersion{py311}
		for i, py := range pyVersions {
			py.index = i
		}
	})
}

func IsSupportedPythonVersion(version string) *PythonVersion {
	if !strings.HasPrefix(version, "python") {
		return nil
	}

	initPython()

	switch version {
	case "python3.11":
		return py311
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

	outPath := filepath.Join(PYTHON_TMP_DIR, uuid.NewString()+"-"+wheel.Name)
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
	return filepath.Join(PYTHON_PACKAGE_CACHE_DIR, pyVersion.version, h0h1, h2h3, sha256), nil
}

func installWheel(wheelPath string, pyVersion *PythonVersion) (string, error) {
	db.DPrintf(db.CONTAINER, "installing python wheel: %v", filepath.Base(wheelPath))

	// Install into temporary directory first, and then move to final location
	// to avoid partially installed wheels if installation fails.
	tmpInstallDir := filepath.Join(PYTHON_TMP_DIR, uuid.NewString())
	if err := os.MkdirAll(tmpInstallDir, 0777); err != nil {
		return "", err
	}

	cmd := exec.Command(filepath.Join(pyVersion.dcontainerPath, "python"), filepath.Join(pyVersion.dcontainerPath, "sigmaos/kernel/install_wheel.py"), wheelPath, tmpInstallDir)
	cmd.Env = append(cmd.Env, "PYTHONPATH="+filepath.Join(pyVersion.dcontainerPath, "sigmaos/kernel/site-packages"))
	err := cmd.Run()
	if err != nil {
		os.RemoveAll(tmpInstallDir)
		return "", fmt.Errorf("failed to install wheel %q: %w", wheelPath, err)
	}

	return tmpInstallDir, nil
}

// TODO: Must keep track of which wheels are currently being used
// by running containers. For automatic eviction of unused wheels,
// we need to only evict wheels not currently in use.
func SetupSitePackages(workingDir string, pyVersion *PythonVersion, pylockPath string, sc *sigmaclnt.SigmaClnt) (string, error) {
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
		path string
		err  error
	}

	// Create PyEnvClnt to communicate with pysrvd
	pyenvClnt, err := clnt.NewPyEnvClnt(sc.FsLib, sc.ProcEnv().GetKernelID())
	if err != nil {
		return "", fmt.Errorf("failed to create pyenv client: %w", err)
	}

	results := make([]result, len(wheels))
	wg := sync.WaitGroup{}
	for i, wheel := range wheels {
		wg.Add(1)
		go func(idx int, wheel pylock.Wheel) {
			defer wg.Done()
			// Convert to PyVersion for the client
			pyVer := clnt.NewPyVersionFromScontainer(
				pyVersion.Version(),
				pyVersion.PythonPath(),
				pyVersion.DcontainerPath(),
				pyVersion.SysTags(),
				pyVersion.EnvMarkers(),
			)
			path, err := pyenvClnt.InstallWheel(&wheel, pyVer)
			results[idx] = result{path: path, err: err}
		}(i, wheel)
	}
	wg.Wait()

	wheelInstallPaths := make([]string, len(wheels))
	for i, res := range results {
		if res.err != nil {
			return "", res.err
		}
		wheelInstallPaths[i] = res.path
	}

	// Create overlayFS with all the wheels
	// TODO: Switch to symlinks & benchmark which is better
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

func symlinkSitePackages(workingDir string, lowerdirs []string) (string, error) {
	if err := os.MkdirAll(workingDir, 0755); err != nil {
		return "", err
	}

	err := symlinkMerge(workingDir, lowerdirs)
	if err != nil {
		return "", err
	}

	return workingDir, nil
}

// Merges multiple input directories into a single output directory using symlinks.
// In case of filename collisions, creates subdirectories to resolve them.
func symlinkMerge(outDir string, inputDirs []string) error {
	// Map to group paths by their filename
	// Key: Filename (e.g., "a", "b")
	// Value: List of full paths where this file exists (e.g., ["/abs/foo/a", "/abs/bar/a"])
	entries := make(map[string][]string)

	for _, src := range inputDirs {
		items, err := os.ReadDir(src)
		if err != nil {
			return fmt.Errorf("failed to read dir %s: %w", src, err)
		}

		for _, item := range items {
			name := item.Name()
			fullPath := filepath.Join(src, name)
			entries[name] = append(entries[name], fullPath)
		}
	}

	for name, paths := range entries {
		targetOut := filepath.Join(outDir, name)
		// CASE 1: Unique (Only exists in one source)
		// We can just symlink the whole thing, whether it's a file or a folder.
		// This handles the "out/b -> bar/b" requirement.
		if len(paths) == 1 {
			if err := os.Symlink(paths[0], targetOut); err != nil {
				return err
			}
			continue
		}

		// CASE 2: Collision (Exists in multiple sources)
		if err := os.MkdirAll(targetOut, 0755); err != nil {
			return fmt.Errorf("failed to create merge dir %s: %w", targetOut, err)
		}
		if err := symlinkMerge(targetOut, paths); err != nil {
			return err
		}
	}

	return nil
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
