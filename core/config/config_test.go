// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/core/config"
)

func getDefaultValues(t *testing.T) config.Config {
	t.Helper()

	tmp := t.TempDir()
	p := filepath.Join(tmp, "defaults.conf")

	cfg, err := config.LoadOrCreateConfig(p)
	if err != nil {
		t.Fatalf("cannot load defaults: %v", err)
	}
	return cfg
}

func TestLoadOrCreateConfig_CreatesWhenMissing(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "settings.conf")

	cfg, err := config.LoadOrCreateConfig(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defaults := getDefaultValues(t)

	if cfg.RateLimit != defaults.RateLimit {
		t.Fatalf("expected ratelimit %d, got %d", defaults.RateLimit, cfg.RateLimit)
	}

	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("expected config file to be created: %v", err)
	}
}

func TestLoadOrCreateConfig_ParseOverrides(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "settings.conf")

	content := `
rate_limit=99
rate_limit_ceiling=160
cooldown_after=777
autoscale=true
upstream_dns=8.8.8.8:53
dns_retries=5
dns_timeout_ms=999
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	cfg, err := config.LoadOrCreateConfig(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.RateLimit != 99 {
		t.Fatalf("RateLimit override failed: %d", cfg.RateLimit)
	}
	if cfg.RateLimitCeiling != 160 {
		t.Fatalf("RateLimitCeiling override failed: %d", cfg.RateLimitCeiling)
	}
	if cfg.CooldownAfter != 777 {
		t.Fatalf("CooldownAfter override failed: %d", cfg.CooldownAfter)
	}
	if cfg.AutoScale != true {
		t.Fatalf("AutoScale should be true")
	}
	if cfg.UpstreamDNS != "8.8.8.8:53" {
		t.Fatalf("wrong upstream dns: %s", cfg.UpstreamDNS)
	}
	if cfg.DNSRetries != 5 {
		t.Fatalf("wrong dns retries: %d", cfg.DNSRetries)
	}
	if cfg.DNSTimeoutMS != 999 {
		t.Fatalf("wrong dns timeout: %d", cfg.DNSTimeoutMS)
	}
}

func TestLoadOrCreateConfig_ParseAutoValues(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "settings.conf")

	content := `
rate_limit=auto
cooldown_after=auto
cooldown_duration=auto
timeout_seconds=auto
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	cfg, err := config.LoadOrCreateConfig(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.RateLimit != 0 {
		t.Fatalf("expected auto ratelimit=0 got %d", cfg.RateLimit)
	}
	if cfg.CooldownAfter != 0 {
		t.Fatalf("expected auto cooldown_after=0 got %d", cfg.CooldownAfter)
	}
	if cfg.CooldownDuration != 0 {
		t.Fatalf("expected auto cooldown_duration=0 got %d", cfg.CooldownDuration)
	}
	if cfg.TimeoutSeconds != 0 {
		t.Fatalf("expected auto timeout_seconds=0 got %d", cfg.TimeoutSeconds)
	}
}

func TestParseConfig_IgnoresInvalid(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "invalid.conf")

	content := `
invalid_line
anotherbad=
rate_limit=123
junk_key=whatever
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.LoadOrCreateConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.RateLimit != 123 {
		t.Fatalf("expected 123, got %d", cfg.RateLimit)
	}
}

func TestBoolParsing(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "bool.conf")

	if err := os.WriteFile(cfgPath, []byte("autoscale=true"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _ := config.LoadOrCreateConfig(cfgPath)
	if cfg.AutoScale != true {
		t.Fatalf("expected autoscale=true")
	}

	if err := os.WriteFile(cfgPath, []byte("autoscale=false"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _ = config.LoadOrCreateConfig(cfgPath)
	if cfg.AutoScale != false {
		t.Fatalf("expected autoscale=false")
	}
}
