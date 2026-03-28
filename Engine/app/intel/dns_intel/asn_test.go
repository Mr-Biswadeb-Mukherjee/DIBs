package dns_intel

import (
	"context"
	"errors"
	"testing"
)

type fakeASNLookup struct {
	records map[string]ASNRecord
}

func (f *fakeASNLookup) Lookup(_ context.Context, ip string) (ASNRecord, error) {
	record, ok := f.records[ip]
	if !ok {
		return ASNRecord{}, errors.New("not found")
	}
	return record, nil
}

func TestParseASNLineExtractsFields(t *testing.T) {
	line := "15169 | 8.8.8.8 | 8.8.8.0/24 | US | arin | 1992-12-01 | GOOGLE - Google LLC"
	record, ok := parseASNLine(line)
	if !ok {
		t.Fatal("expected parseASNLine to parse data row")
	}
	if record.ASN != "AS15169" {
		t.Fatalf("unexpected asn: %q", record.ASN)
	}
	if record.IP != "8.8.8.8" {
		t.Fatalf("unexpected ip: %q", record.IP)
	}
	if record.Prefix != "8.8.8.0/24" {
		t.Fatalf("unexpected prefix: %q", record.Prefix)
	}
	if record.ASName == "" {
		t.Fatal("expected as_name to be populated")
	}
}

func TestParseASNLineRejectsHeaders(t *testing.T) {
	if _, ok := parseASNLine("AS | IP | BGP Prefix | CC | Registry | Allocated | AS Name"); ok {
		t.Fatal("header row must not parse as ASN record")
	}
	if _, ok := parseASNLine("NA | NA | NA | ZZ"); ok {
		t.Fatal("invalid row must not parse as ASN record")
	}
}

func TestCombineIPsNormalizesAndDeduplicates(t *testing.T) {
	ips := combineIPs(
		[]string{" 8.8.8.8 ", "8.8.8.8", "invalid"},
		[]string{"2001:4860:4860::8888", "2001:4860:4860::8888"},
	)
	if len(ips) != 2 {
		t.Fatalf("unexpected ip count: %d", len(ips))
	}
	if ips[0] != "8.8.8.8" {
		t.Fatalf("unexpected first ip: %q", ips[0])
	}
	if ips[1] != "2001:4860:4860::8888" {
		t.Fatalf("unexpected second ip: %q", ips[1])
	}
}

func TestLookupASNsCollectsUniqueNormalizedRecords(t *testing.T) {
	lookup := &fakeASNLookup{
		records: map[string]ASNRecord{
			"8.8.8.8": {
				IP:     "8.8.8.8",
				ASN:    "15169",
				Prefix: "8.8.8.0/24",
				ASName: "GOOGLE - Google LLC",
			},
			"1.1.1.1": {
				IP:  "1.1.1.1",
				ASN: "AS13335",
			},
		},
	}
	p := &Processor{asn: lookup}

	records := p.lookupASNs(context.Background(), []string{"8.8.8.8", "1.1.1.1", "8.8.8.8", "invalid"})
	if len(records) != 2 {
		t.Fatalf("expected two unique ASN records, got %d", len(records))
	}
	if records[0].ASN != "AS13335" || records[0].IP != "1.1.1.1" {
		t.Fatalf("unexpected sorted first record: %#v", records[0])
	}
	if records[1].ASN != "AS15169" || records[1].IP != "8.8.8.8" {
		t.Fatalf("unexpected sorted second record: %#v", records[1])
	}
}
