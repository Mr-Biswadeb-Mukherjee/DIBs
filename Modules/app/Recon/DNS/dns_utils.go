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
	cache     Cache
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
