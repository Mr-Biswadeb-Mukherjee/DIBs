// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package bootstrap

import (
	"os"
	"path/filepath"
	"testing"
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
