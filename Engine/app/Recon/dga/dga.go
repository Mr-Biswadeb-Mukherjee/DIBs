// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package domain_generator

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	bs "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/Recon/dga/bitsquatting"
	cs "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/Recon/dga/combo_squat"
	hg "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/Recon/dga/homograph"
	jw "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/Recon/dga/jarowinkler"
	ss2 "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/Recon/dga/sound_squat"
	ss1 "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/Recon/dga/subdomain_squat"
	ts "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/Recon/dga/typo_squat"
)

const (
	similarityThreshold = 0.75
	defaultSettingsPath = "Setting/setting.conf"
)

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

func appendTLDs(labels, tlds []string) []string {
	var out []string
	for _, lbl := range labels {
		for _, tld := range tlds {
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

func generateForBase(base string, tlds []string) []string {
	rawTypo := ts.TypoSquat(base)
	rawHomo := hg.Homograph(base)
	rawBits := bs.Bitsquatting(base)
	rawCombo := cs.Combosquat(base)
	rawSubs := ss1.Subdomainsquat(base)
	rawSound := ss2.Soundsquat(base)

	typo := filterSimilar(base, appendTLDs(rawTypo, tlds))
	homo := filterSimilar(base, appendTLDs(rawHomo, tlds))
	bits := filterSimilar(base, appendTLDs(rawBits, tlds))
	combo := filterSimilar(base, appendTLDs(rawCombo, tlds))
	subs := filterSimilar(base, appendTLDs(rawSubs, tlds))
	sound := filterSimilar(base, appendTLDs(rawSound, tlds))

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
	tlds := loadTargetTLDs(resolveSettingsPath())

	var all []string
	for _, base := range keywords {
		all = append(all, generateForBase(base, tlds)...)
	}

	return all, nil
}

func resolveSettingsPath() string {
	if root := findModuleRoot(callerSettingsDir()); root != "" {
		return filepath.Join(root, defaultSettingsPath)
	}

	wd, err := os.Getwd()
	if err == nil {
		if root := findModuleRoot(wd); root != "" {
			return filepath.Join(root, defaultSettingsPath)
		}
	}
	return defaultSettingsPath
}

func callerSettingsDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Dir(file)
}

func findModuleRoot(start string) string {
	dir := start
	for dir != "" {
		if hasGoMod(dir) {
			return dir
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
	return ""
}

func hasGoMod(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "go.mod"))
	return err == nil
}
