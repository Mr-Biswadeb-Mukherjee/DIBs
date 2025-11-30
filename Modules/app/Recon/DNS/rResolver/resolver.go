//go:build ignore
// +build ignore

package rresolver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	mdns "github.com/miekg/dns"
)

// Simple recursive DNS resolver
// - Listens on UDP and TCP
// - Performs iterative resolution starting from root servers
// - Handles basic CNAME chasing
// - Includes simple in-memory cache (TTL-respecting)

var rootServers []string // loaded from root.conf at init

// Resolver holds server configuration and cache.
type Resolver struct {
	Addr        string // listen address, e.g. \":53\"
	Timeout     time.Duration
	client      *mdns.Client
	cache       *cache
	listenMux   sync.Mutex
	rootServers []string // populated by LoadRootHints
}

func NewResolver(listenAddr string) *Resolver {
	r := &Resolver{
		Addr:    listenAddr,
		Timeout: 3 * time.Second,
		client: &mdns.Client{
			Net:     "udp",
			Timeout: 3 * time.Second,
		},
		cache: newCache(),
	}

	// auto-load root hints
	_ = r.LoadRootHints("Setting/root.conf")

	return r
}

// Start starts both UDP and TCP servers and blocks.
func (r *Resolver) Start() error {
	r.listenMux.Lock()
	defer r.listenMux.Unlock()

	// UDP server
	udpServer := &mdns.Server{Addr: r.Addr, Net: "udp"}
	tcpServer := &mdns.Server{Addr: r.Addr, Net: "tcp"}

	mdns.HandleFunc(".", r.handleQuery)

	errCh := make(chan error, 2)
	go func() { errCh <- udpServer.ListenAndServe() }()
	go func() { errCh <- tcpServer.ListenAndServe() }()

	// return on first error (or nil if servers run forever)
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			// shutdown both and return error
			udpServer.Shutdown()
			tcpServer.Shutdown()
			return err
		}
	}
	return nil
}

func (r *Resolver) handleQuery(w mdns.ResponseWriter, req *mdns.Msg) {
	ctx := context.Background()
	if len(req.Question) == 0 {
		m := new(mdns.Msg)
		m.SetRcode(req, mdns.RcodeFormatError)
		w.WriteMsg(m)
		return
	}

	q := req.Question[0]
	qtype := q.Qtype

	resp := new(mdns.Msg)
	resp.SetReply(req)

	// try cache
	if answers := r.cache.get(q.Name, qtype); answers != nil {
		resp.Answer = append(resp.Answer, answers...)
		w.WriteMsg(resp)
		return
	}

	ans, err := r.Resolve(ctx, q.Name, qtype)
	if err != nil {
		resp.SetRcode(req, mdns.RcodeServerFailure)
		w.WriteMsg(resp)
		return
	}

	resp.Answer = append(resp.Answer, ans...)
	// store to cache
	if len(ans) > 0 {
		r.cache.set(q.Name, qtype, ans)
	}

	w.WriteMsg(resp)
}

// Resolve performs an iterative (recursive) resolution for a single question.
func (r *Resolver) Resolve(ctx context.Context, qname string, qtype uint16) ([]mdns.RR, error) {
	name := dnsFqdn(qname)
	q := new(mdns.Msg)
	q.Id = 0
	q.RecursionDesired = false // we do iterative queries ourselves
	q.Question = []mdns.Question{{Name: name, Qtype: qtype, Qclass: mdns.ClassINET}}

	// Start from root servers
	servers := append([]string{}, r.rootServers...)
	var lastErr error

	for {
		// iterate servers list
		var resp *mdns.Msg
		for _, srv := range servers {
			resp, _, lastErr = r.tryQuery(ctx, srv, q)
			if lastErr == nil && resp != nil {
				break
			}
		}
		if lastErr != nil {
			return nil, fmt.Errorf("querying servers: %w", lastErr)
		}

		// If we have an answer, return it (handle CNAME chaining)
		if len(resp.Answer) > 0 {
			answers := extractRecords(resp, qname, qtype)
			// handle CNAME chain -> resolve target if necessary
			final := make([]mdns.RR, 0, len(answers))
			for _, a := range answers {
				final = append(final, a)
				if a.Header().Rrtype == mdns.TypeCNAME {
					cname := strings.TrimSuffix(strings.TrimSpace(a.(*mdns.CNAME).Target), ".")
					if cname != "" && !strings.EqualFold(cname, qname) {
						// recursively resolve the CNAME target
						targetAnswers, err := r.Resolve(ctx, cname, qtype)
						if err == nil {
							final = append(final, targetAnswers...)
						}
					}
				}
			}
			return final, nil
		}

		// No direct answers — look for referral in Authority section
		ns := extractNameservers(resp)
		if len(ns) == 0 {
			return nil, errors.New("no answers and no referral")
		}

		// build new servers list using glue/A records in Additional section or by resolving NS names
		servers = resolveServersFromMsg(resp, ns)
		if len(servers) == 0 {
			// as fallback, resolve NS names via system resolver
			for _, n := range ns {
				ips, err := net.DefaultResolver.LookupHost(ctx, n)
				if err != nil {
					continue
				}
				for _, ip := range ips {
					servers = append(servers, net.JoinHostPort(ip, "53"))
				}
			}
			if len(servers) == 0 {
				return nil, errors.New("could not build list of nameserver addresses")
			}
		}
		// loop and query the new set
	}
}

func (r *Resolver) tryQuery(ctx context.Context, server string, q *mdns.Msg) (*mdns.Msg, string, error) {
	// try UDP first
	cli := &mdns.Client{Net: "udp", Timeout: r.Timeout}
	in, _, err := cli.ExchangeContext(ctx, q, server)
	if err == nil && in != nil && in.Rcode == mdns.RcodeSuccess {
		return in, server, nil
	}
	// try TCP as fallback
	cli.Net = "tcp"
	in, _, err = cli.ExchangeContext(ctx, q, server)
	if err == nil && in != nil && in.Rcode == mdns.RcodeSuccess {
		return in, server, nil
	}
	return in, server, err
}

// extractRecords filters answers matching qname and qtype (or all types if qtype==ANY)
func extractRecords(m *mdns.Msg, qname string, qtype uint16) []mdns.RR {
	out := make([]mdns.RR, 0)
	for _, rr := range m.Answer {
		if strings.EqualFold(strings.TrimSuffix(rr.Header().Name, "."), strings.TrimSuffix(qname, ".")) {
			if qtype == mdns.TypeANY || rr.Header().Rrtype == qtype {
				out = append(out, rr)
			}
		}
	}
	return out
}

// extractNameservers returns NS names from Authority section
func extractNameservers(m *mdns.Msg) []string {
	var out []string
	for _, rr := range m.Ns {
		if ns, ok := rr.(*mdns.NS); ok {
			out = append(out, strings.TrimSuffix(ns.Ns, "."))
		}
	}
	return out
}

// resolveServersFromMsg looks into Additional for A/AAAA glue records matching NS names
func resolveServersFromMsg(m *mdns.Msg, nsNames []string) []string {
	addrs := make(map[string]struct{})
	for _, rr := range m.Extra {
		switch v := rr.(type) {
		case *mdns.A:
			name := strings.TrimSuffix(v.Hdr.Name, ".")
			for _, n := range nsNames {
				if strings.EqualFold(name, n) {
					addrs[net.JoinHostPort(v.A.String(), "53")] = struct{}{}
				}
			}
		case *mdns.AAAA:
			name := strings.TrimSuffix(v.Hdr.Name, ".")
			for _, n := range nsNames {
				if strings.EqualFold(name, n) {
					addrs[net.JoinHostPort(v.AAAA.String(), "53")] = struct{}{}
				}
			}
		}
	}
	out := make([]string, 0, len(addrs))
	for k := range addrs {
		out = append(out, k)
	}
	return out
}

func dnsFqdn(name string) string {
	if strings.HasSuffix(name, ".") {
		return name
	}
	return name + "."
}

// ----------------- simple cache -----------------

// LoadRootHints parses a BIND-style root hints file (root.hints) and extracts A/AAAA records.
// It populates r.rootServers with addresses in the form "ip:53".
func (r *Resolver) LoadRootHints(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	zp := mdns.NewZoneParser(bytes.NewReader(data), "", path)
	records := make([]string, 0)
	seen := make(map[string]struct{})

	for {
		rr, ok := zp.Next()
		if !ok {
			break
		}

		switch v := rr.(type) {
		case *mdns.A:
			ip := v.A.String()
			addr := net.JoinHostPort(ip, "53")
			if _, ok := seen[addr]; !ok {
				seen[addr] = struct{}{}
				records = append(records, addr)
			}

		case *mdns.AAAA:
			ip := v.AAAA.String()
			addr := net.JoinHostPort(ip, "53")
			if _, ok := seen[addr]; !ok {
				seen[addr] = struct{}{}
				records = append(records, addr)
			}
		}
	}

	if len(records) == 0 {
		return fmt.Errorf("no A/AAAA records found in root hints: %s", path)
	}

	r.rootServers = records
	return nil
}

type cacheEntry struct {
	rrs       []mdns.RR
	expiresAt time.Time
}

type cache struct {
	mu    sync.RWMutex
	store map[string]cacheEntry // key: name|type
}

func newCache() *cache {
	return &cache{store: make(map[string]cacheEntry)}
}

func keyFor(name string, qtype uint16) string {
	return strings.ToLower(name) + "|" + fmt.Sprint(qtype)
}

func (c *cache) get(name string, qtype uint16) []mdns.RR {
	c.mu.RLock()
	defer c.mu.RUnlock()
	k := keyFor(name, qtype)
	if e, ok := c.store[k]; ok {
		if time.Now().Before(e.expiresAt) {
			out := make([]mdns.RR, len(e.rrs))
			copy(out, e.rrs)
			return out
		}
		delete(c.store, k)
	}
	return nil
}

func ttlFromRR(rr mdns.RR) uint32 {
	return rr.Header().Ttl
}

func (c *cache) set(name string, qtype uint16, rrs []mdns.RR) {
	if len(rrs) == 0 {
		return
	}
	minTTL := uint32(3600)
	for _, r := range rrs {
		t := ttlFromRR(r)
		if t < minTTL {
			minTTL = t
		}
	}
	if minTTL == 0 {
		minTTL = 30
	}
	exp := time.Now().Add(time.Duration(minTTL) * time.Second)
	copyRRs := make([]mdns.RR, len(rrs))
	copy(copyRRs, rrs)
	c.mu.Lock()
	c.store[keyFor(name, qtype)] = cacheEntry{rrs: copyRRs, expiresAt: exp}
	c.mu.Unlock()
}

// ----------------- utility -----------------

func ExampleRun() {
	r := NewResolver(":5353")
	go func() {
		if err := r.Start(); err != nil {
			panic(err)
		}
	}()

	c := &mdns.Client{Net: "udp"}
	m := new(mdns.Msg)
	m.SetQuestion(dnsFqdn("example.com"), mdns.TypeA)
	in, _, err := c.Exchange(m, "127.0.0.1:5353")
	if err != nil {
		fmt.Println("query failed:", err)
	}
	fmt.Println(in)
}
