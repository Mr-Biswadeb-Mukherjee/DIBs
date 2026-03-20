package dns_intel

import (
	"context"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"
)

//
// ==============================
// Models
// ==============================
//

type DomainRecord struct {
	Domain string
	IP     string // optional (from resolved pipeline)
}

type IntelRecord struct {
	Domain    string
	A         []string
	AAAA      []string
	CNAME     []string
	NS        []string
	MX        []string
	TXT       []string
	Providers []string
	Timestamp time.Time
}

//
// ==============================
// Interfaces (NO horizontal deps)
// ==============================
//

type Resolver interface {
	LookupA(ctx context.Context, domain string) ([]string, error)
	LookupAAAA(ctx context.Context, domain string) ([]string, error)
	LookupCNAME(ctx context.Context, domain string) ([]string, error)
	LookupNS(ctx context.Context, domain string) ([]string, error)
	LookupMX(ctx context.Context, domain string) ([]string, error)
	LookupTXT(ctx context.Context, domain string) ([]string, error)
}

type Cache interface {
	Get(domain string) (*IntelRecord, bool)
	Set(domain string, record *IntelRecord)
}

//
// ==============================
// Processor
// ==============================
//

type Processor struct {
	resolver Resolver
	cache    Cache

	workers int
	timeout time.Duration
}

func NewProcessor(r Resolver, c Cache, workers int, timeout time.Duration) *Processor {
	if workers <= 0 {
		workers = 10
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &Processor{
		resolver: r,
		cache:    c,
		workers:  workers,
		timeout:  timeout,
	}
}

//
// ==============================
// Public API
// ==============================
//

func (p *Processor) Process(ctx context.Context, domains []DomainRecord) ([]IntelRecord, error) {
	if len(domains) == 0 {
		return nil, nil
	}

	jobs := make(chan DomainRecord)
	results := make(chan IntelRecord)

	var wg sync.WaitGroup

	// Worker pool
	for i := 0; i < p.workers; i++ {
		wg.Add(1)
		go p.worker(ctx, jobs, results, &wg)
	}

	// Feed jobs
	go func() {
		defer close(jobs)
		for _, d := range domains {
			select {
			case jobs <- d:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Collect results
	out := make([]IntelRecord, 0, len(domains))
	done := make(chan struct{})

	go func() {
		for r := range results {
			out = append(out, r)
		}
		close(done)
	}()

	wg.Wait()
	close(results)
	<-done

	return out, nil
}

//
// ==============================
// Worker
// ==============================
//

func (p *Processor) worker(ctx context.Context, jobs <-chan DomainRecord, results chan<- IntelRecord, wg *sync.WaitGroup) {
	defer wg.Done()

	for d := range jobs {

		// Cache check
		if p.cache != nil {
			if cached, ok := p.cache.Get(d.Domain); ok {
				results <- *cached
				continue
			}
		}

		record := p.processSingle(ctx, d)

		if p.cache != nil {
			p.cache.Set(d.Domain, &record)
		}

		results <- record
	}
}

//
// ==============================
// Single Domain Processing
// ==============================
//

func (p *Processor) processSingle(parentCtx context.Context, d DomainRecord) IntelRecord {
	ctx, cancel := context.WithTimeout(parentCtx, p.timeout)
	defer cancel()

	var (
		a     []string
		aaaa  []string
		cname []string
		ns    []string
		mx    []string
		txt   []string
	)

	var wg sync.WaitGroup
	wg.Add(6)

	// Parallel DNS lookups

	go func() {
		defer wg.Done()
		a, _ = p.resolver.LookupA(ctx, d.Domain)
	}()

	go func() {
		defer wg.Done()
		aaaa, _ = p.resolver.LookupAAAA(ctx, d.Domain)
	}()

	go func() {
		defer wg.Done()
		cname, _ = p.resolver.LookupCNAME(ctx, d.Domain)
	}()

	go func() {
		defer wg.Done()
		ns, _ = p.resolver.LookupNS(ctx, d.Domain)
	}()

	go func() {
		defer wg.Done()
		mx, _ = p.resolver.LookupMX(ctx, d.Domain)
	}()

	go func() {
		defer wg.Done()
		txt, _ = p.resolver.LookupTXT(ctx, d.Domain)
	}()

	wg.Wait()

	providers := extractProviders(cname, ns)

	return IntelRecord{
		Domain:    d.Domain,
		A:         sanitize(a),
		AAAA:      sanitize(aaaa),
		CNAME:     sanitize(cname),
		NS:        sanitize(ns),
		MX:        sanitize(mx),
		TXT:       sanitize(txt),
		Providers: providers,
		Timestamp: time.Now().UTC(),
	}
}

//
// ==============================
// Provider Extraction (NO hardcoding)
// ==============================
//

func extractProviders(cnames, ns []string) []string {
	all := append(cnames, ns...)

	if len(all) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	out := make([]string, 0)

	for _, v := range all {
		root := extractRootDomain(v)
		if root == "" {
			continue
		}

		if _, ok := seen[root]; ok {
			continue
		}

		seen[root] = struct{}{}
		out = append(out, root)
	}

	return out
}

func extractRootDomain(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimSuffix(host, ".")

	if host == "" {
		return ""
	}

	domain, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return ""
	}

	return domain
}

//
// ==============================
// Helpers
// ==============================
//

func sanitize(in []string) []string {
	if len(in) == 0 {
		return nil
	}

	out := make([]string, 0, len(in))
	seen := make(map[string]struct{})

	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}

		if _, ok := seen[v]; ok {
			continue
		}

		seen[v] = struct{}{}
		out = append(out, v)
	}

	return out
}