// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"
)

type intelNDJSONRecord struct {
	Domain               string                 `json:"domain"`
	A                    []string               `json:"a,omitempty"`
	AAAA                 []string               `json:"aaaa,omitempty"`
	CNAME                []string               `json:"cname,omitempty"`
	NS                   []string               `json:"ns,omitempty"`
	MX                   []string               `json:"mx,omitempty"`
	TXT                  []string               `json:"txt,omitempty"`
	ASNs                 []intelASNNDJSONRecord `json:"asn,omitempty"`
	Providers            []string               `json:"providers,omitempty"`
	RegistrarWhoisServer string                 `json:"registrar_whois_server,omitempty"`
	UpdatedDate          string                 `json:"updated_date,omitempty"`
	CreationDate         string                 `json:"creation_date,omitempty"`
	TimestampUTC         string                 `json:"timestamp_utc"`
}

type intelASNNDJSONRecord struct {
	IP     string `json:"ip"`
	ASN    string `json:"asn"`
	Prefix string `json:"prefix,omitempty"`
	ASName string `json:"as_name,omitempty"`
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
	Resolution  string  `json:"resolution"`
}

func closeWriters(writers ...RecordWriter) error {
	var errs []error
	for _, writer := range writers {
		if writer == nil {
			continue
		}
		if err := writer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (p *intelPipeline) generatedMeta(domain string) generatedDomainMeta {
	if p == nil {
		return defaultGeneratedMeta()
	}
	meta, err := loadGeneratedMeta(p.ctx, p.store, p.generated, domain)
	if err != nil {
		if p.logErr != nil {
			p.logErr("generated-meta-read", domain, err)
		}
		return defaultGeneratedMeta()
	}
	return normalizeGeneratedMeta(meta)
}

func (p *intelPipeline) markGeneratedDone(domain string) {
	if p == nil || p.generated == nil {
		return
	}
	domain = normalizeGeneratedDomainName(domain)
	if domain == "" {
		return
	}
	ioCtx, cancel := context.WithTimeout(p.ctx, generatedStoreIOTime)
	defer cancel()
	if err := p.generated.markDone(ioCtx, domain); err != nil && p.logErr != nil {
		p.logErr("generated-spool-mark-done", domain, err)
	}
}

func intelLookupTimeout(dnsTimeoutMS int64) time.Duration {
	if dnsTimeoutMS <= 0 {
		return 3 * time.Second
	}
	timeout := time.Duration(dnsTimeoutMS) * 6 * time.Millisecond
	if timeout < 2*time.Second {
		return 2 * time.Second
	}
	if timeout > 8*time.Second {
		return 8 * time.Second
	}
	return timeout
}

func newDNSIntelWriter(
	paths Paths,
	writers WriterFactory,
	logErr moduleErrorLogger,
) (RecordWriter, error) {
	return newNDJSONWriter(paths.DNSIntelOutput, writers, "dns-intel-writer", logErr)
}

func newGeneratedDomainWriter(
	paths Paths,
	writers WriterFactory,
	logErr moduleErrorLogger,
) (RecordWriter, error) {
	return newNDJSONWriter(paths.GeneratedOutput, writers, "generated-domain-writer", logErr)
}

func newResolvedDomainWriter(
	paths Paths,
	writers WriterFactory,
	logErr moduleErrorLogger,
) (RecordWriter, error) {
	return newNDJSONWriter(paths.ResolvedOutput, writers, "resolved-domain-writer", logErr)
}

func newClusterWriter(
	paths Paths,
	writers WriterFactory,
	logErr moduleErrorLogger,
) (RecordWriter, error) {
	return newNDJSONWriter(paths.ClusterOutput, writers, "cluster-writer", logErr)
}

func newNDJSONWriter(
	path string,
	writers WriterFactory,
	module string,
	logErr moduleErrorLogger,
) (RecordWriter, error) {
	opts := WriterOptions{
		BatchSize:  300,
		FlushEvery: time.Second,
		LogHooks: WriterLogHooks{
			OnError: func(err error) {
				if logErr != nil {
					logErr(module, "write", err)
				}
			},
		},
	}
	return writers.NewNDJSONWriter(path, opts)
}

func resetIntelQueue(store intelQueueStore) error {
	ctx, cancel := context.WithTimeout(context.Background(), intelQueueIOTime)
	defer cancel()
	return store.Delete(ctx, intelQueueKey)
}

func intelRecordToNDJSON(r IntelRecord) intelNDJSONRecord {
	return intelNDJSONRecord{
		Domain:               r.Domain,
		A:                    r.A,
		AAAA:                 r.AAAA,
		CNAME:                r.CNAME,
		NS:                   r.NS,
		MX:                   r.MX,
		TXT:                  r.TXT,
		ASNs:                 asnRecordsToNDJSON(r.ASNs),
		Providers:            r.Providers,
		RegistrarWhoisServer: r.RegistrarWhoisServer,
		UpdatedDate:          r.UpdatedDate,
		CreationDate:         r.CreationDate,
		TimestampUTC:         time.Now().UTC().Format(time.RFC3339),
	}
}

func asnRecordsToNDJSON(records []IntelASNRecord) []intelASNNDJSONRecord {
	if len(records) == 0 {
		return nil
	}
	out := make([]intelASNNDJSONRecord, 0, len(records))
	for _, record := range records {
		out = append(out, intelASNNDJSONRecord{
			IP:     strings.TrimSpace(record.IP),
			ASN:    strings.TrimSpace(record.ASN),
			Prefix: strings.TrimSpace(record.Prefix),
			ASName: strings.TrimSpace(record.ASName),
		})
	}
	return out
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
	return generatedDomainRecord(domain, meta, "unresolved")
}

func resolvedDomainRecord(domain string, meta generatedDomainMeta) generatedDomainNDJSONRecord {
	return generatedDomainRecord(domain, meta, "resolved")
}

func generatedDomainRecord(
	domain string,
	meta generatedDomainMeta,
	resolution string,
) generatedDomainNDJSONRecord {
	normalized := normalizeGeneratedMeta(meta)
	if strings.TrimSpace(resolution) == "" {
		resolution = "unresolved"
	}
	return generatedDomainNDJSONRecord{
		Domain:      strings.ToLower(strings.TrimSpace(domain)),
		Score:       normalized.RiskScore,
		Confidence:  normalized.Confidence,
		GeneratedBy: normalized.GeneratedBy,
		Resolution:  strings.ToLower(strings.TrimSpace(resolution)),
	}
}
