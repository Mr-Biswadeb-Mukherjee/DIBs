// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package intel

import (
	"context"
	"net"
	"os"
	"strings"
	"time"

	"github.com/Mr-Biswadeb-Mukherjee/DIBs/Engine/app/intel/dns_intel"
	mdns "github.com/miekg/dns"
)

type defaultResolver struct {
	resolver *net.Resolver
}

func NewDefaultDNSIntelService(workers int, timeout time.Duration) *DNSIntelService {
	return newDNSIntelServiceWithLookups(
		NewDefaultResolver(),
		nil,
		workers,
		timeout,
		dns_intel.NewDefaultWhoisLookup(),
		dns_intel.NewDefaultASNLookup(),
	)
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

func (r *defaultResolver) LookupTTLAndDNSSEC(
	ctx context.Context,
	domain string,
) (int64, bool, error) {
	server := defaultDNSServer()
	msg := new(mdns.Msg)
	msg.SetQuestion(mdns.Fqdn(strings.TrimSpace(domain)), mdns.TypeA)
	msg.SetEdns0(1232, true)
	client := &mdns.Client{}
	resp, _, err := client.ExchangeContext(ctx, msg, server)
	if err != nil {
		return 0, false, err
	}
	return extractTTLAndDNSSEC(resp), responseHasDNSSEC(resp), nil
}

func defaultDNSServer() string {
	cfg, err := mdns.ClientConfigFromFile("/etc/resolv.conf")
	if err == nil && len(cfg.Servers) > 0 {
		return net.JoinHostPort(cfg.Servers[0], cfg.Port)
	}
	if _, statErr := os.Stat("/etc/resolv.conf"); statErr == nil {
		return "127.0.0.1:53"
	}
	return "8.8.8.8:53"
}

func extractTTLAndDNSSEC(resp *mdns.Msg) int64 {
	if resp == nil {
		return 0
	}
	minTTL := uint32(0)
	for _, rr := range resp.Answer {
		ttl := rr.Header().Ttl
		if minTTL == 0 || ttl < minTTL {
			minTTL = ttl
		}
	}
	return int64(minTTL)
}

func responseHasDNSSEC(resp *mdns.Msg) bool {
	if resp == nil {
		return false
	}
	return hasDNSSECRR(resp.Answer) || hasDNSSECRR(resp.Ns) || hasDNSSECRR(resp.Extra)
}

func hasDNSSECRR(records []mdns.RR) bool {
	for _, rr := range records {
		switch rr.Header().Rrtype {
		case mdns.TypeRRSIG, mdns.TypeDNSKEY, mdns.TypeDS, mdns.TypeNSEC, mdns.TypeNSEC3:
			return true
		}
	}
	return false
}

func trimDNSHost(host string) string {
	return strings.TrimSuffix(host, ".")
}
