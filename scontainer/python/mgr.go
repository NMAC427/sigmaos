package python

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"sigmaos/scontainer/python/pylock"
	"sync"
)

type installResult struct {
	path string
	err  error
}

type PyMgr struct {
	mu               sync.RWMutex
	installedWheels  map[string]*installResult // keyed by sha256 + version
	downloadedWheels map[string]string         // keyed by URL, stores path
	pendingDownloads map[string]*sync.Cond     // keyed by URL
	pendingInstalls  map[string]*sync.Cond     // keyed by sha256 + version

	installSem  chan struct{}
	downloadSem chan struct{}
}

func newPyMgr() *PyMgr {
	numCPU := runtime.NumCPU()

	return &PyMgr{
		installedWheels:  make(map[string]*installResult),
		downloadedWheels: make(map[string]string),
		pendingDownloads: make(map[string]*sync.Cond),
		pendingInstalls:  make(map[string]*sync.Cond),

		installSem:  make(chan struct{}, numCPU),
		downloadSem: make(chan struct{}, min(32, numCPU+4)),
	}
}

var (
	pyMgrOnce     sync.Once
	pyMgrInstance *PyMgr
)

// Singleton PyMgr instance.
// TODO: Should probably be part of the msched instance
func GetPyMgr() *PyMgr {
	pyMgrOnce.Do(func() {
		pyMgrInstance = newPyMgr()
	})
	return pyMgrInstance
}

func (pm *PyMgr) InstallWheel(wheel *pylock.Wheel, pyVersion *PythonVersion) (string, error) {
	sha256, found := wheel.Hashes["sha256"]
	if !found || sha256 == "" {
		return "", fmt.Errorf("cannot install wheel without sha256 hash: %v", wheel.Name)
	}
	key := fmt.Sprintf("%s_%s", wheel.Hashes["sha256"], pyVersion.version)

	// Check if already installed
	//   Not found -> Need to check file system
	//   Found, nil -> Did check file system. Not installed.
	//   Found, !nil -> Installed, or failed to install.

	// Fast path - Already installed
	pm.mu.RLock()
	if result, found := pm.installedWheels[key]; found && result != nil {
		pm.mu.RUnlock()
		return result.path, result.err
	}
	pm.mu.RUnlock()

	// Slow path
	pm.mu.Lock()
	result, found := pm.installedWheels[key]
	if found && result != nil {
		pm.mu.Unlock()
		return result.path, result.err
	}

	// Check file system - this case happens when we start up with a non-empty cache
	if !found {
		result = checkIfInstalled(wheel, pyVersion)
		if result != nil {
			pm.installedWheels[key] = result
			pm.mu.Unlock()
			return result.path, result.err
		}
		pm.installedWheels[key] = nil
	}
	pm.mu.Unlock()

	// Need to install
	if wheel.URL == "" {
		return "", fmt.Errorf("cannot install wheel without URL: %v", wheel.Name)
	}

	// Download (deduplicated by URL)
	wheelPath, err := pm.downloadWheel(wheel)
	if err != nil {
		return "", err
	}

	// Install (deduplicated by sha256 + version)
	return pm.installWheel(wheel, pyVersion, wheelPath, key)
}

func (pm *PyMgr) downloadWheel(wheel *pylock.Wheel) (string, error) {
	pm.mu.Lock()
	// Return cached download
	if path, found := pm.downloadedWheels[wheel.URL]; found {
		pm.mu.Unlock()
		return path, nil
	}

	// Wait for pending download
	if cond, pending := pm.pendingDownloads[wheel.URL]; pending {
		pm.mu.Unlock()
		cond.L.Lock()
		cond.Wait()
		cond.L.Unlock()

		pm.mu.RLock()
		path, found := pm.downloadedWheels[wheel.URL]
		pm.mu.RUnlock()
		if !found {
			return "", fmt.Errorf("download failed for %s", wheel.URL)
		}
		return path, nil
	}
	cond := sync.NewCond(&sync.Mutex{})
	pm.pendingDownloads[wheel.URL] = cond
	pm.mu.Unlock()

	// Perform download (verifies hash internally)
	pm.downloadSem <- struct{}{}
	path, err := downloadWheel(*wheel)
	<-pm.downloadSem

	// Update state and notify waiters
	pm.mu.Lock()
	if err == nil {
		pm.downloadedWheels[wheel.URL] = path
	}
	delete(pm.pendingDownloads, wheel.URL)
	pm.mu.Unlock()

	cond.L.Lock()
	cond.Broadcast()
	cond.L.Unlock()

	return path, err
}

func (pm *PyMgr) installWheel(wheel *pylock.Wheel, pyVersion *PythonVersion, wheelPath string, key string) (string, error) {
	pm.mu.Lock()
	// Check if installed while downloading
	if result, found := pm.installedWheels[key]; found && result != nil {
		pm.mu.Unlock()
		return result.path, result.err
	}

	// Wait for pending install
	if cond, pending := pm.pendingInstalls[key]; pending {
		pm.mu.Unlock()
		cond.L.Lock()
		cond.Wait()
		cond.L.Unlock()

		pm.mu.RLock()
		result := pm.installedWheels[key]
		pm.mu.RUnlock()
		return result.path, result.err
	}
	cond := sync.NewCond(&sync.Mutex{})
	pm.pendingInstalls[key] = cond
	pm.mu.Unlock()

	// Perform installation
	var installPath string
	var tmpInstallPath string
	var err error

	installPath, err = getWheelInstallPath(wheel, pyVersion)
	if err != nil {
		goto exitUnlocked
	}

	pm.installSem <- struct{}{}
	tmpInstallPath, err = installWheel(wheelPath, pyVersion)
	<-pm.installSem

	if err != nil {
		goto exitUnlocked
	}

	err = os.MkdirAll(path.Dir(installPath), 0777)
	if err != nil {
		goto exitLocked
	}

	pm.mu.Lock()
	// Moving to the final location, and updating the installedWheels map must be
	// done while holding the lock.
	err = os.Rename(tmpInstallPath, installPath)
	if err != nil {
		os.RemoveAll(tmpInstallPath)
		goto exitLocked
	}

	goto exitLocked

exitUnlocked:
	pm.mu.Lock()
exitLocked:
	if err != nil {
		installPath = ""
	}
	pm.installedWheels[key] = &installResult{path: installPath, err: err}
	delete(pm.pendingInstalls, key)
	pm.mu.Unlock()

	cond.L.Lock()
	cond.Broadcast()
	cond.L.Unlock()
	return installPath, err
}

// Checks if a wheel installation is already present on disk.
// Should be called with the PyMgr lock held.
func checkIfInstalled(wheel *pylock.Wheel, pyVersion *PythonVersion) *installResult {
	installPath, err := getWheelInstallPath(wheel, pyVersion)
	if err != nil {
		return &installResult{path: "", err: err}
	}

	if s, err := os.Stat(installPath); err == nil && s.IsDir() {
		return &installResult{path: installPath, err: nil}
	}

	return nil
}
