// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package app

import (
	"context"
	"time"
)

type Config struct {
	RateLimit      int
	TimeoutSeconds int
	MaxRetries     int
	AutoScale      bool
	UpstreamDNS    string
	BackupDNS      string
	DNSRetries     int
	DNSTimeoutMS   int64
}

type Paths struct {
	KeywordsCSV     string
	DNSIntelOutput  string
	GeneratedOutput string
}

type Dependencies struct {
	Config      Config
	Paths       Paths
	Startup     Startup
	Logs        LogSet
	Cache       CacheStore
	Limiter     RateLimiter
	WorkerPools WorkerPoolFactory
	Cooldowns   CooldownFactory
	Adaptive    AdaptiveFactory
	Writers     WriterFactory
}

type Startup interface {
	Start(label string) time.Time
	Stop()
	Finish(start time.Time, total, resolved int64)
}

type ModuleLogger interface {
	Info(format string, v ...interface{})
	Warning(format string, v ...interface{})
	Alert(format string, v ...interface{})
	Close() error
}

type LogSet struct {
	App         ModuleLogger
	DNS         ModuleLogger
	RateLimiter ModuleLogger
}

type EvalStore interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
}

type CacheStore interface {
	EvalStore
	GetValue(ctx context.Context, key string) (string, error)
	SetValue(ctx context.Context, key string, v interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	RPush(ctx context.Context, key string, values ...string) error
	BLPop(ctx context.Context, timeout time.Duration, key string) (string, bool, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	Close() error
}

type RateLimiter interface {
	Init(store EvalStore, window time.Duration, maxHits int64, logger ModuleLogger)
	Allow(ctx context.Context, key string) (bool, error)
	SetMaxHits(maxHits int64) error
}

type CooldownManager interface {
	StartWatcher()
	Active() bool
	Remaining() int64
	Gate() chan struct{}
	Trigger(durSeconds int64)
}

type CooldownFactory interface {
	New() CooldownManager
}

type TaskFunc func(ctx context.Context) (interface{}, []string, []string, []error)

type TaskResult struct {
	Result   interface{}
	Info     []string
	Warnings []string
	Errors   []error
}

type TaskPriority int

const (
	Low TaskPriority = iota
	Medium
	High
)

type WorkerPoolOptions struct {
	Timeout         time.Duration
	MaxRetries      int
	AutoScale       bool
	MinWorkers      int
	NonBlockingLogs bool
	OnTaskStart     func(taskID int64)
	OnTaskFinish    func(taskID int64, res TaskResult)
}

type WorkerPool interface {
	SubmitTask(f TaskFunc, p TaskPriority, weight int) (int64, <-chan TaskResult, error)
	Stop()
}

type WorkerPoolFactory interface {
	NewWorkerPool(opts *WorkerPoolOptions, conc int, cache CacheStore) WorkerPool
}

type WriterLogHooks struct {
	OnError func(err error)
}

type WriterOptions struct {
	BatchSize  int
	FlushEvery time.Duration
	LogHooks   WriterLogHooks
}

type RecordWriter interface {
	WriteRecord(record interface{})
	Close() error
}

type WriterFactory interface {
	NewNDJSONWriter(path string, opts WriterOptions) (RecordWriter, error)
}

type AdaptiveSnapshot struct {
	QueueDepth     int64
	InFlight       int64
	ActiveWorkers  int64
	CompletedDelta int64
}

type AdaptiveDecision struct {
	RateLimit int64
	Timeout   time.Duration
	Cooldown  time.Duration
	Pressure  float64
}

type AdaptiveController interface {
	EvalInterval() time.Duration
	ObserveTask(latency time.Duration, pressureErr bool)
	ObserveRateLimited(denied int64)
	ObserveLimiterError()
	Evaluate(snapshot AdaptiveSnapshot) AdaptiveDecision
}

type AdaptiveFactory interface {
	NewController(initialRate int64, initialTimeout time.Duration, workers int64) AdaptiveController
}
