// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package domain_generator

import (
	"bufio"
	"os"
	"strings"
)

const (
	keyTargetTLDs         = "target_tlds"
	keyTargetTLDsIndia    = "target_tlds_india"
	keyTargetTLDsGlobal   = "target_tlds_global"
	keyTargetTLDsEmerging = "target_tlds_emerging"
	keyActiveTLDTiers     = "active_tld_tiers"
)

var defaultTargetTLDs = []string{
	".com", ".net", ".org", ".in", ".io",
	".co", ".xyz", ".info", ".online", ".site",
}

var defaultActiveTLDTiers = []string{"india", "global"}

var defaultTierTLDs = map[string][]string{
	"india": {
		".in", ".co.in", ".net.in", ".org.in", ".in.net",
	},
	"global": {
		".com", ".net", ".org", ".top", ".xyz", ".online", ".shop", ".info",
		".vip", ".sbs", ".icu", ".buzz", ".monster", ".cfd", ".lat",
	},
	"emerging": {
		".zip", ".mov", ".li", ".beauty", ".pics", ".lol", ".finance", ".support",
	},
}

func loadTargetTLDs(path string) []string {
	values, err := readConfigValues(path)
	if err != nil {
		return defaultTargetTLDsCopy()
	}

	if tlds := parseTLDCSV(values[keyTargetTLDs]); len(tlds) > 0 {
		return tlds
	}

	tlds := loadTargetTLDTiers(values)
	if len(tlds) > 0 {
		return tlds
	}

	return defaultTargetTLDsCopy()
}

func readConfigValues(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	values := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseConfigKV(scanner.Text())
		if ok {
			values[key] = value
		}
	}
	return values, scanner.Err()
}

func loadTargetTLDTiers(values map[string]string) []string {
	if !hasTierConfig(values) {
		return nil
	}

	active := parseTierCSV(values[keyActiveTLDTiers])
	if len(active) == 0 {
		active = defaultActiveTLDTiersCopy()
	}
	return resolveTierTLDs(values, active)
}

func hasTierConfig(values map[string]string) bool {
	if _, ok := values[keyActiveTLDTiers]; ok {
		return true
	}
	if _, ok := values[keyTargetTLDsIndia]; ok {
		return true
	}
	if _, ok := values[keyTargetTLDsGlobal]; ok {
		return true
	}
	_, ok := values[keyTargetTLDsEmerging]
	return ok
}

func parseTierCSV(value string) []string {
	parts := strings.Split(value, ",")
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))

	for _, part := range parts {
		tier := normalizeTierName(part)
		if tier == "" {
			continue
		}
		if _, exists := seen[tier]; exists {
			continue
		}
		seen[tier] = struct{}{}
		out = append(out, tier)
	}
	return out
}

func normalizeTierName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func resolveTierTLDs(values map[string]string, active []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, 16)

	for _, tier := range active {
		out = appendUniqueTLDs(out, tierTLDs(values, tier), seen)
	}
	return out
}

func tierTLDs(values map[string]string, tier string) []string {
	key := tierConfigKey(tier)
	if key == "" {
		return nil
	}
	if tlds := parseTLDCSV(values[key]); len(tlds) > 0 {
		return tlds
	}
	return defaultTierTLDsCopy(tier)
}

func tierConfigKey(tier string) string {
	switch tier {
	case "india":
		return keyTargetTLDsIndia
	case "global":
		return keyTargetTLDsGlobal
	case "emerging":
		return keyTargetTLDsEmerging
	default:
		return ""
	}
}

func defaultTierTLDsCopy(tier string) []string {
	values := defaultTierTLDs[tier]
	return append([]string(nil), values...)
}

func defaultActiveTLDTiersCopy() []string {
	return append([]string(nil), defaultActiveTLDTiers...)
}

func appendUniqueTLDs(dst, tlds []string, seen map[string]struct{}) []string {
	for _, tld := range tlds {
		if _, ok := seen[tld]; ok {
			continue
		}
		seen[tld] = struct{}{}
		dst = append(dst, tld)
	}
	return dst
}

func parseConfigKV(line string) (string, string, bool) {
	raw := strings.TrimSpace(line)
	if raw == "" || strings.HasPrefix(raw, "#") {
		return "", "", false
	}

	parts := strings.SplitN(raw, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), true
}

func parseTLDCSV(value string) []string {
	parts := strings.Split(value, ",")
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))

	for _, part := range parts {
		tld := normalizeTLD(part)
		if tld == "" {
			continue
		}
		if _, exists := seen[tld]; exists {
			continue
		}
		seen[tld] = struct{}{}
		out = append(out, tld)
	}
	return out
}

func normalizeTLD(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, ".") {
		return trimmed
	}
	return "." + trimmed
}

func defaultTargetTLDsCopy() []string {
	return append([]string(nil), defaultTargetTLDs...)
}
