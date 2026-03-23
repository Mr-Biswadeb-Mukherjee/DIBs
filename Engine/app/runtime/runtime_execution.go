// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type scanRunner struct {
	rt      *appRuntime
	total   int64
	workers int

	tuner    *runtimeTuner
	cdm      CooldownManager
	pool     WorkerPool
	progress *liveProgress

	successTTL time.Duration
	failTTL    time.Duration

	submitted int64
	completed int64
	resolved  int64
	intelDone int64
	active    int64
}

func newScanRunner(rt *appRuntime, total int64) *scanRunner {
	workers := runtime.NumCPU() * 4
	return &scanRunner{
		rt:         rt,
		total:      total,
		workers:    workers,
		tuner:      newRuntimeTuner(rt.cfg, total, workers, rt.adapt, rt.limiter),
		cdm:        rt.cds.New(),
		successTTL: 48 * time.Hour,
		failTTL:    10 * time.Second,
	}
}

func (sr *scanRunner) onIntelDone() func() {
	return func() {
		atomic.AddInt64(&sr.intelDone, 1)
	}
}

func (sr *scanRunner) run(ctx context.Context, domains []string, modules *appModules) int64 {
	sr.prepare()
	cancel := sr.startAdaptiveControl(ctx)
	defer cancel()

	sr.rt.startup.Stop()
	sr.progress.Start()
	sr.processDomains(domains, modules)
	sr.shutdown(modules)
	return atomic.LoadInt64(&sr.resolved)
}

func (sr *scanRunner) prepare() {
	sr.rt.initRateLimiter(sr.total, sr.workers)
	sr.pool = newRuntimeWorkerPool(
		sr.rt.cfg,
		sr.rt.cache,
		sr.rt.workers,
		sr.workers,
		sr.tuner,
		&sr.active,
	)
	sr.progress = newLiveProgress([]*progressRow{
		generatedDomainsRow(sr.total, &sr.resolved),
		resolveProgressRow(sr.total, &sr.completed, sr.cdm),
		intelProgressRow(sr.total, &sr.intelDone),
	})
}

func (sr *scanRunner) startAdaptiveControl(parent context.Context) context.CancelFunc {
	sr.cdm.StartWatcher()
	controlCtx, cancel := context.WithCancel(parent)
	go sr.tuner.run(controlCtx, sr.cdm, sr.counters(), sr.rt.logs.app)
	return cancel
}

func (sr *scanRunner) counters() runtimeCounters {
	return runtimeCounters{
		submitted: &sr.submitted,
		completed: &sr.completed,
		active:    &sr.active,
	}
}

func newRuntimeWorkerPool(
	cfg Config,
	cache CacheStore,
	factory WorkerPoolFactory,
	workers int,
	tuner *runtimeTuner,
	active *int64,
) WorkerPool {
	timeout := resolveWorkerTimeout(tuner)
	opts := &WorkerPoolOptions{
		Timeout:         timeout,
		MaxRetries:      cfg.MaxRetries,
		AutoScale:       cfg.AutoScale,
		MinWorkers:      1,
		NonBlockingLogs: true,
		OnTaskStart:     activeStart(active),
		OnTaskFinish:    activeFinish(active),
	}
	return factory.NewWorkerPool(opts, workers, cache)
}

func resolveWorkerTimeout(tuner *runtimeTuner) time.Duration {
	timeout := tuner.resolveTimeout() * 4
	if timeout < 12*time.Second {
		return 12 * time.Second
	}
	return timeout
}

func activeStart(active *int64) func(int64) {
	return func(int64) {
		atomic.AddInt64(active, 1)
	}
}

func activeFinish(active *int64) func(int64, TaskResult) {
	return func(int64, TaskResult) {
		atomic.AddInt64(active, -1)
	}
}

func (sr *scanRunner) processDomains(domains []string, modules *appModules) {
	var wg sync.WaitGroup
	for _, domain := range domains {
		sr.submitDomain(domain, modules, &wg)
	}
	wg.Wait()
}

func (sr *scanRunner) submitDomain(domain string, modules *appModules, wg *sync.WaitGroup) {
	taskFunc := makeDomainTask(
		domain,
		modules.resolver,
		sr.rt.cache,
		sr.cdm,
		sr.rt.limiter,
		sr.tuner,
		sr.successTTL,
		sr.failTTL,
		sr.rt.logErr,
	)

	_, resCh, err := sr.pool.SubmitTask(taskFunc, Medium, 0)
	if err != nil {
		sr.handleSubmitError(domain, err, modules.intel)
		return
	}

	atomic.AddInt64(&sr.submitted, 1)
	wg.Add(1)
	go sr.awaitResult(domain, resCh, modules.intel, wg)
}

func (sr *scanRunner) handleSubmitError(domain string, err error, intelPipe *intelPipeline) {
	sr.rt.logErr("worker-submit", domain, err)
	intelPipe.WriteUnresolved(domain)
	atomic.AddInt64(&sr.completed, 1)
	atomic.AddInt64(&sr.intelDone, 1)
}

func (sr *scanRunner) awaitResult(
	domain string,
	rc <-chan TaskResult,
	intelPipe *intelPipeline,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	res, ok := <-rc
	if !ok {
		sr.markUnresolved(domain, intelPipe)
		atomic.AddInt64(&sr.completed, 1)
		return
	}

	sr.handleWorkerResult(domain, res, intelPipe)
	atomic.AddInt64(&sr.completed, 1)
}

func (sr *scanRunner) handleWorkerResult(
	domain string,
	res TaskResult,
	intelPipe *intelPipeline,
) {
	if resolved, ok := res.Result.(string); ok && resolved != "" {
		sr.markResolved(resolved, intelPipe)
	} else {
		sr.markUnresolved(domain, intelPipe)
	}

	for _, taskErr := range res.Errors {
		sr.rt.logErr("worker-task", "result", taskErr)
	}
}

func (sr *scanRunner) markResolved(domain string, intelPipe *intelPipeline) {
	atomic.AddInt64(&sr.resolved, 1)
	if intelPipe.EnqueueResolved(domain) {
		return
	}
	intelPipe.WriteResolvedFallback(domain)
	atomic.AddInt64(&sr.intelDone, 1)
}

func (sr *scanRunner) markUnresolved(domain string, intelPipe *intelPipeline) {
	intelPipe.WriteUnresolved(domain)
	atomic.AddInt64(&sr.intelDone, 1)
}

func (sr *scanRunner) shutdown(modules *appModules) {
	if sr.pool != nil {
		sr.pool.Stop()
	}
	if modules != nil && modules.intel != nil {
		if err := modules.intel.StopAndWait(); err != nil {
			sr.rt.logErr("dns-intel-pipeline", "shutdown", err)
		}
	}
	if sr.progress != nil {
		sr.progress.Stop()
		sr.progress.Finish()
	}
}
