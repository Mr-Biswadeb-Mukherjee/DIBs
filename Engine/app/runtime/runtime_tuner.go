// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package runtime

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"time"
)

type runtimeCounters struct {
	submitted *int64
	completed *int64
	active    *int64
}

type runtimeLogger interface {
	Info(format string, v ...interface{})
	Warning(format string, v ...interface{})
}

type runtimeTuner struct {
	controller   AdaptiveController
	limiter      RateLimiter
	timeoutNanos atomic.Int64
	logTick      atomic.Int64
}

func newRuntimeTuner(
	cfg Config,
	total int64,
	workers int,
	factory AdaptiveFactory,
	limiter RateLimiter,
) *runtimeTuner {
	seedRate := seedRateLimit(cfg.RateLimit, total, workers)
	seedTimeout := seedTimeoutValue(cfg.TimeoutSeconds, total)

	tuner := &runtimeTuner{
		controller: factory.NewController(seedRate, seedTimeout, int64(workers)),
		limiter:    limiter,
	}
	tuner.timeoutNanos.Store(seedTimeout.Nanoseconds())
	return tuner
}

func (t *runtimeTuner) resolveTimeout() time.Duration {
	nanos := t.timeoutNanos.Load()
	if nanos <= 0 {
		return 2 * time.Second
	}
	return time.Duration(nanos)
}

func (t *runtimeTuner) observeTask(latency time.Duration, resolveErr error, denied int64) {
	t.controller.ObserveTask(latency, isPressureError(resolveErr))
	if denied > 0 {
		t.controller.ObserveRateLimited(denied)
	}
}

func (t *runtimeTuner) observeLimiterError() {
	t.controller.ObserveLimiterError()
}

func (t *runtimeTuner) run(
	ctx context.Context,
	cdm CooldownManager,
	counters runtimeCounters,
	log runtimeLogger,
) {
	ticker := time.NewTicker(t.controller.EvalInterval())
	defer ticker.Stop()

	var lastCompleted int64
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			snap, completed := buildSnapshot(counters, lastCompleted)
			lastCompleted = completed
			decision := t.controller.Evaluate(snap)
			t.timeoutNanos.Store(decision.Timeout.Nanoseconds())
			if err := t.limiter.SetMaxHits(decision.RateLimit); err != nil && log != nil {
				log.Warning("adaptive ratelimit update failed: %v", err)
			}
			if decision.Cooldown > 0 {
				cdm.Trigger(ceilSeconds(decision.Cooldown))
			}
			t.logDecision(log, decision, snap)
		}
	}
}

func (t *runtimeTuner) logDecision(log runtimeLogger, d AdaptiveDecision, s AdaptiveSnapshot) {
	if log == nil {
		return
	}

	tick := t.logTick.Add(1)
	if d.Cooldown == 0 && tick%6 != 0 {
		return
	}

	log.Info(
		"adaptive pressure=%.2f rate=%d timeout=%s queue=%d inflight=%d active=%d cooldown=%s",
		d.Pressure,
		d.RateLimit,
		d.Timeout,
		s.QueueDepth,
		s.InFlight,
		s.ActiveWorkers,
		d.Cooldown,
	)
}

func buildSnapshot(c runtimeCounters, lastCompleted int64) (AdaptiveSnapshot, int64) {
	submitted := atomic.LoadInt64(c.submitted)
	completed := atomic.LoadInt64(c.completed)
	active := atomic.LoadInt64(c.active)
	if active < 0 {
		active = 0
	}

	inFlight := submitted - completed
	if inFlight < 0 {
		inFlight = 0
	}

	queueDepth := inFlight - active
	if queueDepth < 0 {
		queueDepth = 0
	}

	delta := completed - lastCompleted
	if delta < 0 {
		delta = 0
	}

	return AdaptiveSnapshot{
		QueueDepth:     queueDepth,
		InFlight:       inFlight,
		ActiveWorkers:  active,
		CompletedDelta: delta,
	}, completed
}

func seedRateLimit(configRate int, total int64, workers int) int64 {
	if configRate > 0 {
		return int64(configRate)
	}
	base := int64(maxInt(1, workers) * 8)
	switch {
	case total >= 100000:
		return base * 3
	case total >= 30000:
		return base * 2
	default:
		return base
	}
}

func seedTimeoutValue(configTimeout int, total int64) time.Duration {
	if configTimeout > 0 {
		return time.Duration(configTimeout) * time.Second
	}
	switch {
	case total >= 100000:
		return 4 * time.Second
	case total >= 30000:
		return 3 * time.Second
	default:
		return 2 * time.Second
	}
}

func isPressureError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "no records found") || strings.Contains(msg, "no such host") {
		return false
	}
	keywords := []string{
		"timeout",
		"temporary",
		"connection refused",
		"network is unreachable",
		"server misbehaving",
	}
	for _, kw := range keywords {
		if strings.Contains(msg, kw) {
			return true
		}
	}
	return false
}

func ceilSeconds(d time.Duration) int64 {
	if d <= 0 {
		return 0
	}
	seconds := d / time.Second
	if d%time.Second == 0 {
		return int64(seconds)
	}
	return int64(seconds + 1)
}

func maxInt(a, b int) int {
	if a >= b {
		return a
	}
	return b
}
