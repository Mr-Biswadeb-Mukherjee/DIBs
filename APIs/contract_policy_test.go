package apis

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEndpointContractIncludesRatePolicies(t *testing.T) {
	content := "" +
		`{"kind":"config","api_key_header":"X-API-Key"}` + "\n" +
		`{"kind":"route","name":"health","method":"GET","path":"/healthz","auth":false,"rate_per_sec":10,"rate_per_min":200}` + "\n" +
		`{"kind":"route","name":"start","method":"POST","path":"/api/v3/start","auth":true,"rate_per_sec":2,"rate_per_min":20}` + "\n" +
		`{"kind":"route","name":"stop","method":"POST","path":"/api/v3/stop","auth":true,"rate_per_sec":2,"rate_per_min":20}` + "\n" +
		`{"kind":"route","name":"status","method":"GET","path":"/api/v3/status","auth":true,"rate_per_sec":10,"rate_per_min":200}` + "\n" +
		`{"kind":"route","name":"metrics","method":"GET","path":"/api/v3/metrics","auth":true,"rate_per_sec":10,"rate_per_min":200}` + "\n" +
		`{"kind":"route","name":"events","method":"GET","path":"/api/v3/events.ndjson","auth":true,"rate_per_sec":5,"rate_per_min":100}` + "\n" +
		`{"kind":"route","name":"details","method":"GET","path":"/api/v3/details","auth":true,"rate_per_sec":5,"rate_per_min":100}` + "\n"

	path := filepath.Join(t.TempDir(), "endpoint.ndjson")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write contract failed: %v", err)
	}

	contract, err := LoadEndpointContract(path)
	if err != nil {
		t.Fatalf("LoadEndpointContract error: %v", err)
	}
	spec := contract.Routes["events"]
	if spec.RatePerSec != 5 || spec.RatePerMin != 100 {
		t.Fatalf("unexpected events policy: %#v", spec)
	}
}

func TestLoadEndpointContractRejectsMissingRatePolicy(t *testing.T) {
	content := "" +
		`{"kind":"config","api_key_header":"X-API-Key"}` + "\n" +
		`{"kind":"route","name":"health","method":"GET","path":"/healthz","auth":false}` + "\n"

	path := filepath.Join(t.TempDir(), "endpoint.ndjson")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write contract failed: %v", err)
	}

	if _, err := LoadEndpointContract(path); err == nil {
		t.Fatal("expected error for missing rate policy")
	}
}

func TestLoadEndpointContractFallsBackToEmbeddedDefault(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd failed: %v", err)
	}
	t.Cleanup(func() {
		if chErr := os.Chdir(wd); chErr != nil {
			t.Fatalf("restore working directory failed: %v", chErr)
		}
	})
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	contract, err := LoadEndpointContract(DefaultEndpointContractPath)
	if err != nil {
		t.Fatalf("LoadEndpointContract error: %v", err)
	}
	if contract.APIKeyHeader == "" {
		t.Fatal("expected api key header from embedded contract")
	}
	if _, ok := contract.Routes["health"]; !ok {
		t.Fatal("expected health route from embedded contract")
	}
}
