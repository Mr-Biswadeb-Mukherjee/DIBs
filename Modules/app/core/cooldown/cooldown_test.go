package cooldown

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTriggerSetsActiveAndUntil(t *testing.T) {
	m := NewManager()
	m.Trigger(1)

	if !m.Active() {
		t.Fatalf("expected cooldown to be active after Trigger")
	}

	until := atomic.LoadInt64(&m.until)
	now := time.Now().UnixNano()

	if until <= now {
		t.Fatalf("expected until timestamp to be in the future, got %d <= %d", until, now)
	}
}

func TestRemaining(t *testing.T) {
	m := NewManager()
	m.Trigger(1)

	if m.Remaining() < 0 || m.Remaining() > 1 {
		t.Fatalf("remaining should be between 0 and 1 seconds right after Trigger, got %d", m.Remaining())
	}

	time.Sleep(1100 * time.Millisecond)

	if m.Remaining() != 0 {
		t.Fatalf("expected remaining = 0 after expiration, got %d", m.Remaining())
	}
}

func TestGateBlocksUntilCooldownEnds(t *testing.T) {
	m := NewManager()
	m.Trigger(1)
	m.StartWatcher()

	gate := m.Gate()
	unblocked := make(chan struct{})
	ready := make(chan struct{})

	// Worker goroutine waits on gate
	go func() {
		close(ready)
		<-gate
		close(unblocked)
	}()

	// Ensure worker goroutine started
	select {
	case <-ready:
	case <-time.After(50 * time.Millisecond):
		t.Fatalf("worker did not start in time")
	}

	// Worker must be blocked for at least 150ms
	select {
	case <-unblocked:
		t.Fatalf("worker unblocked too early during cooldown")
	case <-time.After(150 * time.Millisecond):
		// expected
	}

	// Let cooldown expire
	time.Sleep(1100 * time.Millisecond)

	// Worker should now be unblocked
	select {
	case <-unblocked:
		// success
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("worker did not unblock after cooldown expired")
	}
}

func TestStartWatcherDisablesActive(t *testing.T) {
	m := NewManager()
	m.Trigger(1)
	m.StartWatcher()

	if !m.Active() {
		t.Fatalf("expected cooldown to be active immediately after Trigger")
	}

	time.Sleep(1100 * time.Millisecond)

	if m.Active() {
		t.Fatalf("expected cooldown to be inactive after expiration")
	}
}

func TestTriggerRecreatesGate(t *testing.T) {
	m := NewManager()
	oldGate := m.Gate()

	m.Trigger(1)
	newGate := m.Gate()

	if oldGate == newGate {
		t.Fatalf("Trigger should recreate the gate channel")
	}
}

func TestGateClosesOnlyOnce(t *testing.T) {
	m := NewManager()
	m.Trigger(1)
	m.StartWatcher()

	time.Sleep(1100 * time.Millisecond) // allow cooldown to expire

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Gate close caused panic: %v", r)
		}
	}()

	// Multiple reads from closed gate must not panic
	for i := 0; i < 5; i++ {
		select {
		case <-m.Gate():
		default:
		}
	}
}

func TestConcurrentWorkersBlockedThenReleased(t *testing.T) {
	m := NewManager()
	m.Trigger(1)
	m.StartWatcher()

	var wg sync.WaitGroup
	workerCount := 5
	wg.Add(workerCount)

	for i := 0; i < workerCount; i++ {
		go func() {
			defer wg.Done()
			<-m.Gate()
		}()
	}

	// Let goroutines start
	time.Sleep(150 * time.Millisecond)

	// Must still be active
	if !m.Active() {
		t.Fatalf("expected cooldown to be active during worker block")
	}

	// Wait for cooldown to expire
	time.Sleep(1100 * time.Millisecond)

	// All workers should now complete
	wg.Wait()
}
