package dns_intel

import "testing"

func TestParseWhoisRecordExtractsLifecycleFields(t *testing.T) {
	body := `
Domain Name: example.com
Registrar WHOIS Server: whois.registrar.example
Updated Date: 2026-02-06T09:10:11Z
Creation Date: 2026-01-20T01:02:03Z
`
	record := parseWhoisRecord(body)

	if record.RegistrarWhoisServer != "whois.registrar.example" {
		t.Fatalf("unexpected registrar whois server: %q", record.RegistrarWhoisServer)
	}
	if record.UpdatedDate != "2026-02-06" {
		t.Fatalf("unexpected updated date: %q", record.UpdatedDate)
	}
	if record.CreationDate != "2026-01-20" {
		t.Fatalf("unexpected creation date: %q", record.CreationDate)
	}
}

func TestParseReferralServerUsesIANAFields(t *testing.T) {
	body := `
domain:       COM
whois:        whois.verisign-grs.com
refer:        whois.verisign-grs.com
`
	server := parseReferralServer(body)
	if server != "whois.verisign-grs.com" {
		t.Fatalf("unexpected referral server: %q", server)
	}
}

func TestNormalizeWhoisDateHandlesDifferentLayouts(t *testing.T) {
	cases := map[string]string{
		"2026-02-06T09:10:11Z": "2026-02-06",
		"2026-02-06 09:10:11":  "2026-02-06",
		"2026.02.06 09:10:11":  "2026-02-06",
		"2026-02-06":           "2026-02-06",
	}
	for raw, want := range cases {
		if got := normalizeWhoisDate(raw); got != want {
			t.Fatalf("normalizeWhoisDate(%q) = %q, want %q", raw, got, want)
		}
	}
}
