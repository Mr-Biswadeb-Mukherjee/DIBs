package recon_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/official-biswadeb941/Infermal_v2/Modules/app/Recon"
)

// mockDNS implements the DNS interface for testing.
type mockDNS struct {
	resolveFunc func(ctx context.Context, domain string) (bool, error)
}

func (m *mockDNS) Resolve(ctx context.Context, domain string) (bool, error) {
	return m.resolveFunc(ctx, domain)
}

// -------------------------
// Resolve() Tests
// -------------------------

func TestResolve_Success(t *testing.T) {
	mock := &mockDNS{
		resolveFunc: func(ctx context.Context, domain string) (bool, error) {
			return true, nil
		},
	}

	r := &recon.Recon{DNS: mock}

	ok, err := r.Resolve(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected true, got false")
	}
}

func TestResolve_Error(t *testing.T) {
	mock := &mockDNS{
		resolveFunc: func(ctx context.Context, domain string) (bool, error) {
			return false, errors.New("dns failure")
		},
	}

	r := &recon.Recon{DNS: mock}

	ok, err := r.Resolve(context.Background(), "invalid.com")
	if err == nil {
		t.Fatalf("expected error but got nil")
	}
	if ok {
		t.Fatalf("expected false on DNS failure")
	}
}

func TestResolve_ContextCancelled(t *testing.T) {
	mock := &mockDNS{
		resolveFunc: func(ctx context.Context, domain string) (bool, error) {
			<-ctx.Done()
			return false, ctx.Err()
		},
	}

	r := &recon.Recon{DNS: mock}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ok, err := r.Resolve(ctx, "example.com")
	if err == nil {
		t.Fatalf("expected context cancel error")
	}
	if ok {
		t.Fatalf("expected false due to cancellation")
	}
}

func TestResolve_Timeout(t *testing.T) {
	mock := &mockDNS{
		resolveFunc: func(ctx context.Context, domain string) (bool, error) {
			time.Sleep(80 * time.Millisecond)
			return false, errors.New("should not get here")
		},
	}

	r := &recon.Recon{DNS: mock}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	ok, err := r.Resolve(ctx, "slow.com")
	if err == nil {
		t.Fatalf("expected deadline exceeded error")
	}
	if ok {
		t.Fatalf("expected false")
	}
}

// -------------------------
// New() Tests
// -------------------------

func TestNewRecon(t *testing.T) {
	r := recon.New("8.8.8.8", "1.1.1.1", 3, 500)
	if r == nil {
		t.Fatalf("expected non-nil Recon instance")
	}
	if r.DNS == nil {
		t.Fatalf("expected DNS engine inside Recon")
	}
}

// -------------------------
// GenerateDomains() Tests
// -------------------------

func TestGenerateDomains_HappyPath(t *testing.T) {
	content := "domain\nexample.com\n"
	path := "test_domains.csv"

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test CSV: %v", err)
	}
	defer os.Remove(path)

	list, err := recon.GenerateDomains(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// DGA expands input heavily, so only assert >0
	if len(list) == 0 {
		t.Fatalf("expected at least one generated domain, got zero")
	}
}

func TestGenerateDomains_FileNotFound(t *testing.T) {
	_, err := recon.GenerateDomains("nonexistent.csv")
	if err == nil {
		t.Fatalf("expected error for missing file")
	}
}

func TestGenerateDomains_EmptyFile(t *testing.T) {
	path := "empty.csv"
	if err := os.WriteFile(path, []byte(""), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	defer os.Remove(path)

	list, err := recon.GenerateDomains(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty CSV → normally 0 domains; if DGA library behaves differently,
	// we still allow it as long as no error occurs.
	if len(list) != 0 {
		t.Logf("empty CSV produced %d domains (allowed)", len(list))
	}
}

func TestGenerateDomains_MalformedCSV(t *testing.T) {
	path := "bad.csv"
	if err := os.WriteFile(path, []byte("\x00\x01\x02notcsv"), 0600); err != nil {
		t.Fatalf("failed to write malformed file: %v", err)
	}
	defer os.Remove(path)

	_, err := recon.GenerateDomains(path)

	// DGA generator does NOT error on malformed CSV, it only errors on I/O
	if err != nil {
		t.Fatalf("unexpected error for malformed CSV: %v", err)
	}
}

func TestGenerateScoredDomains_HappyPath(t *testing.T) {
	content := "domain\nexample\n"
	path := "scored_domains.csv"

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test CSV: %v", err)
	}
	defer os.Remove(path)

	list, err := recon.GenerateScoredDomains(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) == 0 {
		t.Fatalf("expected scored generated domains, got zero")
	}

	row := list[0]
	if row.Domain == "" {
		t.Fatalf("expected domain in scored output")
	}
	if row.RiskScore < 0 || row.RiskScore > 1 {
		t.Fatalf("risk score must be normalized, got %f", row.RiskScore)
	}
	if row.Confidence == "" {
		t.Fatalf("expected non-empty confidence")
	}
	if row.GeneratedBy == "" {
		t.Fatalf("expected generated_by to be set")
	}
}

func TestGenerateScoredDomains_FileNotFound(t *testing.T) {
	_, err := recon.GenerateScoredDomains("missing_scored.csv")
	if err == nil {
		t.Fatalf("expected error for missing scored-domain input file")
	}
}

func TestIsValidDomain_RejectsIllegalChars(t *testing.T) {
	cases := []struct {
		domain string
		valid  bool
	}{
		{domain: "example.in", valid: true},
		{domain: "&mem.in", valid: false},
		{domain: "a-b.in", valid: false},
		{domain: `a\\b.in`, valid: false},
	}

	for _, tc := range cases {
		got := recon.IsValidDomain(tc.domain)
		if got != tc.valid {
			t.Fatalf("IsValidDomain(%q)=%v want=%v", tc.domain, got, tc.valid)
		}
	}
}

func TestIsHumanLikeDomain_FiltersMachineLikeLabels(t *testing.T) {
	if recon.IsHumanLikeDomain("x9q2z8m1.in") {
		t.Fatalf("expected machine-like domain to be rejected")
	}
	if !recon.IsHumanLikeDomain("securepay.in") {
		t.Fatalf("expected attacker-like readable domain to pass")
	}
}
