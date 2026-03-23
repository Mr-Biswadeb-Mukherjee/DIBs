// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package app

import (
	"time"

	runtime "github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/runtime"
)

type cooldownFactoryAdapter struct {
	inner CooldownFactory
}

func (a cooldownFactoryAdapter) New() runtime.CooldownManager {
	return cooldownManagerAdapter{inner: a.inner.New()}
}

type cooldownManagerAdapter struct {
	inner CooldownManager
}

func (a cooldownManagerAdapter) StartWatcher() {
	a.inner.StartWatcher()
}

func (a cooldownManagerAdapter) Active() bool {
	return a.inner.Active()
}

func (a cooldownManagerAdapter) Remaining() int64 {
	return a.inner.Remaining()
}

func (a cooldownManagerAdapter) Gate() chan struct{} {
	return a.inner.Gate()
}

func (a cooldownManagerAdapter) Trigger(durSeconds int64) {
	a.inner.Trigger(durSeconds)
}

type adaptiveFactoryAdapter struct {
	inner AdaptiveFactory
}

func (a adaptiveFactoryAdapter) NewController(
	initialRate int64,
	initialTimeout time.Duration,
	workers int64,
) runtime.AdaptiveController {
	ctrl := a.inner.NewController(initialRate, initialTimeout, workers)
	return adaptiveControllerAdapter{inner: ctrl}
}

type adaptiveControllerAdapter struct {
	inner AdaptiveController
}

func (a adaptiveControllerAdapter) EvalInterval() time.Duration {
	return a.inner.EvalInterval()
}

func (a adaptiveControllerAdapter) ObserveTask(latency time.Duration, pressureErr bool) {
	a.inner.ObserveTask(latency, pressureErr)
}

func (a adaptiveControllerAdapter) ObserveRateLimited(denied int64) {
	a.inner.ObserveRateLimited(denied)
}

func (a adaptiveControllerAdapter) ObserveLimiterError() {
	a.inner.ObserveLimiterError()
}

func (a adaptiveControllerAdapter) Evaluate(snapshot runtime.AdaptiveSnapshot) runtime.AdaptiveDecision {
	decision := a.inner.Evaluate(AdaptiveSnapshot{
		QueueDepth:     snapshot.QueueDepth,
		InFlight:       snapshot.InFlight,
		ActiveWorkers:  snapshot.ActiveWorkers,
		CompletedDelta: snapshot.CompletedDelta,
	})
	return runtime.AdaptiveDecision{
		RateLimit: decision.RateLimit,
		Timeout:   decision.Timeout,
		Cooldown:  decision.Cooldown,
		Pressure:  decision.Pressure,
	}
}

func toAppLogger(logger runtime.ModuleLogger, fallback ModuleLogger) ModuleLogger {
	if lg, ok := logger.(ModuleLogger); ok {
		return lg
	}
	if fallback != nil {
		return fallback
	}
	if logger == nil {
		return nil
	}
	return moduleLoggerAdapter{inner: logger}
}

type moduleLoggerAdapter struct {
	inner runtime.ModuleLogger
}

func (a moduleLoggerAdapter) Info(format string, v ...interface{}) {
	a.inner.Info(format, v...)
}

func (a moduleLoggerAdapter) Warning(format string, v ...interface{}) {
	a.inner.Warning(format, v...)
}

func (a moduleLoggerAdapter) Alert(format string, v ...interface{}) {
	a.inner.Alert(format, v...)
}

func (a moduleLoggerAdapter) Close() error {
	return a.inner.Close()
}
