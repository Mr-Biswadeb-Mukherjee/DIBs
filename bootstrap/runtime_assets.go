// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"
)

func ensureRuntimeAssets(paths runtimePaths) error {
	targetPath := filepath.Clean(paths.keywordsCSV)
	if targetPath == "." || targetPath == "" {
		return fmt.Errorf("invalid runtime keywords path: %s", paths.keywordsCSV)
	}
	if exists, err := fileExists(targetPath); err != nil {
		return err
	} else if exists {
		return nil
	}
	return fmt.Errorf("runtime keywords file missing: %s", targetPath)
}

func fileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	switch {
	case err == nil:
		if info.IsDir() {
			return false, fmt.Errorf("runtime asset path is directory: %s", path)
		}
		return true, nil
	case os.IsNotExist(err):
		return false, nil
	default:
		return false, fmt.Errorf("stat runtime asset path %s: %w", path, err)
	}
}
