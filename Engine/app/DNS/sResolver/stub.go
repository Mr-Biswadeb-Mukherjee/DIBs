// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package sresolver

import (
	"context"
	"errors"
	"time"

	"github.com/miekg/dns"
)

type dnsClient interface {
	Exchange(m *dns.Msg, address string) (*dns.Msg, time.Duration, error)
}

var newDNSClient = func(timeout time.Duration) dnsClient {
	return &dns.Client{
		Net:            "udp",
		Timeout:        timeout,
		UDPSize:        4096,
		SingleInflight: true,
	}
}

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

	client := newDNSClient(r.Timeout)

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
	if err := r.validateResolveInput(domain); err != nil {
		return false, err
	}

	fqdn := dns.Fqdn(domain)
	for _, qtype := range []uint16{dns.TypeA, dns.TypeAAAA} {
		found, err := r.resolveRecordType(ctx, fqdn, qtype)
		if found || err != nil {
			return found, err
		}
	}

	return false, nil
}

func (r *StubResolver) validateResolveInput(domain string) error {
	switch {
	case domain == "":
		return errors.New("stubresolver: empty domain")
	case r.Upstream == "":
		return errors.New("stubresolver: upstream not configured")
	case r.Retries <= 0:
		return errors.New("stubresolver: retries not set")
	case r.Timeout <= 0:
		return errors.New("stubresolver: timeout not set")
	default:
		return nil
	}
}

func (r *StubResolver) resolveRecordType(ctx context.Context, fqdn string, qtype uint16) (bool, error) {
	for attempt := 0; attempt < r.Retries; attempt++ {
		attemptCtx, cancel := context.WithTimeout(ctx, r.Timeout)
		resp, err := r.resolveOnce(attemptCtx, fqdn, qtype)
		cancel()

		if err == nil && resp != nil && len(resp.Answer) > 0 {
			return true, nil
		}
		if err := waitRetryDelay(ctx, r.Delay); err != nil {
			return false, err
		}
	}
	return false, nil
}

func waitRetryDelay(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
