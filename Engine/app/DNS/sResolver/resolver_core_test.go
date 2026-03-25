// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package sresolver

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/miekg/dns"
)

type fakeDNSClient struct {
	resp *dns.Msg
	err  error
}

func (c *fakeDNSClient) Exchange(*dns.Msg, string) (*dns.Msg, time.Duration, error) {
	return c.resp, 0, c.err
}

// ------------------------------
// Validation
// ------------------------------

func TestStubResolverValidation(t *testing.T) {
	tests := []struct {
		name   string
		r      *StubResolver
		domain string
	}{
		{"empty domain", New(WithUpstream("127.0.0.1:53"), WithRetries(1), WithTimeout(time.Second)), ""},
		{"missing upstream", New(WithRetries(1), WithTimeout(time.Second)), "example.test"},
		{"missing retries", New(WithUpstream("127.0.0.1:53"), WithTimeout(time.Second)), "example.test"},
		{"missing timeout", New(WithUpstream("127.0.0.1:53"), WithRetries(1)), "example.test"},
		{"whitespace", New(WithUpstream("127.0.0.1:53"), WithRetries(1), WithTimeout(time.Second)), "   "},
	}

	for _, tt := range tests {
		ok, err := tt.r.Resolve(context.Background(), tt.domain)
		if err == nil || ok {
			t.Fatalf("%s: expected failure", tt.name)
		}
	}
}

// ------------------------------
// Success + NXDOMAIN + error
// ------------------------------

func TestStubResolverBasicPaths(t *testing.T) {
	old := newDNSClient
	t.Cleanup(func() { newDNSClient = old })

	success := new(dns.Msg)
	success.Answer = append(success.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: "example.test.", Rrtype: dns.TypeA, Class: dns.ClassINET},
		A:   net.ParseIP("127.0.0.1"),
	})

	tests := []struct {
		name   string
		resp   *dns.Msg
		err    error
		expect bool
	}{
		{"success", success, nil, true},
		{"nxdomain", &dns.Msg{Rcode: dns.RcodeNameError}, nil, false},
		{"servfail", &dns.Msg{Rcode: dns.RcodeServerFailure}, nil, false},
		{"network error", nil, context.DeadlineExceeded, false},
	}

	for _, tt := range tests {
		newDNSClient = func(time.Duration) dnsClient {
			return &fakeDNSClient{resp: tt.resp, err: tt.err}
		}

		r := New(
			WithUpstream("127.0.0.1:53"),
			WithRetries(1),
			WithTimeout(time.Second),
		)

		ok, err := r.Resolve(context.Background(), "example.test")

		if tt.expect && (err != nil || !ok) {
			t.Fatalf("%s: expected success", tt.name)
		}
		if !tt.expect && (err == nil && ok) {
			t.Fatalf("%s: expected failure", tt.name)
		}
	}
}

// ------------------------------
// Retry + Timeout
// ------------------------------

func TestStubResolverRetryAndTimeout(t *testing.T) {
	old := newDNSClient
	t.Cleanup(func() { newDNSClient = old })

	attempts := 0

	newDNSClient = func(time.Duration) dnsClient {
		attempts++
		if attempts < 3 {
			return &fakeDNSClient{err: context.DeadlineExceeded}
		}
		resp := new(dns.Msg)
		resp.Answer = append(resp.Answer, &dns.A{
			Hdr: dns.RR_Header{Name: "example.test.", Rrtype: dns.TypeA, Class: dns.ClassINET},
			A:   net.ParseIP("127.0.0.1"),
		})
		return &fakeDNSClient{resp: resp}
	}

	r := New(
		WithUpstream("127.0.0.1:53"),
		WithRetries(3),
		WithTimeout(50*time.Millisecond),
	)

	ok, err := r.Resolve(context.Background(), "example.test")
	if err != nil || !ok {
		t.Fatal("expected retry success")
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}

	// timeout-only path
	newDNSClient = func(time.Duration) dnsClient {
		return &fakeDNSClient{err: context.DeadlineExceeded}
	}

	r = New(
		WithUpstream("127.0.0.1:53"),
		WithRetries(1),
		WithTimeout(10*time.Millisecond),
	)

	ok, err = r.Resolve(context.Background(), "example.test")
	if err == nil || ok {
		t.Fatal("expected timeout failure")
	}
}

// ------------------------------
// System Resolver
// ------------------------------

func TestSystemResolverFull(t *testing.T) {
	old := lookupIPAddr
	t.Cleanup(func() { lookupIPAddr = old })

	// success
	lookupIPAddr = func(_ *net.Resolver, _ context.Context, _ string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}

	r := NewSystem()

	ok, err := r.Resolve(context.Background(), "example.test")
	if err != nil || !ok {
		t.Fatal("expected success")
	}

	// failure
	lookupIPAddr = func(_ *net.Resolver, _ context.Context, _ string) ([]net.IPAddr, error) {
		return nil, context.DeadlineExceeded
	}

	ok, err = r.Resolve(context.Background(), "fail.test")
	if err == nil || ok {
		t.Fatal("expected failure")
	}

	// empty result
	lookupIPAddr = func(_ *net.Resolver, _ context.Context, _ string) ([]net.IPAddr, error) {
		return []net.IPAddr{}, nil
	}

	ok, err = r.Resolve(context.Background(), "empty.test")
	if err != nil || ok {
		t.Fatal("expected unresolved empty result")
	}

	// nil resolver fallback
	r = &SystemResolver{Resolver: nil}

	lookupIPAddr = func(_ *net.Resolver, _ context.Context, _ string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}

	ok, err = r.Resolve(context.Background(), "example.test")
	if err != nil || !ok {
		t.Fatal("expected fallback success")
	}

	// invalid input
	ok, err = r.Resolve(context.Background(), " ")
	if err == nil || ok {
		t.Fatal("expected invalid input failure")
	}
}
