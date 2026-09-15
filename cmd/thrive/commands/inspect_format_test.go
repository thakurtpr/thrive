package commands

import (
	"testing"
)

// TestRenderFormat verifies Go-template rendering of inspect data.
func TestRenderFormat(t *testing.T) {
	data := map[string]any{"id": "abc123", "status": "running"}
	out, err := renderFormat("{{.id}} {{.status}}", data)
	if err != nil {
		t.Fatalf("renderFormat: %v", err)
	}
	if out != "abc123 running" {
		t.Errorf("renderFormat: got %q", out)
	}
}

// TestRenderFormat_Nested verifies nested field access.
func TestRenderFormat_Nested(t *testing.T) {
	data := map[string]any{"config": map[string]any{"Image": "alpine"}}
	out, err := renderFormat("{{.config.Image}}", data)
	if err != nil {
		t.Fatalf("renderFormat: %v", err)
	}
	if out != "alpine" {
		t.Errorf("renderFormat: got %q", out)
	}
}

// TestRenderFormat_BadTemplate verifies template errors surface.
func TestRenderFormat_BadTemplate(t *testing.T) {
	if _, err := renderFormat("{{.unclosed", map[string]any{}); err == nil {
		t.Error("renderFormat: expected error for bad template, got nil")
	}
}
