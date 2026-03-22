package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
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
