// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package app

import (
	"time"

	dns "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/DNS"
	recon "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/Recon"
	"github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/intel"
)

type GeneratedDomain struct {
	Domain      string
	RiskScore   float64
	Confidence  string
	GeneratedBy string
}

func DNSConfig(cfg Config) dns.Config {
	return dns.Config{
		Upstream:  cfg.UpstreamDNS,
		Backup:    cfg.BackupDNS,
		Retries:   int(cfg.DNSRetries),
		TimeoutMS: cfg.DNSTimeoutMS,
	}
}

func NewResolver(cfg Config, dnsLog ModuleLogger) recon.DNS {
	dnsCfg := DNSConfig(cfg)
	return recon.New(
		dnsCfg.Upstream,
		dnsCfg.Backup,
		dnsCfg.Retries,
		int(dnsCfg.TimeoutMS),
		dnsLog,
	)
}

func GenerateDomains(path string) ([]GeneratedDomain, error) {
	scored, err := recon.GenerateScoredDomains(path)
	if err != nil {
		return nil, err
	}
	out := make([]GeneratedDomain, 0, len(scored))
	for _, item := range scored {
		out = append(out, GeneratedDomain{
			Domain:      item.Domain,
			RiskScore:   item.RiskScore,
			Confidence:  item.Confidence,
			GeneratedBy: item.GeneratedBy,
		})
	}
	return out, nil
}

func NewDNSIntelService(dnsTimeoutMS int64) *intel.DNSIntelService {
	return intel.NewDefaultDNSIntelService(1, DNSIntelLookupTimeout(dnsTimeoutMS))
}

func DNSIntelLookupTimeout(dnsTimeoutMS int64) time.Duration {
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
