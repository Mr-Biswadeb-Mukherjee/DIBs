// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package bootstrap

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type runtimePaths struct {
	repo            string
	engine          string
	logsDir         string
	keywordsCSV     string
	settingConf     string
	redisConf       string
	dnsIntelOutput  string
	generatedOutput string
}

var (
	pathsOnce sync.Once
	pathsSet  runtimePaths
)

func loadRuntimePaths() runtimePaths {
	pathsOnce.Do(func() {
		repo := detectRepoRoot()
		engineDir := filepath.Join(repo, "Engine")
		pathsSet = runtimePaths{
			repo:            repo,
			engine:          engineDir,
			logsDir:         filepath.Join(repo, "Logs"),
			keywordsCSV:     filepath.Join(engineDir, "Input", "Keywords.csv"),
			settingConf:     filepath.Join(repo, "Setting", "setting.conf"),
			redisConf:       filepath.Join(repo, "Setting", "redis.yaml"),
			dnsIntelOutput:  filepath.Join(repo, "Output", "DNS_Intel.ndjson"),
			generatedOutput: filepath.Join(repo, "Output", "Generated_Domains.ndjson"),
		}
	})
	return pathsSet
}

func detectRepoRoot() string {
	if root := findGoModuleRoot(callerDir()); root != "" {
		return root
	}
	wd, err := os.Getwd()
	if err == nil {
		if root := findGoModuleRoot(wd); root != "" {
			return root
		}
	}
	return "."
}

func callerDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Dir(file)
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
