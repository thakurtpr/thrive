//go:build linux
// +build linux

package plugin

import (
	"testing"
)

func testOverride(t *testing.T) {
	t.Helper()
	pluginsDirOverride = t.TempDir()
	t.Cleanup(func() { pluginsDirOverride = "" })
}

// TestInstallEnableDisableRemove verifies the plugin state machine.
func TestInstallEnableDisableRemove(t *testing.T) {
	testOverride(t)
	src := t.TempDir()
	p, err := Install("demo", src)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if p.Enabled {
		t.Error("new plugin should be disabled")
	}
	if _, err := Install("demo", src); err == nil {
		t.Error("Install duplicate: expected error, got nil")
	}
	if err := Enable("demo"); err != nil {
		t.Fatalf("Enable: %v", err)
	}
	if err := Remove("demo", false); err == nil {
		t.Error("Remove enabled: expected error, got nil")
	}
	if err := Disable("demo"); err != nil {
		t.Fatalf("Disable: %v", err)
	}
	if err := Remove("demo", false); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if err := Enable("demo"); err == nil {
		t.Error("Enable missing: expected error, got nil")
	}
}

// TestInstall_Validation verifies name and source checks.
func TestInstall_Validation(t *testing.T) {
	testOverride(t)
	if _, err := Install("bad name!", t.TempDir()); err == nil {
		t.Error("Install bad name: expected error, got nil")
	}
	if _, err := Install("demo", "/no/such/src"); err == nil {
		t.Error("Install missing src: expected error, got nil")
	}
}
