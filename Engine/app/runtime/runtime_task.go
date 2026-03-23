// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"context"
	"time"
)

type moduleErrorLogger func(module, scope string, err error)

func makeDomainTask(
	domain string,
	resolver DNSResolver,
	cache CacheStore,
	cdm CooldownManager,
	limiter RateLimiter,
	tuner *runtimeTuner,
	successTTL, failTTL time.Duration,
	logErr moduleErrorLogger,
) TaskFunc {
	return func(ctx context.Context) (interface{}, []string, []string, []error) {
		if val, hit := readDomainCache(ctx, cache, domain); hit {
			if val {
				return domain, nil, nil, nil
			}
			return nil, nil, nil, nil
		}

		if errs := waitForCooldown(ctx, cdm); len(errs) > 0 {
			return nil, nil, nil, errs
		}

		denied, errs := waitForRateLimit(ctx, domain, limiter, tuner, logErr)
		if len(errs) > 0 {
			return nil, nil, nil, errs
		}

		ok, resolveErr, latency := resolveDomain(ctx, domain, resolver, tuner)
		tuner.observeTask(latency, resolveErr, denied)
		logErr("dns", domain, resolveErr)

		if ok {
			writeCache(context.Background(), cache, domain, "1", successTTL, logErr)
			return domain, nil, nil, nil
		}

		writeCache(context.Background(), cache, domain, "0", failTTL, logErr)
		return nil, nil, nil, nil
	}
}

func readDomainCache(ctx context.Context, cache CacheStore, domain string) (bool, bool) {
	cacheCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()

	cached, err := cache.GetValue(cacheCtx, "dns:"+domain)
	if err != nil {
		return false, false
	}
	if cached == "1" {
		return true, true
	}
	if cached == "0" {
		return false, true
	}
	return false, false
}

func waitForCooldown(ctx context.Context, cdm CooldownManager) []error {
	if !cdm.Active() {
		return nil
	}

	select {
	case <-ctx.Done():
		return []error{ctx.Err()}
	case <-cdm.Gate():
		return nil
	}
}

func waitForRateLimit(
	ctx context.Context,
	domain string,
	limiter RateLimiter,
	tuner *runtimeTuner,
	logErr moduleErrorLogger,
) (int64, []error) {
	var denied int64
	for {
		select {
		case <-ctx.Done():
			return denied, []error{ctx.Err()}
		default:
		}

		allowed, err := limiter.Allow(ctx, "dns-rate")
		if err != nil {
			logErr("ratelimiter", domain, err)
			tuner.observeLimiterError()
			return denied, nil
		}
		if allowed {
			return denied, nil
		}

		denied++
		time.Sleep(10 * time.Millisecond)
	}
}

func resolveDomain(
	ctx context.Context,
	domain string,
	resolver DNSResolver,
	tuner *runtimeTuner,
) (bool, error, time.Duration) {
	resolveTimeout := tuner.resolveTimeout()
	startedAt := time.Now()
	resolveCtx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()

	ok, resolveErr := resolver.Resolve(resolveCtx, domain)
	return ok, resolveErr, time.Since(startedAt)
}

func writeCache(
	ctx context.Context,
	cache CacheStore,
	domain, value string,
	ttl time.Duration,
	logErr moduleErrorLogger,
) {
	if err := cache.SetValue(ctx, "dns:"+domain, value, ttl); err != nil {
		logErr("redis-cache-write", domain, err)
	}
}
