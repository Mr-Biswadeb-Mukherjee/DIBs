package character

import (
	"sort"
	"strings"
	"unicode"
)

var keyboardNeighbors = map[rune][]rune{
	'a': {'q', 's', 'z'}, 'b': {'v', 'g', 'h', 'n'}, 'c': {'x', 'd', 'f', 'v'},
	'd': {'s', 'e', 'r', 'f', 'c', 'x'}, 'e': {'w', 's', 'd', 'r'}, 'f': {'d', 'r', 't', 'g', 'v', 'c'},
	'g': {'f', 't', 'y', 'h', 'b', 'v'}, 'h': {'g', 'y', 'u', 'j', 'n', 'b'}, 'i': {'u', 'j', 'k', 'o'},
	'j': {'h', 'u', 'i', 'k', 'm', 'n'}, 'k': {'j', 'i', 'o', 'l', 'm'}, 'l': {'k', 'o', 'p'},
	'm': {'n', 'j', 'k'}, 'n': {'b', 'h', 'j', 'm'}, 'o': {'i', 'k', 'l', 'p'},
	'p': {'o', 'l'}, 'q': {'w', 'a'}, 'r': {'e', 'd', 'f', 't'},
	's': {'a', 'w', 'e', 'd', 'x', 'z'}, 't': {'r', 'f', 'g', 'y'}, 'u': {'y', 'h', 'j', 'i'},
	'v': {'c', 'f', 'g', 'b'}, 'w': {'q', 'a', 's', 'e'}, 'x': {'z', 's', 'd', 'c'},
	'y': {'t', 'g', 'h', 'u'}, 'z': {'a', 's', 'x'},
}

func normalize(base string) string {
	lowered := strings.ToLower(strings.TrimSpace(base))
	var b strings.Builder

	for _, r := range lowered {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func omissionVariants(raw string) []string {
	runes := []rune(raw)
	out := make([]string, 0, len(runes))

	for i := range runes {
		out = append(out, string(runes[:i])+string(runes[i+1:]))
	}
	return out
}

func duplicationVariants(raw string) []string {
	runes := []rune(raw)
	out := make([]string, 0, len(runes))

	for i := range runes {
		left := string(runes[:i+1])
		right := string(runes[i+1:])
		out = append(out, left+string(runes[i])+right)
	}
	return out
}

func transpositionVariants(raw string) []string {
	runes := []rune(raw)
	out := make([]string, 0, len(runes))

	for i := 0; i < len(runes)-1; i++ {
		tmp := make([]rune, len(runes))
		copy(tmp, runes)
		tmp[i], tmp[i+1] = tmp[i+1], tmp[i]
		out = append(out, string(tmp))
	}
	return out
}

func substitutionVariants(raw string) []string {
	runes := []rune(raw)
	out := make([]string, 0, len(runes)*3)

	for i, r := range runes {
		neighbors, ok := keyboardNeighbors[r]
		if !ok {
			continue
		}
		for _, n := range neighbors {
			tmp := make([]rune, len(runes))
			copy(tmp, runes)
			tmp[i] = n
			out = append(out, string(tmp))
		}
	}
	return out
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

func addAll(dst map[string]struct{}, list []string) {
	for _, item := range list {
		if isValidLabel(item) {
			dst[item] = struct{}{}
		}
	}
}

func toSortedSlice(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for candidate := range set {
		out = append(out, candidate)
	}
	sort.Strings(out)
	return out
}

// Character applies character-level mutations for a single base label.
func Character(base string) []string {
	raw := normalize(base)
	if raw == "" {
		return nil
	}

	unique := make(map[string]struct{})
	addAll(unique, omissionVariants(raw))
	addAll(unique, duplicationVariants(raw))
	addAll(unique, transpositionVariants(raw))
	addAll(unique, substitutionVariants(raw))
	delete(unique, raw)

	return toSortedSlice(unique)
}
