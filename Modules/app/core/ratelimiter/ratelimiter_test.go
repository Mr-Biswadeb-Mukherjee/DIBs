package ratelimiter

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// ---------------------------
// Fake RedisStore
// ---------------------------
type fakeRedis struct {
	mu     sync.Mutex
	script string
	keys   []string
	args   []interface{}

	// Behavior config
	result interface{}
	err    error

	// Internal sliding window simulation
	zsets map[string][]int64
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{
		zsets: make(map[string][]int64),
	}
}

func (f *fakeRedis) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.script = script
	f.keys = keys
	f.args = args

	if f.err != nil {
		return nil, f.err
	}

	// If a fixed result was injected, return it
	if f.result != nil {
		return f.result, nil
	}

	// Minimal sliding-window simulation
	key := keys[0]
	maxHits := args[0].(int64)
	window := args[1].(int64)
	now := args[2].(int64)

	vals := f.zsets[key]
	cutoff := now - window

	// remove expired
	var filtered []int64
	for _, ts := range vals {
		if ts >= cutoff {
			filtered = append(filtered, ts)
		}
	}
	f.zsets[key] = filtered

	// check limit
	if int64(len(filtered)) >= maxHits {
		return int64(0), nil
	}

	// add new
	f.zsets[key] = append(f.zsets[key], now)
	return int64(1), nil
}

// Reset global instance for tests
func resetInstance() {
	instance = nil
	instanceOnce = sync.Once{}
}

// ---------------------------
// Tests
// ---------------------------

func TestNewSlidingWindowLimiter(t *testing.T) {
	r := newFakeRedis()
	w := time.Second
	lim := newSlidingWindowLimiter(r, w, 5)

	if lim.rdb != r {
		t.Fatalf("rdb mismatch")
	}
	if lim.window != w {
		t.Fatalf("window mismatch")
	}
	if lim.maxHits != 5 {
		t.Fatalf("maxHits mismatch")
	}
}

func TestInitAndRateLimit(t *testing.T) {
	resetInstance()

	r := newFakeRedis()
	Init(r, time.Second, 2)

	allowed, err := RateLimit(context.Background(), "x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed=true")
	}
}

func TestRateLimitBeforeInit(t *testing.T) {
	resetInstance()

	allowed, err := RateLimit(context.Background(), "x")
	if err == nil || !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("expected ErrNotInitialized")
	}
	if allowed {
		t.Fatalf("expected allowed=false")
	}
}

func TestAllowRespectsMaxHits(t *testing.T) {
	resetInstance()

	r := newFakeRedis()
	Init(r, time.Second, 2)

	ctx := context.Background()
	key := "hitkey"

	// First hit ok
	a1, _ := RateLimit(ctx, key)
	// Second hit ok
	a2, _ := RateLimit(ctx, key)
	// Third hit blocked
	a3, _ := RateLimit(ctx, key)

	if !a1 || !a2 || a3 {
		t.Fatalf("expected 1,1,0 but got %v,%v,%v", a1, a2, a3)
	}
}

func TestAllowHandlesRedisError(t *testing.T) {
	resetInstance()

	r := newFakeRedis()
	r.err = errors.New("redis failure")

	Init(r, time.Second, 2)

	allowed, err := RateLimit(context.Background(), "errkey")
	if err == nil {
		t.Fatalf("expected redis error")
	}
	if allowed {
		t.Fatalf("expected allowed=false")
	}
}

func TestAllowHandlesFloat64Result(t *testing.T) {
	resetInstance()

	r := newFakeRedis()
	r.result = float64(1)

	Init(r, time.Second, 2)

	allowed, err := RateLimit(context.Background(), "floatkey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Fatalf("expected allowed=true from float64")
	}
}

func TestAllowHandlesUnexpectedType(t *testing.T) {
	resetInstance()

	r := newFakeRedis()
	r.result = "weird"

	Init(r, time.Second, 2)

	allowed, err := RateLimit(context.Background(), "weirdkey")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if allowed {
		t.Fatalf("expected allowed=false for unknown type")
	}
}

func TestInitIsSingleton(t *testing.T) {
	resetInstance()

	r1 := newFakeRedis()
	r2 := newFakeRedis()

	Init(r1, time.Second, 5)
	Init(r2, time.Hour, 999) // should not override due to sync.Once

	if instance.rdb != r1 {
		t.Fatalf("singleton violated")
	}
	if instance.window != time.Second || instance.maxHits != 5 {
		t.Fatalf("singleton fields incorrect")
	}
}

func TestLuaScriptConstantExists(t *testing.T) {
	if slidingWindowLua == "" {
		t.Fatalf("expected lua script to be non-empty")
	}
}
