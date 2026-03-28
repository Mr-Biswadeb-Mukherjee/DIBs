// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package bootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type runtimePaths struct {
	repo             string
	engine           string
	logsDir          string
	keywordsCSV      string
	settingConf      string
	redisConf        string
	dnsIntelOutput   string
	generatedOutput  string
	resolvedOutput   string
	clusterOutput    string
	runMetricsOutput string
}

var (
	pathsOnce sync.Once
	pathsSet  runtimePaths
)

func loadRuntimePaths() runtimePaths {
	pathsOnce.Do(func() {
		repo := detectRuntimeRoot()
		engineDir := filepath.Join(repo, "Engine")
		pathsSet = runtimePaths{
			repo:             repo,
			engine:           engineDir,
			logsDir:          filepath.Join(repo, "Logs"),
			keywordsCSV:      filepath.Join(engineDir, "Input", "Keywords.csv"),
			settingConf:      filepath.Join(repo, "Setting", "setting.conf"),
			redisConf:        filepath.Join(repo, "Setting", "redis.yaml"),
			dnsIntelOutput:   filepath.Join(repo, "Output", "DNS_Intel.ndjson"),
			generatedOutput:  filepath.Join(repo, "Output", "Generated_Domain.ndjson"),
			resolvedOutput:   filepath.Join(repo, "Output", "Resolved_Domain.ndjson"),
			clusterOutput:    filepath.Join(repo, "Output", "cluster.ndjson"),
			runMetricsOutput: filepath.Join(repo, "Output", "Run_Metrics.ndjson"),
		}
	})
	return pathsSet
}

func detectRuntimeRoot() string {
	if dir := executableDir(); dir != "" {
		if !isTemporaryBuildDir(dir) {
			return dir
		}
	}
	if root := moduleRootFromWorkingDir(); root != "" {
		return root
	}
	if dir := executableDir(); dir != "" {
		return dir
	}
	return "."
}

func moduleRootFromWorkingDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return findGoModuleRoot(wd)
}

func executableDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Dir(filepath.Clean(exePath))
}

func isTemporaryBuildDir(dir string) bool {
	clean := filepath.Clean(strings.TrimSpace(dir))
	if clean == "" || !hasGoBuildPathSegment(clean) {
		return false
	}
	if isPathUnderRoot(clean, filepath.Clean(os.TempDir())) {
		return true
	}
	if cache := strings.TrimSpace(os.Getenv("GOCACHE")); cache != "" {
		if isPathUnderRoot(clean, filepath.Clean(cache)) {
			return true
		}
	}
	if cache, err := os.UserCacheDir(); err == nil {
		return isPathUnderRoot(clean, filepath.Clean(cache))
	}
	return false
}

func hasGoBuildPathSegment(path string) bool {
	parts := strings.Split(path, string(filepath.Separator))
	for _, part := range parts {
		if strings.HasPrefix(part, "go-build") {
			return true
		}
	}
	return false
}

func isPathUnderRoot(path string, root string) bool {
	if path == "" || root == "" {
		return false
	}
	if path == root {
		return true
	}
	prefix := root + string(filepath.Separator)
	return strings.HasPrefix(path, prefix)
}

func findGoModuleRoot(start string) string {
	dir := start
	for dir != "" {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	return ""
}
