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
	tests := []struct {
		input string
		want  []string
	}{
		{"Robert", []string{"R163"}},
		{" robert ", []string{"R163"}},
		{"", []string{""}},
	}

	for _, tt := range tests {
		got := Soundsquat(tt.input)

		if len(got) != len(tt.want) {
			t.Fatalf("Soundsquat(%q) length mismatch: got %v, want %v", tt.input, got, tt.want)
		}

		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("Soundsquat(%q)[%d] = %q; want %q", tt.input, i, got[i], tt.want[i])
			}
		}
	}
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
