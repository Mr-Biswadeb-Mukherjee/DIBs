package filewriter_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/official-biswadeb941/Infermal_v2/Modules/app/core/filewriter"
)

type dnsIntelNDJSON struct {
	Domain       string   `json:"domain"`
	A            []string `json:"a,omitempty"`
	Providers    []string `json:"providers,omitempty"`
	TimestampUTC string   `json:"timestamp_utc"`
}

func TestNDJSONWriter_WritesValidRows(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "dns_intel.ndjson")

	opts := filewriter.NDJSONOptions{
		BatchSize:  2,
		FlushEvery: 5 * time.Second,
	}
	w, err := filewriter.NewNDJSONWriter(file, opts)
	if err != nil {
		t.Fatalf("failed to create ndjson writer: %v", err)
	}

	w.WriteRecord(dnsIntelNDJSON{Domain: "example.com", A: []string{"1.1.1.1"}, TimestampUTC: time.Now().UTC().Format(time.RFC3339)})
	w.WriteRecord(dnsIntelNDJSON{Domain: "api.example.com", Providers: []string{"cloudflare.com"}, TimestampUTC: time.Now().UTC().Format(time.RFC3339)})

	if err := w.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	raw := readFileString(t, file)
	if !strings.HasSuffix(raw, "\n") {
		t.Fatalf("ndjson output must end with newline")
	}

	rows := readNDJSONRecords(t, file)
	if len(rows) != 2 {
		t.Fatalf("expected 2 records, got %d", len(rows))
	}
	if rows[0].Domain != "example.com" || rows[1].Domain != "api.example.com" {
		t.Fatalf("unexpected domain order: %+v", rows)
	}
}

func TestNDJSONWriter_EmptyCloseWritesEmptyFile(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "empty.ndjson")

	w, err := filewriter.SafeNewNDJSONWriter(file)
	if err != nil {
		t.Fatalf("failed to create ndjson writer: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	raw := readFileString(t, file)
	if raw != "" {
		t.Fatalf("expected empty output, got %q", raw)
	}
}

func readNDJSONRecords(t *testing.T, file string) []dnsIntelNDJSON {
	t.Helper()

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("failed to read ndjson file: %v", err)
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil
	}

	lines := bytes.Split(trimmed, []byte("\n"))
	rows := make([]dnsIntelNDJSON, 0, len(lines))
	for _, line := range lines {
		var row dnsIntelNDJSON
		if err := json.Unmarshal(line, &row); err != nil {
			t.Fatalf("failed to decode ndjson row: %v\nrow=%s", err, string(line))
		}
		rows = append(rows, row)
	}
	return rows
}

func readFileString(t *testing.T, file string) string {
	t.Helper()

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	return string(data)
}
