// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package apis

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const DefaultEndpointContractPath = "APIs/Endpoint.ndjson"

type EndpointContract struct {
	APIKeyHeader string
	Routes       map[string]RouteSpec
}

type RouteSpec struct {
	Name       string
	Method     string
	Path       string
	Auth       bool
	RatePerSec int
	RatePerMin int
}

type contractRecord struct {
	Kind         string `json:"kind"`
	APIKeyHeader string `json:"api_key_header,omitempty"`
	Name         string `json:"name,omitempty"`
	Method       string `json:"method,omitempty"`
	Path         string `json:"path,omitempty"`
	Auth         *bool  `json:"auth,omitempty"`
	RatePerSec   int    `json:"rate_per_sec,omitempty"`
	RatePerMin   int    `json:"rate_per_min,omitempty"`
}

var requiredRouteNames = []string{
	"health",
	"start",
	"stop",
	"status",
	"metrics",
	"events",
}

func LoadEndpointContract(path string) (EndpointContract, error) {
	f, err := os.Open(path)
	if err != nil {
		return EndpointContract{}, err
	}
	defer f.Close()

	contract := EndpointContract{Routes: make(map[string]RouteSpec)}
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if err := applyContractLine(&contract, line, lineNo); err != nil {
			return EndpointContract{}, err
		}
	}
	if err := scanner.Err(); err != nil {
		return EndpointContract{}, err
	}
	return contract, validateContract(contract)
}

func applyContractLine(contract *EndpointContract, line string, lineNo int) error {
	var rec contractRecord
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		return fmt.Errorf("endpoint contract line %d parse error: %w", lineNo, err)
	}
	kind := strings.ToLower(strings.TrimSpace(rec.Kind))
	switch kind {
	case "config":
		return applyConfigRecord(contract, rec, lineNo)
	case "route":
		return applyRouteRecord(contract, rec, lineNo)
	default:
		return fmt.Errorf("endpoint contract line %d invalid kind: %q", lineNo, rec.Kind)
	}
}

func applyConfigRecord(contract *EndpointContract, rec contractRecord, lineNo int) error {
	header := strings.TrimSpace(rec.APIKeyHeader)
	if header == "" {
		return fmt.Errorf("endpoint contract line %d missing api_key_header", lineNo)
	}
	contract.APIKeyHeader = header
	return nil
}

func applyRouteRecord(contract *EndpointContract, rec contractRecord, lineNo int) error {
	spec, err := buildRouteSpec(rec)
	if err != nil {
		return fmt.Errorf("endpoint contract line %d %w", lineNo, err)
	}
	if _, exists := contract.Routes[spec.Name]; exists {
		return fmt.Errorf("endpoint contract line %d duplicate route name: %s", lineNo, spec.Name)
	}
	contract.Routes[spec.Name] = spec
	return nil
}

func buildRouteSpec(rec contractRecord) (RouteSpec, error) {
	name := strings.TrimSpace(rec.Name)
	method := strings.ToUpper(strings.TrimSpace(rec.Method))
	path := strings.TrimSpace(rec.Path)
	if name == "" {
		return RouteSpec{}, errors.New("missing route name")
	}
	if method == "" {
		return RouteSpec{}, fmt.Errorf("route %s missing method", name)
	}
	if path == "" {
		return RouteSpec{}, fmt.Errorf("route %s missing path", name)
	}
	if rec.RatePerSec <= 0 {
		return RouteSpec{}, fmt.Errorf("route %s missing or invalid rate_per_sec", name)
	}
	if rec.RatePerMin <= 0 {
		return RouteSpec{}, fmt.Errorf("route %s missing or invalid rate_per_min", name)
	}
	auth := false
	if rec.Auth != nil {
		auth = *rec.Auth
	}
	return RouteSpec{
		Name:       name,
		Method:     method,
		Path:       path,
		Auth:       auth,
		RatePerSec: rec.RatePerSec,
		RatePerMin: rec.RatePerMin,
	}, nil
}

func validateContract(contract EndpointContract) error {
	if strings.TrimSpace(contract.APIKeyHeader) == "" {
		return errors.New("endpoint contract missing api key header config")
	}
	for _, name := range requiredRouteNames {
		if _, ok := contract.Routes[name]; ok {
			continue
		}
		return fmt.Errorf("endpoint contract missing required route: %s", name)
	}
	return nil
}

func withCORS(next http.Handler, apiKeyHeader string) http.Handler {
	allow := strings.Join(
		[]string{"Content-Type", "Authorization", strings.TrimSpace(apiKeyHeader)},
		", ",
	)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", allow)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		fmt.Printf("api write failed: %v\n", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
