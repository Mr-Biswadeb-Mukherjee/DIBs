// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package domain_generator

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	bs "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/Modules/app/Recon/dga/bitsquatting"
	cs "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/Modules/app/Recon/dga/combo_squat"
	hg "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/Modules/app/Recon/dga/homograph"
	jw "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/Modules/app/Recon/dga/jarowinkler"
	ss2 "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/Modules/app/Recon/dga/sound_squat"
	ss1 "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/Modules/app/Recon/dga/subdomain_squat"
	ts "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/Modules/app/Recon/dga/typo_squat"
)

// Default target TLDs
var targetTLDs = []string{".in"}

const similarityThreshold = 0.10

// ---------------------------------------------------
// Sanitizer
// ---------------------------------------------------

func sanitizeKeyword(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, " ", "")

	remove := []string{
		".", ",", "/", "\\", ":", ";", "'", "\"", "?",
		"!", "(", ")", "[", "]", "{", "}", "|",
	}
	for _, r := range remove {
		s = strings.ReplaceAll(s, r, "")
	}
	return s
}

// ---------------------------------------------------
// Internal CSV loader
// ---------------------------------------------------

func loadKeywords(path string) ([]string, error) {
	cleanPath, err := sanitizeCSVPath(path)
	if err != nil {
		return nil, err
	}
	// #nosec G304 -- cleanPath is normalized and validated by sanitizeCSVPath.
	f, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	var list []string
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}

		// Skip header if present
		if i == 0 && (row[0] == "domain" || row[0] == "Domain") {
			continue
		}

		cleaned := sanitizeKeyword(row[0])
		if cleaned != "" {
			list = append(list, cleaned)
		}
	}
	return list, nil
}

func sanitizeCSVPath(path string) (string, error) {
	raw := strings.TrimSpace(path)
	if raw == "" {
		return "", fmt.Errorf("csv path is empty")
	}

	cleanPath := filepath.Clean(raw)
	if !strings.EqualFold(filepath.Ext(cleanPath), ".csv") {
		return "", fmt.Errorf("csv path must end with .csv: %s", cleanPath)
	}
	if info, err := os.Stat(cleanPath); err == nil && info.IsDir() {
		return "", fmt.Errorf("csv path is a directory: %s", cleanPath)
	}
	return cleanPath, nil
}

// ---------------------------------------------------
// Internal helpers
// ---------------------------------------------------

func appendTLDs(labels []string) []string {
	var out []string
	for _, lbl := range labels {
		for _, tld := range targetTLDs {
			out = append(out, lbl+tld)
		}
	}
	return out
}

func filterSimilar(base string, list []string) []string {
	var out []string
	for _, d := range list {
		if jw.JaroWinklerDistance(base, d) >= similarityThreshold {
			out = append(out, d)
		}
	}
	return out
}

func generateForBase(base string) []string {
	rawTypo := ts.TypoSquat(base)
	rawHomo := hg.Homograph(base)
	rawBits := bs.Bitsquatting(base)
	rawCombo := cs.Combosquat(base)
	rawSubs := ss1.Subdomainsquat(base)
	rawSound := ss2.Soundsquat(base)

	typo := filterSimilar(base, appendTLDs(rawTypo))
	homo := filterSimilar(base, appendTLDs(rawHomo))
	bits := filterSimilar(base, appendTLDs(rawBits))
	combo := filterSimilar(base, appendTLDs(rawCombo))
	subs := filterSimilar(base, appendTLDs(rawSubs))
	sound := filterSimilar(base, appendTLDs(rawSound))

	var all []string
	all = append(all, typo...)
	all = append(all, homo...)
	all = append(all, bits...)
	all = append(all, combo...)
	all = append(all, subs...)
	all = append(all, sound...)

	return all
}

// ---------------------------------------------------
// PUBLIC API (single entry point)
// ---------------------------------------------------

func GenerateFromCSV(path string) ([]string, error) {
	keywords, err := loadKeywords(path)
	if err != nil {
		return nil, err
	}

	var all []string
	for _, base := range keywords {
		all = append(all, generateForBase(base)...)
	}

	return all, nil
}
