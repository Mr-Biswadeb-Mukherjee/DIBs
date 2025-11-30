package ui_test

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/official-biswadeb941/Infermal_v2/Modules/app/core/ui"
	// Update if your module path differs
)

// ----------------------------------------------
// Utility: capture stdout for duration of fn()
// ----------------------------------------------
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	return buf.String()
}

// ----------------------------------------------
// Test: Spinner stops correctly
// ----------------------------------------------
func TestSpinner(t *testing.T) {
	stop := make(chan struct{})

	output := captureOutput(t, func() {
		go func() {
			time.Sleep(150 * time.Millisecond) // enough for animation to run at least once
			close(stop)
		}()

		ui.Spinner(stop, "Loading")
	})

	if !strings.Contains(output, "Loading ✓") {
		t.Fatalf("expected spinner to finish with checkmark, got:\n%s", output)
	}
}

// ----------------------------------------------
// Test: StartBanner prints timestamp & message
// ----------------------------------------------
func TestStartBanner(t *testing.T) {
	out := captureOutput(t, func() {
		ui.StartBanner()
	})

	if !strings.Contains(out, "Engine Started") {
		t.Fatalf("start banner missing expected phrase: %s", out)
	}

	// timestamp check (loose)
	if !strings.Contains(out, "[") || !strings.Contains(out, "]") {
		t.Fatalf("expected timestamp brackets: %s", out)
	}
}

// ----------------------------------------------
// Test: EndBanner prints summary & calculations
// ----------------------------------------------
func TestEndBanner(t *testing.T) {
	start := time.Now().Add(-2 * time.Second) // fake 2s runtime

	out := captureOutput(t, func() {
		ui.EndBanner(start, 42, 30)
	})

	if !strings.Contains(out, "Engine Finished") {
		t.Fatalf("end banner missing finish text: %s", out)
	}

	if !strings.Contains(out, "Total checked: 42") {
		t.Fatalf("missing total checked: %s", out)
	}

	if !strings.Contains(out, "Total resolved: 30") {
		t.Fatalf("missing resolved count: %s", out)
	}

	if !strings.Contains(out, "Resolution time:") {
		t.Fatalf("missing resolution time: %s", out)
	}
}

// ----------------------------------------------
// Test: Spinner does not block if stop is closed immediately
// ----------------------------------------------
func TestSpinnerImmediateStop(t *testing.T) {
	stop := make(chan struct{})
	close(stop)

	out := captureOutput(t, func() {
		ui.Spinner(stop, "FastStop")
	})

	if !strings.Contains(out, "FastStop ✓") {
		t.Fatalf("spinner should exit instantly: %s", out)
	}
}
