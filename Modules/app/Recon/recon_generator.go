package recon

import (
	"math"
	"sort"
	"strings"

	mutationgen "github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon/Mutation"
	dgagen "github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon/dga"
)

const lowEntropyDropThreshold = 1.10

type GeneratedDomain struct {
	Domain      string  `json:"domain"`
	RiskScore   float64 `json:"risk_score"`
	Confidence  string  `json:"confidence"`
	GeneratedBy string  `json:"generated_by"`
}

type candidateMeta struct {
	Mutation   bool
	DGA        bool
	Algorithms map[string]struct{}
}

// GenerateDomains keeps backward-compatible output while using centralized scoring.
func GenerateDomains(path string) ([]string, error) {
	scored, err := GenerateScoredDomains(path)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(scored))
	for _, candidate := range scored {
		out = append(out, candidate.Domain)
	}
	return out, nil
}

func GenerateScoredDomains(path string) ([]GeneratedDomain, error) {
	mutationCandidates, mutationErr := mutationgen.GenerateDetailedFromCSV(path)
	dgaCandidates, dgaErr := dgagen.GenerateDetailedFromCSV(path)
	if mutationErr != nil && dgaErr != nil {
		return nil, mutationErr
	}

	candidates := collectCandidates(mutationCandidates, dgaCandidates)
	return scoreCandidates(candidates), nil
}

func collectCandidates(
	mutationCandidates []mutationgen.GeneratedCandidate,
	dgaCandidates []dgagen.GeneratedCandidate,
) map[string]candidateMeta {
	out := make(map[string]candidateMeta)
	for _, candidate := range mutationCandidates {
		mergeCandidate(out, candidate.Domain, candidate.Algorithm, true)
	}
	for _, candidate := range dgaCandidates {
		mergeCandidate(out, candidate.Domain, candidate.Algorithm, false)
	}
	return out
}

func mergeCandidate(dst map[string]candidateMeta, rawDomain, algorithm string, mutation bool) {
	domain := normalizeDomain(rawDomain)
	if domain == "" {
		return
	}
	algorithm = strings.TrimSpace(strings.ToLower(algorithm))
	if algorithm == "" {
		algorithm = "unknown"
	}

	meta := dst[domain]
	if meta.Algorithms == nil {
		meta.Algorithms = make(map[string]struct{}, 2)
	}
	meta.Algorithms[algorithm] = struct{}{}
	if mutation {
		meta.Mutation = true
	} else {
		meta.DGA = true
	}
	dst[domain] = meta
}

func scoreCandidates(candidates map[string]candidateMeta) []GeneratedDomain {
	out := make([]GeneratedDomain, 0, len(candidates))
	for domain, meta := range candidates {
		candidate, ok := scoreCandidate(domain, meta)
		if ok {
			out = append(out, candidate)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Domain < out[j].Domain
	})
	return out
}

func scoreCandidate(domain string, meta candidateMeta) (GeneratedDomain, bool) {
	label := domainLabel(domain)
	entropy := shannonEntropy(label)
	if entropy < lowEntropyDropThreshold {
		return GeneratedDomain{}, false
	}

	score := baseRisk(meta) + entropyBoost(entropy) + lexicalBoost(label) + algorithmBoost(meta.Algorithms)
	score = clampRisk(score)
	return GeneratedDomain{
		Domain:      domain,
		RiskScore:   score,
		Confidence:  scoreConfidence(score),
		GeneratedBy: algorithmNames(meta.Algorithms),
	}, true
}

func baseRisk(meta candidateMeta) float64 {
	score := 0.24
	if meta.Mutation {
		score += 0.18
	}
	if meta.DGA {
		score += 0.24
	}
	if meta.Mutation && meta.DGA {
		score += 0.10
	}
	return score
}

func entropyBoost(entropy float64) float64 {
	boost := entropy / 10
	if boost > 0.28 {
		return 0.28
	}
	return boost
}

func lexicalBoost(label string) float64 {
	boost := 0.0
	if len(label) > 14 {
		boost += 0.05
	}
	if len(label) > 22 {
		boost += 0.05
	}
	if strings.ContainsRune(label, '-') {
		boost += 0.04
	}
	if strings.IndexFunc(label, isDigitRune) >= 0 {
		boost += 0.05
	}
	return boost
}

func algorithmBoost(algorithms map[string]struct{}) float64 {
	if len(algorithms) <= 1 {
		return 0
	}
	boost := float64(len(algorithms)-1) * 0.03
	if boost > 0.12 {
		return 0.12
	}
	return boost
}

func scoreConfidence(score float64) string {
	switch {
	case score >= 0.75:
		return "high"
	case score >= 0.55:
		return "medium"
	default:
		return "low"
	}
}

func algorithmNames(algorithms map[string]struct{}) string {
	if len(algorithms) == 0 {
		return "unknown"
	}
	out := make([]string, 0, len(algorithms))
	for algorithm := range algorithms {
		out = append(out, algorithm)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func normalizeDomain(raw string) string {
	domain := strings.ToLower(strings.TrimSpace(raw))
	if domain == "" || !strings.Contains(domain, ".") {
		return ""
	}
	return domain
}

func domainLabel(domain string) string {
	if idx := strings.IndexByte(domain, '.'); idx > 0 {
		return domain[:idx]
	}
	return domain
}

func shannonEntropy(label string) float64 {
	runes := []rune(label)
	if len(runes) == 0 {
		return 0
	}
	freq := make(map[rune]int)
	for _, r := range runes {
		freq[r]++
	}
	total := float64(len(runes))
	entropy := 0.0
	for _, count := range freq {
		p := float64(count) / total
		entropy -= p * math.Log2(p)
	}
	return entropy
}

func clampRisk(score float64) float64 {
	switch {
	case score < 0.05:
		score = 0.05
	case score > 0.99:
		score = 0.99
	}
	return math.Round(score*100) / 100
}

func isDigitRune(r rune) bool {
	return r >= '0' && r <= '9'
}
