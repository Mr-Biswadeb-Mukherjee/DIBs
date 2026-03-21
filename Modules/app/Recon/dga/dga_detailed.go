package domain_generator

import (
	"sort"
	"strings"

	bs "github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon/dga/bitsquatting"
	cs "github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon/dga/combo_squat"
	hg "github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon/dga/homograph"
	ss2 "github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon/dga/sound_squat"
	ss1 "github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon/dga/subdomain_squat"
	ts "github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon/dga/typo_squat"
)

const (
	algorithmTypoSquat      = "typo_squat"
	algorithmHomograph      = "homograph"
	algorithmBitsquatting   = "bitsquatting"
	algorithmComboSquat     = "combo_squat"
	algorithmSubdomainSquat = "subdomain_squat"
	algorithmSoundSquat     = "sound_squat"
)

type GeneratedCandidate struct {
	Domain    string
	Algorithm string
}

func GenerateDetailedFromCSV(path string) ([]GeneratedCandidate, error) {
	keywords, err := loadKeywords(path)
	if err != nil {
		return nil, err
	}

	byDomain := make(map[string]GeneratedCandidate)
	for _, base := range keywords {
		for algorithm, domains := range generateForBaseByAlgorithm(base) {
			appendCandidates(byDomain, domains, algorithm)
		}
	}
	return sortedCandidates(byDomain), nil
}

func generateForBaseByAlgorithm(base string) map[string][]string {
	out := make(map[string][]string, 6)
	out[algorithmTypoSquat] = filterSimilar(base, appendTLDs(ts.TypoSquat(base)))
	out[algorithmHomograph] = filterSimilar(base, appendTLDs(hg.Homograph(base)))
	out[algorithmBitsquatting] = filterSimilar(base, appendTLDs(bs.Bitsquatting(base)))
	out[algorithmComboSquat] = filterSimilar(base, appendTLDs(cs.Combosquat(base)))
	out[algorithmSubdomainSquat] = filterSimilar(base, appendTLDs(ss1.Subdomainsquat(base)))
	out[algorithmSoundSquat] = filterSimilar(base, appendTLDs(ss2.Soundsquat(base)))
	return out
}

func appendCandidates(dst map[string]GeneratedCandidate, domains []string, algorithm string) {
	for _, domain := range domains {
		domain = normalizeGeneratedDomain(domain)
		if domain == "" {
			continue
		}
		if _, exists := dst[domain]; exists {
			continue
		}
		dst[domain] = GeneratedCandidate{
			Domain:    domain,
			Algorithm: normalizeAlgorithm(algorithm),
		}
	}
}

func sortedCandidates(set map[string]GeneratedCandidate) []GeneratedCandidate {
	domains := make([]string, 0, len(set))
	for domain := range set {
		domains = append(domains, domain)
	}
	sort.Strings(domains)

	out := make([]GeneratedCandidate, 0, len(domains))
	for _, domain := range domains {
		out = append(out, set[domain])
	}
	return out
}

func normalizeGeneratedDomain(domain string) string {
	return strings.ToLower(strings.TrimSpace(domain))
}

func normalizeAlgorithm(algorithm string) string {
	algorithm = strings.ToLower(strings.TrimSpace(algorithm))
	if algorithm == "" {
		return "unknown"
	}
	return algorithm
}
