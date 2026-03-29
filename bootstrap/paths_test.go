// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsTemporaryBuildDirTempDir(t *testing.T) {
	path := filepath.Join(os.TempDir(), "go-build123", "b001", "exe")
	if !isTemporaryBuildDir(path) {
		t.Fatalf("expected %q to be treated as temporary go-build dir", path)
	}
}

func TestIsTemporaryBuildDirGoCache(t *testing.T) {
	cacheRoot := filepath.Join(t.TempDir(), "gocache")
	t.Setenv("GOCACHE", cacheRoot)

	path := filepath.Join(cacheRoot, "go-build", "ab", "c")
	if !isTemporaryBuildDir(path) {
		t.Fatalf("expected %q to be treated as temporary go-build dir", path)
	}
}

func TestIsTemporaryBuildDirRejectsProjectPath(t *testing.T) {
	path := filepath.Join(string(filepath.Separator), "workspace", "go-build-tools", "project")
	if isTemporaryBuildDir(path) {
		t.Fatalf("did not expect %q to be treated as temporary go-build dir", path)
	}
}

func TestNextIndexedOutputDirUsesNextIndex(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "DIBs_Output")
	if err := os.MkdirAll(filepath.Join(base, "Output_1"), 0o750); err != nil {
		t.Fatalf("mkdir output_1: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(base, "Output_2"), 0o750); err != nil {
		t.Fatalf("mkdir output_2: %v", err)
	}
	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	old := now.Add(-7 * time.Hour)
	if err := os.Chtimes(filepath.Join(base, "Output_2"), old, old); err != nil {
		t.Fatalf("chtimes output_2: %v", err)
	}
	got := outputDirForInterval(base, 6, now)
	want := filepath.Join(base, "Output_3")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestNextIndexedOutputDirFallsBackToOutput1(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "DIBs_Output")
	got := outputDirForInterval(base, 6, time.Now())
	want := filepath.Join(base, "Output_1")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestOutputDirForIntervalReusesLatestWithinInterval(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "DIBs_Output")
	latest := filepath.Join(base, "Output_2")
	if err := os.MkdirAll(latest, 0o750); err != nil {
		t.Fatalf("mkdir output_2: %v", err)
	}
	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-2 * time.Hour)
	if err := os.Chtimes(latest, recent, recent); err != nil {
		t.Fatalf("chtimes output_2: %v", err)
	}
	got := outputDirForInterval(base, 6, now)
	if got != latest {
		t.Fatalf("expected reuse %q, got %q", latest, got)
	}
}
