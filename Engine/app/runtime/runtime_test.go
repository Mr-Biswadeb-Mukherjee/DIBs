package runtime

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestMakeGeneratedDomainIndexNormalizesAndSorts(t *testing.T) {
	items := []GeneratedDomain{
		{Domain: " Beta.com ", RiskScore: 1.4, GeneratedBy: "mutation"},
		{Domain: "alpha.com", RiskScore: 0.12, Confidence: "medium", GeneratedBy: "dga"},
		{Domain: " ALPHA.com ", RiskScore: -1, Confidence: "", GeneratedBy: ""},
		{Domain: "   "},
	}

	domains, index := makeGeneratedDomainIndex(items)

	if len(domains) != 2 || domains[0] != "alpha.com" || domains[1] != "beta.com" {
		t.Fatalf("unexpected domain list: %#v", domains)
	}

	alpha := index["alpha.com"]
	if alpha.RiskScore != 0 || alpha.Confidence != "low" || alpha.GeneratedBy != "unknown" {
		t.Fatalf("unexpected normalized alpha meta: %#v", alpha)
	}

	beta := index["beta.com"]
	if beta.RiskScore != 1 || beta.Confidence != "low" || beta.GeneratedBy != "mutation" {
		t.Fatalf("unexpected normalized beta meta: %#v", beta)
	}
}

func TestGeneratedDomainRecordHelpersNormalizeValues(t *testing.T) {
	meta := generatedDomainMeta{RiskScore: 0.456, Confidence: "", GeneratedBy: ""}

	unresolved := unresolvedDomainRecord(" Example.com ", meta)
	if unresolved.Domain != "example.com" {
		t.Fatalf("unexpected unresolved domain: %q", unresolved.Domain)
	}
	if unresolved.Score != 0.46 || unresolved.Confidence != "low" || unresolved.GeneratedBy != "unknown" {
		t.Fatalf("unexpected unresolved record: %#v", unresolved)
	}

	resolved := resolvedDomainRecord(" MAIL.EXAMPLE.COM ", meta)
	if resolved.Domain != "mail.example.com" {
		t.Fatalf("unexpected resolved domain: %q", resolved.Domain)
	}
}

func TestRuntimeHelpersBuildSnapshotAndTimeouts(t *testing.T) {
	var submitted int64 = 10
	var completed int64 = 4
	var active int64 = 2

	snapshot, lastCompleted := buildSnapshot(runtimeCounters{
		submitted: &submitted,
		completed: &completed,
		active:    &active,
	}, 1)

	assertSnapshotStats(t, snapshot, lastCompleted)
	assertLookupTimeouts(t)
	assertCeilSeconds(t, 1500*time.Millisecond, 2)

	atomic.StoreInt64(&active, -3)
	snapshot, _ = buildSnapshot(runtimeCounters{
		submitted: &submitted,
		completed: &completed,
		active:    &active,
	}, lastCompleted)
	if snapshot.ActiveWorkers != 0 {
		t.Fatalf("expected negative active workers to clamp to zero, got %d", snapshot.ActiveWorkers)
	}
}

func assertSnapshotStats(t *testing.T, snapshot AdaptiveSnapshot, lastCompleted int64) {
	t.Helper()
	if snapshot.InFlight != 6 || snapshot.QueueDepth != 4 || snapshot.ActiveWorkers != 2 {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if snapshot.CompletedDelta != 3 || lastCompleted != 4 {
		t.Fatalf("unexpected completion tracking: delta=%d last=%d", snapshot.CompletedDelta, lastCompleted)
	}
}

func assertLookupTimeouts(t *testing.T) {
	t.Helper()
	cases := []struct {
		active int64
		want   time.Duration
		msg    string
	}{
		{active: 0, want: 3 * time.Second, msg: "unexpected default timeout"},
		{active: 100, want: 2 * time.Second, msg: "unexpected min timeout clamp"},
		{active: 400, want: 2400 * time.Millisecond, msg: "unexpected scaled timeout"},
		{active: 2000, want: 8 * time.Second, msg: "unexpected max timeout clamp"},
	}
	for _, tc := range cases {
		if got := intelLookupTimeout(tc.active); got != tc.want {
			t.Fatalf("%s: %s", tc.msg, got)
		}
	}
}

func assertCeilSeconds(t *testing.T, in time.Duration, want int64) {
	t.Helper()
	if got := ceilSeconds(in); got != want {
		t.Fatalf("unexpected ceilSeconds result: %d", got)
	}
}
