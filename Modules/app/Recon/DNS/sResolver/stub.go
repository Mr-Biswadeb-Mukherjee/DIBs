package sresolver

import (
	"context"
	"errors"
	"time"

	"github.com/miekg/dns"
)

// StubResolver performs a DNS query against ONE upstream DNS server.
type StubResolver struct {
	Upstream string        // primary DNS server (must be provided)
	Retries  int           // retries per record type
	Delay    time.Duration // delay between retries
	Timeout  time.Duration // per-request timeout
}

type Option func(*StubResolver)

// -------------------------
// Functional options
// -------------------------

func WithUpstream(u string) Option {
	return func(r *StubResolver) { r.Upstream = u }
}
func WithRetries(n int) Option {
	return func(r *StubResolver) { r.Retries = n }
}
func WithDelay(d time.Duration) Option {
	return func(r *StubResolver) { r.Delay = d }
}
func WithTimeout(t time.Duration) Option {
	return func(r *StubResolver) { r.Timeout = t }
}

func New(opts ...Option) *StubResolver {
	r := &StubResolver{}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// ---------------------------------------------------------
// Safe context-bound resolveOnce
// ---------------------------------------------------------

func (r *StubResolver) resolveOnce(ctx context.Context, fqdn string, qtype uint16) (*dns.Msg, error) {
	if r.Upstream == "" {
		return nil, errors.New("stubresolver: upstream not configured")
	}

	client := &dns.Client{
		Net:            "udp",
		Timeout:        r.Timeout, // internal timeout as fallback
		UDPSize:        4096,
		SingleInflight: true,
	}

	msg := new(dns.Msg)
	msg.SetQuestion(fqdn, qtype)
	msg.RecursionDesired = true

	resultCh := make(chan *dns.Msg, 1)
	errCh := make(chan error, 1)

	go func() {
		resp, _, err := client.Exchange(msg, r.Upstream)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- resp
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-resultCh:
		return resp, nil
	case err := <-errCh:
		return nil, err
	}
}

// ---------------------------------------------------------
// Resolve A then AAAA with safe retries
// ---------------------------------------------------------

func (r *StubResolver) Resolve(ctx context.Context, domain string) (bool, error) {
	if domain == "" {
		return false, errors.New("stubresolver: empty domain")
	}
	if r.Upstream == "" {
		return false, errors.New("stubresolver: upstream not configured")
	}
	if r.Retries <= 0 {
		return false, errors.New("stubresolver: retries not set")
	}
	if r.Timeout <= 0 {
		return false, errors.New("stubresolver: timeout not set")
	}

	fqdn := dns.Fqdn(domain)
	qtypes := []uint16{dns.TypeA, dns.TypeAAAA}

	for _, qtype := range qtypes {

		for attempt := 0; attempt < r.Retries; attempt++ {

			// Per-attempt context
			attemptCtx, cancel := context.WithTimeout(ctx, r.Timeout)
			resp, err := r.resolveOnce(attemptCtx, fqdn, qtype)
			cancel()

			if err == nil && resp != nil && len(resp.Answer) > 0 {
				return true, nil
			}

			// context-aware delay
			if r.Delay > 0 {
				select {
				case <-ctx.Done():
					return false, ctx.Err()
				case <-time.After(r.Delay):
				}
			}
		}
	}

	return false, nil
}
