// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package app

import (
	"context"

	runtime "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/runtime"
)

type appResolverBuilder func(
	cfg Config,
	dnsLog ModuleLogger,
) interface {
	Resolve(ctx context.Context, domain string) (bool, error)
}

type appDomainStreamer func(path string, sink func(GeneratedDomain) error) error

type appIntelServiceBuilder func(dnsTimeoutMS int64) DNSIntelService

type moduleFactory struct {
	newResolver    appResolverBuilder
	streamDomains  appDomainStreamer
	newDNSIntelSVC appIntelServiceBuilder
}

func newModuleFactory(
	resolver appResolverBuilder,
	streamer appDomainStreamer,
	intelSVC appIntelServiceBuilder,
) runtime.ModuleFactory {
	return moduleFactory{
		newResolver:    resolver,
		streamDomains:  streamer,
		newDNSIntelSVC: intelSVC,
	}
}

func (m moduleFactory) NewResolver(cfg runtime.Config, dnsLog runtime.ModuleLogger) runtime.DNSResolver {
	appCfg := Config{
		UpstreamDNS:  cfg.UpstreamDNS,
		BackupDNS:    cfg.BackupDNS,
		DNSRetries:   cfg.DNSRetries,
		DNSTimeoutMS: cfg.DNSTimeoutMS,
	}
	return m.newResolver(appCfg, toAppLogger(dnsLog, nil))
}

func (m moduleFactory) StreamGeneratedDomains(
	path string,
	sink runtime.GeneratedDomainSink,
) error {
	if m.streamDomains == nil {
		return nil
	}
	return m.streamDomains(path, func(item GeneratedDomain) error {
		return sink(runtime.GeneratedDomain{
			Domain:      item.Domain,
			RiskScore:   item.RiskScore,
			Confidence:  item.Confidence,
			GeneratedBy: item.GeneratedBy,
		})
	})
}

func (m moduleFactory) NewDNSIntelService(dnsTimeoutMS int64) runtime.DNSIntelService {
	return runtimeIntelServiceAdapter{inner: m.newDNSIntelSVC(dnsTimeoutMS)}
}

type runtimeIntelServiceAdapter struct {
	inner DNSIntelService
}

func (a runtimeIntelServiceAdapter) Run(
	ctx context.Context,
	domains []runtime.IntelDomain,
) ([]runtime.IntelRecord, error) {
	if a.inner == nil {
		return nil, nil
	}

	in := make([]IntelDomain, 0, len(domains))
	for _, d := range domains {
		in = append(in, IntelDomain{Name: d.Name})
	}

	records, err := a.inner.Run(ctx, in)
	if err != nil {
		return nil, err
	}

	out := make([]runtime.IntelRecord, 0, len(records))
	for _, rec := range records {
		out = append(out, runtime.IntelRecord{
			Domain:               rec.Domain,
			A:                    rec.A,
			AAAA:                 rec.AAAA,
			CNAME:                rec.CNAME,
			NS:                   rec.NS,
			MX:                   rec.MX,
			TXT:                  rec.TXT,
			Providers:            rec.Providers,
			RegistrarWhoisServer: rec.RegistrarWhoisServer,
			UpdatedDate:          rec.UpdatedDate,
			CreationDate:         rec.CreationDate,
		})
	}
	return out, nil
}
