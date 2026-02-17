package pyenv

import (
	"errors"
	"sync/atomic"
)

var ErrEvicted = errors.New("package has been evicted")

type refCount struct {
	state atomic.Int32 // >=0: refcount, -1: closed
}

func (c *refCount) acquire() (int32, error) {
	for {
		s := c.state.Load()
		if s < 0 {
			return -1, ErrEvicted
		}
		// try to increment refcount
		if c.state.CompareAndSwap(s, s+1) {
			return s + 1, nil
		}
		// CAS failed -> retry
	}
}

func (c *refCount) release() int32 {
	for {
		s := c.state.Load()
		if s <= 0 {
			panic("release on zero or closed refCount")
		}
		if c.state.CompareAndSwap(s, s-1) {
			return s - 1
		}
	}
}

// tryClose marks the resource closed iff there are no active refs.
// It returns true if the caller successfully closed it.
func (c *refCount) tryClose() bool {
	return c.state.CompareAndSwap(0, -1)
}

// isClosed returns true if the resource has been closed
func (c *refCount) isClosed() bool {
	return c.state.Load() < 0
}

// count returns the current reference count (for debugging/testing)
func (c *refCount) count() int32 {
	s := c.state.Load()
	if s < 0 {
		return -1
	}
	return s
}
