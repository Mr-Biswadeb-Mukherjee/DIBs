package domain_generator

import (
    "encoding/csv"
    "fmt"
    "os"
    "strings"

    ts "github.com/official-biswadeb941/Infermal_v2/Modules/Domain_Generator/Typo_squat"
    hg "github.com/official-biswadeb941/Infermal_v2/Modules/Domain_Generator/Homograph"
)

var targetTLDs = []string{
    ".in", // You can add more later
}

// sanitizeKeyword removes noise like spaces, punctuation, and stray characters.
func sanitizeKeyword(s string) string {
    s = strings.TrimSpace(s)
    s = strings.ReplaceAll(s, " ", "")

    // Remove unwanted punctuation
    remove := []string{".", ",", "/", "\\", ":", ";", "'", "\"", "?", "!", "(", ")", "[", "]", "{", "}", "|"}
    for _, r := range remove {
        s = strings.ReplaceAll(s, r, "")
    }

    return s
}

// LoadKeywordsCSV loads the first column of a CSV and applies sanitization.
// Header row is automatically skipped.
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

        // Skip CSV header
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

// appendTLDs attaches all TLDs to each SLD label.
func appendTLDs(labels []string) []string {
    var out []string
    for _, label := range labels {
        for _, tld := range targetTLDs {
            out = append(out, label+tld)
        }
    }
    return out
}

// RunAll generates full domain variants for one base SLD.
func RunAll(base string) map[string][]string {
    rawTypo := ts.GenerateTypoSquatLabels(base)
    rawHomo := hg.GenerateHomographLabels(base)

    return map[string][]string{
        "typo_squat": appendTLDs(rawTypo),
        "homograph":  appendTLDs(rawHomo),
    }
}
