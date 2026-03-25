// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/core/config"
)

func TestDefaultFileContainsAllKeys(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "output.conf")

	_, err := config.LoadOrCreateConfig(cfgPath)
	if err != nil {
		t.Fatalf("error creating default config: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	required := []string{
		"rate_limit=",
		"rate_limit_ceiling=",
		"cooldown_after=",
		"cooldown_duration=",
		"timeout_seconds=",
		"max_retries=",
		"autoscale=",
		"upstream_dns=",
		"backup_dns=",
		"dns_retries=",
		"dns_timeout_ms=",
	}

	text := string(data)
	for _, key := range required {
		if !strings.Contains(text, key) {
			t.Fatalf("missing key in default config: %s", key)
		}
	}
}

func TestLoadOrCreateConfig_TrimsWhitespaceAroundKeys(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "settings.conf")

	content := "rate_limit        = auto\nrate_limit_ceiling = 160\nautoscale = true\n"
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	cfg, err := config.LoadOrCreateConfig(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RateLimit != 0 {
		t.Fatalf("expected auto rate_limit=0 got %d", cfg.RateLimit)
	}
	if cfg.RateLimitCeiling != 160 {
		t.Fatalf("expected rate_limit_ceiling=160 got %d", cfg.RateLimitCeiling)
	}
	if !cfg.AutoScale {
		t.Fatalf("expected autoscale=true")
	}
}

func TestLoadOrCreateConfig_AppendsMissingKeys(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "settings.conf")
	initial := "rate_limit=11\nupstream_dns=8.8.8.8:53\n"
	if err := os.WriteFile(cfgPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	cfg, err := config.LoadOrCreateConfig(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.RateLimit != 11 {
		t.Fatalf("expected rate_limit override, got %d", cfg.RateLimit)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, "dns_timeout_ms=") {
		t.Fatalf("expected missing default key to be injected")
	}
}

func TestLoadOrCreateConfig_RejectsDirectoryPath(t *testing.T) {
	tmp := t.TempDir()

	_, err := config.LoadOrCreateConfig(tmp)
	if err == nil {
		t.Fatal("expected directory path to be rejected")
	}
}
