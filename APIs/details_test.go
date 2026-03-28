package apis

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseDetailsQueryRequiresLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v3/details", nil)
	if _, _, err := parseDetailsQuery(req); err == nil {
		t.Fatal("expected limit required error")
	}
}

func TestParseDetailsQueryRejectsUnknownSection(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v3/details?section=unknown", nil)
	if _, _, err := parseDetailsQuery(req); err == nil {
		t.Fatal("expected unknown section error")
	}
}

func TestParseDetailsLimitAllowsLargeValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v3/details?limit=5000", nil)
	_, limit, err := parseDetailsQuery(req)
	if err != nil {
		t.Fatalf("parseDetailsQuery error: %v", err)
	}
	if limit != 5000 {
		t.Fatalf("limit=%d want 5000", limit)
	}
}

func TestParseDetailsQuerySupportsBareLimitValue(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v3/details?100000", nil)
	sections, limit, err := parseDetailsQuery(req)
	if err != nil {
		t.Fatalf("parseDetailsQuery error: %v", err)
	}
	if limit != 100000 {
		t.Fatalf("limit=%d want 100000", limit)
	}
	if len(sections) != len(detailSectionOrder) {
		t.Fatalf("sections len=%d want %d", len(sections), len(detailSectionOrder))
	}
}

func TestReadNDJSONTailReturnsLastRows(t *testing.T) {
	file := filepath.Join(t.TempDir(), "Generated_Domain.ndjson")
	writeTestFile(t, file, []string{
		`{"domain":"a.com"}`,
		`{"domain":"b.com"}`,
		`{"domain":"c.com"}`,
	})

	rows, err := readNDJSONTail(file, 2)
	if err != nil {
		t.Fatalf("readNDJSONTail error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows len=%d want 2", len(rows))
	}
	if rows[0]["domain"] != "b.com" || rows[1]["domain"] != "c.com" {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}

func TestLatestQPSHistoryPathSelectsNewestName(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "QPS_History_2026-03-28_01-00-00.ndjson"), []string{`{}`})
	writeTestFile(t, filepath.Join(root, "QPS_History_2026-03-28_03-00-00.ndjson"), []string{`{}`})

	path, err := latestQPSHistoryPath(root)
	if err != nil {
		t.Fatalf("latestQPSHistoryPath error: %v", err)
	}
	want := filepath.Join(root, "QPS_History_2026-03-28_03-00-00.ndjson")
	if path != want {
		t.Fatalf("path=%q want %q", path, want)
	}
}

func TestHandleDetailsReturnsRequestedSections(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "Output")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("mkdir output failed: %v", err)
	}
	writeTestFile(t, filepath.Join(outputDir, "Generated_Domain.ndjson"), []string{
		`{"domain":"first.com"}`,
		`{"domain":"last.com"}`,
	})

	manager := NewSessionManager(testRuntimeFactory)
	if _, err := manager.StartSession(); err != nil {
		t.Fatalf("StartSession error: %v", err)
	}
	defer manager.Shutdown(200 * time.Millisecond)

	server := &Server{
		sessions: manager,
		details:  NewDetailsService(outputDir),
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v3/details?section=session,generated&limit=1",
		nil,
	)
	server.handleDetails(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}
	body := decodeMap(t, rec.Body.Bytes())
	details := toMap(t, body["details"])
	generated := toMap(t, details["generated"])
	records := toSlice(t, generated["records"])
	if len(records) != 1 {
		t.Fatalf("records len=%d want 1", len(records))
	}
	last := toMap(t, records[0])
	if last["domain"] != "last.com" {
		t.Fatalf("unexpected record: %#v", last)
	}
}

func TestHandleDetailsReadsCustomNDJSONFile(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "Output")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("mkdir output failed: %v", err)
	}
	writeTestFile(t, filepath.Join(outputDir, "Custom.ndjson"), []string{
		`{"row":1}`,
		`{"row":2}`,
	})

	manager := NewSessionManager(testRuntimeFactory)
	server := &Server{sessions: manager, details: NewDetailsService(outputDir)}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v3/details?file=Custom.ndjson&limit=1",
		nil,
	)
	server.handleDetails(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}
	body := decodeMap(t, rec.Body.Bytes())
	details := toMap(t, body["details"])
	fileDetails := toMap(t, details["file:Custom.ndjson"])
	records := toSlice(t, fileDetails["records"])
	if len(records) != 1 {
		t.Fatalf("records len=%d want 1", len(records))
	}
	row := toMap(t, records[0])
	if row["row"] != float64(2) {
		t.Fatalf("unexpected row: %#v", row)
	}
}

func TestHandleDetailsReadsCustomJSONFile(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "Output")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("mkdir output failed: %v", err)
	}
	path := filepath.Join(outputDir, "events.json")
	payload := `[{"event":"a"},{"event":"b"}]`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write file failed: %v", err)
	}

	manager := NewSessionManager(testRuntimeFactory)
	server := &Server{sessions: manager, details: NewDetailsService(outputDir)}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v3/details?file=events.json&limit=1",
		nil,
	)
	server.handleDetails(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d want 200 body=%s", rec.Code, rec.Body.String())
	}
	body := decodeMap(t, rec.Body.Bytes())
	details := toMap(t, body["details"])
	fileDetails := toMap(t, details["file:events.json"])
	records := toSlice(t, fileDetails["records"])
	if len(records) != 1 {
		t.Fatalf("records len=%d want 1", len(records))
	}
	row := toMap(t, records[0])
	if row["event"] != "b" {
		t.Fatalf("unexpected row: %#v", row)
	}
}

func TestRouteHandlersIncludeDetails(t *testing.T) {
	server := &Server{}
	handlers := server.routeHandlers()
	if _, ok := handlers["details"]; !ok {
		t.Fatal("details handler missing")
	}
}

func writeTestFile(t *testing.T, path string, lines []string) {
	t.Helper()
	content := ""
	for _, line := range lines {
		content += line + "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file %s failed: %v", path, err)
	}
}

func decodeMap(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("json decode failed: %v", err)
	}
	return payload
}

func toMap(t *testing.T, value any) map[string]any {
	t.Helper()
	out, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("value is not map: %#v", value)
	}
	return out
}

func toSlice(t *testing.T, value any) []any {
	t.Helper()
	out, ok := value.([]any)
	if !ok {
		t.Fatalf("value is not slice: %#v", value)
	}
	return out
}

type testRuntime struct{}

func (testRuntime) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func testRuntimeFactory() (Runtime, error) {
	return testRuntime{}, nil
}
