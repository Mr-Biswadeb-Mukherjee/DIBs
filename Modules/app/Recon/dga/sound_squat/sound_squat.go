package soundsquat

import (
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

// ---------------------------------------------------
// UNCHANGEABLE PUBLIC API FUNCTION
// ---------------------------------------------------

func Soundsquat(s string) []string {
	s = strings.ToLower(strings.TrimSpace(s))
	code := soundexInternal(s)
	return []string{code}
}
