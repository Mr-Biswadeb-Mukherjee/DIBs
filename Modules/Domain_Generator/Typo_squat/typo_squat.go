package typosquat

import "strings"

// qwertyMap is a simplified QWERTY keyboard adjacency map.
var qwertyMap = map[rune][]rune{
	'q': {'w', 'a'}, 'w': {'q', 'e', 's'}, 'e': {'w', 'r', 'd', 's'},
	'r': {'e', 't', 'f', 'd'}, 't': {'r', 'y', 'g', 'f'}, 'y': {'t', 'u', 'h', 'g'},
	'u': {'y', 'i', 'j', 'h'}, 'i': {'u', 'o', 'k', 'j'}, 'o': {'i', 'p', 'l', 'k'},
	'p': {'o', 'l'},

	'a': {'q', 's', 'z'}, 's': {'a', 'w', 'd', 'z', 'x'}, 'd': {'s', 'e', 'f', 'x', 'c'},
	'f': {'d', 'r', 'g', 'c', 'v'}, 'g': {'f', 't', 'h', 'v', 'b'}, 'h': {'g', 'y', 'j', 'b', 'n'},
	'j': {'h', 'u', 'k', 'n', 'm'}, 'k': {'j', 'i', 'l', 'm'}, 'l': {'k', 'o', 'p'},

	'z': {'a', 's', 'x'}, 'x': {'z', 's', 'd', 'c'}, 'c': {'x', 'd', 'f', 'v'},
	'v': {'c', 'f', 'g', 'b'}, 'b': {'v', 'g', 'h', 'n'}, 'n': {'b', 'h', 'j', 'm'},
	'm': {'n', 'j', 'k'},
}

// GenerateOmissions removes exactly one character.
func GenerateOmissions(domain string) []string {
	var out []string
	chars := []rune(domain)

	for i := 0; i < len(chars); i++ {
		out = append(out, string(chars[:i])+string(chars[i+1:]))
	}
	return out
}

// GenerateTranspositions swaps adjacent characters.
func GenerateTranspositions(domain string) []string {
	var out []string
	chars := []rune(domain)

	for i := 0; i < len(chars)-1; i++ {
		tmp := make([]rune, len(chars))
		copy(tmp, chars)
		tmp[i], tmp[i+1] = tmp[i+1], tmp[i]
		out = append(out, string(tmp))
	}
	return out
}

// GenerateSubstitutions replaces characters with QWERTY-adjacent keys.
func GenerateSubstitutions(domain string) []string {
	var out []string
	chars := []rune(domain)

	for i, c := range chars {
		if neighbors, ok := qwertyMap[c]; ok {
			for _, n := range neighbors {
				tmp := make([]rune, len(chars))
				copy(tmp, chars)
				tmp[i] = n
				out = append(out, string(tmp))
			}
		}
	}
	return out
}

// GenerateTypoSquatLabels returns raw labels (no TLD applied).
func GenerateTypoSquatLabels(base string) []string {
	raw := strings.ToLower(base)
	unique := make(map[string]bool)

	// omissions
	for _, v := range GenerateOmissions(raw) {
		unique[v] = true
	}

	// transposition
	for _, v := range GenerateTranspositions(raw) {
		unique[v] = true
	}

	// substitutions
	for _, v := range GenerateSubstitutions(raw) {
		unique[v] = true
	}

	// convert to slice
	out := make([]string, 0, len(unique))
	for label := range unique {
		if len(label) >= 3 {
			out = append(out, label)
		}
	}

	return out
}
