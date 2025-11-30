package typosquat

import "strings"

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

func generateOmissions(domain string) []string {
	var out []string
	chars := []rune(domain)

	for i := 0; i < len(chars); i++ {
		out = append(out, string(chars[:i])+string(chars[i+1:]))
	}
	return out
}

func generateTranspositions(domain string) []string {
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

func generateSubstitutions(domain string) []string {
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

func generateInternal(base string) []string {
	raw := strings.ToLower(base)
	unique := make(map[string]bool)

	// omissions
	for _, v := range generateOmissions(raw) {
		unique[v] = true
	}

	// transpositions
	for _, v := range generateTranspositions(raw) {
		unique[v] = true
	}

	// substitutions
	for _, v := range generateSubstitutions(raw) {
		unique[v] = true
	}

	// final filtered output
	out := make([]string, 0, len(unique))
	for label := range unique {
		if len(label) >= 3 { // avoid garbage labels
			out = append(out, label)
		}
	}

	return out
}

// ---------------------------------------------------
// UNCHANGEABLE PUBLIC API FUNCTION
// ---------------------------------------------------
func TypoSquat(base string) []string {
	return generateInternal(base)
}
