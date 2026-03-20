package app

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/official-biswadeb941/Infermal_v2/Modules/app/intel"
)

type intelNDJSONRecord struct {
	Domain       string   `json:"domain"`
	A            []string `json:"a,omitempty"`
	AAAA         []string `json:"aaaa,omitempty"`
	CNAME        []string `json:"cname,omitempty"`
	NS           []string `json:"ns,omitempty"`
	MX           []string `json:"mx,omitempty"`
	TXT          []string `json:"txt,omitempty"`
	Providers    []string `json:"providers,omitempty"`
	TimestampUTC string   `json:"timestamp_utc"`
}

func intelRecordToNDJSON(r intel.Record) intelNDJSONRecord {
	return intelNDJSONRecord{
		Domain:       r.Domain,
		A:            r.A,
		AAAA:         r.AAAA,
		CNAME:        r.CNAME,
		NS:           r.NS,
		MX:           r.MX,
		TXT:          r.TXT,
		Providers:    r.Providers,
		TimestampUTC: time.Now().UTC().Format(time.RFC3339),
	}
}

type dnsIntelResolver struct {
	resolver *net.Resolver
}

func newDNSIntelResolver() *dnsIntelResolver {
	return &dnsIntelResolver{resolver: net.DefaultResolver}
}

func (r *dnsIntelResolver) LookupA(ctx context.Context, domain string) ([]string, error) {
	return r.lookupIP(ctx, domain, "ip4")
}

func (r *dnsIntelResolver) LookupAAAA(ctx context.Context, domain string) ([]string, error) {
	return r.lookupIP(ctx, domain, "ip6")
}

func (r *dnsIntelResolver) lookupIP(ctx context.Context, domain, network string) ([]string, error) {
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

func (r *dnsIntelResolver) LookupCNAME(ctx context.Context, domain string) ([]string, error) {
	cname, err := r.resolver.LookupCNAME(ctx, domain)
	if err != nil {
		return nil, err
	}
	return []string{trimDNSHost(cname)}, nil
}

func (r *dnsIntelResolver) LookupNS(ctx context.Context, domain string) ([]string, error) {
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

func (r *dnsIntelResolver) LookupMX(ctx context.Context, domain string) ([]string, error) {
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

func (r *dnsIntelResolver) LookupTXT(ctx context.Context, domain string) ([]string, error) {
	return r.resolver.LookupTXT(ctx, domain)
}

func trimDNSHost(host string) string {
	return strings.TrimSuffix(host, ".")
}
