package runtime

import (
	"strings"
	"testing"
	"time"
)

func TestBuildRunMetricsRecordIncludesQPS(t *testing.T) {
	started := time.Date(2026, time.March, 26, 10, 0, 0, 0, time.UTC)
	finished := started.Add(4 * time.Second)
	record := buildRunMetricsRecord(started, finished, 200, 80, Config{
		RateLimit:        120,
		RateLimitCeiling: 180,
	})

	if record.DurationSeconds != 4 {
		t.Fatalf("unexpected duration seconds: %v", record.DurationSeconds)
	}
	if record.QPS != 20 {
		t.Fatalf("unexpected qps: %v", record.QPS)
	}
	if record.GeneratedQPS != 50 {
		t.Fatalf("unexpected generated qps: %v", record.GeneratedQPS)
	}
	if record.RateLimit != 120 || record.RateLimitCeiling != 180 {
		t.Fatalf("unexpected rate fields: %#v", record)
	}
}

func TestRunDurationSecondsClampsNonPositiveWindow(t *testing.T) {
	ts := time.Date(2026, time.March, 26, 11, 0, 0, 0, time.UTC)
	if got := runDurationSeconds(ts, ts); got != 0.001 {
		t.Fatalf("expected duration clamp 0.001, got %v", got)
	}
}

func TestQPSHistoryOutputPathIncludesTimestampedName(t *testing.T) {
	started := time.Date(2026, time.March, 26, 10, 10, 5, 0, time.UTC)
	path := qpsHistoryOutputPath("/tmp/Run_Metrics.ndjson", started)
	if !strings.Contains(path, "/tmp/QPS_History_2026-03-26_10-10-05.ndjson") {
		t.Fatalf("unexpected qps history path: %s", path)
	}
}

func TestQPSHistoryOutputPathFallbackDir(t *testing.T) {
	started := time.Date(2026, time.March, 26, 10, 10, 5, 0, time.UTC)
	path := qpsHistoryOutputPath("Run_Metrics.ndjson", started)
	if !strings.Contains(path, "QPS_History_2026-03-26_10-10-05.ndjson") {
		t.Fatalf("unexpected qps history file name: %s", path)
	}
}

func TestDeltaQPS(t *testing.T) {
	if got := deltaQPS(50, 2); got != 25 {
		t.Fatalf("expected 25 qps, got %v", got)
	}
	if got := deltaQPS(0, 2); got != 0 {
		t.Fatalf("expected zero qps for zero delta, got %v", got)
	}
	if got := deltaQPS(5, 0); got != 0 {
		t.Fatalf("expected zero qps for zero seconds, got %v", got)
	}
	if got := deltaQPS(-10, 3); got != 0 {
		t.Fatalf("expected zero qps for negative delta, got %v", got)
	}
}

func TestLoadCounterNil(t *testing.T) {
	if got := loadCounter(nil); got != 0 {
		t.Fatalf("expected zero for nil counter, got %d", got)
	}
}

func TestQPSHistoryOutputPathTrimsInput(t *testing.T) {
	started := time.Date(2026, time.March, 26, 11, 30, 0, 0, time.UTC)
	path := qpsHistoryOutputPath("  /tmp/Run_Metrics.ndjson  ", started)
	if !strings.Contains(path, "/tmp/QPS_History_2026-03-26_11-30-00.ndjson") {
		t.Fatalf("unexpected trimmed qps history path: %s", path)
	}
}
