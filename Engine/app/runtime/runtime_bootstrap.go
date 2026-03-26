// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

const maxModuleErrorLogs = 25

type runtimeLogs struct {
	app         ModuleLogger
	dns         ModuleLogger
	rateLimiter ModuleLogger
}

type appRuntime struct {
	cfg         Config
	paths       Paths
	started     time.Time
	qpsHistory  string
	cache       CacheStore
	generated   *generatedDomainSpool
	limiter     RateLimiter
	initLimiter LimiterInitFunc
	workers     WorkerPoolFactory
	cds         CooldownFactory
	adapt       AdaptiveFactory
	writers     WriterFactory
	modules     ModuleFactory
	startup     Startup
	printLine   func(args ...any)
	logs        runtimeLogs
	logErr      moduleErrorLogger
}

type appModules struct {
	resolver DNSResolver
	intel    *intelPipeline
}

func newAppRuntime(deps Dependencies) (*appRuntime, error) {
	if err := validateDependencies(deps); err != nil {
		return nil, err
	}

	startedAt := deps.Startup.Start("Starting Infermal_v2 Engine")
	rt := &appRuntime{
		cfg:         deps.Config,
		paths:       deps.Paths,
		started:     startedAt,
		qpsHistory:  qpsHistoryOutputPath(deps.Paths.RunMetricsOutput, startedAt),
		cache:       deps.Cache,
		limiter:     deps.Limiter,
		initLimiter: deps.InitLimiter,
		workers:     deps.WorkerPools,
		cds:         deps.Cooldowns,
		adapt:       deps.Adaptive,
		writers:     deps.Writers,
		modules:     deps.Modules,
		startup:     deps.Startup,
		printLine:   deps.PrintLine,
		logs: runtimeLogs{
			app:         deps.Logs.App,
			dns:         deps.Logs.DNS,
			rateLimiter: deps.Logs.RateLimiter,
		},
	}
	rt.logErr = newModuleErrorLogger(rt.logs.app)
	return rt, nil
}

func validateDependencies(deps Dependencies) error {
	for _, check := range dependencyChecks {
		if check.ok(deps) {
			continue
		}
		return errors.New(check.err)
	}
	return nil
}

type dependencyCheck struct {
	ok  func(Dependencies) bool
	err string
}

var dependencyChecks = []dependencyCheck{
	{ok: func(d Dependencies) bool { return d.Startup != nil }, err: "app startup dependency is required"},
	{ok: hasAppLoggers, err: "app loggers are required"},
	{ok: func(d Dependencies) bool { return d.Cache != nil }, err: "app cache dependency is required"},
	{ok: func(d Dependencies) bool { return d.Limiter != nil }, err: "app limiter dependency is required"},
	{ok: func(d Dependencies) bool { return d.InitLimiter != nil }, err: "app limiter init dependency is required"},
	{ok: func(d Dependencies) bool { return d.WorkerPools != nil }, err: "app worker pool factory is required"},
	{ok: func(d Dependencies) bool { return d.Cooldowns != nil }, err: "app cooldown factory is required"},
	{ok: func(d Dependencies) bool { return d.Adaptive != nil }, err: "app adaptive factory is required"},
	{ok: func(d Dependencies) bool { return d.Writers != nil }, err: "app writer factory is required"},
	{ok: func(d Dependencies) bool { return d.Modules != nil }, err: "app module factory is required"},
	{ok: func(d Dependencies) bool { return d.Paths.KeywordsCSV != "" }, err: "keywords path is required"},
	{ok: hasOutputPaths, err: "output paths are required"},
}

func hasAppLoggers(deps Dependencies) bool {
	return deps.Logs.App != nil && deps.Logs.DNS != nil && deps.Logs.RateLimiter != nil
}

func hasOutputPaths(deps Dependencies) bool {
	return deps.Paths.DNSIntelOutput != "" &&
		deps.Paths.GeneratedOutput != "" &&
		deps.Paths.ResolvedOutput != "" &&
		deps.Paths.RunMetricsOutput != ""
}

func newModuleErrorLogger(appLog ModuleLogger) moduleErrorLogger {
	var count int64
	return func(module, scope string, err error) {
		if err == nil || appLog == nil {
			return
		}
		if atomic.AddInt64(&count, 1) > maxModuleErrorLogs {
			return
		}
		appLog.Warning("%s error scope=%s err=%v", module, scope, err)
	}
}

func (rt *appRuntime) Close() error {
	if rt == nil {
		return nil
	}

	rt.startup.Stop()
	return errors.Join(rt.cache.Close(), closeGeneratedSpool(rt.generated), rt.logs.Close())
}

func (logs runtimeLogs) Close() error {
	return errors.Join(logs.app.Close(), logs.dns.Close(), logs.rateLimiter.Close())
}

func (rt *appRuntime) finishRun(total, resolved int64) {
	finishedAt := time.Now()
	rt.startup.Finish(rt.started, total, resolved)
	if err := rt.writeRunMetrics(total, resolved, finishedAt); err != nil {
		rt.logErr("run-metrics", rt.paths.RunMetricsOutput, err)
	}
	fmt.Printf("✔ Generated domains written to %s\n", rt.paths.GeneratedOutput)
	fmt.Printf("✔ Resolved domains written to %s\n", rt.paths.ResolvedOutput)
	fmt.Printf("✔ DNS intel written to %s\n", rt.paths.DNSIntelOutput)
	fmt.Printf("✔ Run metrics written to %s\n", rt.paths.RunMetricsOutput)
	fmt.Printf("✔ QPS history written to %s\n", rt.qpsHistory)
}

func (rt *appRuntime) initRateLimiter(total int64, workers int) {
	initialRate := seedRateLimit(rt.cfg.RateLimit, total, workers, rt.cfg.RateLimitCeiling)
	rt.initLimiter(rt.cache, time.Second, initialRate, rt.logs.rateLimiter)
}

func (rt *appRuntime) newModules(
	ctx context.Context,
	onIntelDone func(),
) (*appModules, error) {
	intelPipe, err := newIntelPipeline(
		ctx,
		rt.cache,
		rt.generated,
		rt.modules.NewDNSIntelService(rt.cfg.DNSTimeoutMS),
		rt.writers,
		rt.paths,
		rt.logErr,
		onIntelDone,
	)
	if err != nil {
		return nil, err
	}

	return &appModules{
		resolver: rt.modules.NewResolver(rt.cfg, rt.logs.dns),
		intel:    intelPipe,
	}, nil
}

func loadGeneratedDomains(
	ctx context.Context,
	generatedOutput string,
	path string,
	modules ModuleFactory,
	store CacheStore,
) (int64, *generatedDomainSpool, error) {
	total, spool, err := streamGeneratedDomainsToSpool(ctx, path, modules, store, generatedOutput)
	if err != nil {
		return 0, nil, err
	}
	return total, spool, nil
}

func closeGeneratedSpool(spool *generatedDomainSpool) error {
	if spool == nil {
		return nil
	}
	return spool.Close()
}
