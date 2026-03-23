package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunInvokesAppRunner(t *testing.T) {
	oldRunner := runApp
	oldPrinter := printLine
	t.Cleanup(func() {
		runApp = oldRunner
		printLine = oldPrinter
	})

	var gotCtx context.Context
	var lines []string
	runApp = func(ctx context.Context, deps Dependencies) error {
		gotCtx = ctx
		_ = deps
		return nil
	}
	printLine = func(args ...any) {
		lines = append(lines, strings.TrimSpace(fmt.Sprintln(args...)))
	}

	Run(Dependencies{})

	if gotCtx == nil {
		t.Fatal("expected Run to pass a context to app runner")
	}
	if len(lines) != 1 {
		t.Fatalf("expected one printed line, got %d", len(lines))
	}
	if lines[0] != "Shutdown complete." {
		t.Fatalf("unexpected output: %q", lines[0])
	}
}

func TestRunPrintsErrorAndShutdown(t *testing.T) {
	oldRunner := runApp
	oldPrinter := printLine
	t.Cleanup(func() {
		runApp = oldRunner
		printLine = oldPrinter
	})

	var lines []string
	runApp = func(context.Context, Dependencies) error {
		return errors.New("boom")
	}
	printLine = func(args ...any) {
		lines = append(lines, strings.TrimSpace(fmt.Sprintln(args...)))
	}

	Run(Dependencies{})

	if len(lines) != 2 {
		t.Fatalf("expected two printed lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "Error: boom") {
		t.Fatalf("expected error output, got %q", lines[0])
	}
	if lines[1] != "Shutdown complete." {
		t.Fatalf("unexpected shutdown output: %q", lines[1])
	}
}

func TestRunCreatesUsableContext(t *testing.T) {
	oldRunner := runApp
	oldPrinter := printLine
	t.Cleanup(func() {
		runApp = oldRunner
		printLine = oldPrinter
	})

	runApp = func(ctx context.Context, deps Dependencies) error {
		_ = deps
		select {
		case <-ctx.Done():
			t.Fatal("background context should not be canceled")
		default:
		}
		return nil
	}
	printLine = func(args ...any) {}

	Run(Dependencies{})
}

func TestMakeGeneratedDomainIndexNormalizesAndSorts(t *testing.T) {
	items := []GeneratedDomain{
		{Domain: " Beta.com ", RiskScore: 1.4, GeneratedBy: "mutation"},
		{Domain: "alpha.com", RiskScore: 0.12, Confidence: "medium", GeneratedBy: "dga"},
		{Domain: " ALPHA.com ", RiskScore: -1, Confidence: "", GeneratedBy: ""},
		{Domain: "   "},
	}

	domains, index := makeGeneratedDomainIndex(items)

	if len(domains) != 2 || domains[0] != "alpha.com" || domains[1] != "beta.com" {
		t.Fatalf("unexpected domain list: %#v", domains)
	}

	alpha := index["alpha.com"]
	if alpha.RiskScore != 0 || alpha.Confidence != "low" || alpha.GeneratedBy != "unknown" {
		t.Fatalf("unexpected normalized alpha meta: %#v", alpha)
	}

	beta := index["beta.com"]
	if beta.RiskScore != 1 || beta.Confidence != "low" || beta.GeneratedBy != "mutation" {
		t.Fatalf("unexpected normalized beta meta: %#v", beta)
	}
}

func TestGeneratedDomainRecordHelpersNormalizeValues(t *testing.T) {
	meta := generatedDomainMeta{RiskScore: 0.456, Confidence: "", GeneratedBy: ""}

	unresolved := unresolvedDomainRecord(" Example.com ", meta)
	if unresolved.Domain != "example.com" {
		t.Fatalf("unexpected unresolved domain: %q", unresolved.Domain)
	}
	if unresolved.Score != 0.46 || unresolved.Confidence != "low" || unresolved.GeneratedBy != "unknown" {
		t.Fatalf("unexpected unresolved record: %#v", unresolved)
	}

	resolved := resolvedDomainRecord(" MAIL.EXAMPLE.COM ", meta)
	if resolved.Domain != "mail.example.com" {
		t.Fatalf("unexpected resolved domain: %q", resolved.Domain)
	}
}

func TestRuntimeHelpersBuildSnapshotAndTimeouts(t *testing.T) {
	var submitted int64 = 10
	var completed int64 = 4
	var active int64 = 2

	snapshot, lastCompleted := buildSnapshot(runtimeCounters{
		submitted: &submitted,
		completed: &completed,
		active:    &active,
	}, 1)

	if snapshot.InFlight != 6 || snapshot.QueueDepth != 4 || snapshot.ActiveWorkers != 2 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if snapshot.CompletedDelta != 3 || lastCompleted != 4 {
		t.Fatalf("unexpected completion tracking: delta=%d last=%d", snapshot.CompletedDelta, lastCompleted)
	}
	if got := intelLookupTimeout(0); got != 3*time.Second {
		t.Fatalf("unexpected default timeout: %s", got)
	}
	if got := intelLookupTimeout(100); got != 2*time.Second {
		t.Fatalf("unexpected min timeout clamp: %s", got)
	}
	if got := intelLookupTimeout(400); got != 2400*time.Millisecond {
		t.Fatalf("unexpected scaled timeout: %s", got)
	}
	if got := intelLookupTimeout(2000); got != 8*time.Second {
		t.Fatalf("unexpected max timeout clamp: %s", got)
	}
	if got := ceilSeconds(1500 * time.Millisecond); got != 2 {
		t.Fatalf("unexpected ceilSeconds result: %d", got)
	}

	atomic.StoreInt64(&active, -3)
	snapshot, _ = buildSnapshot(runtimeCounters{
		submitted: &submitted,
		completed: &completed,
		active:    &active,
	}, lastCompleted)
	if snapshot.ActiveWorkers != 0 {
		t.Fatalf("expected negative active workers to clamp to zero, got %d", snapshot.ActiveWorkers)
	}
}
