package homograph

import "strings"

var homoglyphMap = map[rune][]rune{
	'a': {'а'},
	'c': {'ϲ'},
	'e': {'е'},
	'i': {'і', 'ı'},
	'o': {'о'},
	'p': {'р'},
	's': {'ѕ'},
	'x': {'х'},
	'y': {'у'},
	'v': {'ν'},
}

// generateInternal performs homograph substitution on SLD (lowercased).
func generateInternal(domain string) []string {
	raw := strings.ToLower(domain)
	chars := []rune(raw)

	var out []string

	for i, char := range chars {
		if equivalents, ok := homoglyphMap[char]; ok {
			for _, eq := range equivalents {
				mod := make([]rune, len(chars))
				copy(mod, chars)
				mod[i] = eq
				out = append(out, string(mod))
			}
		}
	}

	return out
}

// ---------------------------------------------------
// UNCHANGEABLE PUBLIC API FUNCTION
// ---------------------------------------------------

func Homograph(base string) []string {
	return generateInternal(base)
}
