// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"math"
	"sort"
	"strings"
	"time"
)

type intelNDJSONRecord struct {
	Domain       string   `json:"domain"`
	A            []string `json:"a,omitempty"`
	AAAA         []string `json:"aaaa,omitempty"`
	CNAME        []string `json:"cname,omitempty"`
	NS           []string `json:"ns,omitempty"`
	MX           []string `json:"mx,omitempty"`
	TXT          []string `json:"txt,omitempty"`
	Providers    []string `json:"providers,omitempty"`
	TimestampUTC string   `json:"timestamp_utc"`
}

type generatedDomainMeta struct {
	RiskScore   float64
	Confidence  string
	GeneratedBy string
}

type generatedDomainNDJSONRecord struct {
	Domain      string  `json:"domain"`
	Score       float64 `json:"score"`
	Confidence  string  `json:"confidence"`
	GeneratedBy string  `json:"generated_by"`
}

func intelRecordToNDJSON(r IntelRecord) intelNDJSONRecord {
	return intelNDJSONRecord{
		Domain:       r.Domain,
		A:            r.A,
		AAAA:         r.AAAA,
		CNAME:        r.CNAME,
		NS:           r.NS,
		MX:           r.MX,
		TXT:          r.TXT,
		Providers:    r.Providers,
		TimestampUTC: time.Now().UTC().Format(time.RFC3339),
	}
}

func makeGeneratedDomainIndex(items []GeneratedDomain) ([]string, map[string]generatedDomainMeta) {
	domains := make([]string, 0, len(items))
	index := make(map[string]generatedDomainMeta, len(items))

	for _, item := range items {
		domain := strings.ToLower(strings.TrimSpace(item.Domain))
		if domain == "" {
			continue
		}
		if _, ok := index[domain]; !ok {
			domains = append(domains, domain)
		}
		index[domain] = normalizeGeneratedMeta(generatedDomainMeta{
			RiskScore:   item.RiskScore,
			Confidence:  item.Confidence,
			GeneratedBy: item.GeneratedBy,
		})
	}
	sort.Strings(domains)
	return domains, index
}

func defaultGeneratedMeta() generatedDomainMeta {
	return generatedDomainMeta{RiskScore: 0.25, Confidence: "low", GeneratedBy: "unknown"}
}

func normalizeGeneratedMeta(meta generatedDomainMeta) generatedDomainMeta {
	fallback := defaultGeneratedMeta()
	if meta.RiskScore < 0.0 {
		meta.RiskScore = 0.0
	}
	if meta.RiskScore > 1.0 {
		meta.RiskScore = 1.0
	}
	meta.RiskScore = math.Round(meta.RiskScore*100) / 100
	if strings.TrimSpace(meta.Confidence) == "" {
		meta.Confidence = fallback.Confidence
	}
	if strings.TrimSpace(meta.GeneratedBy) == "" {
		meta.GeneratedBy = fallback.GeneratedBy
	}
	return meta
}

func unresolvedDomainRecord(domain string, meta generatedDomainMeta) generatedDomainNDJSONRecord {
	normalized := normalizeGeneratedMeta(meta)
	return generatedDomainNDJSONRecord{
		Domain:      strings.ToLower(strings.TrimSpace(domain)),
		Score:       normalized.RiskScore,
		Confidence:  normalized.Confidence,
		GeneratedBy: normalized.GeneratedBy,
	}
}

func resolvedDomainRecord(domain string, meta generatedDomainMeta) generatedDomainNDJSONRecord {
	normalized := normalizeGeneratedMeta(meta)
	return generatedDomainNDJSONRecord{
		Domain:      strings.ToLower(strings.TrimSpace(domain)),
		Score:       normalized.RiskScore,
		Confidence:  normalized.Confidence,
		GeneratedBy: normalized.GeneratedBy,
	}
}
