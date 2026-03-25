// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package bootstrap

import (
	"context"
	"testing"
	"time"
)

func TestIsLocalRedisHost(t *testing.T) {
	cases := map[string]bool{
		"":          true,
		"localhost": true,
		"127.0.0.1": true,
		"::1":       true,
		"[::1]":     true,
		"10.0.0.1":  false,
	}
	for host, want := range cases {
		got := isLocalRedisHost(host)
		if got != want {
			t.Fatalf("isLocalRedisHost(%q)=%v want %v", host, got, want)
		}
	}
}

func TestRedisHostCandidates(t *testing.T) {
	tests := []struct {
		host string
		want []string
	}{
		{host: "", want: []string{"127.0.0.1", "::1"}},
		{host: "localhost", want: []string{"127.0.0.1", "::1"}},
		{host: "[::1]", want: []string{"::1"}},
		{host: "127.0.0.1", want: []string{"127.0.0.1"}},
	}
	for _, tt := range tests {
		got := redisHostCandidates(tt.host)
		if len(got) != len(tt.want) {
			t.Fatalf("candidates(%q) len=%d want=%d", tt.host, len(got), len(tt.want))
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Fatalf("candidates(%q)[%d]=%q want %q", tt.host, i, got[i], tt.want[i])
			}
		}
	}
}

func TestWaitOrCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if waitOrCancel(ctx, time.Second) {
		t.Fatal("expected waitOrCancel to stop on canceled context")
	}
}

func TestWaitOrCancelTimerPath(t *testing.T) {
	ctx := context.Background()
	start := time.Now()
	if !waitOrCancel(ctx, 10*time.Millisecond) {
		t.Fatal("expected timer path to complete")
	}
	if time.Since(start) < 8*time.Millisecond {
		t.Fatal("expected waitOrCancel to wait at least close to duration")
	}
}

func TestRedisInstallersAndStartCommands(t *testing.T) {
	installers := redisInstallers()
	if len(installers) == 0 {
		t.Fatal("expected at least one installer")
	}
	validateInstallerEntries(t, installers)

	startCmds := redisStartCommands()
	if len(startCmds) == 0 {
		t.Fatal("expected at least one startup command")
	}
	validateStartCommands(t, startCmds)
}

func validateInstallerEntries(t *testing.T, installers []installer) {
	t.Helper()
	for _, item := range installers {
		if item.name == "" || item.check == "" || item.install == nil {
			t.Fatal("installer entry must be fully configured")
		}
	}
}

func validateStartCommands(t *testing.T, startCmds []startupCmd) {
	t.Helper()
	for _, item := range startCmds {
		if item.name == "" || item.check == "" || item.cmd == "" {
			t.Fatal("startup command entry must be fully configured")
		}
	}
}
