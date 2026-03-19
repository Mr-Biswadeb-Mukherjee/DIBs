package dns

import (
	"context"
	"errors"
	"strings"
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

	if d.primary == nil && d.system == nil {
		return nil, errors.New("dns: no resolver configured")
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
	if defaultDNS.primary == nil && defaultDNS.system == nil {
		return errors.New("dns: no resolver available")
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

func New(cfg Config, loggers ...ModuleLogger) *DNS {
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	delay := time.Duration(cfg.DelayMS) * time.Millisecond
	if delay <= 0 {
		delay = 50 * time.Millisecond
	}
	lg := firstLogger(loggers)

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
		system:    stubresolver.NewSystem(),
		cache:     nil, // app.go will attach redis cache via setter
		logger:    lg,
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
	if val, hit := d.readCache(ctx, domain); hit {
		return val, nil
	}

	ok, err := d.resolveWithAdaptiveFallback(ctx, domain)
	if err != nil {
		d.warnf("dns resolve failed domain=%s err=%v", domain, err)
	}

	if ok {
		d.asyncCacheWrite(domain, "1", 48*time.Hour)
		return true, nil
	}

	d.asyncCacheWrite(domain, "0", 12*time.Hour)
	return false, err
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
		if err := d.cache.SetValue(cctx, "dns:"+domain, val, ttl); err != nil {
			d.warnf("dns cache write failed key=dns:%s err=%v", domain, err)
		}
		cancel()
	}()
}

func (d *DNS) readCache(ctx context.Context, domain string) (bool, bool) {
	if d.cache == nil {
		return false, false
	}

	cacheCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	val, err := d.cache.GetValue(cacheCtx, "dns:"+domain)
	cancel()

	if err != nil {
		if shouldLogCacheReadError(err) {
			d.warnf("dns cache read failed key=dns:%s err=%v", domain, err)
		}
		return false, false
	}

	if val == "1" {
		return true, true
	}
	if val == "0" {
		return false, true
	}
	return false, false
}

func firstLogger(loggers []ModuleLogger) ModuleLogger {
	if len(loggers) == 0 {
		return nil
	}
	return loggers[0]
}

func shouldLogCacheReadError(err error) bool {
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "not found") {
		return false
	}
	if strings.Contains(msg, "redis: nil") {
		return false
	}
	return true
}
