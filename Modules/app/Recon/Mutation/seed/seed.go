package seed

import (
	"sort"
	"strings"
	"unicode"
)

var defaultSeeds = []string{
	"login", "secure", "portal", "account", "auth",
	"verify", "update", "support", "pay", "billing",
	"status", "team", "service", "cloud", "mail",
}

var separators = []string{"", "-"}

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

func buildPrefix(base, token string) []string {
	out := make([]string, 0, len(separators))
	for _, sep := range separators {
		out = append(out, token+sep+base)
	}
	return out
}

func buildSuffix(base, token string) []string {
	out := make([]string, 0, len(separators))
	for _, sep := range separators {
		out = append(out, base+sep+token)
	}
	return out
}

func buildInfix(base, token string) []string {
	mid := len(base) / 2
	left := base[:mid]
	right := base[mid:]
	out := make([]string, 0, len(separators))

	for _, sep := range separators {
		out = append(out, left+sep+token+sep+right)
	}
	return out
}

func appendValid(unique map[string]struct{}, list []string) {
	for _, candidate := range list {
		if isValidLabel(candidate) {
			unique[candidate] = struct{}{}
		}
	}
}

func toSorted(unique map[string]struct{}) []string {
	out := make([]string, 0, len(unique))
	for value := range unique {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

// Seed applies token-based mutations by combining the base with known seed words.
func Seed(base string) []string {
	raw := normalize(base)
	if raw == "" {
		return nil
	}

	unique := make(map[string]struct{})
	for _, seed := range defaultSeeds {
		token := normalize(seed)
		if token == "" {
			continue
		}
		appendValid(unique, buildPrefix(raw, token))
		appendValid(unique, buildSuffix(raw, token))
		appendValid(unique, buildInfix(raw, token))
	}

	delete(unique, raw)
	return toSorted(unique)
}
