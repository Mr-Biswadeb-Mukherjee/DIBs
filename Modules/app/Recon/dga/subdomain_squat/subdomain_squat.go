package subdomainsquat

import (
	"strings"
)

// -------------------------------------------------------------
// Internal Utility (not part of public API)
// -------------------------------------------------------------
func toRuneSlice(s string) []rune {
	return []rune(strings.ToLower(s))
}

// -------------------------------------------------------------
// Internal: Transposition Squat Generator
// -------------------------------------------------------------
func generateTranspositionSquatsInternal(s string) []string {
	r := toRuneSlice(s)
	var out []string

	for i := 0; i < len(r)-1; i++ {
		tmp := make([]rune, len(r))
		copy(tmp, r)

		tmp[i], tmp[i+1] = tmp[i+1], tmp[i]
		out = append(out, string(tmp))
	}

	return out
}

// -------------------------------------------------------------
// Internal: Delimiter Squat Generator
// -------------------------------------------------------------
func generateDelimiterSquatsInternal(s string) []string {
	r := []rune(strings.ToLower(s))
	var out []string

	for i := 0; i < len(r)+1; i++ {
		// Insert hyphen
		withHyphen := append([]rune{}, r[:i]...)
		withHyphen = append(withHyphen, '-')
		withHyphen = append(withHyphen, r[i:]...)
		out = append(out, string(withHyphen))

		// Insert dot
		withDot := append([]rune{}, r[:i]...)
		withDot = append(withDot, '.')
		withDot = append(withDot, r[i:]...)
		out = append(out, string(withDot))
	}

	return out
}

// ---------------------------------------------------
// UNCHANGEABLE PUBLIC API FUNCTION
// ---------------------------------------------------

func Subdomainsquat(target string) []string {
	target = strings.ToLower(strings.TrimSpace(target))

	var out []string

	// Generate transposition variants
	out = append(out, generateTranspositionSquatsInternal(target)...)

	// Generate delimiter variants
	out = append(out, generateDelimiterSquatsInternal(target)...)

	return out
}
