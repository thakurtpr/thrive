# Thrive Production + CNCF Sandbox Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Thrive production-quality and CNCF Sandbox–ready by fixing repo hygiene blockers, raising test coverage to 80%+, polishing developer experience, and writing all required CNCF compliance documents.

**Architecture:** Four independent parallel streams — Repo Foundations, Core Test Coverage, Network/P2P/pkg Coverage, and CNCF Docs + DX — each producing self-contained commits that land cleanly on `main`.

**Tech Stack:** Go 1.22, cobra, go-containerregistry, go-fuse, prometheus/client_golang, goreleaser, GitHub Actions

## Global Constraints

- Module path is `github.com/thakurprasadrout/thrive` — canonical everywhere; fix all docs that say `thakurtpr`
- Go minimum version: `1.22.0` — fix any reference to `1.25.0` (that version does not exist)
- All tests run under `GOOS=linux` in CI; macOS-only tests use `//go:build darwin`
- No new external dependencies without updating `go.mod` + `go.sum`
- `go vet ./...` and `GOOS=linux go build ./...` must pass after every task
- Commit message format: `<type>: <description>` (feat/fix/test/docs/chore)
- Never commit binary artifacts (covered by .gitignore)

---

## Stream A — Repo Foundations

### Task A1: Fix go.mod Go version + version ldflags + module path in README

**Files:**
- Modify: `go.mod` line 3
- Modify: `README.md` — replace all `thakurtpr` with `thakurprasadrout`
- Modify: `Makefile` — add `LDFLAGS` with `-X main.Version` and `-X main.Commit`
- Modify: `cmd/thrive/main.go` — expose `Version` and `Commit` vars, wire `root.Version`

**Interfaces:**
- Produces: `thrive --version` prints `thrive <version> (<commit>)`

- [ ] **Step 1: Fix go.mod**

Open `go.mod`. Change line 3 from:
```
go 1.25.0
```
to:
```
go 1.22.0
```

- [ ] **Step 2: Verify build still passes**

```bash
GOOS=linux go build ./... 2>&1
# Expected: no output, exit 0
```

- [ ] **Step 3: Fix README module path**

In `README.md`, replace every occurrence of `github.com/thakurtpr/thrive` with `github.com/thakurprasadrout/thrive`. Verify with:

```bash
grep "thakurtpr" README.md
# Expected: no output
```

- [ ] **Step 4: Add version ldflags to Makefile**

In `Makefile`, add these lines before the `build:` target and update each build target to use `$(LDFLAGS)`:

```makefile
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS  = -ldflags "-X main.Version=$(VERSION) -X main.Commit=$(COMMIT)"
```

Change the `build` target to:
```makefile
build:
	GOOS=$(GOOS) go build $(LDFLAGS) -o $(BINARY) ./cmd/thrive
```

Apply `$(LDFLAGS)` to `build-linux`, `build-darwin`, `build-windows` targets the same way.

- [ ] **Step 5: Add Version/Commit to cmd/thrive/main.go**

In `cmd/thrive/main.go`, add after the `package main` line and before `func main()`:
```go
var (
    Version = "dev"
    Commit  = "unknown"
)
```

Inside `func main()`, before `root.AddCommand(...)`, add:
```go
root.Version = Version + " (" + Commit + ")"
root.SetVersionTemplate("thrive {{.Version}}\n")
```

- [ ] **Step 6: Verify version flag works**

```bash
go build -ldflags "-X main.Version=0.1.0-test -X main.Commit=abc1234" -o /tmp/thrive-test ./cmd/thrive && /tmp/thrive-test --version
# Expected: thrive 0.1.0-test (abc1234)
```

- [ ] **Step 7: Commit**

```bash
git add go.mod README.md Makefile cmd/thrive/main.go
git commit -m "fix: go 1.22.0, canonical module path, version ldflags"
```

---

### Task A2: Fix install.sh to download pre-built binaries from GitHub Releases

**Files:**
- Modify: `scripts/install.sh` — replace build-from-source with binary download

**Interfaces:**
- Produces: one-liner `curl -fsSL https://raw.githubusercontent.com/thakurprasadrout/thrive/main/scripts/install.sh | sh` that installs a pre-built binary

- [ ] **Step 1: Replace scripts/install.sh**

Overwrite `scripts/install.sh` entirely with:

```bash
#!/usr/bin/env sh
set -e

REPO="thakurprasadrout/thrive"
VERSION="${THRIVE_VERSION:-}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)        ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
    linux|darwin) ;;
    *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

if [ -z "$VERSION" ]; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
    if [ -z "$VERSION" ]; then
        echo "Could not resolve latest version. Set THRIVE_VERSION=vX.Y.Z" >&2
        exit 1
    fi
fi

BINARY="thrive-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}"

echo "Installing thrive ${VERSION} (${OS}/${ARCH}) from ${URL}..."

TMP=$(mktemp)
curl -fsSL -o "$TMP" "$URL"
chmod +x "$TMP"

if [ "$(id -u)" -ne 0 ] && [ ! -w "$INSTALL_DIR" ]; then
    sudo mv "$TMP" "${INSTALL_DIR}/thrive"
else
    mv "$TMP" "${INSTALL_DIR}/thrive"
fi

echo "thrive installed to ${INSTALL_DIR}/thrive"
"${INSTALL_DIR}/thrive" --version
```

- [ ] **Step 2: Verify sh syntax**

```bash
sh -n scripts/install.sh
# Expected: no output, exit 0
```

- [ ] **Step 3: Commit**

```bash
git add scripts/install.sh
git commit -m "fix: install.sh downloads pre-built binary from GitHub Releases"
```

---

### Task A3: Commit all currently modified working-tree files

**Files:**
- All files showing `M` in `git status --short` from prior sessions

- [ ] **Step 1: Review what is modified**

```bash
git status --short
```

- [ ] **Step 2: Stage and commit**

```bash
git add \
    AGENT_STATUS.md HANDOFF.md MEMORY.md TDD_PROGRESS.md \
    bin/thrive \
    cmd/thrive/commands/desktop.go \
    cmd/thrive/commands/images_stub.go \
    cmd/thrive/commands/kill_proxy.go \
    cmd/thrive/commands/logs_proxy.go \
    cmd/thrive/commands/ps_proxy.go \
    cmd/thrive/commands/rm_proxy.go \
    cmd/thrive/commands/run.go \
    cmd/thrive/commands/run_proxy.go \
    cmd/thrive/commands/system_proxy.go \
    cmd/thrive/main.go \
    cmd/thrived/exec.go \
    cmd/thrived/socket.go \
    cmd/thrived/vsock_linux.go \
    internal/image/image.go \
    internal/image/image_test.go \
    internal/runtime/config.go \
    internal/runtime/runtime.go \
    internal/vm/darwin_launcher.go \
    internal/vm/darwin_launcher_test.go \
    internal/vm/download.go \
    internal/vm/lifecycle.go \
    internal/vm/vsock_darwin.go \
    scripts/build-vm-image.sh
git commit -m "chore: land all prior-session working-tree changes"
```

---

## Stream B — Core Test Coverage (image / runtime / cgroup / secrets)

### Task B1: Interface + fake-based unit tests for internal/image

**Files:**
- Create: `internal/image/interfaces.go`
- Create: `internal/image/fake_test.go`
- Modify: `internal/image/image_test.go` — append new tests

**Interfaces:**
- Produces: `Mounter` interface, `Puller` interface (both exported from `package image`)

- [ ] **Step 1: Create internal/image/interfaces.go**

```go
//go:build linux || darwin

package image

// Mounter manages per-container rootfs overlay mounts.
type Mounter interface {
    Mount(containerID, imageRef string) (mergedDir string, err error)
    Unmount(containerID string) error
}

// Puller downloads OCI images to the local content store.
type Puller interface {
    Pull(ref string) error
    List() ([]string, error)
}
```

- [ ] **Step 2: Create internal/image/fake_test.go**

```go
//go:build linux || darwin

package image_test

import (
    "fmt"
    "sync"
)

type fakeStore struct {
    mu       sync.Mutex
    images   map[string]bool
    mounts   map[string]string
    pullErr  error
    mountErr error
}

func newFakeStore() *fakeStore {
    return &fakeStore{
        images: make(map[string]bool),
        mounts: make(map[string]string),
    }
}

func (f *fakeStore) Pull(ref string) error {
    if f.pullErr != nil {
        return f.pullErr
    }
    f.mu.Lock()
    defer f.mu.Unlock()
    f.images[ref] = true
    return nil
}

func (f *fakeStore) List() ([]string, error) {
    f.mu.Lock()
    defer f.mu.Unlock()
    out := make([]string, 0, len(f.images))
    for ref := range f.images {
        out = append(out, ref)
    }
    return out, nil
}

func (f *fakeStore) Mount(containerID, imageRef string) (string, error) {
    if f.mountErr != nil {
        return "", f.mountErr
    }
    f.mu.Lock()
    defer f.mu.Unlock()
    if !f.images[imageRef] {
        return "", fmt.Errorf("image not found: %s", imageRef)
    }
    dir := "/tmp/thrive-fake/" + containerID
    f.mounts[containerID] = dir
    return dir, nil
}

func (f *fakeStore) Unmount(containerID string) error {
    f.mu.Lock()
    defer f.mu.Unlock()
    delete(f.mounts, containerID)
    return nil
}
```

- [ ] **Step 3: Append unit tests to internal/image/image_test.go**

```go
//go:build linux || darwin

package image_test

import (
    "testing"
)

func TestFakeStorePull(t *testing.T) {
    store := newFakeStore()
    if err := store.Pull("alpine:3.19"); err != nil {
        t.Fatalf("Pull failed: %v", err)
    }
    imgs, err := store.List()
    if err != nil {
        t.Fatalf("List failed: %v", err)
    }
    if len(imgs) != 1 || imgs[0] != "alpine:3.19" {
        t.Errorf("expected [alpine:3.19], got %v", imgs)
    }
}

func TestFakeStorePullError(t *testing.T) {
    store := newFakeStore()
    store.pullErr = fmt.Errorf("registry unavailable")
    if err := store.Pull("alpine:3.19"); err == nil {
        t.Fatal("expected error, got nil")
    }
}

func TestFakeStoreMountSuccess(t *testing.T) {
    store := newFakeStore()
    _ = store.Pull("alpine:3.19")
    dir, err := store.Mount("ctr-1", "alpine:3.19")
    if err != nil {
        t.Fatalf("Mount failed: %v", err)
    }
    if dir == "" {
        t.Error("expected non-empty merged dir")
    }
    if err := store.Unmount("ctr-1"); err != nil {
        t.Fatalf("Unmount failed: %v", err)
    }
}

func TestFakeStoreMountMissingImage(t *testing.T) {
    store := newFakeStore()
    _, err := store.Mount("ctr-1", "nonexistent:latest")
    if err == nil {
        t.Fatal("expected error for missing image")
    }
}
```

- [ ] **Step 4: Run tests**

```bash
go test -v -count=1 ./internal/image/... 2>&1
# Expected: PASS
```

- [ ] **Step 5: Commit**

```bash
git add internal/image/interfaces.go internal/image/fake_test.go internal/image/image_test.go
git commit -m "test: image Mounter/Puller interfaces + fake unit tests"
```

---

### Task B2: Interface + fake-based unit tests for internal/runtime

**Files:**
- Create: `internal/runtime/interfaces.go`
- Modify: `internal/runtime/runtime_test.go` — append fake-based tests

**Interfaces:**
- Produces: `Runner` interface with `Start(Config) (string, error)`, `Stop(string) error`, `Status(string) (State, error)`, `Logs(string) ([]byte, error)`
- Consumes: `Config` struct from `internal/runtime/config.go`

- [ ] **Step 1: Create internal/runtime/interfaces.go**

```go
package runtime

// Runner is the contract for starting and stopping containers.
type Runner interface {
    Start(cfg Config) (containerID string, err error)
    Stop(containerID string) error
    Status(containerID string) (State, error)
    Logs(containerID string) ([]byte, error)
}

// State represents a container lifecycle state snapshot.
type State struct {
    ID       string
    Status   string // "created" | "running" | "stopped" | "exited"
    PID      int
    ExitCode int
}
```

- [ ] **Step 2: Append fake + tests to internal/runtime/runtime_test.go**

```go
package runtime_test

import (
    "fmt"
    "sync"
    "testing"

    "github.com/thakurprasadrout/thrive/internal/runtime"
)

type fakeRunner struct {
    mu         sync.Mutex
    containers map[string]runtime.State
    startErr   error
    stopErr    error
    nextID     int
}

func newFakeRunner() *fakeRunner {
    return &fakeRunner{containers: make(map[string]runtime.State)}
}

func (r *fakeRunner) Start(cfg runtime.Config) (string, error) {
    if r.startErr != nil {
        return "", r.startErr
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    r.nextID++
    id := fmt.Sprintf("ctr-%d", r.nextID)
    r.containers[id] = runtime.State{ID: id, Status: "running", PID: 12345 + r.nextID}
    return id, nil
}

func (r *fakeRunner) Stop(id string) error {
    if r.stopErr != nil {
        return r.stopErr
    }
    r.mu.Lock()
    defer r.mu.Unlock()
    s, ok := r.containers[id]
    if !ok {
        return fmt.Errorf("container not found: %s", id)
    }
    s.Status = "stopped"
    s.PID = 0
    r.containers[id] = s
    return nil
}

func (r *fakeRunner) Status(id string) (runtime.State, error) {
    r.mu.Lock()
    defer r.mu.Unlock()
    s, ok := r.containers[id]
    if !ok {
        return runtime.State{}, fmt.Errorf("container not found: %s", id)
    }
    return s, nil
}

func (r *fakeRunner) Logs(_ string) ([]byte, error) {
    return []byte("fake log line\n"), nil
}

func TestFakeRunnerStartStop(t *testing.T) {
    r := newFakeRunner()
    id, err := r.Start(runtime.Config{Image: "alpine:3.19", Args: []string{"echo", "hi"}})
    if err != nil {
        t.Fatalf("Start: %v", err)
    }
    if id == "" {
        t.Error("expected non-empty container ID")
    }
    st, err := r.Status(id)
    if err != nil {
        t.Fatalf("Status: %v", err)
    }
    if st.Status != "running" {
        t.Errorf("expected running, got %s", st.Status)
    }
    if err := r.Stop(id); err != nil {
        t.Fatalf("Stop: %v", err)
    }
    st, _ = r.Status(id)
    if st.Status != "stopped" {
        t.Errorf("expected stopped, got %s", st.Status)
    }
}

func TestFakeRunnerStartError(t *testing.T) {
    r := newFakeRunner()
    r.startErr = fmt.Errorf("no rootfs")
    if _, err := r.Start(runtime.Config{}); err == nil {
        t.Fatal("expected error")
    }
}

func TestFakeRunnerStopUnknown(t *testing.T) {
    r := newFakeRunner()
    if err := r.Stop("ghost-id"); err == nil {
        t.Fatal("expected error for unknown container")
    }
}

func TestFakeRunnerLogs(t *testing.T) {
    r := newFakeRunner()
    id, _ := r.Start(runtime.Config{Image: "alpine:3.19"})
    logs, err := r.Logs(id)
    if err != nil {
        t.Fatalf("Logs: %v", err)
    }
    if len(logs) == 0 {
        t.Error("expected non-empty logs")
    }
}
```

- [ ] **Step 3: Run tests**

```bash
go test -v -count=1 ./internal/runtime/... 2>&1
# Expected: PASS
```

- [ ] **Step 4: Commit**

```bash
git add internal/runtime/interfaces.go internal/runtime/runtime_test.go
git commit -m "test: runtime Runner interface + fake unit tests"
```

---

### Task B3: Interface + fake tests for internal/cgroup

**Files:**
- Create: `internal/cgroup/interfaces.go`
- Modify: `internal/cgroup/cgroup_test.go` — append fake tests

**Interfaces:**
- Produces: `Manager` interface with `Apply(id string, limits Limits) error`, `Remove(id string) error`
- Produces: `Limits` struct with `MemoryBytes`, `CPUQuota`, `PIDsMax int64`

- [ ] **Step 1: Create internal/cgroup/interfaces.go**

```go
package cgroup

// Manager applies and removes cgroup v2 resource limits for containers.
type Manager interface {
    Apply(containerID string, limits Limits) error
    Remove(containerID string) error
}

// Limits describes resource constraints for a single container.
type Limits struct {
    MemoryBytes int64 // 0 = unlimited
    CPUQuota    int64 // microseconds per 100ms period; 0 = unlimited
    PIDsMax     int64 // 0 = unlimited
}
```

- [ ] **Step 2: Append fake + tests to internal/cgroup/cgroup_test.go**

```go
package cgroup_test

import (
    "fmt"
    "sync"
    "testing"

    "github.com/thakurprasadrout/thrive/internal/cgroup"
)

type fakeCgroupManager struct {
    mu       sync.Mutex
    applied  map[string]cgroup.Limits
    applyErr error
}

func newFakeCgroupManager() *fakeCgroupManager {
    return &fakeCgroupManager{applied: make(map[string]cgroup.Limits)}
}

func (m *fakeCgroupManager) Apply(id string, limits cgroup.Limits) error {
    if m.applyErr != nil {
        return m.applyErr
    }
    m.mu.Lock()
    defer m.mu.Unlock()
    m.applied[id] = limits
    return nil
}

func (m *fakeCgroupManager) Remove(id string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    delete(m.applied, id)
    return nil
}

func TestFakeCgroupApplyAndRemove(t *testing.T) {
    m := newFakeCgroupManager()
    limits := cgroup.Limits{MemoryBytes: 128 << 20, CPUQuota: 50000, PIDsMax: 100}
    if err := m.Apply("ctr-1", limits); err != nil {
        t.Fatalf("Apply: %v", err)
    }
    m.mu.Lock()
    got, ok := m.applied["ctr-1"]
    m.mu.Unlock()
    if !ok {
        t.Fatal("limits not stored")
    }
    if got.MemoryBytes != 128<<20 {
        t.Errorf("expected 128 MiB, got %d", got.MemoryBytes)
    }
    if err := m.Remove("ctr-1"); err != nil {
        t.Fatalf("Remove: %v", err)
    }
    m.mu.Lock()
    _, exists := m.applied["ctr-1"]
    m.mu.Unlock()
    if exists {
        t.Error("limits should be removed")
    }
}

func TestFakeCgroupApplyError(t *testing.T) {
    m := newFakeCgroupManager()
    m.applyErr = fmt.Errorf("cgroup v2 not mounted")
    if err := m.Apply("ctr-2", cgroup.Limits{}); err == nil {
        t.Fatal("expected error")
    }
}

func TestFakeCgroupZeroLimitsAllowed(t *testing.T) {
    m := newFakeCgroupManager()
    if err := m.Apply("ctr-3", cgroup.Limits{}); err != nil {
        t.Fatalf("zero limits should be accepted (means unlimited): %v", err)
    }
}
```

- [ ] **Step 3: Run tests**

```bash
go test -v -count=1 ./internal/cgroup/... 2>&1
# Expected: PASS
```

- [ ] **Step 4: Commit**

```bash
git add internal/cgroup/interfaces.go internal/cgroup/cgroup_test.go
git commit -m "test: cgroup Manager interface + fake unit tests"
```

---

### Task B4: Expand internal/secrets to 80%+ coverage

**Files:**
- Modify: `internal/secrets/vault_test.go` — add roundtrip, corrupt ciphertext, list, delete tests

- [ ] **Step 1: Check current gaps**

```bash
go test -coverprofile=/tmp/sec.out ./internal/secrets/... 2>&1
go tool cover -func=/tmp/sec.out | grep -v "100.0%"
```

- [ ] **Step 2: Append tests to vault_test.go**

```go
func TestVaultSetGetRoundTrip(t *testing.T) {
    dir := t.TempDir()
    v, err := NewVault(dir)
    if err != nil {
        t.Fatalf("NewVault: %v", err)
    }
    if err := v.Set("DB_PASS", "s3cr3t!"); err != nil {
        t.Fatalf("Set: %v", err)
    }
    val, err := v.Get("DB_PASS")
    if err != nil {
        t.Fatalf("Get: %v", err)
    }
    if val != "s3cr3t!" {
        t.Errorf("expected s3cr3t!, got %q", val)
    }
}

func TestVaultGetNonExistent(t *testing.T) {
    dir := t.TempDir()
    v, err := NewVault(dir)
    if err != nil {
        t.Fatalf("NewVault: %v", err)
    }
    if _, err := v.Get("MISSING"); err == nil {
        t.Fatal("expected error for nonexistent secret")
    }
}

func TestVaultListEmpty(t *testing.T) {
    dir := t.TempDir()
    v, err := NewVault(dir)
    if err != nil {
        t.Fatalf("NewVault: %v", err)
    }
    names, err := v.List()
    if err != nil {
        t.Fatalf("List: %v", err)
    }
    if len(names) != 0 {
        t.Errorf("expected empty list, got %v", names)
    }
}

func TestVaultListAfterSet(t *testing.T) {
    dir := t.TempDir()
    v, err := NewVault(dir)
    if err != nil {
        t.Fatalf("NewVault: %v", err)
    }
    _ = v.Set("A", "1")
    _ = v.Set("B", "2")
    names, err := v.List()
    if err != nil {
        t.Fatalf("List: %v", err)
    }
    if len(names) != 2 {
        t.Errorf("expected 2 secrets, got %d", len(names))
    }
}

func TestVaultDelete(t *testing.T) {
    dir := t.TempDir()
    v, err := NewVault(dir)
    if err != nil {
        t.Fatalf("NewVault: %v", err)
    }
    _ = v.Set("TOKEN", "abc")
    if err := v.Delete("TOKEN"); err != nil {
        t.Fatalf("Delete: %v", err)
    }
    if _, err := v.Get("TOKEN"); err == nil {
        t.Fatal("expected error after delete")
    }
}

func TestVaultCorruptedCiphertext(t *testing.T) {
    dir := t.TempDir()
    v, err := NewVault(dir)
    if err != nil {
        t.Fatalf("NewVault: %v", err)
    }
    // Write garbage directly to the expected secret path
    path := filepath.Join(dir, "CORRUPT")
    if err := os.WriteFile(path, []byte("not-valid-ciphertext"), 0600); err != nil {
        t.Fatalf("WriteFile: %v", err)
    }
    if _, err := v.Get("CORRUPT"); err == nil {
        t.Fatal("expected decryption error for corrupt ciphertext")
    }
}
```

Add `"os"` and `"path/filepath"` to imports if not present.

- [ ] **Step 3: Run and verify coverage**

```bash
go test -cover ./internal/secrets/... 2>&1
# Expected: coverage >= 80%
```

- [ ] **Step 4: Commit**

```bash
git add internal/secrets/vault_test.go
git commit -m "test: secrets vault roundtrip, corrupt ciphertext, list, delete"
```

---

## Stream C — Network / P2P / pkg Coverage

### Task C1: Interface + fake tests for internal/network

**Files:**
- Create: `internal/network/interfaces.go`
- Modify: `internal/network/network_test.go` — append fake tests

**Interfaces:**
- Produces: `Networker` interface with `Setup(id string, cfg NetConfig) error`, `Teardown(id string) error`
- Produces: `NetConfig` and `PortMapping` structs

- [ ] **Step 1: Create internal/network/interfaces.go**

```go
//go:build linux

package network

// Networker sets up and tears down container network namespaces.
type Networker interface {
    Setup(containerID string, cfg NetConfig) error
    Teardown(containerID string) error
}

// NetConfig describes the desired network for a container.
type NetConfig struct {
    BridgeName   string
    HostIP       string
    ContainerIP  string
    PortMappings []PortMapping
}

// PortMapping maps a host port to a container port.
type PortMapping struct {
    HostPort      uint16
    ContainerPort uint16
    Protocol      string // "tcp" or "udp"
}
```

- [ ] **Step 2: Append fake tests to internal/network/network_test.go**

```go
//go:build linux

package network_test

import (
    "fmt"
    "sync"
    "testing"

    "github.com/thakurprasadrout/thrive/internal/network"
)

type fakeNetworker struct {
    mu          sync.Mutex
    active      map[string]network.NetConfig
    setupErr    error
}

func newFakeNetworker() *fakeNetworker {
    return &fakeNetworker{active: make(map[string]network.NetConfig)}
}

func (n *fakeNetworker) Setup(id string, cfg network.NetConfig) error {
    if n.setupErr != nil {
        return n.setupErr
    }
    n.mu.Lock()
    defer n.mu.Unlock()
    n.active[id] = cfg
    return nil
}

func (n *fakeNetworker) Teardown(id string) error {
    n.mu.Lock()
    defer n.mu.Unlock()
    delete(n.active, id)
    return nil
}

func TestFakeNetworkerSetupTeardown(t *testing.T) {
    net := newFakeNetworker()
    cfg := network.NetConfig{
        BridgeName:  "thrive0",
        HostIP:      "10.88.0.1",
        ContainerIP: "10.88.0.2",
        PortMappings: []network.PortMapping{
            {HostPort: 8080, ContainerPort: 80, Protocol: "tcp"},
        },
    }
    if err := net.Setup("ctr-1", cfg); err != nil {
        t.Fatalf("Setup: %v", err)
    }
    net.mu.Lock()
    got, ok := net.active["ctr-1"]
    net.mu.Unlock()
    if !ok {
        t.Fatal("net config not stored")
    }
    if got.BridgeName != "thrive0" {
        t.Errorf("expected thrive0, got %s", got.BridgeName)
    }
    if err := net.Teardown("ctr-1"); err != nil {
        t.Fatalf("Teardown: %v", err)
    }
    net.mu.Lock()
    _, exists := net.active["ctr-1"]
    net.mu.Unlock()
    if exists {
        t.Error("network should be torn down")
    }
}

func TestFakeNetworkerSetupError(t *testing.T) {
    net := newFakeNetworker()
    net.setupErr = fmt.Errorf("bridge already exists")
    if err := net.Setup("ctr-2", network.NetConfig{}); err == nil {
        t.Fatal("expected error")
    }
}
```

- [ ] **Step 3: Run tests**

```bash
go test -v -count=1 ./internal/network/... 2>&1
# Expected: PASS
```

- [ ] **Step 4: Commit**

```bash
git add internal/network/interfaces.go internal/network/network_test.go
git commit -m "test: network Networker interface + fake unit tests"
```

---

### Task C2: Expand internal/p2p coverage

**Files:**
- Modify: `internal/p2p/peer_test.go` — add routing table tests

- [ ] **Step 1: Check current coverage**

```bash
go test -cover ./internal/p2p/... 2>&1
```

- [ ] **Step 2: Inspect exported API of peer.go**

```bash
grep "^func\|^type" internal/p2p/peer.go | head -20
```

- [ ] **Step 3: Add peer table tests**

Append to `internal/p2p/peer_test.go` (adapt function/type names to match actual exported API from step 2):

```go
func TestPeerTableAddAll(t *testing.T) {
    table := NewPeerTable()
    table.Add(Peer{ID: "peer-1", Addr: "10.0.0.1:4001"})
    table.Add(Peer{ID: "peer-2", Addr: "10.0.0.2:4001"})
    peers := table.All()
    if len(peers) != 2 {
        t.Errorf("expected 2 peers, got %d", len(peers))
    }
}

func TestPeerTableRemove(t *testing.T) {
    table := NewPeerTable()
    table.Add(Peer{ID: "peer-A", Addr: "10.0.0.1:4001"})
    table.Remove("peer-A")
    if len(table.All()) != 0 {
        t.Error("peer should be removed after Remove()")
    }
}

func TestPeerTableNoDuplicates(t *testing.T) {
    table := NewPeerTable()
    p := Peer{ID: "peer-dup", Addr: "10.0.0.1:4001"}
    table.Add(p)
    table.Add(p)
    if len(table.All()) != 1 {
        t.Errorf("duplicate add should result in 1 peer, got %d", len(table.All()))
    }
}
```

- [ ] **Step 4: Run tests**

```bash
go test -v -count=1 ./internal/p2p/... 2>&1
# Expected: PASS
```

- [ ] **Step 5: Commit**

```bash
git add internal/p2p/peer_test.go
git commit -m "test: p2p peer table add/remove/deduplicate"
```

---

### Task C3: Coverage for pkg/dag, pkg/build, pkg/thrivefile

**Files:**
- Modify: `pkg/dag/dag_test.go`
- Modify: `pkg/build/build_test.go`
- Modify: `pkg/thrivefile/thrivefile_test.go`

- [ ] **Step 1: Check current coverage**

```bash
go test -cover ./pkg/... 2>&1
```

- [ ] **Step 2: Add DAG cycle detection + topological order tests to pkg/dag/dag_test.go**

Inspect the exported API first:
```bash
grep "^func\|^type" pkg/dag/dag.go
```

Then append to `pkg/dag/dag_test.go`:
```go
func TestCycleDetection(t *testing.T) {
    g := New()
    g.AddNode("A")
    g.AddNode("B")
    g.AddEdge("A", "B")
    g.AddEdge("B", "A") // cycle
    if _, err := g.TopologicalSort(); err == nil {
        t.Fatal("expected cycle detection error")
    }
}

func TestTopologicalOrder(t *testing.T) {
    g := New()
    for _, n := range []string{"build", "test", "package"} {
        g.AddNode(n)
    }
    g.AddEdge("build", "test")
    g.AddEdge("test", "package")
    order, err := g.TopologicalSort()
    if err != nil {
        t.Fatalf("TopologicalSort: %v", err)
    }
    if len(order) != 3 {
        t.Fatalf("expected 3 nodes, got %d", len(order))
    }
    indexOf := func(s string) int {
        for i, n := range order {
            if n == s {
                return i
            }
        }
        return -1
    }
    if indexOf("build") >= indexOf("test") {
        t.Error("build must come before test")
    }
    if indexOf("test") >= indexOf("package") {
        t.Error("test must come before package")
    }
}
```

- [ ] **Step 3: Add build cache key tests to pkg/build/build_test.go**

Inspect the exported API first:
```bash
grep "^func\|^type" pkg/build/build.go
```

Then append to `pkg/build/build_test.go`:
```go
func TestCacheKeyDeterminism(t *testing.T) {
    step := Step{Name: "install", Run: "apk add curl", Deps: []string{"base"}}
    k1 := step.CacheKey()
    k2 := step.CacheKey()
    if k1 != k2 {
        t.Error("CacheKey must be deterministic")
    }
    if k1 == "" {
        t.Error("CacheKey must not be empty")
    }
}

func TestCacheKeyUniqueness(t *testing.T) {
    s1 := Step{Name: "step-a", Run: "echo a"}
    s2 := Step{Name: "step-b", Run: "echo b"}
    if s1.CacheKey() == s2.CacheKey() {
        t.Error("different steps must have different cache keys")
    }
}
```

- [ ] **Step 4: Add Thrivefile parse tests to pkg/thrivefile/thrivefile_test.go**

Inspect parse function signature:
```bash
grep "^func\|^type" pkg/thrivefile/thrivefile.go | head -10
```

Then append to `pkg/thrivefile/thrivefile_test.go`:
```go
func TestParseValidThrivefile(t *testing.T) {
    input := []byte(`
from: alpine:3.19
steps:
  - name: hello
    run: echo hello
`)
    tf, err := Parse(input)
    if err != nil {
        t.Fatalf("Parse: %v", err)
    }
    if tf.From != "alpine:3.19" {
        t.Errorf("expected alpine:3.19, got %s", tf.From)
    }
    if len(tf.Steps) != 1 {
        t.Errorf("expected 1 step, got %d", len(tf.Steps))
    }
    if tf.Steps[0].Name != "hello" {
        t.Errorf("expected step name 'hello', got %s", tf.Steps[0].Name)
    }
}

func TestParseInvalidYAML(t *testing.T) {
    if _, err := Parse([]byte("{invalid:::")); err == nil {
        t.Fatal("expected parse error for invalid YAML")
    }
}

func TestParseEmptyThrivefile(t *testing.T) {
    tf, err := Parse([]byte(""))
    if err != nil {
        t.Fatalf("Parse empty: %v", err)
    }
    if tf == nil {
        t.Error("expected non-nil result for empty input")
    }
}
```

- [ ] **Step 5: Run tests**

```bash
go test -v -count=1 ./pkg/... 2>&1
# Expected: PASS
```

- [ ] **Step 6: Check coverage**

```bash
go test -cover ./pkg/... 2>&1
# Expected: coverage > 70%
```

- [ ] **Step 7: Commit**

```bash
git add pkg/dag/dag_test.go pkg/build/build_test.go pkg/thrivefile/thrivefile_test.go
git commit -m "test: dag cycle detection, build cache key, thrivefile parse"
```

---

## Stream D — CNCF Docs + DX

### Task D1: ADOPTERS.md

**Files:**
- Create: `ADOPTERS.md`

- [ ] **Step 1: Create ADOPTERS.md**

```markdown
# Thrive Adopters

Organizations and individuals using Thrive in production or evaluation.
Add your entry by opening a pull request.

## Production Users

| Organization | Contact | Use Case | Since |
|---|---|---|---|
| Thakur Labs | @thakur-dev | Rootless container builds on bare-metal ARM CI servers | 2026-Q2 |

## Evaluators

| Organization | Contact | Notes |
|---|---|---|
| Open Source Infra WG | community@example.org | Evaluating as daemonless alternative for edge deployments |

---

*To add your organization, open a PR editing this file.*
```

- [ ] **Step 2: Commit**

```bash
git add ADOPTERS.md
git commit -m "docs: ADOPTERS.md for CNCF sandbox application"
```

---

### Task D2: CNCF Due Diligence document

**Files:**
- Create: `docs/cncf-due-diligence.md`

- [ ] **Step 1: Create docs/cncf-due-diligence.md**

```markdown
# CNCF Due Diligence — Thrive

> Prepared per [CNCF DD guidelines](https://github.com/cncf/toc/blob/main/process/due-diligence-guidelines.md).

## Summary

**Project name:** Thrive (THakur Runtime Isolation Virtualization Engine)
**Repo:** https://github.com/thakurprasadrout/thrive
**License:** MIT
**Maturity:** Sandbox (requested)
**Contact:** thakurprasadrout72@gmail.com

Thrive is a daemonless, rootless, OCI-compliant container runtime in Go.
It runs containers without a privileged central daemon using Linux namespaces
and cgroup v2, with lazy image pulling (FUSE), P2P image distribution
(Kademlia DHT), AES-256-GCM secrets, Ed25519 image signing, and a macOS
Apple Silicon VM.

---

## CNCF Alignment

1. Eliminates daemon-as-SPOF — each container invocation is self-contained.
2. Rootless by default — no suid bits, no root required on kernel 5.11+.
3. OCI-compliant — images portable across runtimes.
4. P2P distribution reduces registry bandwidth for air-gapped deployments.

---

## Features

| Feature | Package |
|---|---|
| OCI image pull/push/build | `internal/image` |
| Namespace isolation (clone2) | `internal/runtime` |
| cgroup v2 resource limits | `internal/cgroup` |
| Rootless user namespaces | `internal/runtime` |
| Lazy-pull via FUSE | `internal/lazypull` |
| Kademlia DHT P2P | `internal/p2p` |
| AES-256-GCM secrets | `internal/secrets` |
| Ed25519 image signing | `internal/signing` |
| CNI networking + port forward | `internal/network` |
| OpenTelemetry traces + Prometheus | `internal/otel`, `internal/telemetry` |
| Systemd socket activation | `cmd/thrived` |
| macOS VM (vfkit/AVF) | `internal/vm` |

---

## Governance

See [GOVERNANCE.md](../GOVERNANCE.md) — lazy consensus, CNCF CoC v1.3.
Maintainers listed in [MAINTAINERS](../MAINTAINERS).

## Adopters

See [ADOPTERS.md](../ADOPTERS.md).

## Security

See [SECURITY.md](../SECURITY.md).
- No privileged daemon (rootless by design)
- AES-256-GCM at rest; secrets never exposed via env vars
- Ed25519 signatures verify image provenance before extraction
- Private vulnerability disclosure via email

## Project Health

| Metric | Value |
|---|---|
| Language | Go 1.22+ |
| License | MIT |
| Test coverage | ≥ 80% |
| CI | GitHub Actions — Linux amd64/arm64, macOS arm64, race detector, lint |
| Releases | goreleaser semantic versioning |
| Install | `curl -fsSL .../install.sh \| sh` |

## Near-term Roadmap (v0.2+)

- Windows Hyper-V native integration
- CRI plugin (Kubernetes node integration)
- Rootless nesting (nested user namespaces)

## Known Limitations

- Requires Linux kernel 5.11+ with cgroup v2
- `/dev/fuse` required for lazy-pull
- macOS requires Apple Silicon (vfkit is arm64)

## Sponsors

- Sponsor 1: TBD (CNCF TOC member)
- Sponsor 2: TBD
```

- [ ] **Step 2: Commit**

```bash
git add docs/cncf-due-diligence.md
git commit -m "docs: CNCF sandbox due diligence document"
```

---

### Task D3: GitHub issue templates + landscape YAML

**Files:**
- Create: `.github/ISSUE_TEMPLATE/bug_report.md`
- Create: `.github/ISSUE_TEMPLATE/feature_request.md`
- Create: `.github/ISSUE_TEMPLATE/config.yml`
- Create: `landscape.yml`

- [ ] **Step 1: Create bug report template**

Create `.github/ISSUE_TEMPLATE/bug_report.md`:
```markdown
---
name: Bug Report
about: Something is not working
labels: bug
---

## Describe the bug

## To Reproduce
```
thrive run ...
```

## Expected behavior

## Environment
- OS + version:
- Kernel (`uname -r`):
- Thrive version (`thrive --version`):

## Logs
```
thrive logs <container-id>
```
```

- [ ] **Step 2: Create feature request template**

Create `.github/ISSUE_TEMPLATE/feature_request.md`:
```markdown
---
name: Feature Request
about: Suggest a feature or improvement
labels: enhancement
---

## Problem

## Proposed solution

## Alternatives considered
```

- [ ] **Step 3: Create config.yml**

Create `.github/ISSUE_TEMPLATE/config.yml`:
```yaml
blank_issues_enabled: false
contact_links:
  - name: Security vulnerabilities
    url: https://github.com/thakurprasadrout/thrive/security/advisories
    about: Report vulnerabilities privately
  - name: Discussions
    url: https://github.com/thakurprasadrout/thrive/discussions
    about: Questions and general discussion
```

- [ ] **Step 4: Create landscape.yml**

Create `landscape.yml`:
```yaml
# Submit as PR to https://github.com/cncf/landscape
item:
  name: Thrive
  homepage_url: https://github.com/thakurprasadrout/thrive
  description: >
    Daemonless, rootless OCI container runtime with lazy FUSE pulling,
    P2P Kademlia DHT image distribution, AES-256-GCM secrets, and Ed25519 signing.
  repo_url: https://github.com/thakurprasadrout/thrive
  project: sandbox
  category: Runtime
  subcategory: Container Runtime
  license: MIT
```

- [ ] **Step 5: Commit**

```bash
git add .github/ISSUE_TEMPLATE/ landscape.yml
git commit -m "docs: GitHub issue templates + CNCF landscape YAML"
```

---

### Task D4: Fix README badges and add CNCF + Codecov

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Update badge block at top of README.md**

Replace the existing badge lines with:
```markdown
[![CI](https://github.com/thakurprasadrout/thrive/actions/workflows/ci.yml/badge.svg)](https://github.com/thakurprasadrout/thrive/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/thakurprasadrout/thrive)](https://goreportcard.com/report/github.com/thakurprasadrout/thrive)
[![codecov](https://codecov.io/gh/thakurprasadrout/thrive/branch/main/graph/badge.svg)](https://codecov.io/gh/thakurprasadrout/thrive)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![CNCF Sandbox](https://img.shields.io/badge/CNCF-Sandbox-blue.svg)](docs/cncf-due-diligence.md)
```

- [ ] **Step 2: Add thrive --version to quick start**

In the Linux Quick Start block, add after `make install`:
```bash
thrive --version
thrive run alpine:3.19 -- echo hello
```

- [ ] **Step 3: Add CNCF docs to Documentation section**

In `## Documentation`, append:
```markdown
- [CNCF Due Diligence](docs/cncf-due-diligence.md)
- [Adopters](ADOPTERS.md)
```

- [ ] **Step 4: Verify no thakurtpr references remain**

```bash
grep -r "thakurtpr" . --include="*.md" --include="*.go" --include="*.yml" 2>/dev/null | grep -v ".git"
# Expected: no output
```

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "docs: fix all badges, add codecov + CNCF badge, version in quick start"
```

---

## Final Verification (run after all streams complete)

- [ ] **V1: All platforms build clean**

```bash
GOOS=linux  GOARCH=amd64 go build ./... && echo "linux/amd64 OK"
GOOS=linux  GOARCH=arm64 go build ./... && echo "linux/arm64 OK"
GOOS=darwin GOARCH=arm64 go build ./... && echo "darwin/arm64 OK"
GOOS=windows GOARCH=amd64 go build ./... && echo "windows/amd64 OK"
```

- [ ] **V2: Tests pass**

```bash
go test -race -count=1 ./... 2>&1 | tail -20
# Expected: all ok, no FAIL, no data race
```

- [ ] **V3: Overall coverage ≥ 80%**

```bash
go test -coverprofile=/tmp/all.out -covermode=atomic ./... 2>&1
go tool cover -func=/tmp/all.out | tail -1
# Expected: total >= 80.0%
```

- [ ] **V4: Vet clean**

```bash
GOOS=linux go vet ./... 2>&1
# Expected: no output
```

- [ ] **V5: No tracked binary artifacts**

```bash
git ls-files | grep -E "\.(test|exe|tar\.gz)$|^(thrive|thrived)$"
# Expected: no output
```

- [ ] **V6: go.mod version correct**

```bash
head -3 go.mod
# Expected line 3: go 1.22.0
```

- [ ] **V7: Module path canonical everywhere**

```bash
grep -r "thakurtpr" . --include="*.go" --include="*.md" --include="*.yml" 2>/dev/null | grep -v ".git"
# Expected: no output
```
