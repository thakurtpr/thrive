//go:build linux
// +build linux

package thrivefile

import (
	"os"
	"testing"
)

func TestParse(t *testing.T) {
	content := `
name: testapp
base: ubuntu:22.04

steps:
  install-deps:
    run: apt-get update && apt-get install -y curl

  copy-package:
    depends-on: [install-deps]
    copy:
      - src: package.json
        dst: /app/package.json

  npm-install:
    depends-on: [copy-package]
    run: cd /app && npm ci
`
	tmpFile, err := os.CreateTemp("", "thrivefile-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	graph, err := Parse(tmpFile.Name())
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if graph.BaseImage != "ubuntu:22.04" {
		t.Errorf("Expected BaseImage 'ubuntu:22.04', got '%s'", graph.BaseImage)
	}

	if len(graph.Steps) != 3 {
		t.Errorf("Expected 3 steps, got %d", len(graph.Steps))
	}

	if graph.Steps["install-deps"].Run == "" {
		t.Error("install-deps step missing Run command")
	}

	if len(graph.Steps["npm-install"].DependsOn) != 1 {
		t.Errorf("Expected 1 dependency for npm-install, got %d", len(graph.Steps["npm-install"].DependsOn))
	}
}

func TestParseNotFound(t *testing.T) {
	_, err := Parse("/nonexistent/thrivefile.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestParseInvalidYAML(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "thrivefile-bad-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	// Write deliberately malformed YAML
	if _, err := tmpFile.WriteString("{invalid::: yaml content"); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// YAML is lenient — if it doesn't error, the graph should be non-nil
	graph, err := Parse(tmpFile.Name())
	if err != nil {
		// Some YAML parsers reject this; that is acceptable
		return
	}
	// If no error, graph must be non-nil
	if graph == nil {
		t.Fatal("Parse: returned nil graph for malformed input that did not error")
	}
}

func TestParseEmpty(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "thrivefile-empty-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	graph, err := Parse(tmpFile.Name())
	if err != nil {
		t.Fatalf("Parse empty file: unexpected error: %v", err)
	}
	if graph == nil {
		t.Fatal("Parse empty file: returned nil graph")
	}
}

func TestParseWithCopySpec(t *testing.T) {
	content := `
name: copyapp
base: alpine:3.19
steps:
  copy-files:
    copy:
      - src: ./app
        dst: /app
`
	tmpFile, err := os.CreateTemp("", "thrivefile-copy-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	graph, err := Parse(tmpFile.Name())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	step, ok := graph.Steps["copy-files"]
	if !ok {
		t.Fatal("missing 'copy-files' step")
	}
	if len(step.Copy) != 1 {
		t.Fatalf("expected 1 CopySpec, got %d", len(step.Copy))
	}
	if step.Copy[0].Source != "./app" {
		t.Errorf("CopySpec.Source: got %q, want %q", step.Copy[0].Source, "./app")
	}
	if step.Copy[0].Dest != "/app" {
		t.Errorf("CopySpec.Dest: got %q, want %q", step.Copy[0].Dest, "/app")
	}
}

func TestParseWithEntrypoint(t *testing.T) {
	content := `
name: entryapp
base: debian:bookworm-slim
entrypoint: ["/usr/bin/myapp", "--port", "8080"]
steps:
  build:
    run: make install
`
	tmpFile, err := os.CreateTemp("", "thrivefile-entry-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	graph, err := Parse(tmpFile.Name())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(graph.EntryCmd) != 3 {
		t.Errorf("EntryCmd: got %v, want 3 elements", graph.EntryCmd)
	}
	if graph.EntryCmd[0] != "/usr/bin/myapp" {
		t.Errorf("EntryCmd[0]: got %q, want /usr/bin/myapp", graph.EntryCmd[0])
	}
}
