package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoggerWritesLevelsAndFlushes(t *testing.T) {
	dir := t.TempDir()
	log := NewLoggerInDir("dns module", dir)

	log.Info("hello %s", "world")
	log.Warning("warn %d", 7)
	log.Alert("boom")

	if err := log.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	data, err := os.ReadFile(log.GetFilePath())
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "INFO: hello world") {
		t.Fatalf("expected info log in %q", content)
	}
	if !strings.Contains(content, "WARNING: warn 7") {
		t.Fatalf("expected warning log in %q", content)
	}
	if !strings.Contains(content, "ALERT: boom") {
		t.Fatalf("expected alert log in %q", content)
	}
	if filepath.Dir(log.GetFilePath()) != dir {
		t.Fatalf("expected log file in %q, got %q", dir, log.GetFilePath())
	}
}

func TestLoggerHelpersNormalizeNames(t *testing.T) {
	if got := sanitizeModuleName(" /dns:module\\child "); got != "dns_module_child" {
		t.Fatalf("unexpected sanitized name: %q", got)
	}
	if got := sanitizeModuleName("   "); got != "module" {
		t.Fatalf("expected fallback module name, got %q", got)
	}
	if got := logFileName("custom.log"); got != "custom.log" {
		t.Fatalf("expected existing log suffix to be preserved, got %q", got)
	}

	dir := filepath.Join(t.TempDir(), "nested", "logs")
	path := buildLogPath("dns", dir)
	if !strings.HasPrefix(path, dir) {
		t.Fatalf("expected path inside %q, got %q", dir, path)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("expected buildLogPath to create dir: %v", err)
	}
}

func TestNilLoggerIsSafe(t *testing.T) {
	var log *Logger

	log.Info("ignored")
	log.Warning("ignored")
	log.Alert("ignored")

	if err := log.Close(); err != nil {
		t.Fatalf("expected nil Close to succeed, got %v", err)
	}
	if got := log.GetFilePath(); got != "" {
		t.Fatalf("expected empty path for nil logger, got %q", got)
	}
}

func TestNewLoggerUsesModuleFallbackName(t *testing.T) {
	dir := t.TempDir()
	log := NewLoggerInDir("   ", dir)
	defer log.Close()

	base := filepath.Base(log.GetFilePath())
	if !strings.HasPrefix(base, "module_") {
		t.Fatalf("expected fallback module prefix, got %q", base)
	}
}

func TestLoggerWriteAfterCloseIsSafe(t *testing.T) {
	dir := t.TempDir()
	log := NewLoggerInDir("app", dir)
	if err := log.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("expected log after close to be safe, panic=%v", r)
		}
	}()
	log.Warning("late log should not panic")
}
