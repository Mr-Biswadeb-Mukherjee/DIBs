package mutation

import "strings"

const (
	algorithmCharacter = "character"
	algorithmSeed      = "seed"
	algorithmHashchain = "hashchain"
)

type GeneratedCandidate struct {
	Domain    string
	Algorithm string
}

func appendDetailed(dst map[string]GeneratedCandidate, domains []string, algorithm string) {
	for _, domain := range domains {
		domain = normalizeDetailedDomain(domain)
		if domain == "" {
			continue
		}
		if _, ok := dst[domain]; ok {
			continue
		}
		dst[domain] = GeneratedCandidate{
			Domain:    domain,
			Algorithm: normalizeAlgorithmName(algorithm),
		}
	}
}

func GenerateDetailedFromCSV(path string) ([]GeneratedCandidate, error) {
	cfg, err := loadModuleConfig(resolveSettingsPath())
	if err != nil {
		return nil, err
	}

	keywords, err := loadKeywords(path)
	if err != nil {
		return nil, err
	}

	unique := make(map[string]GeneratedCandidate)
	for _, keyword := range keywords {
		clean := sanitizeKeyword(keyword)
		if clean == "" || !cfg.Enabled {
			continue
		}

		for algorithm, labels := range runAlgorithmsByType(clean, cfg) {
			domains := appendTLDs(labels, cfg.TargetTLDs)
			appendDetailed(unique, domains, algorithm)
		}
	}

	ordered := toSortedCandidates(unique)
	return ordered, nil
}

func toSortedCandidates(set map[string]GeneratedCandidate) []GeneratedCandidate {
	orderedDomains := make(map[string]struct{}, len(set))
	for domain := range set {
		orderedDomains[domain] = struct{}{}
	}
	sortedDomains := toSorted(orderedDomains)

	out := make([]GeneratedCandidate, 0, len(sortedDomains))
	for _, domain := range sortedDomains {
		out = append(out, set[domain])
	}
	return out
}

func normalizeDetailedDomain(domain string) string {
	return strings.ToLower(strings.TrimSpace(domain))
}

func normalizeAlgorithmName(algorithm string) string {
	algorithm = strings.ToLower(strings.TrimSpace(algorithm))
	if algorithm == "" {
		return "unknown"
	}
	return algorithm
}
