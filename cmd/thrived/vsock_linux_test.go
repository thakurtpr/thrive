//go:build linux

package main

import (
	"testing"
)

func TestVsockPort_ReadsEnvVar(t *testing.T) {
	t.Setenv("THRIVE_VSOCK_PORT", "2048")
	got := vsockPort()
	if got != 2048 {
		t.Errorf("expected vsockPort()=2048, got %d", got)
	}
}

func TestVsockPort_DefaultsTo1024(t *testing.T) {
	t.Setenv("THRIVE_VSOCK_PORT", "")
	got := vsockPort()
	if got != 1024 {
		t.Errorf("expected vsockPort()=1024, got %d", got)
	}
}

func TestVsockPort_InvalidEnvReturnsZero(t *testing.T) {
	t.Setenv("THRIVE_VSOCK_PORT", "notanumber")
	got := vsockPort()
	if got != 0 {
		t.Errorf("expected vsockPort()=0 for invalid env, got %d", got)
	}
}
