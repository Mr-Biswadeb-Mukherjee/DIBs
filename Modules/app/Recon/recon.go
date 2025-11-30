package recon

import (
	"context"

	dnsengine "github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon/DNS"
	domaingen "github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon/dga"
)

// DNS interface defines only what recon actually uses.
// This avoids depending on the real engine struct name.
type DNS interface {
	Resolve(ctx context.Context, domain string) (bool, error)
}

type Recon struct {
	DNS DNS
}

// New builds a DNS engine using only primitive parameters.
// No external imports beyond dnsengine.
func New(upstream, backup string, retries int, timeoutMS int) *Recon {
	engine := dnsengine.New(dnsengine.Config{
		Upstream:  upstream,
		Backup:    backup,
		Retries:   retries,
		TimeoutMS: int64(timeoutMS), // FIXED: Cast int → int64
	})

	return &Recon{
		DNS: engine, // FIXED: no Engine type required
	}
}

// GenerateDomains wraps domain generator with clean signature.
func GenerateDomains(path string) ([]string, error) {
	return domaingen.GenerateFromCSV(path)
}

// Resolve uses only the interface, not any concrete type.
func (r *Recon) Resolve(ctx context.Context, domain string) (bool, error) {
	return r.DNS.Resolve(ctx, domain)
}
