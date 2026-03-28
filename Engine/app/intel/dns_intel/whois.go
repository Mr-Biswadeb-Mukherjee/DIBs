// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package dns_intel

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const (
	whoisBootstrapAddress = "whois.iana.org:43"
	whoisDefaultPort      = "43"
	whoisResponseLimit    = 512 * 1024
)

type WhoisRecord struct {
	RegistrarWhoisServer string
	UpdatedDate          string
	CreationDate         string
}

type whoisClient struct {
	bootstrapAddress string
}

func NewDefaultWhoisLookup() WhoisLookup {
	return &whoisClient{bootstrapAddress: whoisBootstrapAddress}
}

func (c *whoisClient) Lookup(ctx context.Context, domain string) (WhoisRecord, error) {
	domain = normalizeWhoisDomain(domain)
	if domain == "" {
		return WhoisRecord{}, errors.New("whois lookup requires domain")
	}

	server, err := c.lookupServer(ctx, domain)
	if err != nil {
		return WhoisRecord{}, err
	}
	body, err := queryWhois(ctx, server, domain)
	if err != nil {
		return WhoisRecord{}, err
	}

	record := parseWhoisRecord(body)
	if record.RegistrarWhoisServer == "" {
		record.RegistrarWhoisServer = trimWhoisServer(server)
	}
	return record, nil
}

func (c *whoisClient) lookupServer(ctx context.Context, domain string) (string, error) {
	tld, err := domainTLD(domain)
	if err != nil {
		return "", err
	}
	body, err := queryWhois(ctx, c.bootstrapAddress, tld)
	if err != nil {
		return "", err
	}
	server := parseReferralServer(body)
	if server == "" {
		return "", fmt.Errorf("whois referral not found for %s", domain)
	}
	return server, nil
}

func domainTLD(domain string) (string, error) {
	idx := strings.LastIndex(domain, ".")
	if idx <= 0 || idx == len(domain)-1 {
		return "", fmt.Errorf("invalid domain %q", domain)
	}
	return domain[idx+1:], nil
}

func queryWhois(ctx context.Context, server, query string) (string, error) {
	address := withWhoisPort(server)
	if address == "" {
		return "", errors.New("whois server is empty")
	}

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return "", err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	if _, err := io.WriteString(conn, query+"\r\n"); err != nil {
		return "", err
	}

	payload, err := io.ReadAll(io.LimitReader(conn, whoisResponseLimit))
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func withWhoisPort(server string) string {
	server = trimWhoisServer(server)
	if server == "" {
		return ""
	}
	if _, _, err := net.SplitHostPort(server); err == nil {
		return server
	}
	return net.JoinHostPort(server, whoisDefaultPort)
}

func parseReferralServer(body string) string {
	scanner := newWhoisScanner(body)
	for scanner.Scan() {
		key, value, ok := splitWhoisLine(scanner.Text())
		if !ok {
			continue
		}
		key = normalizeWhoisKey(key)
		if key == "refer" || key == "whois" {
			return trimWhoisServer(value)
		}
	}
	return ""
}

func parseWhoisRecord(body string) WhoisRecord {
	var record WhoisRecord
	scanner := newWhoisScanner(body)
	for scanner.Scan() {
		key, value, ok := splitWhoisLine(scanner.Text())
		if !ok {
			continue
		}
		switch normalizeWhoisKey(key) {
		case "registrar whois server", "whois server":
			if record.RegistrarWhoisServer == "" {
				record.RegistrarWhoisServer = trimWhoisServer(value)
			}
		case "updated date", "last updated on", "last modified":
			if record.UpdatedDate == "" {
				record.UpdatedDate = normalizeWhoisDate(value)
			}
		case "creation date", "created on", "created date", "registration time":
			if record.CreationDate == "" {
				record.CreationDate = normalizeWhoisDate(value)
			}
		}
	}
	return record
}

func splitWhoisLine(line string) (string, string, bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "%") {
		return "", "", false
	}
	idx := strings.Index(line, ":")
	if idx <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:idx])
	value := cleanWhoisValue(line[idx+1:])
	if key == "" || value == "" {
		return "", "", false
	}
	return key, value, true
}

func cleanWhoisValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"")
	return strings.TrimSpace(value)
}

func normalizeWhoisKey(key string) string {
	key = strings.TrimSpace(strings.ToLower(key))
	return strings.Join(strings.Fields(key), " ")
}

func normalizeWhoisDate(raw string) string {
	raw = cleanWhoisValue(raw)
	if raw == "" {
		return ""
	}
	for _, layout := range whoisDateLayouts() {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed.UTC().Format("2006-01-02")
		}
	}
	if len(raw) >= 10 {
		prefix := raw[:10]
		if _, err := time.Parse("2006-01-02", prefix); err == nil {
			return prefix
		}
	}
	return raw
}

func whoisDateLayouts() []string {
	return []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
		"2006.01.02 15:04:05",
		"2006.01.02",
	}
}

func normalizeWhoisDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	return strings.TrimSuffix(domain, ".")
}

func trimWhoisServer(server string) string {
	server = normalizeWhoisDomain(server)
	server = strings.TrimPrefix(server, "whois://")
	server = strings.TrimPrefix(server, "http://")
	server = strings.TrimPrefix(server, "https://")
	return strings.TrimSuffix(server, ".")
}

func newWhoisScanner(body string) *bufio.Scanner {
	scanner := bufio.NewScanner(strings.NewReader(body))
	scanner.Buffer(make([]byte, 0, 64*1024), whoisResponseLimit)
	return scanner
}
