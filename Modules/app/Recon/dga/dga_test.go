package domain_generator

import (
	"encoding/csv"
	"os"
	"path/filepath"
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
	got := appendTLDs(lbls)

	if len(got) != len(lbls)*len(targetTLDs) {
		t.Fatalf("appendTLDs unexpected count: %d", len(got))
	}

	for _, d := range got {
		if d[len(d)-3:] != ".in" {
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
