// PyMgr manages Python wheel installation and caching across all SigmaOS instances.
// This is a MACHINE-SCOPED manager - all instances on the same machine share the same
// wheel cache at /tmp/python/package-cache. PyMgr should only exist as a service
// (pysrvd), not as a library imported by multiple services.
package pyenv

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"sync"
	"sync/atomic"

	"sigmaos/pyenv/pylock"
	sessp "sigmaos/session/proto"
)

type installResult struct {
	path     string
	err      error
	refCount zeroListItem
}

// LockHandle uniquely identifies a set of acquired locks
type LockHandle struct {
	SessionID   sessp.Tsession
	HandleID    uint64
	refs        []*installResult // References to release on unlock
	releaseOnce sync.Once        // Ensures refs are released exactly once
}

// PyMgr manages Python wheel installation and caching.
// This is a MACHINE-SCOPED SINGLETON, not instance-scoped.
// All SigmaOS instances on the same machine share the same wheel cache
// at /tmp/python/package-cache. This ensures:
// - Wheels are downloaded/installed only once per machine
// - Concurrent operations across instances are coordinated
// - Cache persists across instance restarts
type PyMgr struct {
	mu               sync.RWMutex
	installedWheels  []map[string]*installResult // keyed by sha256, indexed by pyVersion.index
	downloadedWheels map[string]string           // keyed by URL, stores path
	pendingDownloads map[string]*sync.Cond       // keyed by URL
	pendingInstalls  []map[string]*sync.Cond     // keyed by sha256, indexed by pyVersion.index

	installSem  chan struct{}
	downloadSem chan struct{}

	// Session tracking for automatic lock release
	sessionLocks   map[sessp.Tsession]map[uint64]*LockHandle // session -> handleID -> handle
	sessionLocksMu sync.Mutex

	// Handle counter for generating unique lock handles (starts at 1)
	handleCounter atomic.Uint64

	evictionList zeroList // List of wheels with refcount=0, ordered by recency of becoming unused
}

// generateHandleID returns a unique uint64 handle ID
func (pm *PyMgr) generateHandleID() uint64 {
	return pm.handleCounter.Add(1)
}

func NewPyMgr() *PyMgr {
	numCPU := runtime.NumCPU()

	// Initialize Python versions first to get the count
	initPythonVersions()
	numVersions := len(pyVersions)

	// Create two-tiered lookup structures - one map per Python version
	installedWheels := make([]map[string]*installResult, numVersions)
	pendingInstalls := make([]map[string]*sync.Cond, numVersions)
	for i := 0; i < numVersions; i++ {
		installedWheels[i] = make(map[string]*installResult)
		pendingInstalls[i] = make(map[string]*sync.Cond)
	}

	return &PyMgr{
		installedWheels:  installedWheels,
		downloadedWheels: make(map[string]string),
		pendingDownloads: make(map[string]*sync.Cond),
		pendingInstalls:  pendingInstalls,

		installSem:  make(chan struct{}, numCPU),
		downloadSem: make(chan struct{}, min(32, numCPU+4)),

		sessionLocks: make(map[sessp.Tsession]map[uint64]*LockHandle),

		evictionList: newZeroList(),
	}
}

// InstallWheels installs all wheels and acquires read locks on them atomically.
// All-or-nothing semantics: either all locks are acquired and all wheels are installed,
// or the operation fails and no locks are held.
func (pm *PyMgr) InstallWheels(wheels []*pylock.Wheel, pyVersion *PythonVersion, sessionID sessp.Tsession) (*LockHandle, []string, error) {
	pyIdx := pyVersion.Index()

	// First pass: check if all wheels can be acquired (not evicted) and install missing ones
	// We need to do this carefully to avoid deadlocks and ensure atomicity

	// Generate unique handle ID
	handleID := pm.generateHandleID()

	// Results and acquired locks
	installPaths := make([]string, len(wheels))
	acquiredRefs := make([]*installResult, 0, len(wheels))
	sha256s := make([]string, len(wheels))

	// Step 1: Get sha256 for all wheels and verify they have hashes
	for i, wheel := range wheels {
		sha256, found := wheel.Hashes["sha256"]
		if !found || sha256 == "" {
			return nil, nil, fmt.Errorf("wheel %s missing sha256 hash", wheel.Name)
		}
		sha256s[i] = sha256
	}

	// Step 2: Try to acquire all wheels atomically
	// We use a two-phase approach: first install all missing wheels, then acquire locks
	// If any installation fails, we roll back by not acquiring any locks

	// Phase 1: Ensure all wheels are installed (or install them)
	for i, wheel := range wheels {
		sha256 := sha256s[i]

		// Try to get or create the install result
		result, err := pm.getOrInstallWheel(wheel, pyVersion, pyIdx, sha256)
		if err != nil {
			// Installation failed - rollback any acquired refs and return error
			return nil, nil, fmt.Errorf("failed to install %s: %w", wheel.Name, err)
		}

		installPaths[i] = result.path
	}

	// Phase 2: Acquire locks on all installed wheels
	// Try to acquire all locks
	pm.mu.RLock()
	for i, sha256 := range sha256s {
		result := pm.installedWheels[pyIdx][sha256]
		if result == nil {
			pm.mu.RUnlock()
			return nil, nil, fmt.Errorf("wheel %s disappeared during installation", wheels[i].Name)
		}

		// Try to acquire the lock
		err := result.refCount.acquire(&pm.evictionList)
		if err != nil {
			// Failed to acquire - release all previously acquired locks
			pm.mu.RUnlock()
			for _, ref := range acquiredRefs {
				ref.refCount.release(&pm.evictionList)
			}
			return nil, nil, fmt.Errorf("wheel %s was evicted", wheels[i].Name)
		}

		acquiredRefs = append(acquiredRefs, result)
	}
	pm.mu.RUnlock()

	// Create the lock handle
	handle := &LockHandle{
		SessionID: sessionID,
		HandleID:  handleID,
		refs:      acquiredRefs,
	}

	// Register the handle with the session
	pm.sessionLocksMu.Lock()
	if pm.sessionLocks[sessionID] == nil {
		pm.sessionLocks[sessionID] = make(map[uint64]*LockHandle)
	}
	pm.sessionLocks[sessionID][handleID] = handle
	pm.sessionLocksMu.Unlock()

	return handle, installPaths, nil
}

// getOrInstallWheel ensures a wheel is installed and returns its result.
// This doesn't acquire any locks, just ensures installation.
func (pm *PyMgr) getOrInstallWheel(wheel *pylock.Wheel, pyVersion *PythonVersion, pyIdx int, sha256 string) (*installResult, error) {
	// Fast path - Already installed
	pm.mu.RLock()
	if result := pm.installedWheels[pyIdx][sha256]; result != nil {
		if result.err != nil {
			pm.mu.RUnlock()
			return nil, result.err
		}
		pm.mu.RUnlock()
		return result, nil
	}
	pm.mu.RUnlock()

	// Slow path
	pm.mu.Lock()
	result := pm.installedWheels[pyIdx][sha256]
	if result != nil {
		if result.err != nil {
			pm.mu.Unlock()
			return nil, result.err
		}
		pm.mu.Unlock()
		return result, nil
	}

	// Check file system - this case happens when we start up with a non-empty cache
	if result == nil {
		result = checkIfInstalled(wheel, pyVersion)
		if result != nil {
			pm.installedWheels[pyIdx][sha256] = result
			pm.mu.Unlock()
			return result, nil
		}
		pm.installedWheels[pyIdx][sha256] = nil
	}
	pm.mu.Unlock()

	// Need to install
	if wheel.URL == "" {
		return nil, fmt.Errorf("cannot install wheel without URL: %v", wheel.Name)
	}

	// Download (deduplicated by URL)
	wheelPath, err := pm.downloadWheel(wheel)
	if err != nil {
		return nil, err
	}

	// Install (deduplicated by sha256 + version)
	return pm.installWheel(wheel, pyVersion, wheelPath, sha256)
}

// ReleaseLocks releases all locks associated with a handle.
func (pm *PyMgr) ReleaseLocks(handle *LockHandle) error {
	if handle == nil {
		return nil
	}

	// Release all refs exactly once using sync.Once
	handle.releaseOnce.Do(func() {
		for _, ref := range handle.refs {
			ref.refCount.release(&pm.evictionList)
		}
	})

	// Remove from session tracking
	pm.sessionLocksMu.Lock()
	if sessionHandles, ok := pm.sessionLocks[handle.SessionID]; ok {
		delete(sessionHandles, handle.HandleID)
		if len(sessionHandles) == 0 {
			delete(pm.sessionLocks, handle.SessionID)
		}
	}
	pm.sessionLocksMu.Unlock()

	return nil
}

// ReleaseAllSessionLocks releases all locks held by a session.
// This is called automatically when a session disconnects.
func (pm *PyMgr) ReleaseAllSessionLocks(sessionID sessp.Tsession) {
	pm.sessionLocksMu.Lock()
	sessionHandles, ok := pm.sessionLocks[sessionID]
	if !ok {
		pm.sessionLocksMu.Unlock()
		return
	}
	// Copy handles to avoid holding lock during release
	handles := make([]*LockHandle, 0, len(sessionHandles))
	for _, handle := range sessionHandles {
		handles = append(handles, handle)
	}
	delete(pm.sessionLocks, sessionID)
	pm.sessionLocksMu.Unlock()

	// Release all locks (each handle's refs are released at most once via sync.Once)
	for _, handle := range handles {
		handle.releaseOnce.Do(func() {
			for _, ref := range handle.refs {
				ref.refCount.release(&pm.evictionList)
			}
		})
	}
}

// SessionLocksMu returns the mutex for sessionLocks (for external access).
func (pm *PyMgr) SessionLocksMu() *sync.Mutex {
	return &pm.sessionLocksMu
}

// SessionLocks returns the sessionLocks map (for external access).
// Caller must hold SessionLocksMu().
func (pm *PyMgr) SessionLocks() map[sessp.Tsession]map[uint64]*LockHandle {
	return pm.sessionLocks
}

// TryEvict attempts to evict a specific wheel if its refcount is 0.
// Returns true if successfully evicted.
func (pm *PyMgr) TryEvict(sha256 string, pyIdx int) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	result := pm.installedWheels[pyIdx][sha256]
	if result == nil || result.err != nil {
		return false // Not installed or failed installation
	}

	// Try to close (mark as evicted)
	if !result.refCount.tryClose(&pm.evictionList) {
		return false
	}

	// Remove from installed wheels
	delete(pm.installedWheels[pyIdx], sha256)

	// TODO: Also remove from disk
	// os.RemoveAll(result.path)

	return true
}

func (pm *PyMgr) downloadWheel(wheel *pylock.Wheel) (string, error) {
	pm.mu.Lock()
	// Return cached download
	if p, found := pm.downloadedWheels[wheel.URL]; found {
		pm.mu.Unlock()
		return p, nil
	}

	// Wait for pending download
	if cond, pending := pm.pendingDownloads[wheel.URL]; pending {
		pm.mu.Unlock()
		cond.L.Lock()
		cond.Wait()
		cond.L.Unlock()

		pm.mu.RLock()
		p, found := pm.downloadedWheels[wheel.URL]
		pm.mu.RUnlock()
		if !found {
			return "", fmt.Errorf("download failed for %s", wheel.URL)
		}
		return p, nil
	}
	cond := sync.NewCond(&sync.Mutex{})
	pm.pendingDownloads[wheel.URL] = cond
	pm.mu.Unlock()

	// Perform download (verifies hash internally)
	p, err := func() (string, error) {
		pm.downloadSem <- struct{}{}
		defer func() { <-pm.downloadSem }()
		return DownloadWheel(*wheel)
	}()

	// Update state and notify waiters
	pm.mu.Lock()
	if err == nil {
		pm.downloadedWheels[wheel.URL] = p
	}
	delete(pm.pendingDownloads, wheel.URL)
	pm.mu.Unlock()

	cond.L.Lock()
	cond.Broadcast()
	cond.L.Unlock()

	return p, err
}

func (pm *PyMgr) installWheel(wheel *pylock.Wheel, pyVersion *PythonVersion, wheelPath string, sha256 string) (*installResult, error) {
	pyIdx := pyVersion.Index()

	pm.mu.Lock()
	// Check if installed while downloading
	if result := pm.installedWheels[pyIdx][sha256]; result != nil {
		pm.mu.Unlock()
		return result, nil
	}

	// Wait for pending install
	if cond := pm.pendingInstalls[pyIdx][sha256]; cond != nil {
		pm.mu.Unlock()
		cond.L.Lock()
		cond.Wait()
		cond.L.Unlock()

		pm.mu.RLock()
		result := pm.installedWheels[pyIdx][sha256]
		pm.mu.RUnlock()
		return result, nil
	}
	cond := sync.NewCond(&sync.Mutex{})
	pm.pendingInstalls[pyIdx][sha256] = cond
	pm.mu.Unlock()

	// Perform installation
	var installPath string
	var tmpInstallPath string
	var err error

	installPath, err = GetWheelInstallPath(wheel, pyVersion)
	if err != nil {
		goto exitUnlocked
	}

	tmpInstallPath, err = func() (string, error) {
		pm.installSem <- struct{}{}
		defer func() { <-pm.installSem }()
		return InstallWheel(wheelPath, pyVersion)
	}()

	if err != nil {
		goto exitUnlocked
	}

	err = os.MkdirAll(path.Dir(installPath), 0777)
	if err != nil {
		goto exitUnlocked
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
	result := &installResult{path: installPath, err: err}
	pm.installedWheels[pyIdx][sha256] = result
	delete(pm.pendingInstalls[pyIdx], sha256)
	pm.mu.Unlock()

	cond.L.Lock()
	cond.Broadcast()
	cond.L.Unlock()
	return result, err
}

// Checks if a wheel installation is already present on disk.
func checkIfInstalled(wheel *pylock.Wheel, pyVersion *PythonVersion) *installResult {
	installPath, err := GetWheelInstallPath(wheel, pyVersion)
	if err != nil {
		return &installResult{path: "", err: err}
	}

	if s, err := os.Stat(installPath); err == nil && s.IsDir() {
		return &installResult{path: installPath, err: nil}
	}

	return nil
}
