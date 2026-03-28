package runtime

import "testing"

func TestASNClusterIndexEmitsOnSharedInfrastructure(t *testing.T) {
	index := newASNClusterIndex()
	records := []IntelASNRecord{
		{IP: "1.1.1.1", ASN: "13335", Prefix: "1.1.1.0/24", ASName: "CLOUDFLARENET"},
	}

	if got := index.add("alpha.example", records); len(got) != 0 {
		t.Fatalf("single domain must not create cluster output: %#v", got)
	}

	found := index.add("beta.example", records)
	if len(found) != 1 {
		t.Fatalf("expected one cluster output, got %d", len(found))
	}
	if found[0].ASN != "AS13335" || found[0].IP != "1.1.1.1" {
		t.Fatalf("unexpected cluster identity: %#v", found[0])
	}
	if found[0].ClusterSize != 2 {
		t.Fatalf("unexpected cluster size: %d", found[0].ClusterSize)
	}
	if len(found[0].Domains) != 2 || found[0].Domains[0] != "alpha.example" || found[0].Domains[1] != "beta.example" {
		t.Fatalf("unexpected clustered domains: %#v", found[0].Domains)
	}

	if got := index.add("beta.example", records); len(got) != 0 {
		t.Fatalf("duplicate domain must not emit cluster output: %#v", got)
	}

	grown := index.add("gamma.example", records)
	if len(grown) != 1 {
		t.Fatalf("expected one updated cluster output, got %d", len(grown))
	}
	if grown[0].ClusterSize != 3 {
		t.Fatalf("expected cluster size 3, got %d", grown[0].ClusterSize)
	}
}

func TestIntelRecordToNDJSONIncludesASNEntries(t *testing.T) {
	record := IntelRecord{
		Domain: "dns.google",
		ASNs: []IntelASNRecord{
			{
				IP:     " 8.8.8.8 ",
				ASN:    " AS15169 ",
				Prefix: "8.8.8.0/24",
				ASName: "GOOGLE - Google LLC",
			},
		},
	}

	out := intelRecordToNDJSON(record)
	if len(out.ASNs) != 1 {
		t.Fatalf("expected one ASN record in ndjson output, got %d", len(out.ASNs))
	}
	if out.ASNs[0].IP != "8.8.8.8" {
		t.Fatalf("unexpected ndjson asn ip: %q", out.ASNs[0].IP)
	}
	if out.ASNs[0].ASN != "AS15169" {
		t.Fatalf("unexpected ndjson asn number: %q", out.ASNs[0].ASN)
	}
}
