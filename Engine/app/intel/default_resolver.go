// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package intel

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/Mr-Biswadeb-Mukherjee/Infermal_v2/Engine/app/intel/dns_intel"
)

type defaultResolver struct {
	resolver *net.Resolver
}

func NewDefaultDNSIntelService(workers int, timeout time.Duration) *DNSIntelService {
	return NewDNSIntelService(NewDefaultResolver(), nil, workers, timeout)
}

func NewDefaultResolver() dns_intel.Resolver {
	return &defaultResolver{resolver: net.DefaultResolver}
}

func (r *defaultResolver) LookupA(ctx context.Context, domain string) ([]string, error) {
	return r.lookupIP(ctx, domain, "ip4")
}

func (r *defaultResolver) LookupAAAA(ctx context.Context, domain string) ([]string, error) {
	return r.lookupIP(ctx, domain, "ip6")
}

func (r *defaultResolver) lookupIP(
	ctx context.Context,
	domain, network string,
) ([]string, error) {
	ips, err := r.resolver.LookupIP(ctx, network, domain)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		out = append(out, ip.String())
	}
	return out, nil
}

func (r *defaultResolver) LookupCNAME(ctx context.Context, domain string) ([]string, error) {
	cname, err := r.resolver.LookupCNAME(ctx, domain)
	if err != nil {
		return nil, err
	}
	return []string{trimDNSHost(cname)}, nil
}

func (r *defaultResolver) LookupNS(ctx context.Context, domain string) ([]string, error) {
	ns, err := r.resolver.LookupNS(ctx, domain)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(ns))
	for _, rec := range ns {
		out = append(out, trimDNSHost(rec.Host))
	}
	return out, nil
}

func (r *defaultResolver) LookupMX(ctx context.Context, domain string) ([]string, error) {
	mx, err := r.resolver.LookupMX(ctx, domain)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(mx))
	for _, rec := range mx {
		out = append(out, trimDNSHost(rec.Host))
	}
	return out, nil
}

func (r *defaultResolver) LookupTXT(ctx context.Context, domain string) ([]string, error) {
	return r.resolver.LookupTXT(ctx, domain)
}

func trimDNSHost(host string) string {
	return strings.TrimSuffix(host, ".")
}
