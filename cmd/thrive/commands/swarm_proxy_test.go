//go:build !linux

package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadComposeSpec verifies file reads and the missing-file error.
func TestReadComposeSpec(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "compose.yml")
	if err := os.WriteFile(p, []byte("services:\n  web:\n"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := readComposeSpec(p)
	if err != nil || !strings.Contains(got, "services:") {
		t.Errorf("readComposeSpec: got %q, %v", got, err)
	}
	if _, err := readComposeSpec(filepath.Join(dir, "missing.yml")); err == nil {
		t.Error("readComposeSpec missing: expected error, got nil")
	}
}

// captureStdout swaps os.Stdout for a test, returning captured output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	prev := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	defer func() { os.Stdout = prev }()
	fn()
	w.Close()
	os.Stdout = prev
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// TestPrintResultJSON verifies pretty-printed output and silent invalid input.
func TestPrintResultJSON(t *testing.T) {
	out := captureStdout(t, func() {
		printResultJSON([]byte(`{"a":1,"b":[1,2]}`))
	})
	if !strings.Contains(out, `"a": 1`) || !strings.Contains(out, "\"b\": [") {
		t.Errorf("printResultJSON: got %q", out)
	}
	out = captureStdout(t, func() {
		printResultJSON([]byte(`{bogus`))
	})
	if out != "{bogus" {
		t.Errorf("printResultJSON invalid: got %q, want raw passthrough", out)
	}
}
