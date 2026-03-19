//dns_utils.go

package dns

import (
	"context"
	"time"
)

// ---------------------------------------------------
//
//	OPTIONAL CACHE INTERFACE (No Redis dependency)
//
// ---------------------------------------------------
type Cache interface {
	GetValue(ctx context.Context, key string) (string, error)
	SetValue(ctx context.Context, key string, v interface{}, ttl time.Duration) error
}

// ModuleLogger is injected by app/recon and implemented by logger module.
// DNS must not import logger directly.
type ModuleLogger interface {
	Warning(format string, v ...interface{})
}

// ---------------------------------------------------
//
//	RESOLVER INTERFACE
//
// ---------------------------------------------------
type Resolver interface {
	Resolve(ctx context.Context, domain string) (bool, error)
}

// ---------------------------------------------------
//
//	CONFIG STRUCT
//
// ---------------------------------------------------
type Config struct {
	Upstream  string
	Backup    string
	Retries   int
	TimeoutMS int64
	DelayMS   int64
}

// ---------------------------------------------------
//
//	DNS OBJECT
//
// ---------------------------------------------------
type DNS struct {
	primary   Resolver
	backup    Resolver
	recursive Resolver
	system    Resolver
	cache     Cache
	logger    ModuleLogger
}

// ---------------------------------------------------
//
//	ATTACH OPTIONAL COMPONENTS
//
// ---------------------------------------------------
func (d *DNS) AttachRecursive(r Resolver) {
	d.recursive = r
}

func (d *DNS) AttachCache(c Cache) {
	d.cache = c
}

func (d *DNS) AttachLogger(l ModuleLogger) {
	d.logger = l
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

func (d *DNS) SwapSystem(r Resolver) {
	d.system = r
}

func (d *DNS) warnf(format string, v ...interface{}) {
	if d == nil || d.logger == nil {
		return
	}
	d.logger.Warning(format, v...)
}
