// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package adaptive

import (
	"sync"
	"time"
)

type Config struct {
	InitialRate      int64
	MinRate          int64
	MaxRate          int64
	InitialTimeout   time.Duration
	MinTimeout       time.Duration
	MaxTimeout       time.Duration
	EvalInterval     time.Duration
	TargetLatency    time.Duration
	CooldownMin      time.Duration
	CooldownMax      time.Duration
	CooldownMinGap   time.Duration
	StallTickTrigger int
}

type Snapshot struct {
	QueueDepth     int64
	InFlight       int64
	ActiveWorkers  int64
	CompletedDelta int64
}

type Decision struct {
	RateLimit int64
	Timeout   time.Duration
	Cooldown  time.Duration
	Pressure  float64
}

type sampleWindow struct {
	tasks        int64
	pressureErrs int64
	rateLimited  int64
	limiterErrs  int64
	latencyNanos int64
}

type Controller struct {
	mu sync.Mutex

	cfg Config

	rateLimit int64
	timeout   time.Duration

	latencyEWMA    float64
	errorEWMA      float64
	rateWaitEWMA   float64
	limiterErrEWMA float64

	lastCooldown time.Time
	stallTicks   int
	window       sampleWindow
}

func DefaultConfig(initialRate int64, initialTimeout time.Duration, workers int64) Config {
	if workers < 1 {
		workers = 1
	}
	if initialRate <= 0 {
		initialRate = workers * 12
	}
	if initialTimeout <= 0 {
		initialTimeout = 3 * time.Second
	}

	maxRate := maxInt64(initialRate*6, workers*64)
	minRate := maxInt64(8, initialRate/4)

	return Config{
		InitialRate:      initialRate,
		MinRate:          minRate,
		MaxRate:          maxRate,
		InitialTimeout:   initialTimeout,
		MinTimeout:       900 * time.Millisecond,
		MaxTimeout:       12 * time.Second,
		EvalInterval:     1 * time.Second,
		TargetLatency:    800 * time.Millisecond,
		CooldownMin:      2 * time.Second,
		CooldownMax:      20 * time.Second,
		CooldownMinGap:   10 * time.Second,
		StallTickTrigger: 3,
	}
}

func NewController(cfg Config) *Controller {
	cfg.InitialRate = clampInt64(cfg.InitialRate, 1, maxInt64(cfg.MaxRate, 1))
	cfg.MinRate = clampInt64(cfg.MinRate, 1, cfg.InitialRate)
	cfg.MaxRate = maxInt64(cfg.MaxRate, cfg.InitialRate)
	cfg.MinTimeout = clampDuration(cfg.MinTimeout, 500*time.Millisecond, cfg.MaxTimeout)
	cfg.MaxTimeout = maxDuration(cfg.MaxTimeout, cfg.MinTimeout)
	cfg.InitialTimeout = clampDuration(cfg.InitialTimeout, cfg.MinTimeout, cfg.MaxTimeout)
	cfg.TargetLatency = maxDuration(cfg.TargetLatency, 200*time.Millisecond)
	cfg.EvalInterval = maxDuration(cfg.EvalInterval, 200*time.Millisecond)
	cfg.CooldownMin = maxDuration(cfg.CooldownMin, 1*time.Second)
	cfg.CooldownMax = maxDuration(cfg.CooldownMax, cfg.CooldownMin)
	cfg.CooldownMinGap = maxDuration(cfg.CooldownMinGap, 1*time.Second)
	if cfg.StallTickTrigger < 2 {
		cfg.StallTickTrigger = 2
	}

	return &Controller{
		cfg:         cfg,
		rateLimit:   cfg.InitialRate,
		timeout:     cfg.InitialTimeout,
		latencyEWMA: float64(cfg.TargetLatency),
	}
}

func (c *Controller) EvalInterval() time.Duration {
	return c.cfg.EvalInterval
}

func (c *Controller) ObserveTask(latency time.Duration, pressureErr bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.window.tasks++
	c.window.latencyNanos += latency.Nanoseconds()
	if pressureErr {
		c.window.pressureErrs++
	}
}

func (c *Controller) ObserveRateLimited(denied int64) {
	if denied <= 0 {
		return
	}

	c.mu.Lock()
	c.window.rateLimited += denied
	c.mu.Unlock()
}

func (c *Controller) ObserveLimiterError() {
	c.mu.Lock()
	c.window.limiterErrs++
	c.mu.Unlock()
}

func (c *Controller) Evaluate(s Snapshot) Decision {
	c.mu.Lock()
	defer c.mu.Unlock()

	w := c.consumeWindowLocked()
	c.updateAveragesLocked(w)
	c.updateStallLocked(s)

	pressure := c.pressureLocked(s)
	rate := c.tuneRateLocked(pressure)
	timeout := c.tuneTimeoutLocked(pressure)
	cooldown := c.cooldownLocked(pressure)

	return Decision{
		RateLimit: rate,
		Timeout:   timeout,
		Cooldown:  cooldown,
		Pressure:  pressure,
	}
}

func (c *Controller) consumeWindowLocked() sampleWindow {
	w := c.window
	c.window = sampleWindow{}
	return w
}
