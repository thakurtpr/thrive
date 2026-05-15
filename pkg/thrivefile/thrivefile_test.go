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
