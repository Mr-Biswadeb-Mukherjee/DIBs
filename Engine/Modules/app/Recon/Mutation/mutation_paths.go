// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package mutation

import (
	"os"
	"path/filepath"
	"runtime"
)

const defaultSettingsPath = "Setting/setting.conf"

func resolveSettingsPath() string {
	if root := findModuleRoot(callerSettingsDir()); root != "" {
		return filepath.Join(root, defaultSettingsPath)
	}

	wd, err := os.Getwd()
	if err == nil {
		if root := findModuleRoot(wd); root != "" {
			return filepath.Join(root, defaultSettingsPath)
		}
	}
	return defaultSettingsPath
}

func callerSettingsDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Dir(file)
}

func findModuleRoot(start string) string {
	dir := start
	for dir != "" {
		if hasGoMod(dir) {
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

func hasGoMod(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}
