// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package hashchain

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"unicode"
)

const defaultRounds = 3

func normalize(base string) string {
	raw := strings.ToLower(strings.TrimSpace(base))
	var b strings.Builder

	for _, r := range raw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isValidLabel(label string) bool {
	if len(label) < 3 || len(label) > 63 {
		return false
	}
	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return false
	}

	for _, r := range label {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			continue
		}
		return false
	}
	return true
}

func chainHexes(base string, rounds int) []string {
	seed := []byte(base)
	out := make([]string, 0, rounds)

	for i := 0; i < rounds; i++ {
		sum := sha256.Sum256(seed)
		hexed := hex.EncodeToString(sum[:])
		out = append(out, hexed)
		seed = []byte(hexed)
	}
	return out
}

func fragments(hash string) []string {
	if len(hash) < 12 {
		return nil
	}

	return []string{
		hash[:4],
		hash[:6],
		hash[len(hash)-4:],
		hash[len(hash)-6:],
		hash[8:12],
	}
}

func buildVariants(base, fragment string) []string {
	return []string{
		base + fragment,
		base + "-" + fragment,
		fragment + "-" + base,
	}
}

func appendValid(dst map[string]struct{}, values []string) {
	for _, value := range values {
		if isValidLabel(value) {
			dst[value] = struct{}{}
		}
	}
}

func toSorted(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// Hashchain creates deterministic hash-fragment mutations from a base label.
func Hashchain(base string) []string {
	raw := normalize(base)
	if raw == "" {
		return nil
	}

	unique := make(map[string]struct{})
	for _, h := range chainHexes(raw, defaultRounds) {
		for _, fragment := range fragments(h) {
			appendValid(unique, buildVariants(raw, fragment))
		}
	}

	delete(unique, raw)
	return toSorted(unique)
}
