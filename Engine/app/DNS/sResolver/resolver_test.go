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

// ------------------------------
// resolveOnce branches
// ------------------------------

func TestResolveOncePaths(t *testing.T) {
	old := newDNSClient
	t.Cleanup(func() { newDNSClient = old })

	r := New(WithUpstream("127.0.0.1:53"), WithTimeout(100*time.Millisecond))

	// success
	newDNSClient = func(time.Duration) dnsClient {
		return &fakeDNSClient{resp: new(dns.Msg)}
	}
	if _, err := r.resolveOnce(context.Background(), "example.test.", dns.TypeA); err != nil {
		t.Fatal("expected success")
	}

	// error
	newDNSClient = func(time.Duration) dnsClient {
		return &fakeDNSClient{err: context.DeadlineExceeded}
	}
	if _, err := r.resolveOnce(context.Background(), "example.test.", dns.TypeA); err == nil {
		t.Fatal("expected error")
	}

	// ctx cancel
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	newDNSClient = func(time.Duration) dnsClient {
		time.Sleep(50 * time.Millisecond)
		return &fakeDNSClient{}
	}

	if _, err := r.resolveOnce(ctx, "example.test.", dns.TypeA); err == nil {
		t.Fatal("expected ctx cancel")
	}
}

// ------------------------------
// Deep stub branches
// ------------------------------

func TestStubResolverEdgeBranches(t *testing.T) {
	old := newDNSClient
	t.Cleanup(func() { newDNSClient = old })

	r := New(
		WithUpstream("127.0.0.1:53"),
		WithRetries(1),
		WithTimeout(50*time.Millisecond),
		WithDelay(20*time.Millisecond),
	)

	// nil response
	newDNSClient = func(time.Duration) dnsClient {
		return &fakeDNSClient{resp: nil}
	}
	ok, err := r.Resolve(context.Background(), "example.test")
	if err != nil || ok {
		t.Fatal("expected unresolved nil response")
	}

	// empty answer
	newDNSClient = func(time.Duration) dnsClient {
		return &fakeDNSClient{resp: new(dns.Msg)}
	}
	ok, err = r.Resolve(context.Background(), "example.test")
	if err != nil || ok {
		t.Fatal("expected unresolved empty answer")
	}

	// ctx cancel during retry delay
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	newDNSClient = func(time.Duration) dnsClient {
		return &fakeDNSClient{err: context.DeadlineExceeded}
	}

	ok, err = r.Resolve(ctx, "example.test")
	if err == nil || ok {
		t.Fatal("expected ctx cancel")
	}
}

// ------------------------------
// Concurrency
// ------------------------------

func TestStubResolverConcurrent(t *testing.T) {
	old := newDNSClient
	t.Cleanup(func() { newDNSClient = old })

	resp := new(dns.Msg)
	resp.Answer = append(resp.Answer, &dns.A{
		Hdr: dns.RR_Header{Name: "example.test.", Rrtype: dns.TypeA, Class: dns.ClassINET},
		A:   net.ParseIP("127.0.0.1"),
	})

	newDNSClient = func(time.Duration) dnsClient {
		return &fakeDNSClient{resp: resp}
	}

	r := New(
		WithUpstream("127.0.0.1:53"),
		WithRetries(1),
		WithTimeout(time.Second),
	)

	const n = 40
	errCh := make(chan error, n)

	for i := 0; i < n; i++ {
		go func() {
			ok, err := r.Resolve(context.Background(), "example.test")
			if err != nil || !ok {
				errCh <- err
				return
			}
			errCh <- nil
		}()
	}

	for i := 0; i < n; i++ {
		if err := <-errCh; err != nil {
			t.Fatalf("failure: %v", err)
		}
	}
}

// ------------------------------
// Weird inputs
// ------------------------------

func TestStubResolverWeirdInputs(t *testing.T) {
	r := New(
		WithUpstream("127.0.0.1:53"),
		WithRetries(1),
		WithTimeout(time.Second),
	)

	inputs := []string{
		"",
		" ",
		"   example.test   ",
		".",
		"..",
		"invalid..domain",
		"exa mple.test",
		"\x00example.test",
	}

	for _, in := range inputs {
		ok, err := r.Resolve(context.Background(), in)
		if err == nil && ok {
			t.Fatalf("unexpected success: %q", in)
		}
	}
}
