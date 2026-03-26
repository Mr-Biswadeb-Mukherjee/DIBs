// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"math"
	"path/filepath"
	"strings"
	"time"
)

type runMetricsRecord struct {
	TimestampUTC     string  `json:"timestamp_utc"`
	DurationSeconds  float64 `json:"duration_seconds"`
	GeneratedTotal   int64   `json:"generated_total"`
	ResolvedTotal    int64   `json:"resolved_total"`
	QPS              float64 `json:"qps"`
	GeneratedQPS     float64 `json:"generated_qps"`
	RateLimit        int     `json:"rate_limit"`
	RateLimitCeiling int     `json:"rate_limit_ceiling"`
}

func qpsHistoryOutputPath(runMetricsPath string, started time.Time) string {
	base := strings.TrimSpace(runMetricsPath)
	dir := filepath.Dir(base)
	if dir == "" || dir == "." {
		dir = "."
	}
	stamp := started.UTC().Format("2006-01-02_15-04-05")
	return filepath.Join(dir, "QPS_History_"+stamp+".ndjson")
}

func (rt *appRuntime) writeRunMetrics(total, resolved int64, finishedAt time.Time) error {
	writer, err := newNDJSONWriter(rt.paths.RunMetricsOutput, rt.writers, "run-metrics-writer", rt.logErr)
	if err != nil {
		return err
	}
	writer.WriteRecord(buildRunMetricsRecord(rt.started, finishedAt, total, resolved, rt.cfg))
	return writer.Close()
}

func buildRunMetricsRecord(
	startedAt time.Time,
	finishedAt time.Time,
	total int64,
	resolved int64,
	cfg Config,
) runMetricsRecord {
	durationSeconds := runDurationSeconds(startedAt, finishedAt)
	return runMetricsRecord{
		TimestampUTC:     finishedAt.UTC().Format(time.RFC3339),
		DurationSeconds:  durationSeconds,
		GeneratedTotal:   total,
		ResolvedTotal:    resolved,
		QPS:              roundedQPS(resolved, durationSeconds),
		GeneratedQPS:     roundedQPS(total, durationSeconds),
		RateLimit:        cfg.RateLimit,
		RateLimitCeiling: cfg.RateLimitCeiling,
	}
}

func runDurationSeconds(startedAt, finishedAt time.Time) float64 {
	seconds := finishedAt.Sub(startedAt).Seconds()
	if seconds <= 0 {
		return 0.001
	}
	return roundTo2(seconds)
}

func roundedQPS(count int64, seconds float64) float64 {
	if count <= 0 || seconds <= 0 {
		return 0
	}
	return roundTo2(float64(count) / seconds)
}

func roundTo2(value float64) float64 {
	return math.Round(value*100) / 100
}
