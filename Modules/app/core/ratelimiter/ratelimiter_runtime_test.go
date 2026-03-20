package ratelimiter

import (
	"context"
	"testing"
	"time"
)

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
	Init(r2, time.Hour, 999)

	if instance.rdb != r1 {
		t.Fatalf("singleton violated")
	}
	if instance.window != time.Second || instance.Limit() != 5 {
		t.Fatalf("singleton fields incorrect")
	}
}

func TestLuaScriptConstantExists(t *testing.T) {
	if slidingWindowLua == "" {
		t.Fatalf("expected lua script to be non-empty")
	}
}

func TestSetMaxHitsRuntimeUpdate(t *testing.T) {
	resetInstance()

	r := newFakeRedis()
	Init(r, time.Second, 1)

	key := "runtime-limit"
	a1, _ := RateLimit(context.Background(), key)
	a2, _ := RateLimit(context.Background(), key)
	if !a1 || a2 {
		t.Fatalf("expected second call to be blocked before update")
	}

	if err := SetMaxHits(3); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	current, err := CurrentLimit()
	if err != nil {
		t.Fatalf("current limit failed: %v", err)
	}
	if current != 3 {
		t.Fatalf("expected current limit=3 got %d", current)
	}

	key = "runtime-limit-2"
	a1, _ = RateLimit(context.Background(), key)
	a2, _ = RateLimit(context.Background(), key)
	a3, _ := RateLimit(context.Background(), key)
	a4, _ := RateLimit(context.Background(), key)
	if !a1 || !a2 || !a3 || a4 {
		t.Fatalf("expected pattern 1,1,1,0 after runtime update")
	}
}
