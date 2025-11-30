package dns_test

import (
	"context"
	"errors"
	"testing"
	"time"

	dns "github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon/DNS"
)

// ------------------------------------------------------------
//
//	MOCK RESOLVER
//
// ------------------------------------------------------------
type mockResolver struct {
	shouldResolve bool
	err           error
	called        bool
}

func (m *mockResolver) Resolve(ctx context.Context, domain string) (bool, error) {
	m.called = true
	return m.shouldResolve, m.err
}

// ------------------------------------------------------------
//
//	MOCK CACHE
//
// ------------------------------------------------------------
type mockCache struct {
	store map[string]string
}

func newMockCache() *mockCache {
	return &mockCache{store: make(map[string]string)}
}

func (c *mockCache) GetValue(ctx context.Context, key string) (string, error) {
	val, ok := c.store[key]
	if !ok {
		return "", errors.New("not found")
	}
	return val, nil
}

func (c *mockCache) SetValue(ctx context.Context, key string, v interface{}, ttl time.Duration) error {
	c.store[key] = v.(string)
	return nil
}

// ------------------------------------------------------------
//
//	BASIC INIT TEST
//
// ------------------------------------------------------------
func TestInitDNS(t *testing.T) {
	cfg := dns.Config{
		Upstream:  "1.1.1.1",
		Backup:    "8.8.8.8",
		Retries:   1,
		TimeoutMS: 300,
		DelayMS:   20,
	}

	d, err := dns.InitDNS(cfg)
	if err != nil {
		t.Fatalf("InitDNS failed: %v", err)
	}

	if d == nil {
		t.Fatal("InitDNS returned nil DNS struct")
	}
}

// ------------------------------------------------------------
//
//	RESOLUTION WITH PRIMARY ONLY
//
// ------------------------------------------------------------
func TestResolvePrimarySuccess(t *testing.T) {
	mPrimary := &mockResolver{shouldResolve: true}
	d := &dns.DNS{}
	d.SwapPrimary(mPrimary)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ok, err := d.Resolve(ctx, "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected primary resolver success")
	}
	if !mPrimary.called {
		t.Fatal("primary resolver not called")
	}
}

// ------------------------------------------------------------
//
//	FALLBACK TO BACKUP RESOLVER
//
// ------------------------------------------------------------
func TestResolveBackupFallback(t *testing.T) {
	mPrimary := &mockResolver{shouldResolve: false, err: errors.New("fail")}
	mBackup := &mockResolver{shouldResolve: true}

	d := &dns.DNS{}
	d.SwapPrimary(mPrimary)
	d.SwapBackup(mBackup)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ok, err := d.Resolve(ctx, "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected backup resolver success")
	}
	if !mBackup.called {
		t.Fatal("backup resolver was not called")
	}
}

// ------------------------------------------------------------
//
//	FALLBACK TO RECURSIVE RESOLVER
//
// ------------------------------------------------------------
func TestResolveRecursiveFallback(t *testing.T) {
	mPrimary := &mockResolver{shouldResolve: false, err: errors.New("fail")}
	mBackup := &mockResolver{shouldResolve: false, err: errors.New("fail")}
	mRecursive := &mockResolver{shouldResolve: true}

	d := &dns.DNS{}
	d.SwapPrimary(mPrimary)
	d.SwapBackup(mBackup)
	d.AttachRecursive(mRecursive)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ok, err := d.Resolve(ctx, "fallback.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("recursive resolver should resolve")
	}
	if !mRecursive.called {
		t.Fatal("recursive resolver not called")
	}
}

// ------------------------------------------------------------
//
//	CACHE HIT (POS / NEG)
//
// ------------------------------------------------------------
func TestResolveCacheHit(t *testing.T) {
	cache := newMockCache()
	cache.store["dns:example.com"] = "1"

	d := &dns.DNS{}
	d.AttachCache(cache)

	ok, err := d.Resolve(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected cached positive result")
	}
}

func TestResolveCacheNegativeHit(t *testing.T) {
	cache := newMockCache()
	cache.store["dns:blocked.com"] = "0"

	d := &dns.DNS{}
	d.AttachCache(cache)

	ok, err := d.Resolve(context.Background(), "blocked.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected cached negative result")
	}
}

// ------------------------------------------------------------
//
//	NEGATIVE CACHE AFTER FAIL
//
// ------------------------------------------------------------
func TestResolveNegativeCaching(t *testing.T) {
	mPrimary := &mockResolver{shouldResolve: false, err: errors.New("nope")}

	cache := newMockCache()

	d := &dns.DNS{}
	d.SwapPrimary(mPrimary)
	d.AttachCache(cache)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	ok, err := d.Resolve(ctx, "fail.com")
	if ok {
		t.Fatal("expected failure")
	}
	if err == nil {
		t.Fatal("expected an error")
	}

	// small delay for async writer
	time.Sleep(50 * time.Millisecond)

	if cache.store["dns:fail.com"] != "0" {
		t.Fatal("negative result not cached")
	}
}

// ------------------------------------------------------------
//
//	STABLE API LAYER
//
// ------------------------------------------------------------
func TestStableAPI(t *testing.T) {

	cfg := dns.Config{
		Upstream:  "1.1.1.1:53", // real resolver
		Retries:   2,
		TimeoutMS: 1200,
		DelayMS:   80,
	}

	if _, err := dns.InitDNS(cfg); err != nil {
		t.Fatalf("InitDNS failed: %v", err)
	}

	// Health must pass
	if err := dns.Health(); err != nil {
		t.Fatalf("Health failed: %v", err)
	}

	// Use a domain guaranteed to resolve
	ok, err := dns.ResolveDomain("google.com")
	if err != nil {
		t.Fatalf("ResolveDomain failed: %v", err)
	}
	if !ok {
		t.Fatal("expected resolution success")
	}

	// Test context-based API
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	ok, err = dns.ResolveWithContext(ctx, "cloudflare.com")
	if err != nil {
		t.Fatalf("ResolveWithContext failed: %v", err)
	}
	if !ok {
		t.Fatal("expected resolution success")
	}
}
