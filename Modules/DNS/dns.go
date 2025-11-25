package dns

import (
	"context"
	"errors"
	"time"

	stubresolver "github.com/official-biswadeb941/Infermal_v2/Modules/DNS/stub-resolver"
)

//
// ---------------------------------------------------
//   OPTIONAL CACHE INTERFACE (No Redis dependency)
// ---------------------------------------------------
//
// Any cache backend (Redis, Memory, FS, DB) must satisfy this.
// Declared here so DNS never imports the Redis package.
//
type Cache interface {
	GetValue(ctx context.Context, key string) (string, error)
	SetValue(ctx context.Context, key string, v interface{}, ttl time.Duration) error
}

//
// ---------------------------------------------------
//                RESOLVER INTERFACE
// ---------------------------------------------------
// Resolver is the minimal interface any resolver must implement.
//
type Resolver interface {
	Resolve(ctx context.Context, domain string) (bool, error)
}

//
// ---------------------------------------------------
//                DNS ORCHESTRATOR
// ---------------------------------------------------
// DNS orchestrator supports:
//   - primary resolver
//   - optional backup resolver
//   - optional recursive resolver
//   - optional cache (Redis)
//
// Cache is injected ONLY from main.go via AttachCache.
// No imports from Redis module.
//

type DNS struct {
	primary   Resolver
	backup    Resolver
	recursive Resolver
	cache     Cache // Optional (attached from main.go)
}

//
// ---------------------------------------------------
//                  CONFIG STRUCT
// ---------------------------------------------------
// Simple and explicit, mapped from setting.conf.
//

type Config struct {
	Upstream  string // upstream_dns
	Backup    string // backup_dns
	Retries   int    // dns_retries
	TimeoutMS int64  // dns_timeout_ms
	DelayMS   int64  // optional retry delay
}

//
// ---------------------------------------------------
//                   CONSTRUCTOR
// ---------------------------------------------------
// Automatically builds resolvers from Config.
//

func New(cfg Config) *DNS {
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	delay := time.Duration(cfg.DelayMS) * time.Millisecond
	if delay <= 0 {
		delay = 50 * time.Millisecond
	}

	var primary Resolver
	if cfg.Upstream != "" {
		primary = stubresolver.New(
			stubresolver.WithUpstream(cfg.Upstream),
			stubresolver.WithRetries(cfg.Retries),
			stubresolver.WithTimeout(timeout),
			stubresolver.WithDelay(delay),
		)
	}

	var backup Resolver
	if cfg.Backup != "" {
		backup = stubresolver.New(
			stubresolver.WithUpstream(cfg.Backup),
			stubresolver.WithRetries(cfg.Retries),
			stubresolver.WithTimeout(timeout),
			stubresolver.WithDelay(delay),
		)
	}

	return &DNS{
		primary: primary,
		backup:  backup,
		// recursive remains nil unless explicitly attached
	}
}

//
// ---------------------------------------------------
//            ATTACH OPTIONAL COMPONENTS
// ---------------------------------------------------
//
func (d *DNS) AttachRecursive(r Resolver) {
	d.recursive = r
}

func (d *DNS) AttachCache(c Cache) {
	d.cache = c
}

//
// ---------------------------------------------------
//                     RESOLUTION
// ---------------------------------------------------
//
// Resolution order:
//
//   1) cache lookup (optional)
//   2) primary resolver
//   3) backup resolver (optional)
//   4) recursive resolver (optional)
//
// Cache is written only after a resolver stage completes.
//

func (d *DNS) Resolve(ctx context.Context, domain string) (bool, error) {

	// -------------------------------
	// 0) CACHE LOOKUP (optional) with short timeout
	// -------------------------------
	if d.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		v, err := d.cache.GetValue(cacheCtx, "dns:"+domain)
		cancel()
		if err == nil {
			if v == "1" {
				return true, nil
			}
			if v == "0" {
				return false, nil
			}
			// if unknown, continue to resolvers
		}
	}

	// -------------------------------
	// prepare a strict per-domain timeout
	// -------------------------------
	// Use a conservative per-domain timeout to avoid a single resolver blocking forever.
	domainCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()

	// -------------------------------
	// 1) PRIMARY RESOLVER
	// -------------------------------
	if d.primary == nil {
		return false, errors.New("dns: primary resolver not configured")
	}

	ok, err := d.primary.Resolve(domainCtx, domain)
	if err == nil && ok {
		if d.cache != nil {
			// write cache asynchronously with a short timeout (best-effort)
			go func() {
				cctx, ccancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
				_ = d.cache.SetValue(cctx, "dns:"+domain, "1", 48*time.Hour)
				ccancel()
			}()
		}
		return true, nil
	}

	// -------------------------------
	// 2) BACKUP RESOLVER (optional)
	// -------------------------------
	if d.backup != nil {
		ok2, err2 := d.backup.Resolve(domainCtx, domain)
		if err2 == nil && ok2 {
			if d.cache != nil {
				go func() {
					cctx, ccancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
					_ = d.cache.SetValue(cctx, "dns:"+domain, "1", 48*time.Hour)
					ccancel()
				}()
			}
			return true, nil
		}
		// if backup also fails, continue
	}

	// -------------------------------
	// 3) RECURSIVE RESOLVER (optional)
	// -------------------------------
	if d.recursive != nil {
		ok3, err3 := d.recursive.Resolve(domainCtx, domain)
		if err3 == nil && ok3 {
			if d.cache != nil {
				go func() {
					cctx, ccancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
					_ = d.cache.SetValue(cctx, "dns:"+domain, "1", 48*time.Hour)
					ccancel()
				}()
			}
			return true, nil
		}
	}

	// -------------------------------
	// ALL FAILED → write cache miss (best-effort)
	// -------------------------------
	if d.cache != nil {
		go func() {
			cctx, ccancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			_ = d.cache.SetValue(cctx, "dns:"+domain, "0", 12*time.Hour)
			ccancel()
		}()
	}

	// return original primary error if available
	if err != nil {
		return false, err
	}

	return false, errors.New("dns: no records found")
}

//
// ---------------------------------------------------
//         RUNTIME SWAP METHODS (optional)
// ---------------------------------------------------
//

func (d *DNS) SwapPrimary(r Resolver) {
	d.primary = r
}

func (d *DNS) SwapBackup(r Resolver) {
	d.backup = r
}
