package commands

import (
	"testing"
)

// TestParseComposeServiceNames verifies sorted extraction, empty specs,
// and invalid YAML handling.
func TestParseComposeServiceNames(t *testing.T) {
	got, err := parseComposeServiceNames("services:\n  web:\n    image: x\n  db:\n    image: y\n")
	if err != nil {
		t.Fatalf("valid spec: %v", err)
	}
	if len(got) != 2 || got[0] != "db" || got[1] != "web" {
		t.Errorf("sorted names: got %v", got)
	}
	got, err = parseComposeServiceNames("services: {}\n")
	if err != nil || len(got) != 0 {
		t.Errorf("empty services: got %v, %v", got, err)
	}
	got, err = parseComposeServiceNames("")
	if err != nil || len(got) != 0 {
		t.Errorf("empty spec: got %v, %v", got, err)
	}
	if _, err := parseComposeServiceNames(":\n\tbad"); err == nil {
		t.Error("invalid YAML: expected error, got nil")
	}
}
