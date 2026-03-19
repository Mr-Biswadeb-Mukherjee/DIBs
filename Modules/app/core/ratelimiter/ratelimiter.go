//ratelimiter.go

package ratelimiter

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ----------------------------------------------
// Redis minimal interface (compatible with your app)
// ----------------------------------------------
type RedisStore interface {
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
}

// ModuleLogger is implemented in logger module and injected by app.
// Ratelimiter must not import logger directly.
type ModuleLogger interface {
	Warning(format string, v ...interface{})
}

// ----------------------------------------------
// Sliding Window Rate Limiter (Redis + Lua)
// ----------------------------------------------
type SlidingWindowLimiter struct {
	rdb     RedisStore
	window  time.Duration
	maxHits int64
	logger  ModuleLogger
}

func newSlidingWindowLimiter(
	rdb RedisStore,
	window time.Duration,
	maxHits int64,
	logger ModuleLogger,
) *SlidingWindowLimiter {
	return &SlidingWindowLimiter{
		rdb:     rdb,
		window:  window,
		maxHits: maxHits,
		logger:  logger,
	}
}

// ==============================================
//
//	INTERNAL INSTANCE
//
// ==============================================
var (
	instance     *SlidingWindowLimiter
	instanceOnce sync.Once
)

// ==============================================
//         STABLE PUBLIC INITIALIZATION API
// ==============================================

// Init sets up the global limiter instance.
// This is a stable API that will not change.
func Init(rdb RedisStore, window time.Duration, maxHits int64, loggers ...ModuleLogger) {
	instanceOnce.Do(func() {
		instance = newSlidingWindowLimiter(rdb, window, maxHits, firstLogger(loggers))
	})
}

// ==============================================
//        STABLE PUBLIC RATE LIMIT API
// ==============================================

// RateLimit is the only function app.go needs.
// Returns true if allowed, false if rate limited.
//
// No matter how much the internal code changes,
// this function signature will stay untouched.
func RateLimit(ctx context.Context, key string) (bool, error) {
	if instance == nil {
		return false, ErrNotInitialized
	}
	return instance.Allow(ctx, key)
}

// Error returned when limiter is used before Init().
var ErrNotInitialized = fmt.Errorf("ratelimiter used before Init()")

// ----------------------------------------------
// Core Lua Script for Atomic Sliding Window
// ----------------------------------------------
const slidingWindowLua = `
local key = KEYS[1]
local max_hits = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local cutoff = now - window

redis.call("ZREMRANGEBYSCORE", key, 0, cutoff)

local count = redis.call("ZCARD", key)

if count >= max_hits then
	return 0
end

redis.call("ZADD", key, now, now .. "-" .. math.random())
redis.call("PEXPIRE", key, window)

return 1
`

// Allow checks if the caller may perform one action.
func (l *SlidingWindowLimiter) Allow(ctx context.Context, key string) (bool, error) {

	now := time.Now().UnixMilli()

	res, err := l.rdb.Eval(
		ctx,
		slidingWindowLua,
		[]string{key},
		l.maxHits,
		l.window.Milliseconds(),
		now,
	)
	if err != nil {
		l.warnf("ratelimiter eval failed key=%s err=%v", key, err)
		return false, err
	}

	allowed, ok := res.(int64)
	if !ok {
		if f, ok := res.(float64); ok {
			return int64(f) == 1, nil
		}
		l.warnf("ratelimiter unexpected redis result type key=%s type=%T", key, res)
		return false, nil
	}

	return allowed == 1, nil
}

func firstLogger(loggers []ModuleLogger) ModuleLogger {
	if len(loggers) == 0 {
		return nil
	}
	return loggers[0]
}

func (l *SlidingWindowLimiter) warnf(format string, v ...interface{}) {
	if l == nil || l.logger == nil {
		return
	}
	l.logger.Warning(format, v...)
}
