// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

type configEntry struct {
	key   string
	value string
}

var configEntries = []configEntry{
	{key: "rate_limit", value: formatAutoInt(defaultConfig.RateLimit)},
	{key: "rate_limit_ceiling", value: strconv.Itoa(defaultConfig.RateLimitCeiling)},
	{key: "cooldown_after", value: formatAutoInt(defaultConfig.CooldownAfter)},
	{key: "cooldown_duration", value: formatAutoInt(defaultConfig.CooldownDuration)},
	{key: "timeout_seconds", value: formatAutoInt(defaultConfig.TimeoutSeconds)},
	{key: "max_retries", value: strconv.Itoa(defaultConfig.MaxRetries)},
	{key: "autoscale", value: strconv.FormatBool(defaultConfig.AutoScale)},
	{key: "upstream_dns", value: defaultConfig.UpstreamDNS},
	{key: "backup_dns", value: defaultConfig.BackupDNS},
	{key: "dns_retries", value: strconv.Itoa(defaultConfig.DNSRetries)},
	{key: "dns_timeout_ms", value: strconv.FormatInt(defaultConfig.DNSTimeoutMS, 10)},
}

func ensureConfigEntries(path string) error {
	cleanPath, err := sanitizeConfigPath(path)
	if err != nil {
		return err
	}
	// #nosec G304 -- cleanPath is normalized and validated by sanitizeConfigPath.
	content, err := os.ReadFile(cleanPath)
	if err != nil {
		return err
	}
	existing := existingConfigKeys(content)
	missing := make([]configEntry, 0, len(configEntries))
	for _, entry := range configEntries {
		if _, ok := existing[entry.key]; ok {
			continue
		}
		missing = append(missing, entry)
	}
	if len(missing) == 0 {
		return nil
	}
	return appendMissingConfigEntries(cleanPath, missing)
}

func existingConfigKeys(content []byte) map[string]struct{} {
	keys := make(map[string]struct{}, len(configEntries))
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		keys[strings.TrimSpace(parts[0])] = struct{}{}
	}
	return keys
}

func appendMissingConfigEntries(path string, entries []configEntry) error {
	cleanPath, err := sanitizeConfigPath(path)
	if err != nil {
		return err
	}
	// #nosec G304 -- cleanPath is normalized and validated by sanitizeConfigPath.
	file, err := os.OpenFile(cleanPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	var b strings.Builder
	b.WriteString("\n# Auto-injected defaults\n")
	for _, entry := range entries {
		b.WriteString(entry.key)
		b.WriteString("=")
		b.WriteString(entry.value)
		b.WriteString("\n")
	}
	_, err = file.WriteString(b.String())
	return err
}
