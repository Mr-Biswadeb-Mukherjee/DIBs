// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureRuntimeAssetsRequiresKeywordsFile(t *testing.T) {
	root := t.TempDir()
	paths := testRuntimePaths(root)

	err := ensureRuntimeAssets(paths)
	if err == nil {
		t.Fatal("expected error when Keywords.csv is missing")
	}
}

func TestEnsureRuntimeAssetsAcceptsExistingKeywordsFile(t *testing.T) {
	root := t.TempDir()
	paths := testRuntimePaths(root)

	if err := os.MkdirAll(filepath.Dir(paths.keywordsCSV), 0o750); err != nil {
		t.Fatalf("mkdir keywords dir: %v", err)
	}
	if err := os.WriteFile(paths.keywordsCSV, []byte("my-keyword\n"), 0o600); err != nil {
		t.Fatalf("write custom keywords: %v", err)
	}

	if err := ensureRuntimeAssets(paths); err != nil {
		t.Fatalf("ensureRuntimeAssets error: %v", err)
	}
}

func TestEnsureRuntimeAssetsRejectsDirectoryPath(t *testing.T) {
	root := t.TempDir()
	paths := testRuntimePaths(root)

	if err := os.MkdirAll(paths.keywordsCSV, 0o750); err != nil {
		t.Fatalf("mkdir keywords path: %v", err)
	}

	err := ensureRuntimeAssets(paths)
	if err == nil {
		t.Fatal("expected error when Keywords.csv path is a directory")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Fatalf("expected directory error, got: %v", err)
	}
}

func testRuntimePaths(root string) runtimePaths {
	engine := filepath.Join(root, "Engine")
	setting := filepath.Join(root, "Setting")
	output := filepath.Join(root, "Output")
	return runtimePaths{
		repo:            root,
		engine:          engine,
		logsDir:         filepath.Join(root, "Logs"),
		keywordsCSV:     filepath.Join(engine, "Input", "Keywords.csv"),
		settingConf:     filepath.Join(setting, "setting.conf"),
		redisConf:       filepath.Join(setting, "redis.yaml"),
		dnsIntelOutput:  filepath.Join(output, "DNS_Intel.ndjson"),
		generatedOutput: filepath.Join(output, "Generated_Domain.ndjson"),
		resolvedOutput:  filepath.Join(output, "Resolved_Domain.ndjson"),
	}
}
