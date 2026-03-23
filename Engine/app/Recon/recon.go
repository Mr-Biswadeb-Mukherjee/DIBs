// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package recon

import (
	"context"
)

// DNS interface defines only what recon actually uses.
// This avoids depending on the real engine struct name.
type DNS interface {
	Resolve(ctx context.Context, domain string) (bool, error)
}

type Recon struct {
	DNS DNS
}

func New(resolver DNS) *Recon {
	return &Recon{DNS: resolver}
}

// Resolve uses only the interface, not any concrete type.
func (r *Recon) Resolve(ctx context.Context, domain string) (bool, error) {
	return r.DNS.Resolve(ctx, domain)
}
