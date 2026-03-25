// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

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

	matchCount, matches, m2 := findMatches(r1, r2)
	if matchCount == 0 {
		return 0.0
	}

	transpositions := countTranspositions(matches, r2, m2)

	j := (matchCount/float64(len1) +
		matchCount/float64(len2) +
		(matchCount-transpositions)/matchCount) / 3.0

	return j
}

func findMatches(r1, r2 []rune) (float64, []rune, []bool) {
	matchRange := max(len(r1), len(r2))/2 - 1
	if matchRange < 0 {
		matchRange = 0
	}
	m2 := make([]bool, len(r2))
	matches := make([]rune, 0, len(r1))
	var matchCount float64

	for i := 0; i < len(r1); i++ {
		start := max(0, i-matchRange)
		end := min(len(r2)-1, i+matchRange)
		for j := start; j <= end; j++ {
			if m2[j] || r1[i] != r2[j] {
				continue
			}
			m2[j] = true
			matches = append(matches, r1[i])
			matchCount++
			break
		}
	}
	return matchCount, matches, m2
}

func countTranspositions(matches, r2 []rune, m2 []bool) float64 {
	matches2 := make([]rune, 0, len(matches))
	for j := 0; j < len(r2); j++ {
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
	return transpositions / 2.0
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
