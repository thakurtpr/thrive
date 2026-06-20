//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// thriveBinary returns the path to the thrive binary under test.
// Resolution order:
//  1. THRIVE_BINARY env var (explicit override)
//  2. ../../thrive relative to the test directory (repo root binary)
//  3. thrive found on $PATH
func thriveBinary(t *testing.T) string {
	t.Helper()
	if env := os.Getenv("THRIVE_BINARY"); env != "" {
		if _, statErr := os.Stat(env); statErr == nil {
			return env
		}
		t.Fatalf("THRIVE_BINARY=%q does not exist", env)
	}

	// tests/integration/ → tests/ → repo root
	dir, err := filepath.Abs("../..")
	if err == nil {
		candidate := filepath.Join(dir, "thrive")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
	}

	// Fallback: search PATH
	p, lookErr := exec.LookPath("thrive")
	if lookErr != nil {
		t.Fatalf("thrive binary not found: set THRIVE_BINARY env var or place binary in PATH")
	}
	return p
}

// thriveCLI runs the thrive binary with the given arguments using the provided
// context and returns (stdout, stderr, error). Both streams are logged with
// t.Logf so output is visible in verbose test runs.
func thriveCLI(ctx context.Context, t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	bin := thriveBinary(t)
	cmd := exec.CommandContext(ctx, bin, args...)

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	runErr := cmd.Run()

	stdout = outBuf.String()
	stderr = errBuf.String()

	t.Logf("$ thrive %s\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), stdout, stderr)

	return stdout, stderr, runErr
}

// mustPull pulls the given image reference and fails the test immediately if
// an error occurs. Returns the stdout from the pull command.
func mustPull(t *testing.T, image string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	stdout, stderr, err := thriveCLI(ctx, t, "pull", image)
	if err != nil {
		t.Fatalf("mustPull(%q): pull failed: %v\nstdout: %s\nstderr: %s", image, err, stdout, stderr)
	}
	return stdout
}

// mustRun runs a container from the given image with extra args and waits for
// it to finish. The container is run with --rm so it is cleaned up on exit.
// Fails the test immediately on error and returns stdout.
func mustRun(t *testing.T, image string, args ...string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmdArgs := append([]string{"run", "--rm", image}, args...)
	stdout, stderr, err := thriveCLI(ctx, t, cmdArgs...)
	if err != nil {
		t.Fatalf("mustRun(%q %v): %v\nstdout: %s\nstderr: %s", image, args, err, stdout, stderr)
	}
	return stdout
}

// imageInList returns true when `thrive images` output contains the given ref.
func imageInList(t *testing.T, ref string) bool {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stdout, _, err := thriveCLI(ctx, t, "images")
	if err != nil {
		t.Logf("imageInList: images command failed: %v", err)
		return false
	}
	return strings.Contains(stdout, ref)
}

// requireRoot skips the test immediately when the process is not root.
// Namespace operations used by the runtime require root privileges.
func requireRoot(t *testing.T) {
	t.Helper()
	if os.Getuid() != 0 {
		t.Skip("integration tests require root (os.Getuid() != 0)")
	}
}

// imagesBaseDir is the on-disk directory where thrive stores image manifests
// and layer directories. Matches the hard-coded path in internal/image/image.go.
const imagesBaseDir = "/var/lib/thrive/images"

// containersBaseDir is the on-disk directory where thrive stores per-container
// state. Matches the hard-coded path in internal/runtime/runtime.go.
const containersBaseDir = "/run/thrive/containers"

// safeRef converts an image reference to the filesystem-safe directory name
// that thrive uses on disk. Mirrors image.SafeRef from internal/image/types.go.
func safeRef(ref string) string {
	return strings.NewReplacer("/", "_", ":", "_", "@", "_").Replace(ref)
}

// uniqueContainerID generates a test-scoped container name that avoids
// collisions between parallel tests by embedding the test name and PID.
func uniqueContainerID(t *testing.T, suffix string) string {
	t.Helper()
	safe := strings.NewReplacer(
		"/", "-",
		":", "-",
		" ", "-",
		"(", "",
		")", "",
	).Replace(t.Name())
	return fmt.Sprintf("%s-%s-%d", safe, suffix, os.Getpid())
}
