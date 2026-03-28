// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package apis

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Query parsing and custom file extraction are isolated here so
// details.go can focus on section fetch/read behavior while this file
// keeps request-level validation and file selection rules together.
func parseDetailsQuery(r *http.Request) ([]string, int, error) {
	sections, err := parseDetailsSections(r.URL.Query().Get("section"))
	if err != nil {
		return nil, 0, err
	}
	if custom := strings.TrimSpace(r.URL.Query().Get("file")); custom != "" {
		sections = append(sections, "file:"+custom)
	}
	limitRaw, ok := extractDetailsLimitRaw(r)
	if !ok {
		return nil, 0, errors.New("limit query is required")
	}
	limit, err := parseDetailsLimit(limitRaw)
	if err != nil {
		return nil, 0, err
	}
	return sections, limit, nil
}

func extractDetailsLimitRaw(r *http.Request) (string, bool) {
	if r == nil || r.URL == nil {
		return "", false
	}
	value := strings.TrimSpace(r.URL.Query().Get("limit"))
	if value != "" {
		return value, true
	}
	raw := strings.TrimSpace(r.URL.RawQuery)
	if raw == "" || strings.Contains(raw, "=") || strings.Contains(raw, "&") {
		return "", false
	}
	return raw, true
}

func parseDetailsSections(raw string) ([]string, error) {
	clean := strings.TrimSpace(raw)
	if clean == "" || strings.EqualFold(clean, "all") {
		return slices.Clone(detailSectionOrder), nil
	}
	items := strings.Split(clean, ",")
	sections := make([]string, 0, len(items))
	for _, item := range items {
		section, ok := normalizeDetailsSection(item)
		if !ok {
			return nil, fmt.Errorf("unsupported details section: %s", strings.TrimSpace(item))
		}
		if slices.Contains(sections, section) {
			continue
		}
		sections = append(sections, section)
	}
	if len(sections) == 0 {
		return nil, errors.New("section query cannot be empty")
	}
	return sections, nil
}

func normalizeDetailsSection(value string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(value))
	section, ok := detailSectionAliases[key]
	return section, ok
}

func parseDetailsLimit(raw string) (int, error) {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return 0, errors.New("limit query is required")
	}
	value, err := strconv.Atoi(clean)
	if err != nil {
		return 0, fmt.Errorf("invalid limit value: %s", clean)
	}
	if value <= 0 {
		return 0, errors.New("limit must be greater than zero")
	}
	return value, nil
}

func resolveCustomDetailsFile(outputDir, raw string) (string, error) {
	clean := strings.TrimSpace(raw)
	if clean == "" {
		return "", errors.New("file query cannot be empty")
	}
	ext := strings.ToLower(filepath.Ext(clean))
	if ext != ".json" && ext != ".ndjson" {
		return "", errors.New("file must end with .json or .ndjson")
	}
	baseAbs, err := normalizeOutputDir(outputDir)
	if err != nil {
		return "", err
	}
	targetAbs, err := filepath.Abs(filepath.Join(outputDir, clean))
	if err != nil {
		return "", err
	}
	if err := ensurePathWithinRoot(baseAbs, targetAbs); err != nil {
		return "", err
	}
	return targetAbs, nil
}

func readJSONTail(path string, limit int) ([]map[string]any, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	payload, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	var arr []map[string]any
	if err := json.Unmarshal(payload, &arr); err == nil {
		return tailMaps(arr, limit), nil
	}
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		return nil, err
	}
	return []map[string]any{obj}, nil
}

func tailMaps(items []map[string]any, limit int) []map[string]any {
	if len(items) <= limit {
		return items
	}
	return items[len(items)-limit:]
}
