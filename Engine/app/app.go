// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package app

import (
	"context"
	"fmt"
	"time"

	dns "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/DNS"
	recon "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/Recon"
	"github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/intel"
	runtime "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/runtime"
)

type GeneratedDomain struct {
	Domain      string
	RiskScore   float64
	Confidence  string
	GeneratedBy string
}

type IntelDomain struct {
	Name string
}

type IntelRecord struct {
	Domain    string
	A         []string
	AAAA      []string
	CNAME     []string
	NS        []string
	MX        []string
	TXT       []string
	Providers []string
}

type DNSIntelService interface {
	Run(ctx context.Context, domains []IntelDomain) ([]IntelRecord, error)
}

var runtimeRun = func(ctx context.Context, deps runtime.Dependencies) error {
	return runtime.Run(ctx, deps)
}

var runtimePrintLine = func(args ...any) {
	fmt.Println(args...)
}

func Run(ctx context.Context, deps Dependencies) error {
	return runtimeRun(normalizeContext(ctx), buildRuntimeDependencies(deps))
}

func normalizeContext(ctx context.Context) context.Context {
	if ctx != nil {
		return ctx
	}
	return context.Background()
}

func buildRuntimeDependencies(deps Dependencies) runtime.Dependencies {
	return runtime.Dependencies{
		Config:      toRuntimeConfig(deps.Config),
		Paths:       toRuntimePaths(deps.Paths),
		Startup:     deps.Startup,
		Logs:        toRuntimeLogs(deps.Logs),
		Cache:       deps.Cache,
		Limiter:     deps.Limiter,
		InitLimiter: initRuntimeLimiter(deps),
		WorkerPools: workerPoolFactoryAdapter{inner: deps.WorkerPools},
		Cooldowns:   cooldownFactoryAdapter{inner: deps.Cooldowns},
		Adaptive:    adaptiveFactoryAdapter{inner: deps.Adaptive},
		Writers:     writerFactoryAdapter{inner: deps.Writers},
		Modules:     newRuntimeModuleFactory(),
		PrintLine:   runtimePrintLine,
	}
}

func toRuntimeConfig(cfg Config) runtime.Config {
	return runtime.Config{
		RateLimit:        cfg.RateLimit,
		RateLimitCeiling: cfg.RateLimitCeiling,
		TimeoutSeconds:   cfg.TimeoutSeconds,
		MaxRetries:       cfg.MaxRetries,
		AutoScale:        cfg.AutoScale,
		UpstreamDNS:      cfg.UpstreamDNS,
		BackupDNS:        cfg.BackupDNS,
		DNSRetries:       cfg.DNSRetries,
		DNSTimeoutMS:     cfg.DNSTimeoutMS,
	}
}

func toRuntimePaths(paths Paths) runtime.Paths {
	return runtime.Paths{
		KeywordsCSV:     paths.KeywordsCSV,
		DNSIntelOutput:  paths.DNSIntelOutput,
		GeneratedOutput: paths.GeneratedOutput,
	}
}

func toRuntimeLogs(logs LogSet) runtime.LogSet {
	return runtime.LogSet{
		App:         logs.App,
		DNS:         logs.DNS,
		RateLimiter: logs.RateLimiter,
	}
}

func initRuntimeLimiter(deps Dependencies) runtime.LimiterInitFunc {
	return func(_ runtime.CacheStore, window time.Duration, maxHits int64, logger runtime.ModuleLogger) {
		deps.Limiter.Init(deps.Cache, window, maxHits, toAppLogger(logger, deps.Logs.RateLimiter))
	}
}

func newRuntimeModuleFactory() runtime.ModuleFactory {
	return newModuleFactory(
		func(cfg Config, dnsLog ModuleLogger) interface {
			Resolve(ctx context.Context, domain string) (bool, error)
		} {
			return NewResolver(cfg, dnsLog)
		},
		GenerateDomains,
		NewDNSIntelService,
	)
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
	engine := dns.New(dnsCfg, dnsLog)
	return recon.New(engine)
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

func NewDNSIntelService(dnsTimeoutMS int64) DNSIntelService {
	svc := intel.NewDefaultDNSIntelService(1, DNSIntelLookupTimeout(dnsTimeoutMS))
	return intelServiceAdapter{inner: svc}
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

type intelServiceAdapter struct {
	inner *intel.DNSIntelService
}

func (a intelServiceAdapter) Run(ctx context.Context, domains []IntelDomain) ([]IntelRecord, error) {
	if a.inner == nil {
		return nil, nil
	}

	in := make([]intel.Domain, 0, len(domains))
	for _, domain := range domains {
		in = append(in, intel.Domain{Name: domain.Name})
	}

	records, err := a.inner.Run(ctx, in)
	if err != nil {
		return nil, err
	}
	return mapIntelRecords(records), nil
}

func mapIntelRecords(records []intel.Record) []IntelRecord {
	out := make([]IntelRecord, 0, len(records))
	for _, rec := range records {
		out = append(out, IntelRecord{
			Domain:    rec.Domain,
			A:         rec.A,
			AAAA:      rec.AAAA,
			CNAME:     rec.CNAME,
			NS:        rec.NS,
			MX:        rec.MX,
			TXT:       rec.TXT,
			Providers: rec.Providers,
		})
	}
	return out
}
