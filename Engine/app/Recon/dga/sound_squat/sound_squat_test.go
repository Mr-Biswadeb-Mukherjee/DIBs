// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Biswadeb Mukherjee

package soundsquat

import (
	"testing"
)

// -------------------------------------------------------------
// Test normalizeInternal
// -------------------------------------------------------------
func TestNormalizeInternal(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "HELLO"},
		{" Hello ", "HELLO"},
		{"\tworld\n", "WORLD"},
		{"MiXeD", "MIXED"},
		{"", ""},
	}

	for _, tt := range tests {
		got := normalizeInternal(tt.input)
		if got != tt.want {
			t.Errorf("normalizeInternal(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

// -------------------------------------------------------------
// Test soundexInternal - standard cases
// -------------------------------------------------------------
func TestSoundexInternal_Basic(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Robert", "R163"},
		{"Rupert", "R163"},
		{"Rubin", "R150"},
		{"Ashcraft", "A226"},
		{"Tymczak", "T522"},
		{"Pfister", "P123"},
	}

	for _, tt := range tests {
		got := soundexInternal(tt.input)
		if got != tt.want {
			t.Errorf("soundexInternal(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

// -------------------------------------------------------------
// Edge cases
// -------------------------------------------------------------
func TestSoundexInternal_EdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"spaces only", "   ", ""},
		{"non letters", "12345", "1000"}, // first rune = '1'
		{"symbols mixed", "A!@#B", "A100"},
		{"single char", "A", "A000"},
		{"unicode letters", "Éclair", "É246"}, // depends on rune mapping
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := soundexInternal(tt.input)
			if got != tt.want {
				t.Errorf("soundexInternal(%q) = %q; want %q", tt.input, got, tt.want)
			}
		})
	}
}

// -------------------------------------------------------------
// Deduplication behavior (critical logic)
// -------------------------------------------------------------
func TestSoundexInternal_DuplicateSuppression(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"BFPV", "B100"}, // all map to same code '1'
		{"BBB", "B100"},
		{"BCDFG", "B231"}, // distinct transitions
	}

	for _, tt := range tests {
		got := soundexInternal(tt.input)
		if got != tt.want {
			t.Errorf("soundexInternal(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

// -------------------------------------------------------------
// Test padding behavior
// -------------------------------------------------------------
func TestSoundexInternal_Padding(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"A", "A000"},
		{"AB", "A100"},
		{"ABC", "A120"},
	}

	for _, tt := range tests {
		got := soundexInternal(tt.input)
		if got != tt.want {
			t.Errorf("soundexInternal(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

// -------------------------------------------------------------
// Public API test
// -------------------------------------------------------------
func TestSoundsquat(t *testing.T) {
	got := Soundsquat("phone")
	if len(got) == 0 {
		t.Fatal("Soundsquat should emit phoneme variants")
	}
	if !containsString(got, "fone") {
		t.Fatalf("expected phoneme variant %q in %v", "fone", got)
	}
	if containsString(got, "P500") {
		t.Fatalf("expected label variants, not soundex code: %v", got)
	}
}

func TestSoundsquat_EmptyInput(t *testing.T) {
	got := Soundsquat("  ")
	if len(got) != 0 {
		t.Fatalf("expected empty output for empty input, got %v", got)
	}
}

func TestSoundsquat_Deduplicates(t *testing.T) {
	got := Soundsquat("zoo")
	if len(got) == 0 {
		t.Fatal("expected non-empty output for zoo")
	}

	seen := make(map[string]struct{}, len(got))
	for _, variant := range got {
		if _, ok := seen[variant]; ok {
			t.Fatalf("duplicate variant found: %q", variant)
		}
		seen[variant] = struct{}{}
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

// -------------------------------------------------------------
// Determinism test (important for security tooling)
// -------------------------------------------------------------
func TestSoundexInternal_Deterministic(t *testing.T) {
	input := "Security"

	first := soundexInternal(input)
	for i := 0; i < 1000; i++ {
		if soundexInternal(input) != first {
			t.Fatalf("Non-deterministic output detected")
		}
	}
}
