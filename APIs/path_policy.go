// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package apis

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	outputDirName  = "Output"
	settingDirName = "Setting"
)

func normalizeOutputDir(path string) (string, error) {
	return normalizeNamedDir(path, outputDirName, defaultDetailsOutputDir)
}

func normalizeSettingFilePath(path string) (string, error) {
	clean := strings.TrimSpace(path)
	if clean == "" {
		clean = defaultPrivateKeyPath
	}
	absPath, err := filepath.Abs(clean)
	if err != nil {
		return "", err
	}
	parent := filepath.Base(filepath.Dir(absPath))
	if !strings.EqualFold(parent, settingDirName) {
		return "", fmt.Errorf("private key path must be inside %s directory", settingDirName)
	}
	if strings.EqualFold(filepath.Base(absPath), settingDirName) {
		return "", errors.New("private key path must be a file")
	}
	return absPath, nil
}

func normalizeNamedDir(path, expectedBase, fallback string) (string, error) {
	clean := strings.TrimSpace(path)
	if clean == "" {
		clean = fallback
	}
	absPath, err := filepath.Abs(clean)
	if err != nil {
		return "", err
	}
	base := filepath.Base(absPath)
	if !strings.EqualFold(base, expectedBase) {
		return "", fmt.Errorf("path must target %s directory", expectedBase)
	}
	return absPath, nil
}

func ensurePathWithinRoot(rootPath, targetPath string) error {
	rootAbs, err := filepath.Abs(strings.TrimSpace(rootPath))
	if err != nil {
		return err
	}
	targetAbs, err := filepath.Abs(strings.TrimSpace(targetPath))
	if err != nil {
		return err
	}
	if targetAbs == rootAbs {
		return nil
	}
	prefix := rootAbs + string(os.PathSeparator)
	if strings.HasPrefix(targetAbs, prefix) {
		return nil
	}
	return errors.New("file must be inside output directory")
}
