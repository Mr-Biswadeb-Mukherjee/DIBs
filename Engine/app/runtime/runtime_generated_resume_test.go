package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStreamGeneratedDomainsToSpoolResumesFromGeneratedOutput(t *testing.T) {
	modules := fakeGeneratedModuleFactory{
		items: []GeneratedDomain{
			{Domain: "alpha.example", RiskScore: 0.31, Confidence: "low", GeneratedBy: "mutation"},
			{Domain: "beta.example", RiskScore: 0.52, Confidence: "medium", GeneratedBy: "dga"},
			{Domain: "gamma.example", RiskScore: 0.73, Confidence: "high", GeneratedBy: "combo"},
		},
	}
	store := newFakeGeneratedCacheStore()
	outputPath := filepath.Join(t.TempDir(), "Generated_Domain.ndjson")
	if err := os.WriteFile(outputPath, []byte("{\"domain\":\"alpha.example\",\"resolution\":\"resolved\"}\n"), 0o600); err != nil {
		t.Fatalf("write generated output fixture: %v", err)
	}

	total, spool, err := streamGeneratedDomainsToSpool(context.Background(), "", modules, store, outputPath)
	if err != nil {
		t.Fatalf("streamGeneratedDomainsToSpool error: %v", err)
	}
	defer func() { _ = spool.Close() }()

	if total != 2 {
		t.Fatalf("expected 2 pending domains after resume, got %d", total)
	}
	got := popAllDomains(t, store, spool, 2)
	if got["alpha.example"] {
		t.Fatalf("resumed queue unexpectedly included already-flushed domain")
	}
}

func TestStreamGeneratedDomainsToSpoolReuseDBWithoutRegeneration(t *testing.T) {
	var streamCalls int
	modules := fakeGeneratedModuleFactory{
		streamCalls: &streamCalls,
		items: []GeneratedDomain{
			{Domain: "resume-1.example", RiskScore: 0.21, Confidence: "low", GeneratedBy: "dga"},
			{Domain: "resume-2.example", RiskScore: 0.61, Confidence: "high", GeneratedBy: "mutation"},
		},
	}
	outputPath := filepath.Join(t.TempDir(), "Generated_Domain.ndjson")

	_, spool, err := streamGeneratedDomainsToSpool(context.Background(), "", modules, newFakeGeneratedCacheStore(), outputPath)
	if err != nil {
		t.Fatalf("first spool run failed: %v", err)
	}
	_ = spool.Close()

	content := []byte("{\"domain\":\"resume-1.example\",\"resolution\":\"resolved\"}\n")
	if err := os.WriteFile(outputPath, content, 0o600); err != nil {
		t.Fatalf("write generated output fixture: %v", err)
	}
	total, spool, err := streamGeneratedDomainsToSpool(context.Background(), "", modules, newFakeGeneratedCacheStore(), outputPath)
	if err != nil {
		t.Fatalf("second spool run failed: %v", err)
	}
	_ = spool.Close()

	if streamCalls != 1 {
		t.Fatalf("expected regeneration to happen once, got %d", streamCalls)
	}
	if total != 1 {
		t.Fatalf("expected exactly one pending domain on second run, got %d", total)
	}
}

func TestStreamGeneratedDomainsToSpoolHandlesDuplicateOutputDomains(t *testing.T) {
	modules := fakeGeneratedModuleFactory{
		items: []GeneratedDomain{
			{Domain: "alpha.example", RiskScore: 0.31, Confidence: "low", GeneratedBy: "mutation"},
			{Domain: "beta.example", RiskScore: 0.52, Confidence: "medium", GeneratedBy: "dga"},
			{Domain: "gamma.example", RiskScore: 0.73, Confidence: "high", GeneratedBy: "combo"},
		},
	}
	store := newFakeGeneratedCacheStore()
	outputPath := filepath.Join(t.TempDir(), "Generated_Domain.ndjson")
	content := []byte(
		"{\"domain\":\"alpha.example\",\"resolution\":\"resolved\"}\n" +
			"{\"domain\":\"alpha.example\",\"resolution\":\"resolved\"}\n" +
			"{\"domain\":\"beta.example\",\"resolution\":\"resolved\"}\n" +
			"not-json\n",
	)
	if err := os.WriteFile(outputPath, content, 0o600); err != nil {
		t.Fatalf("write generated output fixture: %v", err)
	}

	total, spool, err := streamGeneratedDomainsToSpool(context.Background(), "", modules, store, outputPath)
	if err != nil {
		t.Fatalf("streamGeneratedDomainsToSpool error: %v", err)
	}
	defer func() { _ = spool.Close() }()

	if total != 1 {
		t.Fatalf("expected exactly one pending domain after done-sync, got %d", total)
	}
	got := popAllDomains(t, store, spool, 1)
	if !got["gamma.example"] {
		t.Fatalf("expected remaining pending domain to be gamma.example, got=%v", got)
	}
}

func TestStreamGeneratedDomainsToSpoolRegeneratesOnSignatureMismatch(t *testing.T) {
	root := t.TempDir()
	pathA := writeKeywordsFixture(t, root, "Keywords_A.csv", "amazon")
	pathB := writeKeywordsFixture(t, root, "Keywords_B.csv", "flipkart")
	outputPath := filepath.Join(root, "Generated_Domain.ndjson")

	firstModules := fakeGeneratedModuleFactory{
		items: []GeneratedDomain{{Domain: "old.example", RiskScore: 0.41, Confidence: "low", GeneratedBy: "dga"}},
	}
	_, spool, err := streamGeneratedDomainsToSpool(context.Background(), pathA, firstModules, newFakeGeneratedCacheStore(), outputPath)
	if err != nil {
		t.Fatalf("first spool run failed: %v", err)
	}
	_ = spool.Close()

	var streamCalls int
	secondModules := fakeGeneratedModuleFactory{
		streamCalls: &streamCalls,
		items:       []GeneratedDomain{{Domain: "new.example", RiskScore: 0.81, Confidence: "high", GeneratedBy: "mutation"}},
	}
	store := newFakeGeneratedCacheStore()
	total, spool, err := streamGeneratedDomainsToSpool(context.Background(), pathB, secondModules, store, outputPath)
	if err != nil {
		t.Fatalf("second spool run failed: %v", err)
	}
	defer func() { _ = spool.Close() }()

	if streamCalls != 1 || total != 1 {
		t.Fatalf("expected fresh regeneration after signature mismatch, calls=%d total=%d", streamCalls, total)
	}
	got := popAllDomains(t, store, spool, 1)
	if !got["new.example"] {
		t.Fatalf("expected regenerated queue to include only new.example, got=%v", got)
	}
}

func TestStreamGeneratedDomainsToSpoolRegeneratesLegacyDataset(t *testing.T) {
	root := t.TempDir()
	path := writeKeywordsFixture(t, root, "Keywords.csv", "myntra")
	outputPath := filepath.Join(root, "Generated_Domain.ndjson")

	firstModules := fakeGeneratedModuleFactory{
		items: []GeneratedDomain{{Domain: "legacy.example", RiskScore: 0.22, Confidence: "low", GeneratedBy: "dga"}},
	}
	_, spool, err := streamGeneratedDomainsToSpool(context.Background(), path, firstModules, newFakeGeneratedCacheStore(), outputPath)
	if err != nil {
		t.Fatalf("first spool run failed: %v", err)
	}
	if _, err := spool.db.Exec("DELETE FROM " + generatedSpoolMetaTable); err != nil {
		t.Fatalf("clear spool metadata fixture failed: %v", err)
	}
	_ = spool.Close()

	var streamCalls int
	secondModules := fakeGeneratedModuleFactory{
		streamCalls: &streamCalls,
		items:       []GeneratedDomain{{Domain: "fresh.example", RiskScore: 0.72, Confidence: "high", GeneratedBy: "mutation"}},
	}
	store := newFakeGeneratedCacheStore()
	total, spool, err := streamGeneratedDomainsToSpool(context.Background(), path, secondModules, store, outputPath)
	if err != nil {
		t.Fatalf("legacy-resume spool run failed: %v", err)
	}
	defer func() { _ = spool.Close() }()

	if streamCalls != 1 || total != 1 {
		t.Fatalf("expected legacy dataset reset + regeneration, calls=%d total=%d", streamCalls, total)
	}
	got := popAllDomains(t, store, spool, 1)
	if !got["fresh.example"] {
		t.Fatalf("expected fresh queue to include only fresh.example, got=%v", got)
	}
}

func writeKeywordsFixture(t *testing.T, root, name, line string) string {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(line+"\n"), 0o600); err != nil {
		t.Fatalf("write keywords fixture: %v", err)
	}
	return path
}
