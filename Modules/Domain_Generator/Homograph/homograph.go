package homograph

import "strings"

var HomoglyphMap = map[rune][]rune{
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

// GenerateHomographLabels returns SLD-only homograph variants.
func GenerateHomographLabels(domain string) []string {
	raw := strings.ToLower(domain)
	chars := []rune(raw)

	var out []string

	for i, char := range chars {
		if equivalents, ok := HomoglyphMap[char]; ok {
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
