// filewriter_test.go

package filewriter_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/official-biswadeb941/Infermal_v2/Modules/app/core/filewriter"
)

// ----------------------------------------------
// Helper: Read CSV file into array of lines
// ----------------------------------------------
func readLines(t *testing.T, file string) []string {
	t.Helper()

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	raw := string(data)
	if raw == "" {
		return []string{}
	}

	lines := []string{}
	for _, l := range splitKeep(raw, '\n') {
		if len(l) > 0 {
			lines = append(lines, l[:len(l)-1]) // trim trailing \n
		}
	}
	return lines
}

// Keeps delimiter and splits like bufio.Scanner but simpler
func splitKeep(s string, sep rune) []string {
	out := []string{}
	start := 0
	for i, r := range s {
		if r == sep {
			out = append(out, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// ----------------------------------------------
// Test: Normal writer (overwrite mode)
// ----------------------------------------------
func TestCSVWriter_Normal(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "test_normal.csv")

	w, err := filewriter.SafeNewCSVWriter(file, filewriter.Overwrite)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	w.WriteRow([]string{"a", "b", "c"})
	w.WriteRow([]string{"1", "2", "3"})

	// Wait for flush timer to trigger
	time.Sleep(2500 * time.Millisecond)

	if err := w.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	lines := readLines(t, file)
	if len(lines) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(lines))
	}

	if lines[0] != "a,b,c" {
		t.Fatalf("first row mismatch: %s", lines[0])
	}
	if lines[1] != "1,2,3" {
		t.Fatalf("second row mismatch: %s", lines[1])
	}
}

// ----------------------------------------------
// Test: Append mode
// ----------------------------------------------
func TestCSVWriter_Append(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "append_test.csv")

	// First write
	w1, _ := filewriter.SafeNewCSVWriter(file, filewriter.Overwrite)
	w1.WriteRow([]string{"x", "y"})
	time.Sleep(2200 * time.Millisecond)
	_ = w1.Close()

	// Append
	w2, _ := filewriter.SafeNewCSVWriter(file, filewriter.Append)
	w2.WriteRow([]string{"a", "b"})
	w2.WriteRow([]string{"c", "d"})
	time.Sleep(2200 * time.Millisecond)
	_ = w2.Close()

	lines := readLines(t, file)
	if len(lines) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(lines))
	}
}

// ----------------------------------------------
// Test: Atomic writer (temp file → rename)
// ----------------------------------------------
func TestCSVWriter_Atomic(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "atomic.csv")
	tmpFile := file + ".tmp"

	w, err := filewriter.SafeNewAtomicCSVWriter(file)
	if err != nil {
		t.Fatalf("failed to create atomic writer: %v", err)
	}

	w.WriteRow([]string{"alpha", "beta"})
	time.Sleep(2300 * time.Millisecond)

	if _, err := os.Stat(tmpFile); err != nil {
		t.Fatalf("expected temp file to exist during write")
	}

	if err := w.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	// Temp file must be gone
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Fatalf("tmp file must be removed after atomic commit")
	}

	// Final file must exist
	lines := readLines(t, file)
	if len(lines) != 1 || lines[0] != "alpha,beta" {
		t.Fatalf("atomic output incorrect: %v", lines)
	}
}

// ----------------------------------------------
// Test: Batch size behavior (forced flush)
// ----------------------------------------------
func TestCSVWriter_BatchFlush(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "batch_test.csv")

	opts := filewriter.CSVOptions{
		Mode:       filewriter.Overwrite,
		Atomic:     false,
		BatchSize:  3,
		FlushEvery: 5 * time.Second, // should NOT fire naturally
		LogHooks:   filewriter.LogHooks{},
	}

	w, err := filewriter.NewWriter(file, opts)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}

	// Write exactly 3 items -> batch flush should trigger
	w.WriteRow([]string{"1"})
	w.WriteRow([]string{"2"})
	w.WriteRow([]string{"3"}) // flush should happen here

	// Wait small time for async flush
	time.Sleep(300 * time.Millisecond)

	_ = w.Close()

	lines := readLines(t, file)
	if len(lines) != 3 {
		t.Fatalf("batch flush failed: got %d lines", len(lines))
	}
}
