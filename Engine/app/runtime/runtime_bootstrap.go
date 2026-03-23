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
	cache       CacheStore
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

	rt := &appRuntime{
		cfg:         deps.Config,
		paths:       deps.Paths,
		started:     deps.Startup.Start("Starting Infermal_v2 Engine"),
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
	switch {
	case deps.Startup == nil:
		return errors.New("app startup dependency is required")
	case deps.Logs.App == nil || deps.Logs.DNS == nil || deps.Logs.RateLimiter == nil:
		return errors.New("app loggers are required")
	case deps.Cache == nil:
		return errors.New("app cache dependency is required")
	case deps.Limiter == nil:
		return errors.New("app limiter dependency is required")
	case deps.InitLimiter == nil:
		return errors.New("app limiter init dependency is required")
	case deps.WorkerPools == nil:
		return errors.New("app worker pool factory is required")
	case deps.Cooldowns == nil:
		return errors.New("app cooldown factory is required")
	case deps.Adaptive == nil:
		return errors.New("app adaptive factory is required")
	case deps.Writers == nil:
		return errors.New("app writer factory is required")
	case deps.Modules == nil:
		return errors.New("app module factory is required")
	case deps.Paths.KeywordsCSV == "":
		return errors.New("keywords path is required")
	case deps.Paths.DNSIntelOutput == "" || deps.Paths.GeneratedOutput == "":
		return errors.New("output paths are required")
	default:
		return nil
	}
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
	return errors.Join(rt.cache.Close(), rt.logs.Close())
}

func (logs runtimeLogs) Close() error {
	return errors.Join(logs.app.Close(), logs.dns.Close(), logs.rateLimiter.Close())
}

func (rt *appRuntime) finishRun(total, resolved int64) {
	rt.startup.Finish(rt.started, total, resolved)
	fmt.Printf("✔ Generated domains written to %s\n", rt.paths.GeneratedOutput)
	fmt.Printf("✔ DNS intel written to %s\n", rt.paths.DNSIntelOutput)
}

func (rt *appRuntime) initRateLimiter(total int64, workers int) {
	initialRate := seedRateLimit(rt.cfg.RateLimit, total, workers)
	rt.initLimiter(rt.cache, time.Second, initialRate, rt.logs.rateLimiter)
}

func (rt *appRuntime) newModules(
	ctx context.Context,
	generated map[string]generatedDomainMeta,
	onIntelDone func(),
) (*appModules, error) {
	intelPipe, err := newIntelPipeline(
		ctx,
		rt.cache,
		rt.modules.NewDNSIntelService(rt.cfg.DNSTimeoutMS),
		rt.writers,
		rt.paths,
		generated,
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
	path string,
	modules ModuleFactory,
) ([]string, map[string]generatedDomainMeta, error) {
	scored, err := modules.GenerateDomains(path)
	if err != nil {
		return nil, nil, err
	}
	domains, meta := makeGeneratedDomainIndex(scored)
	return domains, meta, nil
}
