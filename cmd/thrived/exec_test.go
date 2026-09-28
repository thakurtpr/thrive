//go:build linux

package main

import (
	"syscall"
	"testing"
)

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
