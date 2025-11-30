package combosquat

import (
	"strings"
)

// -------------------------------------------------------------
// Unicode-safe rune conversion
// -------------------------------------------------------------
func toRunes(s string) []rune {
	return []rune(strings.ToLower(s))
}

// -------------------------------------------------------------
// Levenshtein Distance (rune-safe)
// -------------------------------------------------------------
func levenshteinInternal(s1, s2 string) int {
	r1 := toRunes(s1)
	r2 := toRunes(s2)

	len1 := len(r1)
	len2 := len(r2)

	if len1 == 0 {
		return len2
	}
	if len2 == 0 {
		return len1
	}

	prev := make([]int, len2+1)
	curr := make([]int, len2+1)

	for j := 0; j <= len2; j++ {
		prev[j] = j
	}

	for i := 1; i <= len1; i++ {
		curr[0] = i

		for j := 1; j <= len2; j++ {
			cost := 0
			if r1[i-1] != r2[j-1] {
				cost = 1
			}

			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost

			if del < ins {
				if del < sub {
					curr[j] = del
				} else {
					curr[j] = sub
				}
			} else {
				if ins < sub {
					curr[j] = ins
				} else {
					curr[j] = sub
				}
			}
		}

		prev, curr = curr, prev
	}

	return prev[len2]
}

// -------------------------------------------------------------
// Internal generator rules
// -------------------------------------------------------------

// single-edit variants
func generateEdits(s string) []string {
	r := toRunes(s)
	out := []string{}

	// deletion
	for i := range r {
		out = append(out, string(append(r[:i], r[i+1:]...)))
	}

	// insertion
	for i := range r {
		for ch := 'a'; ch <= 'z'; ch++ {
			variant := append(
				append([]rune{}, r[:i]...),
				append([]rune{ch}, r[i:]...)...,
			)
			out = append(out, string(variant))
		}
	}

	// substitution
	for i := range r {
		for ch := 'a'; ch <= 'z'; ch++ {
			if r[i] != ch {
				v := make([]rune, len(r))
				copy(v, r)
				v[i] = ch
				out = append(out, string(v))
			}
		}
	}

	// transposition
	for i := 0; i < len(r)-1; i++ {
		v := make([]rune, len(r))
		copy(v, r)
		v[i], v[i+1] = v[i+1], v[i]
		out = append(out, string(v))
	}

	return out
}

// ---------------------------------------------------
// UNCHANGEABLE PUBLIC API FUNCTION
// ---------------------------------------------------

func Combosquat(target string) []string {
	target = strings.ToLower(target)

	candidates := generateEdits(target)
	seen := map[string]bool{}
	output := []string{}

	for _, c := range candidates {
		if c == target {
			continue
		}
		if seen[c] {
			continue
		}
		seen[c] = true

		// default generator rule: Levenshtein distance threshold of 3
		if levenshteinInternal(target, c) <= 3 {
			output = append(output, c)
		}
	}

	return output
}
