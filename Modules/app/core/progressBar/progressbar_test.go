package progressBar

import (
	"sync/atomic"
	"testing"
	"time"
)

// ------------------------------------
// NewProgressBar tests
// ------------------------------------

func TestNewProgressBar(t *testing.T) {
	p := NewProgressBar(100, "desc", "blue")
	if p.colorCode != ColorBlue {
		t.Fatalf("expected blue colorCode")
	}

	if p.bar == nil {
		t.Fatalf("progressbar instance nil")
	}
}

// ------------------------------------
// Update logic (internal state only)
// ------------------------------------

func TestUpdateLogic(t *testing.T) {
	p := NewProgressBar(100, "test", "green")

	p.lastCount = 0
	p.lastTime = time.Now().Add(-1 * time.Second)

	p.Update(20, 100, false, 0)

	if p.lastCount != 20 {
		t.Fatalf("lastCount not updated")
	}

	if time.Since(p.lastTime) > time.Second {
		t.Fatalf("lastTime not updated")
	}
}

// ------------------------------------
// Cooldown color logic
// ------------------------------------

func TestColorForCooldown(t *testing.T) {
	p := NewProgressBar(100, "x", "red")

	if p.colorForCooldown(false) != ColorRed {
		t.Fatalf("color incorrect")
	}

	if p.colorForCooldown(true) != ColorYellow {
		t.Fatalf("cooldown color incorrect")
	}
}

// ------------------------------------
// Auto render logic without checking stdout
// ------------------------------------

func TestStartAutoRender(t *testing.T) {
	var callCount int64 = 0

	p := NewProgressBar(100, "auto", "green")

	get := func() (int64, int64, bool, int64) {
		atomic.AddInt64(&callCount, 1)
		return 10, 100, false, 0
	}

	p.StartAutoRender(get)

	time.Sleep(350 * time.Millisecond)
	p.StopAutoRender()

	if atomic.LoadInt64(&callCount) < 1 {
		t.Fatalf("AutoRender did not invoke getter")
	}
}

// ------------------------------------
// Add increments internal bar counter
// ------------------------------------

func TestAdd(t *testing.T) {
	p := NewProgressBar(50, "add", "green")

	before := p.bar.State().CurrentPercent
	p.Add(5)
	after := p.bar.State().CurrentPercent

	if after <= before {
		t.Fatalf("Add() did not increase progressbar")
	}
}

// ------------------------------------
// Render should not panic
// ------------------------------------

func TestRender(t *testing.T) {
	p := NewProgressBar(20, "render", "green")
	if p.Render() != nil {
		t.Fatalf("Render returned error")
	}
}

// ------------------------------------
// Finish should complete the bar
// ------------------------------------

func TestFinish(t *testing.T) {
	p := NewProgressBar(10, "finish", "yellow")
	p.Finish()

	if !p.bar.IsFinished() {
		t.Fatalf("bar not finished after Finish()")
	}
}
