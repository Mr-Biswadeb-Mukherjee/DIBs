// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package soundsquat

import (
	"sort"
	"strings"
	"unicode"
)

// -------------------------------------------------------------
// Internal: Normalize input (Unicode-safe)
// -------------------------------------------------------------
func normalizeInternal(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// -------------------------------------------------------------
// Internal: Soundex (unchanged logic)
// -------------------------------------------------------------
func soundexInternal(s string) string {
	s = normalizeInternal(s)
	if s == "" {
		return ""
	}

	runes := []rune(s)
	first := runes[0]

	codeMap := map[rune]rune{
		'B': '1', 'P': '1', 'F': '1', 'V': '1',
		'C': '2', 'G': '2', 'J': '2', 'K': '2',
		'Q': '2', 'S': '2', 'X': '2', 'Z': '2',
		'D': '3', 'T': '3',
		'L': '4',
		'M': '5', 'N': '5',
		'R': '6',
	}

	result := []rune{first}
	lastCode := rune('0')

	for i := 1; i < len(runes); i++ {
		ch := runes[i]

		if !unicode.IsLetter(ch) {
			lastCode = '0'
			continue
		}

		code, ok := codeMap[ch]
		if !ok {
			lastCode = '0'
			continue
		}

		if code != lastCode {
			if len(result) < 4 {
				result = append(result, code)
			}
			lastCode = code
		}
	}

	for len(result) < 4 {
		result = append(result, '0')
	}

	return string(result[:4])
}

type phonemeRule struct {
	From string
	To   string
}

var phonemeRules = []phonemeRule{
	{From: "ph", To: "f"},
	{From: "f", To: "ph"},
	{From: "ck", To: "k"},
	{From: "qu", To: "kw"},
	{From: "q", To: "k"},
	{From: "x", To: "ks"},
	{From: "wh", To: "w"},
	{From: "v", To: "f"},
	{From: "z", To: "s"},
	{From: "oo", To: "u"},
	{From: "ee", To: "i"},
	{From: "a", To: "e"},
	{From: "e", To: "a"},
	{From: "i", To: "e"},
	{From: "o", To: "u"},
	{From: "u", To: "o"},
}

func normalizeLabel(s string) string {
	raw := strings.ToLower(strings.TrimSpace(s))
	if raw == "" {
		return ""
	}

	var b strings.Builder
	for _, r := range raw {
		if isLabelRune(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isLabelRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
}

func generatePhonemeVariants(base string) []string {
	unique := make(map[string]struct{})
	for _, rule := range phonemeRules {
		addRuleVariants(base, rule, unique)
	}

	delete(unique, base)
	out := make([]string, 0, len(unique))
	for variant := range unique {
		if isValidLabel(variant) {
			out = append(out, variant)
		}
	}
	sort.Strings(out)
	return out
}

func addRuleVariants(base string, rule phonemeRule, dst map[string]struct{}) {
	for pos := 0; pos < len(base); {
		idx := strings.Index(base[pos:], rule.From)
		if idx < 0 {
			return
		}

		start := pos + idx
		end := start + len(rule.From)
		variant := base[:start] + rule.To + base[end:]
		dst[variant] = struct{}{}
		pos = start + 1
	}
}

func isValidLabel(label string) bool {
	if len(label) < 3 || len(label) > 63 {
		return false
	}
	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return false
	}
	for _, r := range label {
		if !isLabelRune(r) {
			return false
		}
	}
	return true
}

// ---------------------------------------------------
// UNCHANGEABLE PUBLIC API FUNCTION
// ---------------------------------------------------

func Soundsquat(s string) []string {
	base := normalizeLabel(s)
	if base == "" {
		return nil
	}
	return generatePhonemeVariants(base)
}
