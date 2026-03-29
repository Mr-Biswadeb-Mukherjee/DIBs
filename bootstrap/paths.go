// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package bootstrap

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
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
		dibsOutputDir := filepath.Join(repo, "DIBs_Output")
		outputDir := outputDirForInterval(dibsOutputDir, 6, time.Now())
		pathsSet = runtimePaths{
			repo:             repo,
			engine:           engineDir,
			logsDir:          filepath.Join(repo, "Logs"),
			keywordsCSV:      filepath.Join(engineDir, "Input", "Keywords.csv"),
			settingConf:      filepath.Join(repo, "Setting", "setting.conf"),
			redisConf:        filepath.Join(repo, "Setting", "redis.yaml"),
			dnsIntelOutput:   filepath.Join(outputDir, "DNS_Intel.ndjson"),
			generatedOutput:  filepath.Join(outputDir, "Generated_Domain.ndjson"),
			resolvedOutput:   filepath.Join(outputDir, "Resolved_Domain.ndjson"),
			clusterOutput:    filepath.Join(outputDir, "cluster.ndjson"),
			runMetricsOutput: filepath.Join(outputDir, "Run_Metrics.ndjson"),
		}
	})
	return pathsSet
}

func alignRuntimeOutputPaths(paths runtimePaths, intervalHours int, now time.Time) runtimePaths {
	baseDir := filepath.Dir(filepath.Clean(paths.generatedOutput))
	rootDir := filepath.Dir(baseDir)
	outputDir := outputDirForInterval(rootDir, intervalHours, now)
	return withOutputDir(paths, outputDir)
}

func outputDirForInterval(baseDir string, intervalHours int, now time.Time) string {
	if intervalHours <= 0 {
		intervalHours = 6
	}
	latestDir, latestIndex, latestTime, ok := latestIndexedOutputDir(baseDir)
	if !ok {
		return filepath.Join(baseDir, "Output_1")
	}
	if now.Sub(latestTime) < time.Duration(intervalHours)*time.Hour {
		return latestDir
	}
	return filepath.Join(baseDir, "Output_"+strconv.Itoa(latestIndex+1))
}

func latestIndexedOutputDir(baseDir string) (string, int, time.Time, bool) {
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return "", 0, time.Time{}, false
	}
	var latestTime time.Time
	maxIndex := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		idx, ok := parseOutputIndex(entry.Name())
		if !ok {
			continue
		}
		if idx > maxIndex {
			maxIndex = idx
			info, err := entry.Info()
			if err != nil {
				latestTime = time.Time{}
				continue
			}
			latestTime = info.ModTime()
		}
	}
	if maxIndex <= 0 {
		return "", 0, time.Time{}, false
	}
	return filepath.Join(baseDir, "Output_"+strconv.Itoa(maxIndex)), maxIndex, latestTime, true
}

func parseOutputIndex(name string) (int, bool) {
	if !strings.HasPrefix(name, "Output_") {
		return 0, false
	}
	raw := strings.TrimPrefix(name, "Output_")
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func withOutputDir(paths runtimePaths, outputDir string) runtimePaths {
	paths.dnsIntelOutput = filepath.Join(outputDir, "DNS_Intel.ndjson")
	paths.generatedOutput = filepath.Join(outputDir, "Generated_Domain.ndjson")
	paths.resolvedOutput = filepath.Join(outputDir, "Resolved_Domain.ndjson")
	paths.clusterOutput = filepath.Join(outputDir, "cluster.ndjson")
	paths.runMetricsOutput = filepath.Join(outputDir, "Run_Metrics.ndjson")
	return paths
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
