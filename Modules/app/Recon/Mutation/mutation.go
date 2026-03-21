package mutation

import (
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"

	charalg "github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon/Mutation/character"
	hashalg "github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon/Mutation/hashchain"
	seedalg "github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon/Mutation/seed"
)

const defaultSettingsPath = "Setting/setting.conf"

func sanitizeKeyword(value string) string {
	raw := strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder

	for _, r := range raw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func loadKeywords(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}

	keywords := make([]string, 0, len(rows))
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		clean := sanitizeKeyword(row[0])
		if clean != "" && clean != "domain" {
			keywords = append(keywords, clean)
		}
	}
	return keywords, nil
}

func runAlgorithmsByType(base string, cfg moduleConfig) map[string][]string {
	byType := make(map[string][]string, 3)
	if cfg.Character {
		byType[algorithmCharacter] = charalg.Character(base)
	}
	if cfg.Seed {
		byType[algorithmSeed] = seedalg.Seed(base)
	}
	if cfg.Hashchain {
		byType[algorithmHashchain] = hashalg.Hashchain(base)
	}
	return byType
}

func runAlgorithms(base string, cfg moduleConfig) map[string]struct{} {
	unique := make(map[string]struct{})
	for _, candidates := range runAlgorithmsByType(base, cfg) {
		appendUnique(unique, candidates)
	}
	delete(unique, base)
	return unique
}

func appendUnique(dst map[string]struct{}, values []string) {
	for _, value := range values {
		if value == "" {
			continue
		}
		dst[value] = struct{}{}
	}
}

func toSorted(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for candidate := range set {
		out = append(out, candidate)
	}
	sort.Strings(out)
	return out
}

func appendTLDs(labels, tlds []string) []string {
	if len(tlds) == 0 {
		return labels
	}

	unique := make(map[string]struct{}, len(labels)*len(tlds))
	for _, label := range labels {
		for _, tld := range tlds {
			unique[label+tld] = struct{}{}
		}
	}
	return toSorted(unique)
}

// Mutate runs all mounted mutation algorithms for a single keyword.
func Mutate(base string) []string {
	clean := sanitizeKeyword(base)
	if clean == "" {
		return nil
	}
	return toSorted(runAlgorithms(clean, defaultModuleConfig))
}

func mutateWithConfig(base string, cfg moduleConfig) []string {
	clean := sanitizeKeyword(base)
	if clean == "" || !cfg.Enabled {
		return nil
	}

	mutated := toSorted(runAlgorithms(clean, cfg))
	return appendTLDs(mutated, cfg.TargetTLDs)
}

// GenerateFromCSV loads keywords from a CSV and returns deduplicated mutations.
func GenerateFromCSV(path string) ([]string, error) {
	cfg, err := loadModuleConfig(resolveSettingsPath())
	if err != nil {
		return nil, err
	}

	keywords, err := loadKeywords(path)
	if err != nil {
		return nil, err
	}

	unique := make(map[string]struct{})
	for _, keyword := range keywords {
		appendUnique(unique, mutateWithConfig(keyword, cfg))
	}
	return toSorted(unique), nil
}
