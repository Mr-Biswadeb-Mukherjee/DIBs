package recon

import "strings"

func filterHumanLikeCandidates(scored []GeneratedDomain) []GeneratedDomain {
	out := make([]GeneratedDomain, 0, len(scored))
	for _, candidate := range scored {
		if !IsHumanLikeDomain(candidate.Domain) {
			continue
		}
		out = append(out, candidate)
	}
	return out
}

func IsHumanLikeDomain(domain string) bool {
	domain = normalizeDomain(domain)
	if domain == "" {
		return false
	}
	return isHumanLikeLabel(domainLabel(domain))
}

func isHumanLikeLabel(label string) bool {
	if len(label) < 3 {
		return false
	}
	if digitRatio(label) > 0.30 {
		return false
	}
	if letterRatio(label) < 0.60 {
		return false
	}
	if len(label) >= 8 && vowelRatio(label) < 0.18 {
		return false
	}
	if longestConsonantRun(label) > 5 {
		return false
	}
	if longestRepeatedRun(label) > 3 {
		return false
	}
	return true
}

func digitRatio(label string) float64 {
	if label == "" {
		return 0
	}
	digits := 0
	for _, r := range label {
		if isDigitRune(r) {
			digits++
		}
	}
	return float64(digits) / float64(len(label))
}

func letterRatio(label string) float64 {
	if label == "" {
		return 0
	}
	letters := 0
	for _, r := range label {
		if isAsciiLetter(r) {
			letters++
		}
	}
	return float64(letters) / float64(len(label))
}

func vowelRatio(label string) float64 {
	letters := 0
	vowels := 0
	for _, r := range label {
		if !isAsciiLetter(r) {
			continue
		}
		letters++
		if isVowel(r) {
			vowels++
		}
	}
	if letters == 0 {
		return 0
	}
	return float64(vowels) / float64(letters)
}

func longestConsonantRun(label string) int {
	run := 0
	best := 0
	for _, r := range strings.ToLower(label) {
		if isAsciiLetter(r) && !isVowel(r) {
			run++
			if run > best {
				best = run
			}
			continue
		}
		run = 0
	}
	return best
}

func longestRepeatedRun(label string) int {
	runes := []rune(strings.ToLower(label))
	if len(runes) == 0 {
		return 0
	}
	run := 1
	best := 1
	for i := 1; i < len(runes); i++ {
		if runes[i] == runes[i-1] {
			run++
			if run > best {
				best = run
			}
			continue
		}
		run = 1
	}
	return best
}

func isAsciiLetter(r rune) bool {
	return r >= 'a' && r <= 'z'
}

func isVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	default:
		return false
	}
}
