package domain_generator

import (
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	ts "github.com/official-biswadeb941/Infermal_v2/Modules/Domain_Generator/Typo_squat"
	hg "github.com/official-biswadeb941/Infermal_v2/Modules/Domain_Generator/Homograph"
	bs "github.com/official-biswadeb941/Infermal_v2/Modules/Domain_Generator/Bitsquatting"
    cs "github.com/official-biswadeb941/Infermal_v2/Modules/Domain_Generator/Combosquat"
	ss1 "github.com/official-biswadeb941/Infermal_v2/Modules/Domain_Generator/Subdomain_squat"
	ss2 "github.com/official-biswadeb941/Infermal_v2/Modules/Domain_Generator/Soundsquat"

	jw "github.com/official-biswadeb941/Infermal_v2/Modules/Domain_Generator/Jarowinkler"

)

var targetTLDs = []string{
	".in",
}

// Similarity threshold for Jaro-Winkler
const similarityThreshold = 0.10

func sanitizeKeyword(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")

	remove := []string{".", ",", "/", "\\", ":", ";", "'", "\"", "?", "!", "(", ")", "[", "]", "{", "}", "|"}
	for _, r := range remove {
		s = strings.ReplaceAll(s, r, "")
	}

	return s
}

func LoadKeywordsCSV(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	var domains []string
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}
		if i == 0 && (row[0] == "domain" || row[0] == "Domain") {
			continue
		}

		cleaned := sanitizeKeyword(row[0])
		if cleaned != "" {
			domains = append(domains, cleaned)
		}
	}

	return domains, nil
}

func appendTLDs(labels []string) []string {
	var out []string
	for _, label := range labels {
		for _, tld := range targetTLDs {
			out = append(out, label+tld)
		}
	}
	return out
}

// ---------------------------------------------------
// SIMILARITY FILTER
// ---------------------------------------------------

func filterSimilar(base string, domains []string) []string {
	var out []string

	for _, d := range domains {
		score := jw.JaroWinklerDistance(base, d)
		if score >= similarityThreshold {
			out = append(out, d)
		}
	}

	return out
}

func runAllInternal(base string) map[string][]string {
	rawTypo := ts.TypoSquat(base)
	rawHomo := hg.Homograph(base)
	rawBits := bs.Bitsquatting(base)
    rawcombo := cs.Combosquat(base)
    rawsubdomain := ss1.Subdomainsquat(base)
    rawsoundsquat := ss2.Soundsquat(base)


	typo := appendTLDs(rawTypo)
	homo := appendTLDs(rawHomo)
	bits := appendTLDs(rawBits)
    combo := appendTLDs(rawcombo)
    subdomain := appendTLDs(rawsubdomain)
    soundsquat := appendTLDs(rawsoundsquat)

	// integrate Jaro-Winkler filtering internally
	typo = filterSimilar(base, typo)
	homo = filterSimilar(base, homo)
	bits = filterSimilar(base, bits)
    combo = filterSimilar(base, combo)
    subdomain = filterSimilar(base, subdomain)
    soundsquat = filterSimilar(base, soundsquat)


	return map[string][]string{
		"typo_squat": typo,
		"homograph":  homo,
		"bitsquat":   bits,
        "combosquat": combo,
        "subdomainsquat": subdomain,
        "soundsquat": soundsquat,
	}
}

// ---------------------------------------------------
// UNCHANGEABLE PUBLIC API FUNCTION 
// ---------------------------------------------------

func DomainGenerator(base string) map[string][]string {
	return runAllInternal(base)
}
