package jarowinkler

import "math"

// -------------------------------------------------------------
// Helpers
// -------------------------------------------------------------

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// -------------------------------------------------------------
// Jaro Distance
// -------------------------------------------------------------

func JaroDistance(s1, s2 string) float64 {
	r1, r2 := []rune(s1), []rune(s2)
	len1, len2 := len(r1), len(r2)

	if len1 == 0 && len2 == 0 {
		return 1.0
	}
	if len1 == 0 || len2 == 0 {
		return 0.0
	}
	if s1 == s2 {
		return 1.0
	}

	matchRange := max(len1, len2)/2 - 1
	if matchRange < 0 {
		matchRange = 0
	}

	m1 := make([]bool, len1)
	m2 := make([]bool, len2)

	var matches []rune
	var matchCount float64

	// find matches
	for i := 0; i < len1; i++ {
		start := max(0, i-matchRange)
		end := min(len2-1, i+matchRange)

		for j := start; j <= end; j++ {
			if !m2[j] && r1[i] == r2[j] {
				m1[i] = true
				m2[j] = true
				matches = append(matches, r1[i])
				matchCount++
				break
			}
		}
	}

	if matchCount == 0 {
		return 0.0
	}

	// transpositions
	var matches2 []rune
	for j := 0; j < len2; j++ {
		if m2[j] {
			matches2 = append(matches2, r2[j])
		}
	}

	var transpositions float64
	for i := 0; i < len(matches); i++ {
		if matches[i] != matches2[i] {
			transpositions++
		}
	}
	transpositions /= 2.0

	j := (matchCount/float64(len1) +
		matchCount/float64(len2) +
		(matchCount-transpositions)/matchCount) / 3.0

	return j
}

// -------------------------------------------------------------
// Jaro-Winkler Distance
// -------------------------------------------------------------

func JaroWinklerDistance(s1, s2 string) float64 {
	j := JaroDistance(s1, s2)

	if j < 0.7 {
		return j
	}

	r1, r2 := []rune(s1), []rune(s2)
	l1, l2 := len(r1), len(r2)

	prefixLimit := min(min(l1, l2), 4)
	prefix := 0

	for i := 0; i < prefixLimit; i++ {
		if r1[i] == r2[i] {
			prefix++
		} else {
			break
		}
	}

	p := 0.1
	jw := j + float64(prefix)*p*(1.0-j)

	return math.Min(1.0, jw)
}
