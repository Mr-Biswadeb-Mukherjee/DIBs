package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/official-biswadeb941/Infermal_v2/Modules/app/core/config"
)

// ----------------------------------------------
// Helper: get defaults indirectly
// ----------------------------------------------
// Since defaultConfig is unexported, we obtain the defaults
// by calling LoadOrCreateConfig() on a new file.
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

// ----------------------------------------------
// Test: LoadOrCreateConfig creates file + loads defaults
// ----------------------------------------------
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

// ----------------------------------------------
// Test: Parsing existing config with overrides
// ----------------------------------------------
func TestLoadOrCreateConfig_ParseOverrides(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "settings.conf")

	content := `
rate_limit=99
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

// ----------------------------------------------
// Test: Invalid entries ignored
// ----------------------------------------------
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

// ----------------------------------------------
// Test: Boolean parsing
// ----------------------------------------------
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

	// change to false
	if err := os.WriteFile(cfgPath, []byte("autoscale=false"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _ = config.LoadOrCreateConfig(cfgPath)
	if cfg.AutoScale != false {
		t.Fatalf("expected autoscale=false")
	}
}

// ----------------------------------------------
// Test: Validate default file content
// ----------------------------------------------
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
