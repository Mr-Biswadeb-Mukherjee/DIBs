// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package apis

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	defaultDetailsOutputDir = "Output"
	maxNDJSONLineBytes      = 2 * 1024 * 1024
	tailSliceInitCapLimit   = 1024
)

var detailSectionOrder = []string{
	"session",
	"metrics",
	"generated",
	"resolved",
	"dns_intel",
	"run_metrics",
	"qps_history",
}

var detailSectionAliases = map[string]string{
	"session":     "session",
	"metrics":     "metrics",
	"generated":   "generated",
	"resolved":    "resolved",
	"dns_intel":   "dns_intel",
	"dns-intel":   "dns_intel",
	"run_metrics": "run_metrics",
	"run-metrics": "run_metrics",
	"qps_history": "qps_history",
	"qps-history": "qps_history",
}

type DetailsService struct {
	outputDir string
}

func NewDetailsService(outputDir string) *DetailsService {
	base, err := normalizeOutputDir(outputDir)
	if err != nil {
		base = defaultDetailsOutputDir
	}
	return &DetailsService{outputDir: base}
}

func (s *DetailsService) Fetch(
	sections []string,
	limit int,
	sessions *SessionManager,
) map[string]any {
	payload := make(map[string]any, len(sections))
	for _, section := range sections {
		payload[section] = s.fetchSection(section, limit, sessions)
	}
	return payload
}

func (s *DetailsService) fetchSection(
	section string,
	limit int,
	sessions *SessionManager,
) map[string]any {
	switch section {
	case "session":
		return fetchSessionSection(sessions)
	case "metrics":
		return fetchMetricsSection(sessions)
	default:
		return s.fetchFileSection(section, limit)
	}
}

func fetchSessionSection(sessions *SessionManager) map[string]any {
	if sessions == nil {
		return map[string]any{"error": "session manager not configured"}
	}
	info, ok := sessions.CurrentSession()
	if !ok {
		return map[string]any{"error": "session not found"}
	}
	return map[string]any{"session": info}
}

func fetchMetricsSection(sessions *SessionManager) map[string]any {
	if sessions == nil {
		return map[string]any{"error": "session manager not configured"}
	}
	info, ok := sessions.CurrentSession()
	if !ok {
		return map[string]any{"error": "session not found"}
	}
	return map[string]any{
		"status":     info.Status,
		"started_at": info.StartedAt,
		"ended_at":   info.EndedAt,
		"error":      info.Error,
	}
}

func (s *DetailsService) fetchFileSection(section string, limit int) map[string]any {
	path, err := s.resolvePath(section)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	records, err := readNDJSONTail(path, limit)
	if err != nil {
		return map[string]any{
			"path":  path,
			"error": err.Error(),
		}
	}
	return map[string]any{
		"path":    path,
		"count":   len(records),
		"records": records,
	}
}

func (s *DetailsService) resolvePath(section string) (string, error) {
	base := s.outputDir
	if raw, ok := strings.CutPrefix(section, "file:"); ok {
		return resolveCustomDetailsFile(base, raw)
	}
	switch section {
	case "generated":
		return filepath.Join(base, "Generated_Domain.ndjson"), nil
	case "resolved":
		return filepath.Join(base, "Resolved_Domain.ndjson"), nil
	case "dns_intel":
		return filepath.Join(base, "DNS_Intel.ndjson"), nil
	case "run_metrics":
		return filepath.Join(base, "Run_Metrics.ndjson"), nil
	case "qps_history":
		return latestQPSHistoryPath(base)
	default:
		return "", fmt.Errorf("unsupported details section: %s", section)
	}
}

func latestQPSHistoryPath(outputDir string) (string, error) {
	pattern := filepath.Join(outputDir, "QPS_History_*.ndjson")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("invalid qps history pattern: %w", err)
	}
	if len(matches) == 0 {
		return "", os.ErrNotExist
	}
	slices.Sort(matches)
	return matches[len(matches)-1], nil
}

func readNDJSONTail(path string, limit int) ([]map[string]any, error) {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return readJSONTail(path, limit)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 4096), maxNDJSONLineBytes)
	records := make([]map[string]any, 0, tailSliceInitCap(limit))
	for scanner.Scan() {
		row, err := parseNDJSONLine(scanner.Text())
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if row == nil {
			continue
		}
		records = append(records, row)
		if len(records) > limit {
			records = records[1:]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func tailSliceInitCap(limit int) int {
	if limit < tailSliceInitCapLimit {
		return limit
	}
	return tailSliceInitCapLimit
}

func parseNDJSONLine(line string) (map[string]any, error) {
	raw := strings.TrimSpace(line)
	if raw == "" || strings.HasPrefix(raw, "#") {
		return nil, nil
	}
	record := make(map[string]any)
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return nil, fmt.Errorf("invalid ndjson line: %w", err)
	}
	return record, nil
}
