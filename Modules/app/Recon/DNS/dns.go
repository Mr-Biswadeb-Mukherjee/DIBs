package dns

import (
	"context"
	"errors"
	"time"

	stubresolver "github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon/DNS/sResolver"
)

//
// ===================================================
//                   PUBLIC API
// ===================================================
//
// Fully backward compatible with your previous version.
// Now internally uses an injected DNS instance instead
// of global state.
//

// -------------------------------------------------------------------
// IMPORTANT: the public API historically depended on globalDNS.
// We keep the functions, but now they simply call the injected engine.
// -------------------------------------------------------------------

var defaultDNS *DNS

// InitDNS creates a DNS engine and sets it as the default resolver.
// This maintains compatibility with your original API.
func InitDNS(cfg Config) (*DNS, error) {
	d := New(cfg)

	if d.primary == nil {
		return nil, errors.New("dns: upstream resolver not set")
	}

	// Assign to defaultDNS for backward compatibility
	defaultDNS = d
	return d, nil
}

// ResolveDomain uses the defaultDNS engine.
// Fully backward compatible.
func ResolveDomain(domain string) (bool, error) {
	if defaultDNS == nil {
		return false, errors.New("dns: module not initialized (InitDNS not called)")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	return defaultDNS.Resolve(ctx, domain)
}

// ResolveWithContext uses the injected defaultDNS.
// Fully backward compatible.
func ResolveWithContext(ctx context.Context, domain string) (bool, error) {
	if defaultDNS == nil {
		return false, errors.New("dns: module not initialized (InitDNS not called)")
	}
	return defaultDNS.Resolve(ctx, domain)
}

// Health checks whether DNS is ready.
// Fully backward compatible.
func Health() error {
	if defaultDNS == nil {
		return errors.New("dns: not initialized")
	}
	if defaultDNS.primary == nil {
		return errors.New("dns: primary resolver missing")
	}
	return nil
}

//
// ===================================================
//                   INJECTION API
// ===================================================
//
// This is the new, cleaner interface that recon.go and app.go
// will use to avoid ANY global state.
//

// DNSResolver is the interface implemented by *DNS.
// Recon and App will depend on this instead of using globals.
type DNSResolver interface {
	Resolve(ctx context.Context, domain string) (bool, error)
}

//
// ===================================================
//                   CONSTRUCTOR
// ===================================================
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
		primary:   primary,
		backup:    backup,
		recursive: nil, // stays nil until AttachRecursive is used
		cache:     nil, // app.go will attach redis cache via setter
	}
}

//
// ===================================================
//                     RESOLUTION
// ===================================================
//
// Exactly the same behavior as original version.
// NO GLOBAL LOOKUPS. Uses engine-local cache.
// Fully compatible with caching system.
//

func (d *DNS) Resolve(ctx context.Context, domain string) (bool, error) {

	// 0) CACHE LOOKUP
	if d.cache != nil {
		cacheCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		val, err := d.cache.GetValue(cacheCtx, "dns:"+domain)
		cancel()

		if err == nil {
			if val == "1" {
				return true, nil
			}
			if val == "0" {
				return false, nil
			}
		}
	}

	// Domain-level timeout
	domainCtx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()

	// 1) PRIMARY RESOLVER
	if d.primary == nil {
		return false, errors.New("dns: primary resolver not configured")
	}

	ok, err := d.primary.Resolve(domainCtx, domain)
	if err == nil && ok {
		d.asyncCacheWrite(domain, "1", 48*time.Hour)
		return true, nil
	}

	// 2) BACKUP RESOLVER
	if d.backup != nil {
		ok2, err2 := d.backup.Resolve(domainCtx, domain)
		if err2 == nil && ok2 {
			d.asyncCacheWrite(domain, "1", 48*time.Hour)
			return true, nil
		}
	}

	// 3) RECURSIVE RESOLVER (optional)
	if d.recursive != nil {
		ok3, err3 := d.recursive.Resolve(domainCtx, domain)
		if err3 == nil && ok3 {
			d.asyncCacheWrite(domain, "1", 48*time.Hour)
			return true, nil
		}
	}

	// All resolvers failed → Cache negative
	d.asyncCacheWrite(domain, "0", 12*time.Hour)

	if err != nil {
		return false, err
	}

	return false, errors.New("dns: no records found")
}

//
// ===================================================
//              INTERNAL ASYNC CACHE WRITER
// ===================================================
//

func (d *DNS) asyncCacheWrite(domain string, val string, ttl time.Duration) {
	if d.cache == nil {
		return
	}

	go func() {
		cctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
		_ = d.cache.SetValue(cctx, "dns:"+domain, val, ttl)
		cancel()
	}()
}
