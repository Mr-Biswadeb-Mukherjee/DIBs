// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"net"
	"sort"
	"strings"
	"time"
)

type clusterNDJSONRecord struct {
	ASN         string   `json:"asn"`
	IP          string   `json:"ip"`
	Prefix      string   `json:"prefix,omitempty"`
	ASName      string   `json:"as_name,omitempty"`
	Domains     []string `json:"domains"`
	ClusterSize int      `json:"cluster_size"`
	Timestamp   string   `json:"timestamp_ist"`
}

type asnClusterIndex struct {
	groups map[string]*asnClusterGroup
}

type asnClusterGroup struct {
	ASN     string
	IP      string
	Prefix  string
	ASName  string
	Domains map[string]struct{}
}

func newASNClusterIndex() *asnClusterIndex {
	return &asnClusterIndex{groups: make(map[string]*asnClusterGroup)}
}

func (p *intelPipeline) writeClusterRecords(record IntelRecord, fallbackDomain string) {
	if p == nil || p.clusterWriter == nil || p.clusterIndex == nil {
		return
	}
	domain := normalizeClusterDomain(record.Domain, fallbackDomain)
	if domain == "" {
		return
	}
	rows := p.clusterIndex.add(domain, record.ASNs)
	for _, row := range rows {
		p.clusterWriter.WriteRecord(row)
	}
}

func normalizeClusterDomain(primary, fallback string) string {
	domain := strings.ToLower(strings.TrimSpace(primary))
	if domain != "" {
		return domain
	}
	return strings.ToLower(strings.TrimSpace(fallback))
}

func (idx *asnClusterIndex) add(domain string, records []IntelASNRecord) []clusterNDJSONRecord {
	if idx == nil || domain == "" || len(records) == 0 {
		return nil
	}

	out := make([]clusterNDJSONRecord, 0, len(records))
	for _, record := range records {
		key, normalized, ok := normalizeClusterRecord(record)
		if !ok {
			continue
		}
		group := idx.ensureGroup(key, normalized)
		if _, exists := group.Domains[domain]; exists {
			continue
		}
		group.Domains[domain] = struct{}{}
		if len(group.Domains) < 2 {
			continue
		}
		out = append(out, buildClusterRecord(group))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ASN == out[j].ASN {
			return out[i].IP < out[j].IP
		}
		return out[i].ASN < out[j].ASN
	})
	return out
}

func (idx *asnClusterIndex) ensureGroup(key string, record IntelASNRecord) *asnClusterGroup {
	group, exists := idx.groups[key]
	if exists {
		if group.Prefix == "" {
			group.Prefix = record.Prefix
		}
		if group.ASName == "" {
			group.ASName = record.ASName
		}
		return group
	}

	group = &asnClusterGroup{
		ASN:     record.ASN,
		IP:      record.IP,
		Prefix:  record.Prefix,
		ASName:  record.ASName,
		Domains: make(map[string]struct{}),
	}
	idx.groups[key] = group
	return group
}

func normalizeClusterRecord(record IntelASNRecord) (string, IntelASNRecord, bool) {
	asn := normalizeClusterASN(record.ASN)
	ip := normalizeClusterIP(record.IP)
	if asn == "" || ip == "" {
		return "", IntelASNRecord{}, false
	}

	normalized := IntelASNRecord{
		IP:     ip,
		ASN:    asn,
		Prefix: strings.TrimSpace(record.Prefix),
		ASName: strings.TrimSpace(record.ASName),
	}
	return asn + "|" + ip, normalized, true
}

func normalizeClusterASN(raw string) string {
	raw = strings.TrimSpace(strings.ToUpper(raw))
	raw = strings.TrimPrefix(raw, "AS")
	if raw == "" {
		return ""
	}
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return ""
		}
	}
	return "AS" + raw
}

func normalizeClusterIP(raw string) string {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return ""
	}
	return ip.String()
}

func buildClusterRecord(group *asnClusterGroup) clusterNDJSONRecord {
	domains := sortedClusterDomains(group.Domains)
	return clusterNDJSONRecord{
		ASN:         group.ASN,
		IP:          group.IP,
		Prefix:      group.Prefix,
		ASName:      group.ASName,
		Domains:     domains,
		ClusterSize: len(domains),
		Timestamp:   formatISTTimestamp(time.Now()),
	}
}

func sortedClusterDomains(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for domain := range values {
		out = append(out, domain)
	}
	sort.Strings(out)
	return out
}
