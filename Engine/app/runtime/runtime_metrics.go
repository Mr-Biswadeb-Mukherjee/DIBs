// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const qpsHistoryTick = time.Second

var istLocation = mustLoadISTLocation()

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

type qpsHistoryRecord struct {
	TimestampUTC   string  `json:"timestamp_utc"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	CompletedTotal int64   `json:"completed_total"`
	IntelDoneTotal int64   `json:"intel_done_total"`
	ResolveQPS     float64 `json:"resolve_qps"`
	IntelQPS       float64 `json:"intel_qps"`
	CPUPercent     float64 `json:"cpu_percent"`
	RAMMB          float64 `json:"ram_mb"`
}

type qpsHistoryWriter struct {
	start   time.Time
	writer  RecordWriter
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once
	lastAt  time.Time
	lastRes int64
	lastInt int64

	completed *int64
	intelDone *int64
	cpu       *cpuSampler
}

func qpsHistoryOutputPath(runMetricsPath string, started time.Time) string {
	base := strings.TrimSpace(runMetricsPath)
	dir := filepath.Dir(base)
	if dir == "" || dir == "." {
		dir = "."
	}
	stamp := started.In(istLocation).Format("2006-01-02_15-04-05")
	return filepath.Join(dir, "Metrics_History_"+stamp+".ndjson")
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
		TimestampUTC:     formatISTTimestamp(finishedAt),
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

func newQPSHistoryWriter(rt *appRuntime, completed, intelDone *int64) (*qpsHistoryWriter, error) {
	if rt == nil || rt.writers == nil || rt.qpsHistory == "" {
		return nil, nil
	}
	writer, err := newNDJSONWriter(rt.qpsHistory, rt.writers, "qps-history-writer", rt.logErr)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &qpsHistoryWriter{
		start:     rt.started,
		writer:    writer,
		stop:      make(chan struct{}),
		done:      make(chan struct{}),
		lastAt:    now,
		lastRes:   loadCounter(completed),
		lastInt:   loadCounter(intelDone),
		completed: completed,
		intelDone: intelDone,
		cpu:       newCPUSampler(),
	}, nil
}

func (q *qpsHistoryWriter) Start() {
	if q == nil {
		return
	}
	q.writeSnapshot(time.Now())
	go q.loop()
}

func (q *qpsHistoryWriter) loop() {
	ticker := time.NewTicker(qpsHistoryTick)
	defer ticker.Stop()
	for {
		select {
		case <-q.stop:
			q.writeSnapshot(time.Now())
			close(q.done)
			return
		case now := <-ticker.C:
			q.writeSnapshot(now)
		}
	}
}

func (q *qpsHistoryWriter) Stop() error {
	if q == nil {
		return nil
	}
	q.once.Do(func() { close(q.stop) })
	<-q.done
	if q.writer == nil {
		return nil
	}
	return q.writer.Close()
}

func (q *qpsHistoryWriter) writeSnapshot(now time.Time) {
	if q == nil || q.writer == nil {
		return
	}
	completed := loadCounter(q.completed)
	intelDone := loadCounter(q.intelDone)
	window := now.Sub(q.lastAt).Seconds()
	cpuPercent := q.cpu.Percent(now)
	ramMB := processRAMMB()
	record := qpsHistoryRecord{
		TimestampUTC:   formatISTTimestamp(now),
		ElapsedSeconds: roundTo2(now.Sub(q.start).Seconds()),
		CompletedTotal: completed,
		IntelDoneTotal: intelDone,
		ResolveQPS:     deltaQPS(completed-q.lastRes, window),
		IntelQPS:       deltaQPS(intelDone-q.lastInt, window),
		CPUPercent:     cpuPercent,
		RAMMB:          ramMB,
	}
	q.writer.WriteRecord(record)
	q.lastAt = now
	q.lastRes = completed
	q.lastInt = intelDone
}

func loadCounter(counter *int64) int64 {
	if counter == nil {
		return 0
	}
	return atomic.LoadInt64(counter)
}

func deltaQPS(delta int64, seconds float64) float64 {
	if delta <= 0 || seconds <= 0 {
		return 0
	}
	return roundTo2(float64(delta) / seconds)
}

type cpuSampler struct {
	lastAt  time.Time
	lastCPU float64
}

func newCPUSampler() *cpuSampler {
	now := time.Now()
	return &cpuSampler{
		lastAt:  now,
		lastCPU: processCPUSeconds(),
	}
}

func (s *cpuSampler) Percent(now time.Time) float64 {
	if s == nil {
		return 0
	}
	wallSeconds := now.Sub(s.lastAt).Seconds()
	currentCPU := processCPUSeconds()
	if wallSeconds <= 0 {
		s.lastAt = now
		s.lastCPU = currentCPU
		return 0
	}
	used := currentCPU - s.lastCPU
	s.lastAt = now
	s.lastCPU = currentCPU
	if used <= 0 {
		return 0
	}
	return roundTo2((used / wallSeconds) * 100)
}

func processRAMMB() float64 {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	const bytesPerMB = 1024 * 1024
	return roundTo2(float64(stats.Alloc) / bytesPerMB)
}

func formatISTTimestamp(ts time.Time) string {
	return ts.In(istLocation).Format(time.RFC3339)
}

func mustLoadISTLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Kolkata")
	if err == nil {
		return loc
	}
	return time.FixedZone("IST", 5*60*60+30*60)
}
