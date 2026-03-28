// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package dns_intel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
)

const asnWhoisAddress = "whois.cymru.com:43"

type ASNRecord struct {
	IP     string
	ASN    string
	Prefix string
	ASName string
}

type asnClient struct {
	server string
}

func NewDefaultASNLookup() ASNLookup {
	return &asnClient{server: asnWhoisAddress}
}

func (c *asnClient) Lookup(ctx context.Context, ip string) (ASNRecord, error) {
	normalizedIP := normalizeIPAddress(ip)
	if normalizedIP == "" {
		return ASNRecord{}, errors.New("asn lookup requires valid ip address")
	}

	server := strings.TrimSpace(c.server)
	if server == "" {
		return ASNRecord{}, errors.New("asn lookup server is empty")
	}

	body, err := queryWhois(ctx, server, " -v "+normalizedIP)
	if err != nil {
		return ASNRecord{}, err
	}

	record, err := parseASNRecord(body)
	if err != nil {
		return ASNRecord{}, err
	}
	if record.IP == "" {
		record.IP = normalizedIP
	}
	return record, nil
}

func (p *Processor) lookupASNs(ctx context.Context, ips []string) []ASNRecord {
	if p == nil || p.asn == nil || len(ips) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(ips))
	out := make([]ASNRecord, 0, len(ips))
	for _, ip := range ips {
		record, err := p.asn.Lookup(ctx, ip)
		if err != nil {
			continue
		}
		record = normalizeASNResult(record, ip)
		key := asnRecordKey(record)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ASN == out[j].ASN {
			return out[i].IP < out[j].IP
		}
		return out[i].ASN < out[j].ASN
	})
	return out
}

func combineIPs(ipSets ...[]string) []string {
	if len(ipSets) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	out := make([]string, 0, 8)
	for _, set := range ipSets {
		for _, ip := range set {
			normalized := normalizeIPAddress(ip)
			if normalized == "" {
				continue
			}
			if _, exists := seen[normalized]; exists {
				continue
			}
			seen[normalized] = struct{}{}
			out = append(out, normalized)
		}
	}
	return out
}

func parseASNRecord(body string) (ASNRecord, error) {
	scanner := newWhoisScanner(body)
	for scanner.Scan() {
		record, ok := parseASNLine(scanner.Text())
		if ok {
			return record, nil
		}
	}
	return ASNRecord{}, fmt.Errorf("asn record not found in response")
}

func parseASNLine(line string) (ASNRecord, bool) {
	cols := splitASNColumns(line)
	if len(cols) < 2 {
		return ASNRecord{}, false
	}

	asn := normalizeASN(cols[0])
	ip := normalizeIPAddress(cols[1])
	if asn == "" || ip == "" {
		return ASNRecord{}, false
	}

	prefix := ""
	if len(cols) > 2 && !strings.EqualFold(cols[2], "na") {
		prefix = strings.TrimSpace(cols[2])
	}

	asName := ""
	last := cols[len(cols)-1]
	if !strings.EqualFold(last, "na") {
		asName = strings.TrimSpace(last)
	}
	return ASNRecord{IP: ip, ASN: asn, Prefix: prefix, ASName: asName}, true
}

func splitASNColumns(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" || !strings.Contains(line, "|") {
		return nil
	}
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func normalizeASN(raw string) string {
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

func normalizeIPAddress(raw string) string {
	raw = strings.TrimSpace(raw)
	ip := net.ParseIP(raw)
	if ip == nil {
		return ""
	}
	return ip.String()
}

func asnRecordKey(record ASNRecord) string {
	asn := normalizeASN(record.ASN)
	ip := normalizeIPAddress(record.IP)
	if asn == "" || ip == "" {
		return ""
	}
	return asn + "|" + ip
}

func normalizeASNResult(record ASNRecord, fallbackIP string) ASNRecord {
	record.ASN = normalizeASN(record.ASN)
	record.IP = normalizeIPAddress(record.IP)
	if record.IP == "" {
		record.IP = normalizeIPAddress(fallbackIP)
	}
	record.Prefix = strings.TrimSpace(record.Prefix)
	if strings.EqualFold(record.Prefix, "na") {
		record.Prefix = ""
	}
	record.ASName = strings.TrimSpace(record.ASName)
	if strings.EqualFold(record.ASName, "na") {
		record.ASName = ""
	}
	return record
}
