//go:build linux

package main

import (
	"syscall"
	"testing"
)

// TestIntOpt_Types verifies tolerant number coercion for bridge maps
// (malformed input yields 0 instead of panicking the daemon).
func TestIntOpt_Types(t *testing.T) {
	m := map[string]any{
		"f64": float64(8080), "f32": float32(80), "i": 1, "i64": int64(2),
		"s": "not-a-number", "b": true, "nil": nil,
	}
	for k, want := range map[string]int{
		"f64": 8080, "f32": 80, "i": 1, "i64": 2,
		"s": 0, "b": 0, "nil": 0, "missing": 0,
	} {
		if got := intOpt(m, k); got != want {
			t.Errorf("intOpt(%q): got %d want %d", k, got, want)
		}
	}
}

// TestParseSignalString verifies docker-style signal mapping for the
// kill/stop bridge handlers (Linux CI; compile-checked on macOS).
func TestParseSignalString(t *testing.T) {
	cases := []struct {
		in   string
		want syscall.Signal
	}{
		{"", syscall.SIGKILL},
		{"KILL", syscall.SIGKILL},
		{"TERM", syscall.SIGTERM},
		{"9", syscall.Signal(9)},
		{"15", syscall.Signal(15)},
		{"BOGUS", syscall.SIGKILL},
	}
	for _, tc := range cases {
		if got := parseSignalString(tc.in); got != tc.want {
			t.Errorf("parseSignalString(%q): got %v want %v", tc.in, got, tc.want)
		}
	}
}
