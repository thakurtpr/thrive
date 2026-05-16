//go:build linux
// +build linux

package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thakurprasadrout/thrive/pkg/thrivefile"
)

// TestCacheKey_Deterministic verifies that identical inputs always produce
// the same cache key.
func TestCacheKey_Deterministic(t *testing.T) {
	// Arrange
	step := &thrivefile.Step{
		Name:      "compile",
		Run:       "go build ./...",
		DependsOn: []string{},
	}
	baseImage := "golang:1.22"
	inputs := map[string]string{}

	// Act
	key1, err := CacheKey(step, baseImage, inputs)
	if err != nil {
		t.Fatalf("CacheKey first call: %v", err)
	}

	key2, err := CacheKey(step, baseImage, inputs)
	if err != nil {
		t.Fatalf("CacheKey second call: %v", err)
	}

	// Assert
	if key1 != key2 {
		t.Errorf("CacheKey not deterministic: %q != %q", key1, key2)
	}
}

// TestCacheKey_DifferentInputs verifies that steps with different Run commands
// produce distinct cache keys.
func TestCacheKey_DifferentInputs(t *testing.T) {
	// Arrange
	baseImage := "golang:1.22"
	inputs := map[string]string{}

	stepA := &thrivefile.Step{Run: "go build ./..."}
	stepB := &thrivefile.Step{Run: "go test ./..."}

	// Act
	keyA, err := CacheKey(stepA, baseImage, inputs)
	if err != nil {
		t.Fatalf("CacheKey stepA: %v", err)
	}

	keyB, err := CacheKey(stepB, baseImage, inputs)
	if err != nil {
		t.Fatalf("CacheKey stepB: %v", err)
	}

	// Assert
	if keyA == keyB {
		t.Errorf("expected distinct keys for different steps, both produced %q", keyA)
	}
}

// TestParseThrivefile_Valid writes a minimal Thrivefile to disk and verifies
// that ParseThrivefile parses it without error and extracts the base image.
func TestParseThrivefile_Valid(t *testing.T) {
	// Arrange
	content := `
base: golang:1.22
steps:
  build:
    run: go build ./...
`
	dir := t.TempDir()
	path := filepath.Join(dir, "Thrivefile")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Act
	graph, err := ParseThrivefile(path)

	// Assert
	if err != nil {
		t.Fatalf("ParseThrivefile: unexpected error: %v", err)
	}
	if graph == nil {
		t.Fatal("ParseThrivefile: returned nil graph")
	}
	if graph.BaseImage != "golang:1.22" {
		t.Errorf("BaseImage: got %q, want %q", graph.BaseImage, "golang:1.22")
	}
	if _, ok := graph.Steps["build"]; !ok {
		t.Error("Steps: missing 'build' step")
	}
}
