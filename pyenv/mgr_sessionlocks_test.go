package pyenv

import (
	"sync"
	"testing"

	sessp "sigmaos/session/proto"
)

func TestReleaseAllSessionLocksDropsRefs(t *testing.T) {
	pm := NewPyMgr()
	sid := sessp.Tsession(77)

	r1 := &refCount{}
	r2 := &refCount{}
	r3 := &refCount{}
	if err := r1.acquire(); err != nil {
		t.Fatalf("acquire r1: %v", err)
	}
	if err := r2.acquire(); err != nil {
		t.Fatalf("acquire r2: %v", err)
	}
	if err := r3.acquire(); err != nil {
		t.Fatalf("acquire r3: %v", err)
	}

	pm.sessionLocksMu.Lock()
	pm.sessionLocks[sid] = map[uint64]*LockHandle{
		1: {
			SessionID: sid,
			HandleID:  1,
			refs:      []*refCount{r1, r2},
		},
		2: {
			SessionID: sid,
			HandleID:  2,
			refs:      []*refCount{r3},
		},
	}
	pm.sessionLocksMu.Unlock()

	pm.ReleaseAllSessionLocks(sid)

	if r1.count() != 0 || r2.count() != 0 || r3.count() != 0 {
		t.Fatalf("expected all refs released, got r1=%d r2=%d r3=%d", r1.count(), r2.count(), r3.count())
	}

	pm.sessionLocksMu.Lock()
	_, ok := pm.sessionLocks[sid]
	pm.sessionLocksMu.Unlock()
	if ok {
		t.Fatalf("expected session locks entry to be removed")
	}
}

// TestDoubleReleaseRace tests that concurrent ReleaseLocks and ReleaseAllSessionLocks
// don't cause a double-release panic.
func TestDoubleReleaseRace(t *testing.T) {
	pm := NewPyMgr()
	sid := sessp.Tsession(77)

	r1 := &refCount{}
	r2 := &refCount{}
	if err := r1.acquire(); err != nil {
		t.Fatalf("acquire r1: %v", err)
	}
	if err := r2.acquire(); err != nil {
		t.Fatalf("acquire r2: %v", err)
	}

	handle := &LockHandle{
		SessionID: sid,
		HandleID:  1,
		refs:      []*refCount{r1, r2},
	}

	pm.sessionLocksMu.Lock()
	pm.sessionLocks[sid] = map[uint64]*LockHandle{
		1: handle,
	}
	pm.sessionLocksMu.Unlock()

	// Concurrent ReleaseLocks and ReleaseAllSessionLocks should not panic
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		pm.ReleaseLocks(handle)
	}()

	go func() {
		defer wg.Done()
		pm.ReleaseAllSessionLocks(sid)
	}()

	wg.Wait()

	// Verify all refs are released (should be 0, not negative)
	if r1.count() != 0 || r2.count() != 0 {
		t.Fatalf("expected all refs released exactly once, got r1=%d r2=%d", r1.count(), r2.count())
	}
}
