// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	RateLimit        int
	RateLimitCeiling int
	CooldownAfter    int
	CooldownDuration int
	TimeoutSeconds   int // worker timeout
	MaxRetries       int // worker retries
	AutoScale        bool

	// DNS settings
	UpstreamDNS  string
	BackupDNS    string
	DNSRetries   int
	DNSTimeoutMS int64
}

var defaultConfig = Config{
	// Worker / Performance defaults
	RateLimit:        0, // auto
	RateLimitCeiling: 160,
	CooldownAfter:    0, // auto
	CooldownDuration: 0, // auto
	TimeoutSeconds:   0, // auto
	MaxRetries:       3,
	AutoScale:        false,

	// DNS defaults
	UpstreamDNS:  "9.9.9.9:53",
	BackupDNS:    "1.1.1.1:53",
	DNSRetries:   2,
	DNSTimeoutMS: 500,
}

func LoadOrCreateConfig(path string) (Config, error) {
	cleanPath, err := sanitizeConfigPath(path)
	if err != nil {
		return defaultConfig, err
	}
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0o750); err != nil {
		return defaultConfig, err
	}

	if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
		if err := writeDefault(cleanPath); err != nil {
			return defaultConfig, err
		}
		return defaultConfig, nil
	}
	if err := ensureConfigEntries(cleanPath); err != nil {
		return defaultConfig, err
	}
	return parseConfig(cleanPath)
}

func writeDefault(path string) error {
	cleanPath, err := sanitizeConfigPath(path)
	if err != nil {
		return err
	}
	// #nosec G304 -- cleanPath is normalized and validated by sanitizeConfigPath.
	f, err := os.Create(cleanPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(defaultConfigText())
	return err
}

func defaultConfigText() string {
	return fmt.Sprintf(
		"# Worker / Performance\n"+
			"rate_limit=%s\n"+
			"rate_limit_ceiling=%d\n"+
			"cooldown_after=%s\n"+
			"cooldown_duration=%s\n"+
			"timeout_seconds=%s\n"+
			"max_retries=%d\n"+
			"autoscale=%t\n\n"+
			"# DNS settings\n"+
			"upstream_dns=%s\n"+
			"backup_dns=%s\n"+
			"dns_retries=%d\n"+
			"dns_timeout_ms=%d\n",
		formatAutoInt(defaultConfig.RateLimit),
		defaultConfig.RateLimitCeiling,
		formatAutoInt(defaultConfig.CooldownAfter),
		formatAutoInt(defaultConfig.CooldownDuration),
		formatAutoInt(defaultConfig.TimeoutSeconds),
		defaultConfig.MaxRetries,
		defaultConfig.AutoScale,
		defaultConfig.UpstreamDNS,
		defaultConfig.BackupDNS,
		defaultConfig.DNSRetries,
		defaultConfig.DNSTimeoutMS,
	)
}

func parseConfig(path string) (Config, error) {
	cleanPath, err := sanitizeConfigPath(path)
	if err != nil {
		return defaultConfig, err
	}
	// #nosec G304 -- cleanPath is normalized and validated by sanitizeConfigPath.
	file, err := os.Open(cleanPath)
	if err != nil {
		return defaultConfig, err
	}
	defer file.Close()

	cfg := defaultConfig
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		applyConfigLine(&cfg, scanner.Text())
	}
	return cfg, scanner.Err()
}

func applyConfigLine(cfg *Config, line string) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return
	}

	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	applyConfigValue(cfg, key, value)
}

func applyConfigValue(cfg *Config, key, value string) {
	if setter, ok := configSetters[key]; ok {
		setter(cfg, value)
	}
}

var configSetters = map[string]func(*Config, string){
	"rate_limit": func(cfg *Config, value string) { cfg.RateLimit = parseIntOrAuto(value, cfg.RateLimit) },
	"rate_limit_ceiling": func(cfg *Config, value string) {
		cfg.RateLimitCeiling = parsePositiveInt(value, cfg.RateLimitCeiling)
	},
	"cooldown_after":    func(cfg *Config, value string) { cfg.CooldownAfter = parseIntOrAuto(value, cfg.CooldownAfter) },
	"cooldown_duration": func(cfg *Config, value string) { cfg.CooldownDuration = parseIntOrAuto(value, cfg.CooldownDuration) },
	"timeout_seconds":   func(cfg *Config, value string) { cfg.TimeoutSeconds = parseIntOrAuto(value, cfg.TimeoutSeconds) },
	"max_retries":       func(cfg *Config, value string) { cfg.MaxRetries, _ = strconv.Atoi(value) },
	"autoscale":         func(cfg *Config, value string) { cfg.AutoScale = (value == "true") },
	"upstream_dns":      func(cfg *Config, value string) { cfg.UpstreamDNS = value },
	"backup_dns":        func(cfg *Config, value string) { cfg.BackupDNS = value },
	"dns_retries":       func(cfg *Config, value string) { cfg.DNSRetries, _ = strconv.Atoi(value) },
	"dns_timeout_ms": func(cfg *Config, value string) {
		ms, _ := strconv.Atoi(value)
		cfg.DNSTimeoutMS = int64(ms)
	},
}

func sanitizeConfigPath(path string) (string, error) {
	raw := strings.TrimSpace(path)
	if raw == "" {
		return "", fmt.Errorf("config path is empty")
	}

	cleanPath := filepath.Clean(raw)
	if info, err := os.Stat(cleanPath); err == nil && info.IsDir() {
		return "", fmt.Errorf("config path is a directory: %s", cleanPath)
	}
	return cleanPath, nil
}

func formatAutoInt(v int) string {
	if v <= 0 {
		return "auto"
	}
	return strconv.Itoa(v)
}

func parseIntOrAuto(value string, fallback int) int {
	if strings.EqualFold(strings.TrimSpace(value), "auto") {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parsePositiveInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}
