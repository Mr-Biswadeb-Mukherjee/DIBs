package bitsquatting

import "sort"

// isSafeDomainChar checks if the flipped byte is allowed in an SLD.
func isSafeDomainChar(b byte) bool {
	if b >= 'a' && b <= 'z' {
		return true
	}
	if b >= 'A' && b <= 'Z' {
		return true
	}
	if b >= '0' && b <= '9' {
		return true
	}
	if b == '-' {
		return true
	}
	return false
}

// generateInternal is the core algorithm.
func generateInternal(base string) []string {
	orig := []byte(base)
	unique := make(map[string]struct{})

	for i := 0; i < len(orig); i++ {
		original := orig[i]

		for bit := 0; bit < 8; bit++ {
			mask := byte(1 << bit)
			flipped := original ^ mask

			if !isSafeDomainChar(flipped) {
				continue
			}

			mutated := make([]byte, len(orig))
			copy(mutated, orig)
			mutated[i] = flipped

			out := string(mutated)
			if out != base {
				unique[out] = struct{}{}
			}
		}
	}

	var results []string
	for label := range unique {
		results = append(results, label)
	}

	sort.Strings(results)
	return results
}

// ---------------------------------------------------
// UNCHANGEABLE PUBLIC API FUNCTION
// ---------------------------------------------------

func Bitsquatting(base string) []string {
	return generateInternal(base)
}
