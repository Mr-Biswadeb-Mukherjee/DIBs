// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package dns_intel

import (
	"context"
	"sync"
	"time"
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
	Domain               string
	A                    []string
	AAAA                 []string
	CNAME                []string
	NS                   []string
	MX                   []string
	TXT                  []string
	ASNs                 []ASNRecord
	Providers            []string
	RegistrarWhoisServer string
	UpdatedDate          string
	CreationDate         string
	TTL                  int64
	DNSSEC               bool
	Timestamp            time.Time
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
	LookupTTLAndDNSSEC(ctx context.Context, domain string) (int64, bool, error)
}

type Cache interface {
	Get(domain string) (*IntelRecord, bool)
	Set(domain string, record *IntelRecord)
}

type WhoisLookup interface {
	Lookup(ctx context.Context, domain string) (WhoisRecord, error)
}

type ASNLookup interface {
	Lookup(ctx context.Context, ip string) (ASNRecord, error)
}

//
// ==============================
// Processor
// ==============================
//

type Processor struct {
	resolver Resolver
	cache    Cache
	whois    WhoisLookup
	asn      ASNLookup

	workers int
	timeout time.Duration
}

func NewProcessor(r Resolver, c Cache, workers int, timeout time.Duration) *Processor {
	return NewProcessorWithWhois(r, c, workers, timeout, nil)
}

func NewProcessorWithWhois(
	r Resolver,
	c Cache,
	workers int,
	timeout time.Duration,
	whois WhoisLookup,
) *Processor {
	return NewProcessorWithLookups(r, c, workers, timeout, whois, nil)
}

func NewProcessorWithLookups(
	r Resolver,
	c Cache,
	workers int,
	timeout time.Duration,
	whois WhoisLookup,
	asn ASNLookup,
) *Processor {
	if workers <= 0 {
		workers = 10
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	return &Processor{
		resolver: r,
		cache:    c,
		whois:    whois,
		asn:      asn,
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
		a       []string
		aaaa    []string
		cname   []string
		ns      []string
		mx      []string
		txt     []string
		whois   WhoisRecord
		ttl     int64
		dnssec  bool
		lookups = 7
	)
	if p.whois != nil {
		lookups++
	}

	var wg sync.WaitGroup
	wg.Add(lookups)

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
	go func() {
		defer wg.Done()
		ttl, dnssec, _ = p.resolver.LookupTTLAndDNSSEC(ctx, d.Domain)
	}()

	if p.whois != nil {
		go func() {
			defer wg.Done()
			whois, _ = p.whois.Lookup(ctx, d.Domain)
		}()
	}

	wg.Wait()

	a = sanitize(a)
	aaaa = sanitize(aaaa)
	asnRecords := p.lookupASNs(ctx, combineIPs(a, aaaa))
	providers := extractProviders(cname, ns)

	return IntelRecord{
		Domain:               d.Domain,
		A:                    a,
		AAAA:                 aaaa,
		CNAME:                sanitize(cname),
		NS:                   sanitize(ns),
		MX:                   sanitize(mx),
		TXT:                  sanitize(txt),
		ASNs:                 asnRecords,
		Providers:            providers,
		RegistrarWhoisServer: whois.RegistrarWhoisServer,
		UpdatedDate:          whois.UpdatedDate,
		CreationDate:         whois.CreationDate,
		TTL:                  ttl,
		DNSSEC:               dnssec,
		Timestamp:            time.Now().UTC(),
	}
}
