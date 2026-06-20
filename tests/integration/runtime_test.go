//go:build integration

// Package integration_test contains integration tests for the thrive runtime.
// These tests invoke the real thrive binary and exercise actual image pull,
// container run, lifecycle management, and system commands. They must run
// on a Linux machine (or privileged container) as root.
//
// Run with:
//
//	go test -tags integration -v ./tests/integration/ -count=1
//
// Override the binary path:
//
//	THRIVE_BINARY=/path/to/thrive go test -tags integration -v ./tests/integration/
package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Image pull tests
// ---------------------------------------------------------------------------

// imagesToTest lists OCI images exercised by the pull and run test suites.
var imagesToTest = []struct {
	name string
	ref  string
}{
	{"alpine", "alpine:3.19"},
	{"ubuntu", "ubuntu:22.04"},
	{"debian", "debian:12-slim"},
	{"busybox", "busybox:1.36"},
	{"python", "python:3.12-alpine"},
	{"node", "node:20-alpine"},
	{"nginx", "nginx:alpine"},
	{"redis", "redis:alpine"},
	{"golang", "golang:1.23-alpine"},
	{"rockylinux", "rockylinux:9-minimal"},
}

// TestImagePull_AllImages pulls each image in imagesToTest and verifies:
//   - the pull command exits 0
//   - the image reference appears in `thrive images` output
//   - the manifest.json is present on disk
//   - at least one layer directory was written to disk
func TestImagePull_AllImages(t *testing.T) {
	requireRoot(t)

	for _, tc := range imagesToTest {
		tc := tc // capture for parallel sub-test
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mustPull(t, tc.ref)

			if !imageInList(t, tc.ref) {
				t.Errorf("image %q not found in `thrive images` output after pull", tc.ref)
			}

			// Verify manifest.json written to disk.
			imgDir := filepath.Join(imagesBaseDir, safeRef(tc.ref))
			manifestPath := filepath.Join(imgDir, "manifest.json")
			if _, err := os.Stat(manifestPath); err != nil {
				t.Errorf("manifest.json not found at %q: %v", manifestPath, err)
			}

			// Verify at least one layer directory exists.
			layersDir := filepath.Join(imgDir, "layers")
			entries, err := os.ReadDir(layersDir)
			if err != nil {
				t.Errorf("layers directory not readable at %q: %v", layersDir, err)
			} else if len(entries) == 0 {
				t.Errorf("expected at least one layer in %q, got 0", layersDir)
			} else {
				t.Logf("image %s: %d layer(s) on disk", tc.ref, len(entries))
			}
		})
	}
}

// TestImagePull_Alpine is a standalone pull test for alpine:3.19.
func TestImagePull_Alpine(t *testing.T) {
	requireRoot(t)

	const ref = "alpine:3.19"
	mustPull(t, ref)

	if !imageInList(t, ref) {
		t.Fatalf("alpine:3.19 not found in image list after pull")
	}

	manifestPath := filepath.Join(imagesBaseDir, safeRef(ref), "manifest.json")
	if _, err := os.Stat(manifestPath); err != nil {
		t.Fatalf("manifest not on disk: %v", err)
	}
}

// TestImagePull_Ubuntu tests pulling ubuntu:22.04.
func TestImagePull_Ubuntu(t *testing.T) {
	requireRoot(t)

	const ref = "ubuntu:22.04"
	mustPull(t, ref)

	if !imageInList(t, ref) {
		t.Fatalf("ubuntu:22.04 not found in image list after pull")
	}
}

// TestImagePull_Debian tests pulling debian:12-slim.
func TestImagePull_Debian(t *testing.T) {
	requireRoot(t)

	const ref = "debian:12-slim"
	mustPull(t, ref)

	if !imageInList(t, ref) {
		t.Fatalf("debian:12-slim not found in image list after pull")
	}
}

// TestImagePull_Busybox tests pulling busybox:1.36.
func TestImagePull_Busybox(t *testing.T) {
	requireRoot(t)

	const ref = "busybox:1.36"
	mustPull(t, ref)

	if !imageInList(t, ref) {
		t.Fatalf("busybox:1.36 not found in image list after pull")
	}
}

// TestImagePull_Python tests pulling python:3.12-alpine.
func TestImagePull_Python(t *testing.T) {
	requireRoot(t)

	const ref = "python:3.12-alpine"
	mustPull(t, ref)

	if !imageInList(t, ref) {
		t.Fatalf("python:3.12-alpine not found in image list after pull")
	}
}

// TestImagePull_Node tests pulling node:20-alpine.
func TestImagePull_Node(t *testing.T) {
	requireRoot(t)

	const ref = "node:20-alpine"
	mustPull(t, ref)

	if !imageInList(t, ref) {
		t.Fatalf("node:20-alpine not found in image list after pull")
	}
}

// TestImagePull_Nginx tests pulling nginx:alpine.
func TestImagePull_Nginx(t *testing.T) {
	requireRoot(t)

	const ref = "nginx:alpine"
	mustPull(t, ref)

	if !imageInList(t, ref) {
		t.Fatalf("nginx:alpine not found in image list after pull")
	}
}

// TestImagePull_Redis tests pulling redis:alpine.
func TestImagePull_Redis(t *testing.T) {
	requireRoot(t)

	const ref = "redis:alpine"
	mustPull(t, ref)

	if !imageInList(t, ref) {
		t.Fatalf("redis:alpine not found in image list after pull")
	}
}

// TestImagePull_Golang tests pulling golang:1.23-alpine.
func TestImagePull_Golang(t *testing.T) {
	requireRoot(t)

	const ref = "golang:1.23-alpine"
	mustPull(t, ref)

	if !imageInList(t, ref) {
		t.Fatalf("golang:1.23-alpine not found in image list after pull")
	}
}

// TestImagePull_RockyLinux tests pulling rockylinux:9-minimal.
func TestImagePull_RockyLinux(t *testing.T) {
	requireRoot(t)

	const ref = "rockylinux:9-minimal"
	mustPull(t, ref)

	if !imageInList(t, ref) {
		t.Fatalf("rockylinux:9-minimal not found in image list after pull")
	}
}

// ---------------------------------------------------------------------------
// Container run tests
// ---------------------------------------------------------------------------

// TestContainerRun_Echo runs `echo "hello from <image>"` in each image and
// verifies the expected string appears in stdout.
func TestContainerRun_Echo(t *testing.T) {
	requireRoot(t)

	for _, tc := range imagesToTest {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mustPull(t, tc.ref)

			expected := "hello from " + tc.ref
			stdout := mustRun(t, tc.ref, "echo", expected)
			if !strings.Contains(stdout, expected) {
				t.Errorf("expected stdout to contain %q, got: %q", expected, stdout)
			}
		})
	}
}

// TestContainerRun_OsRelease runs `cat /etc/os-release` in each image and
// verifies non-empty output is produced.
func TestContainerRun_OsRelease(t *testing.T) {
	requireRoot(t)

	for _, tc := range imagesToTest {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mustPull(t, tc.ref)

			stdout := mustRun(t, tc.ref, "cat", "/etc/os-release")
			if strings.TrimSpace(stdout) == "" {
				t.Errorf("expected non-empty /etc/os-release output for image %q", tc.ref)
			}
		})
	}
}

// TestContainerRun_UnameR runs `uname -r` in each image and verifies that a
// non-empty kernel version string is printed.
func TestContainerRun_UnameR(t *testing.T) {
	requireRoot(t)

	for _, tc := range imagesToTest {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			mustPull(t, tc.ref)

			stdout := mustRun(t, tc.ref, "uname", "-r")
			if strings.TrimSpace(stdout) == "" {
				t.Errorf("expected non-empty uname -r output for image %q", tc.ref)
			}
			t.Logf("kernel version reported by %s: %s", tc.ref, strings.TrimSpace(stdout))
		})
	}
}

// ---------------------------------------------------------------------------
// Lifecycle test
// ---------------------------------------------------------------------------

// TestContainerLifecycle exercises the full container lifecycle using alpine:3.19:
//  1. pull → verify in image list
//  2. create + start container (run `sleep 30` in background via --detach)
//  3. ps → verify container shows as running
//  4. kill container
//  5. ps → verify container stopped
//  6. rm container → verify directory removed
//  7. rmi image → verify directory removed
//  8. verify image gone from `thrive images`
func TestContainerLifecycle(t *testing.T) {
	requireRoot(t)

	const imageRef = "alpine:3.19"
	containerID := uniqueContainerID(t, "lifecycle")

	// ── Step 1: pull ─────────────────────────────────────────────────────────
	mustPull(t, imageRef)
	if !imageInList(t, imageRef) {
		t.Fatalf("step 1: image %q not in list after pull", imageRef)
	}

	// ── Deferred cleanup ─────────────────────────────────────────────────────
	// Best-effort cleanup in case the test fails mid-lifecycle.
	t.Cleanup(func() {
		cleanCtx, cleanCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cleanCancel()
		thriveCLI(cleanCtx, t, "kill", containerID)
		thriveCLI(cleanCtx, t, "rm", containerID)
		thriveCLI(cleanCtx, t, "rmi", safeRef(imageRef))
	})

	// ── Steps 2+3: start container in background ──────────────────────────────
	// thrive run --detach --name <id> creates + starts the container detached.
	createCtx, createCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer createCancel()

	stdout, stderr, err := thriveCLI(createCtx, t,
		"run", "--detach", "--name", containerID, imageRef, "sleep", "30",
	)
	if err != nil {
		t.Fatalf("step 2: run --detach failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	// Give the runtime a moment to write the state file.
	time.Sleep(500 * time.Millisecond)

	// ── Step 3: verify running ────────────────────────────────────────────────
	psCtx, psCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer psCancel()

	psOut, _, _ := thriveCLI(psCtx, t, "ps")
	if !strings.Contains(psOut, containerID) {
		t.Errorf("step 3: container %q not found in `thrive ps` output:\n%s", containerID, psOut)
	}
	if !strings.Contains(psOut, "running") {
		t.Errorf("step 3: container %q not showing 'running' state:\n%s", containerID, psOut)
	}

	// ── Step 4: kill container ────────────────────────────────────────────────
	killCtx, killCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer killCancel()

	killOut, killErr, err := thriveCLI(killCtx, t, "kill", containerID)
	if err != nil {
		t.Fatalf("step 4: kill failed: %v\nstdout: %s\nstderr: %s", err, killOut, killErr)
	}

	// Give the background goroutine in runtime.Start a moment to write "stopped".
	time.Sleep(500 * time.Millisecond)

	// ── Step 5: verify stopped ────────────────────────────────────────────────
	psCtx2, psCancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer psCancel2()

	psOut2, _, _ := thriveCLI(psCtx2, t, "ps", "--all")
	if strings.Contains(psOut2, containerID) && strings.Contains(psOut2, "running") {
		t.Errorf("step 5: container %q still shows as running after kill:\n%s", containerID, psOut2)
	}

	// ── Step 6: rm container ─────────────────────────────────────────────────
	rmCtx, rmCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer rmCancel()

	rmOut, rmErr, err := thriveCLI(rmCtx, t, "rm", containerID)
	if err != nil {
		t.Fatalf("step 6: rm failed: %v\nstdout: %s\nstderr: %s", err, rmOut, rmErr)
	}

	containerDir := filepath.Join(containersBaseDir, containerID)
	if _, statErr := os.Stat(containerDir); !os.IsNotExist(statErr) {
		t.Errorf("step 6: container directory %q still exists after rm", containerDir)
	}

	// ── Step 7: rmi image ────────────────────────────────────────────────────
	// The rmi command in the current CLI accepts the raw SafeRef name (the
	// directory name on disk), not the original image reference.
	rmiCtx, rmiCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer rmiCancel()

	rmiOut, rmiErr, err := thriveCLI(rmiCtx, t, "rmi", safeRef(imageRef))
	if err != nil {
		t.Fatalf("step 7: rmi failed: %v\nstdout: %s\nstderr: %s", err, rmiOut, rmiErr)
	}

	imgDir := filepath.Join(imagesBaseDir, safeRef(imageRef))
	if _, statErr := os.Stat(imgDir); !os.IsNotExist(statErr) {
		t.Errorf("step 7: image directory %q still exists after rmi", imgDir)
	}

	// ── Step 8: verify image no longer listed ────────────────────────────────
	if imageInList(t, imageRef) {
		t.Errorf("step 8: image %q still appears in `thrive images` after rmi", imageRef)
	}
}

// ---------------------------------------------------------------------------
// System tests
// ---------------------------------------------------------------------------

// TestSystemInfo verifies that `thrive system info` exits 0 and produces output.
// The current implementation is a stub; the test is deliberately lenient and logs
// what it finds. When the stub is filled in, "Version:" and "OS/Arch:" are expected.
func TestSystemInfo(t *testing.T) {
	requireRoot(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stdout, stderr, err := thriveCLI(ctx, t, "system", "info")
	if err != nil {
		t.Fatalf("system info failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if strings.TrimSpace(stdout) == "" && strings.TrimSpace(stderr) == "" {
		t.Error("system info produced no output at all")
	}

	// Note: these are expected once the stub is promoted to a real implementation.
	for _, marker := range []string{"Version:", "OS/Arch:"} {
		if !strings.Contains(stdout, marker) {
			t.Logf("NOTE: system info does not yet contain %q (stub implementation)", marker)
		}
	}
}

// TestSystemClean verifies that `thrive system clean` exits 0 after running
// and stopping a short-lived container. It exercises the prune/clean path.
func TestSystemClean(t *testing.T) {
	requireRoot(t)

	const imageRef = "busybox:1.36"
	containerID := uniqueContainerID(t, "clean")

	mustPull(t, imageRef)

	// Run a container that exits immediately; ignore its exit status.
	runCtx, runCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer runCancel()
	thriveCLI(runCtx, t, "run", "--name", containerID, imageRef, "true")

	// Give the runtime a moment to write "stopped" state.
	time.Sleep(500 * time.Millisecond)

	// Remove the stopped container so the state dir is absent for clean.
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cleanupCancel()
	thriveCLI(cleanupCtx, t, "rm", containerID)

	// Invoke system clean and verify it exits 0.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stdout, stderr, err := thriveCLI(ctx, t, "system", "clean")
	if err != nil {
		t.Fatalf("system clean failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	t.Logf("system clean stdout: %s", stdout)
}

// ---------------------------------------------------------------------------
// Idempotency tests
// ---------------------------------------------------------------------------

// TestImagePull_Idempotent verifies that pulling the same image twice does not
// fail and that the image list still contains the ref.
func TestImagePull_Idempotent(t *testing.T) {
	requireRoot(t)

	const ref = "alpine:3.19"

	mustPull(t, ref)
	mustPull(t, ref) // second pull must succeed

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stdout, _, err := thriveCLI(ctx, t, "images")
	if err != nil {
		t.Fatalf("images command failed: %v", err)
	}

	if !strings.Contains(stdout, ref) {
		t.Errorf("expected %q to appear in `thrive images` after double pull, output:\n%s", ref, stdout)
	}
}
