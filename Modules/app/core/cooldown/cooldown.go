package cooldown

import (
	"sync/atomic"
	"time"
)

// Manager controls cooldown state and gate blocking.
type Manager struct {
	active int32 // 0 = false, 1 = true
	until  int64 // unix timestamp in nanoseconds

	// gate channel blocks workers while cooldown is active
	gate chan struct{}
}

// NewManager creates a new cooldown manager.
func NewManager() *Manager {
	return &Manager{
		active: 0,
		until:  0,
		gate:   make(chan struct{}),
	}
}

// Trigger activates cooldown for durSeconds (precise, millisecond-safe).
func (m *Manager) Trigger(durSeconds int64) {
	atomic.StoreInt32(&m.active, 1)

	expireAt := time.Now().Add(time.Duration(durSeconds) * time.Second).UnixNano()
	atomic.StoreInt64(&m.until, expireAt)

	// Reset gate so new workers will block
	m.gate = make(chan struct{})
}

// Active returns true if cooldown is active.
func (m *Manager) Active() bool {
	return atomic.LoadInt32(&m.active) == 1
}

// Remaining returns seconds left in cooldown (rounded down).
func (m *Manager) Remaining() int64 {
	if !m.Active() {
		return 0
	}

	now := time.Now().UnixNano()
	until := atomic.LoadInt64(&m.until)

	if now >= until {
		return 0
	}

	return int64(time.Until(time.Unix(0, until)).Seconds())
}

// Gate returns the worker-block channel.
func (m *Manager) Gate() chan struct{} {
	return m.gate
}

// StartWatcher monitors cooldown expiration and releases the gate.
func (m *Manager) StartWatcher() {
	ticker := time.NewTicker(50 * time.Millisecond) // faster precision

	go func() {
		defer ticker.Stop()
		for range ticker.C {
			if !m.Active() {
				continue
			}

			now := time.Now().UnixNano()
			until := atomic.LoadInt64(&m.until)

			if now >= until {
				atomic.StoreInt32(&m.active, 0)
				close(m.gate) // safe: only closed once
			}
		}
	}()
}
