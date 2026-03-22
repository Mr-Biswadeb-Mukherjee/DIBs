// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package bootstrap

import (
	"context"
	"time"

	engine "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine"
	adaptive "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/core/adaptive"
	ratelimiter "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/core/ratelimiter"
)

type rateLimiterAdapter struct{}

func (rateLimiterAdapter) Init(
	store engine.EvalStore,
	window time.Duration,
	maxHits int64,
	logger engine.ModuleLogger,
) {
	ratelimiter.Init(store, window, maxHits, logger)
}

func (rateLimiterAdapter) Allow(ctx context.Context, key string) (bool, error) {
	return ratelimiter.RateLimit(ctx, key)
}

func (rateLimiterAdapter) SetMaxHits(maxHits int64) error {
	return ratelimiter.SetMaxHits(maxHits)
}

type adaptiveFactoryAdapter struct{}

type adaptiveControllerAdapter struct {
	controller *adaptive.Controller
}

func (adaptiveFactoryAdapter) NewController(
	initialRate int64,
	initialTimeout time.Duration,
	workers int64,
) engine.AdaptiveController {
	cfg := adaptive.DefaultConfig(initialRate, initialTimeout, workers)
	return adaptiveControllerAdapter{controller: adaptive.NewController(cfg)}
}

func (a adaptiveControllerAdapter) EvalInterval() time.Duration {
	return a.controller.EvalInterval()
}

func (a adaptiveControllerAdapter) ObserveTask(latency time.Duration, pressureErr bool) {
	a.controller.ObserveTask(latency, pressureErr)
}

func (a adaptiveControllerAdapter) ObserveRateLimited(denied int64) {
	a.controller.ObserveRateLimited(denied)
}

func (a adaptiveControllerAdapter) ObserveLimiterError() {
	a.controller.ObserveLimiterError()
}

func (a adaptiveControllerAdapter) Evaluate(s engine.AdaptiveSnapshot) engine.AdaptiveDecision {
	decision := a.controller.Evaluate(adaptive.Snapshot{
		QueueDepth:     s.QueueDepth,
		InFlight:       s.InFlight,
		ActiveWorkers:  s.ActiveWorkers,
		CompletedDelta: s.CompletedDelta,
	})
	return engine.AdaptiveDecision{
		RateLimit: decision.RateLimit,
		Timeout:   decision.Timeout,
		Cooldown:  decision.Cooldown,
		Pressure:  decision.Pressure,
	}
}
