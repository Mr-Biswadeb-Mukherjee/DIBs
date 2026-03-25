// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package domain_generator

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ----------------------------
// Temp CSV helper
// ----------------------------

func writeTempCSV(t *testing.T, rows [][]string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.csv")

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create temp csv: %v", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.WriteAll(rows); err != nil {
		t.Fatalf("failed to write csv: %v", err)
	}
	return path
}

// =====================================================
//  UNIT TESTS FOR INTERNAL FUNCTIONS
// =====================================================

func TestSanitizeKeyword(t *testing.T) {
	in := "  g@@o!o#g l%e.  "
	got := sanitizeKeyword(in)

	if got == "" {
		t.Fatalf("sanitizeKeyword returned empty result")
	}
}

func TestAppendTLDs(t *testing.T) {
	lbls := []string{"example", "test"}
	tlds := []string{".com", ".net"}
	got := appendTLDs(lbls, tlds)

	if len(got) != len(lbls)*len(tlds) {
		t.Fatalf("appendTLDs unexpected count: %d", len(got))
	}

	for _, d := range got {
		if !strings.HasSuffix(d, ".com") && !strings.HasSuffix(d, ".net") {
			t.Fatalf("appendTLDs incorrect TLD: %s", d)
		}
	}
}

func TestLoadKeywords(t *testing.T) {
	path := writeTempCSV(t, [][]string{
		{"domain"},
		{" google "},
		{"  test-domain "},
	})

	words, err := loadKeywords(path)
	if err != nil {
		t.Fatalf("loadKeywords error: %v", err)
	}

	if len(words) != 2 {
		t.Fatalf("expected 2 domains, got %d", len(words))
	}
}

func TestFilterSimilar_NoPanic(t *testing.T) {
	// Only verify it doesn't crash since JW distance is external.
	_ = filterSimilar("test.in", []string{"test.in", "abc.in"})
}

// =====================================================
//  HIGH-LEVEL TEST FOR GenerateFromCSV
// =====================================================

func TestGenerateFromCSV(t *testing.T) {
	path := writeTempCSV(t, [][]string{
		{"domain"},
		{"google"},
	})

	_, err := GenerateFromCSV(path)
	if err != nil {
		t.Fatalf("GenerateFromCSV error: %v", err)
	}
}

func TestLoadKeywordsRejectsNonCSVPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	if err := os.WriteFile(path, []byte("domain\nexample\n"), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	_, err := loadKeywords(path)
	if err == nil {
		t.Fatal("expected non-csv path to be rejected")
	}
}

func TestLoadTargetTLDsDefaultsWhenMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.conf")
	got := loadTargetTLDs(path)

	if !equalStringSlices(got, defaultTargetTLDs) {
		t.Fatalf("expected default tlds %v, got %v", defaultTargetTLDs, got)
	}
}

func TestLoadTargetTLDsFromConfig(t *testing.T) {
	path := writeTempSettings(t, "target_tlds=.com, net, .org, .com\n")
	got := loadTargetTLDs(path)
	want := []string{".com", ".net", ".org"}

	if !equalStringSlices(got, want) {
		t.Fatalf("expected tlds %v, got %v", want, got)
	}
}

func TestLoadTargetTLDsInvalidConfigFallsBack(t *testing.T) {
	path := writeTempSettings(t, "target_tlds= , ,\n")
	got := loadTargetTLDs(path)

	if !equalStringSlices(got, defaultTargetTLDs) {
		t.Fatalf("expected default tlds %v, got %v", defaultTargetTLDs, got)
	}
}

func TestLoadTargetTLDsFromTierConfig(t *testing.T) {
	content := strings.Join([]string{
		"target_tlds_india=.in,.co.in",
		"target_tlds_global=.com,.net,.in",
		"active_tld_tiers=india,global",
	}, "\n")
	path := writeTempSettings(t, content)
	got := loadTargetTLDs(path)
	want := []string{".in", ".co.in", ".com", ".net"}

	if !equalStringSlices(got, want) {
		t.Fatalf("expected tier tlds %v, got %v", want, got)
	}
}

func TestLoadTargetTLDsLegacyOverridesTierConfig(t *testing.T) {
	content := strings.Join([]string{
		"target_tlds=.org,.io",
		"target_tlds_india=.in,.co.in",
		"active_tld_tiers=india",
	}, "\n")
	path := writeTempSettings(t, content)
	got := loadTargetTLDs(path)
	want := []string{".org", ".io"}

	if !equalStringSlices(got, want) {
		t.Fatalf("expected legacy tlds %v, got %v", want, got)
	}
}

func TestLoadTargetTLDsDefaultTierLists(t *testing.T) {
	path := writeTempSettings(t, "active_tld_tiers=india,global\n")
	got := loadTargetTLDs(path)
	want := append(defaultTierTLDsCopy("india"), defaultTierTLDsCopy("global")...)

	if !equalStringSlices(got, want) {
		t.Fatalf("expected default tier tlds %v, got %v", want, got)
	}
}

func writeTempSettings(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "setting.conf")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write settings: %v", err)
	}
	return path
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
