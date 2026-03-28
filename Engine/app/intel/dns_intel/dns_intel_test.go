package dns_intel

import (
	"context"
	"sync"
	"testing"
	"time"
)

type resolverData struct {
	a     []string
	aaaa  []string
	cname []string
	ns    []string
	mx    []string
	txt   []string
}

type fakeResolver struct {
	mu    sync.Mutex
	data  map[string]resolverData
	calls map[string]int
}

func (r *fakeResolver) LookupA(context.Context, string) ([]string, error)    { return r.lookup("A") }
func (r *fakeResolver) LookupAAAA(context.Context, string) ([]string, error) { return r.lookup("AAAA") }
func (r *fakeResolver) LookupCNAME(context.Context, string) ([]string, error) {
	return r.lookup("CNAME")
}
func (r *fakeResolver) LookupNS(context.Context, string) ([]string, error)  { return r.lookup("NS") }
func (r *fakeResolver) LookupMX(context.Context, string) ([]string, error)  { return r.lookup("MX") }
func (r *fakeResolver) LookupTXT(context.Context, string) ([]string, error) { return r.lookup("TXT") }

func (r *fakeResolver) lookup(kind string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls[kind]++
	data := r.data["example.com"]
	switch kind {
	case "A":
		return append([]string(nil), data.a...), nil
	case "AAAA":
		return append([]string(nil), data.aaaa...), nil
	case "CNAME":
		return append([]string(nil), data.cname...), nil
	case "NS":
		return append([]string(nil), data.ns...), nil
	case "MX":
		return append([]string(nil), data.mx...), nil
	default:
		return append([]string(nil), data.txt...), nil
	}
}

type fakeCache struct {
	mu      sync.Mutex
	records map[string]*IntelRecord
}

type fakeWhois struct {
	mu     sync.Mutex
	record WhoisRecord
	calls  int
}

func (w *fakeWhois) Lookup(context.Context, string) (WhoisRecord, error) {
	w.mu.Lock()
	w.calls++
	rec := w.record
	w.mu.Unlock()
	return rec, nil
}

func (c *fakeCache) Get(domain string) (*IntelRecord, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	rec, ok := c.records[domain]
	return rec, ok
}

func (c *fakeCache) Set(domain string, record *IntelRecord) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.records[domain] = record
}

func TestNewProcessorAppliesDefaults(t *testing.T) {
	p := NewProcessor(&fakeResolver{}, nil, 0, 0)
	if p.workers != 10 {
		t.Fatalf("expected default workers, got %d", p.workers)
	}
	if p.timeout != 5*time.Second {
		t.Fatalf("expected default timeout, got %s", p.timeout)
	}
}

func TestProcessUsesCacheAndStoresMisses(t *testing.T) {
	cached := &IntelRecord{Domain: "cached.example", A: []string{"9.9.9.9"}}
	cache := &fakeCache{records: map[string]*IntelRecord{"cached.example": cached}}
	resolver := &fakeResolver{
		data: map[string]resolverData{
			"example.com": {a: []string{"1.1.1.1"}},
		},
		calls: make(map[string]int),
	}
	p := NewProcessor(resolver, cache, 1, time.Second)

	out, err := p.Process(context.Background(), []DomainRecord{
		{Domain: "cached.example"},
		{Domain: "example.com"},
	})
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected two results, got %d", len(out))
	}
	if resolver.calls["A"] == 0 {
		t.Fatal("expected resolver to be used for cache miss")
	}
	if _, ok := cache.Get("example.com"); !ok {
		t.Fatal("expected miss to be cached")
	}
}

func TestProcessSingleSanitizesAndExtractsProviders(t *testing.T) {
	resolver := &fakeResolver{
		data: map[string]resolverData{
			"example.com": {
				a:     []string{"1.1.1.1", " ", "1.1.1.1", "2.2.2.2"},
				aaaa:  []string{"2001:db8::1"},
				cname: []string{" edge.cloudflare.com. ", "edge.cloudflare.com."},
				ns:    []string{"ns1.example.net.", "ns1.example.net."},
				mx:    []string{"mail.example.net."},
				txt:   []string{"v=spf1", "v=spf1"},
			},
		},
		calls: make(map[string]int),
	}
	whois := &fakeWhois{
		record: WhoisRecord{
			RegistrarWhoisServer: "whois.example-registrar.net",
			UpdatedDate:          "2026-02-06",
			CreationDate:         "2026-01-20",
		},
	}
	p := NewProcessorWithWhois(resolver, nil, 1, time.Second, whois)

	record := p.processSingle(context.Background(), DomainRecord{Domain: "example.com"})

	if len(record.A) != 2 || record.A[1] != "2.2.2.2" {
		t.Fatalf("unexpected sanitized A records: %#v", record.A)
	}
	if len(record.Providers) != 2 {
		t.Fatalf("expected two providers, got %#v", record.Providers)
	}
	if record.RegistrarWhoisServer != "whois.example-registrar.net" {
		t.Fatalf("unexpected registrar whois server: %q", record.RegistrarWhoisServer)
	}
	if record.UpdatedDate != "2026-02-06" || record.CreationDate != "2026-01-20" {
		t.Fatalf("unexpected whois dates: updated=%q creation=%q", record.UpdatedDate, record.CreationDate)
	}
	if record.Timestamp.IsZero() {
		t.Fatal("expected timestamp to be set")
	}
}

func TestHelpersExtractRootDomainAndSanitize(t *testing.T) {
	if got := extractRootDomain(" EDGE.CloudFlare.com. "); got != "cloudflare.com" {
		t.Fatalf("unexpected root domain: %q", got)
	}
	if got := extractRootDomain("localhost"); got != "" {
		t.Fatalf("expected empty root domain, got %q", got)
	}

	sanitized := sanitize([]string{"a", " ", "a", "b"})
	if len(sanitized) != 2 || sanitized[0] != "a" || sanitized[1] != "b" {
		t.Fatalf("unexpected sanitized list: %#v", sanitized)
	}
}
