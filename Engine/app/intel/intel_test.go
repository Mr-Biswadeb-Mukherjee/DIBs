package intel

import (
	"context"
	"sync"
	"testing"
	"time"
)

type resolverStub struct {
	mu    sync.Mutex
	calls []string
}

func (r *resolverStub) LookupA(context.Context, string) ([]string, error) {
	r.record("A")
	return []string{"1.1.1.1", "1.1.1.1", " 2.2.2.2 "}, nil
}

func (r *resolverStub) LookupAAAA(context.Context, string) ([]string, error) {
	r.record("AAAA")
	return []string{"2001:db8::1"}, nil
}

func (r *resolverStub) LookupCNAME(context.Context, string) ([]string, error) {
	r.record("CNAME")
	return []string{"edge.cloudflare.com."}, nil
}

func (r *resolverStub) LookupNS(context.Context, string) ([]string, error) {
	r.record("NS")
	return []string{"ns1.example.net."}, nil
}

func (r *resolverStub) LookupMX(context.Context, string) ([]string, error) {
	r.record("MX")
	return []string{"mail.example.net."}, nil
}

func (r *resolverStub) LookupTXT(context.Context, string) ([]string, error) {
	r.record("TXT")
	return []string{"v=spf1 include:example.net"}, nil
}

func (r *resolverStub) LookupTTLAndDNSSEC(context.Context, string) (int64, bool, error) {
	r.record("META")
	return 600, true, nil
}

func (r *resolverStub) record(name string) {
	r.mu.Lock()
	r.calls = append(r.calls, name)
	r.mu.Unlock()
}

func TestDNSIntelServiceRunEmpty(t *testing.T) {
	service := NewDNSIntelService(&resolverStub{}, nil, 1, time.Second)

	records, err := service.Run(context.Background(), nil)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if records != nil {
		t.Fatalf("expected nil records for empty input, got %#v", records)
	}
}

func TestDNSIntelServiceRunMapsProcessorOutput(t *testing.T) {
	resolver := &resolverStub{}
	service := NewDNSIntelService(resolver, nil, 1, time.Second)

	records, err := service.Run(context.Background(), []Domain{{Name: "example.com"}})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one record, got %d", len(records))
	}

	record := records[0]
	if record.Domain != "example.com" {
		t.Fatalf("unexpected domain: %q", record.Domain)
	}
	if len(record.A) != 2 || record.A[0] != "1.1.1.1" || record.A[1] != "2.2.2.2" {
		t.Fatalf("unexpected A records: %#v", record.A)
	}
	if len(record.Providers) != 2 {
		t.Fatalf("expected two providers, got %#v", record.Providers)
	}
	if len(resolver.calls) != 7 {
		t.Fatalf("expected seven resolver calls, got %d", len(resolver.calls))
	}
	if record.TTL != 600 || !record.DNSSEC {
		t.Fatalf("expected ttl/dnssec to be mapped, ttl=%d dnssec=%v", record.TTL, record.DNSSEC)
	}
}
