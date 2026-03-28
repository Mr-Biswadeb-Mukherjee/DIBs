// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package dns_intel

import (
	"strings"

	"golang.org/x/net/publicsuffix"
)

func extractProviders(cnames, ns []string) []string {
	all := append(cnames, ns...)
	if len(all) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	out := make([]string, 0, len(all))
	for _, value := range all {
		root := extractRootDomain(value)
		if root == "" {
			continue
		}
		if _, ok := seen[root]; ok {
			continue
		}
		seen[root] = struct{}{}
		out = append(out, root)
	}
	return out
}

func extractRootDomain(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return ""
	}

	domain, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return ""
	}
	return domain
}

func sanitize(in []string) []string {
	if len(in) == 0 {
		return nil
	}

	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, value := range in {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
