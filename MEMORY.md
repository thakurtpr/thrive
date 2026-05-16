# THRIVE — MEMORY

## What this project is
THRIVE (THakur Runtime Isolation Virtualization Engine) is a daemonless,
rootless OCI-compliant container runtime built in Go. It is a Docker
alternative that solves Docker's core limitations.

## Core design decisions (never change these)
- DAEMONLESS: Every container is a direct child process via clone(2). No central daemon.
- ROOTLESS: User namespaces by default. No sudo ever required.
- OCI COMPLIANT: Fully implements OCI Runtime Spec and Image Spec.
- CONTENT-ADDRESSED STORAGE: SHA-256 chunk store. Identical chunks shared across images.
- LAZY PULLING: Containers boot before full image download via FUSE mount.
- PARALLEL DAG BUILDS: Thrivefile steps as dependency graph, not sequential.
- BUILT-IN OTEL: Metrics, logs, traces exported natively.

## Architecture layers (top to bottom)
1. CLI (cobra) → cmd/thrive/
2. Control Plane (image resolver, DAG engine, secrets, network planner) → pkg/
3. Container Runtime (OCI via crun, namespaces, cgroups v2) → internal/runtime/
4. Storage (OverlayFS, chunk store, lazy puller) → internal/storage/
5. Network (CNI, veth pairs, bridge, eBPF) → internal/network/
6. Registry (OCI client, P2P chunks, signing) → internal/registry/
7. Observability (OTEL collector, metrics, traces) → internal/telemetry/

## Key Linux syscalls used
- clone(2) with CLONE_NEWPID | CLONE_NEWNET | CLONE_NEWNS | CLONE_NEWUTS | CLONE_NEWIPC | CLONE_NEWUSER
- pivot_root(2) for rootfs switch
- unshare(2) for namespace separation
- mount(2) for OverlayFS setup
- prctl(2) for capability management

## Key Go packages
- golang.org/x/sys/unix — syscall wrappers
- github.com/opencontainers/runtime-spec — OCI runtime spec types
- github.com/opencontainers/image-spec — OCI image spec types
- github.com/google/go-containerregistry — registry client
- github.com/spf13/cobra — CLI
- github.com/spf13/viper — config
- go.opentelemetry.io/otel — observability
- github.com/containerd/cgroups/v3 — cgroup v2
- github.com/vishvananda/netlink — network link management
- github.com/hanwen/go-fuse/v2 — FUSE for lazy pulling

## Module name
github.com/thakurprasadrout/thrive

## Build phases
- Phase 1: Core runtime (namespaces + cgroups + supervisor) ✓
- Phase 2: Image management (pull, layers, ChunkStore) ✓
- Phase 3: CLI (thrive run, ps, kill, logs, rm, images, rmi) ✓
- Phase 4: Thrivefile + DAG build engine ✓
- Phase 5: Secrets manager (tmpfs vault) ✓
- Phase 6: Lazy pulling via FUSE ✓
- Phase 7: Built-in OTEL observability ✓
- Phase 8: P2P registry + chunk store ✓

## Current phase
[x] Phase 8 — P2P registry + chunk store complete. ALL PHASES IMPLEMENTED.

## Important file locations
- Container state: /run/thrive/containers/{id}/
- Image store: /var/lib/thrive/images/
- Chunk store: /var/lib/thrive/chunks/
- Network state: /run/thrive/network/
- Secrets store: /var/lib/thrive/secrets/ (encrypted)
- Logs: /var/log/thrive/

## Known gotchas
- pivot_root requires a mount point, always bind-mount the new root first
- User namespace UID/GID mapping must be set before any capability operations
- OverlayFS needs kernel 5.11+ for rootless support (use fuse-overlayfs as fallback)
- cgroup v2 requires systemd or manual cgroupfs mounting in some distros
- FUSE requires /dev/fuse access — check and error clearly if missing

## Desktop subsystem (Phase 9, 2026-05-17)

### Architecture
- `internal/vm/lifecycle.go` exposes a `launcher` interface (Start/Stop) and `selectLauncher` dispatch on `cfg.VMType`.
- Three real launchers: `darwinLauncher` (vfkit), `wsl2Launcher` (wsl.exe), `hyperVLauncher` (powershell.exe).
- All launchers take `commandRunner` / `processStarter` / `pathLookup` function fields so tests inject mocks instead of touching real processes. Production callers receive wired-up defaults via `newXxxLauncher()`.
- Build tag pattern: `//go:build !linux` on every desktop file. Linux uses `cmd/thrive/commands/desktop_linux_stub.go` (intentional no-op).
- `lifecycle.go` `Start`/`Stop` persist `*VMState` via `WriteVMState` automatically after each lifecycle action — callers no longer need to do this themselves (existing desktop.go callers still do, redundant but harmless).

### Subprocess design quirks
- **vfkit is a foreground process** — must use `processStarter` (which calls `cmd.Start()` + `cmd.Process.Release()`), not `commandRunner` (which waits via `CombinedOutput`).
- **WSL/PowerShell commands are one-shots** — return immediately after dispatching to the WSL/Hyper-V services; safe to use `commandRunner` with timeout via ctx.
- **vfkit binary path discovery** — `exec.LookPath("vfkit")` returns `exec.ErrNotFound` cleanly; surface `brew install cfergeau/crc/vfkit` hint in the error message so users have an actionable fix.
- **wsl --list --quiet output is UTF-16LE on older Windows builds** — `distroAlreadyExists` lowercases both sides and does substring match to tolerate either encoding.

### Cross-compile constraints
- `getlantern/systray` (used by `desktop/tray.go`) requires CGO + native macOS SDK for darwin/amd64. Cross-compiling from arm64 Mac fails with `undefined: nativeLoop` etc. Workaround: build darwin/amd64 natively on an Intel Mac. The non-tray code (`./cmd/thrive`) compiles cleanly for every platform.
- Windows builds use `github.com/Microsoft/go-winio` for the bridge — works fine with `GOOS=windows GOARCH=amd64 go build`.
- All other targets (linux/{amd64,arm64}, darwin/arm64, windows/{amd64,arm64}) cross-compile clean from any host.

### Local VM image override
- `DownloadVMImage` checks `$THRIVE_VM_IMAGE_PATH` before the HTTP fallback. Set it to a local tarball to bypass the GitHub release dependency entirely. Useful for: air-gapped installs, dev loops, CI, and the current situation where no VM image has been published yet.
- The tarball must be a gzipped tar; contents are extracted to `~/.thrive/vm/` (or `%LOCALAPPDATA%/Thrive/vm/` on Windows).

### Why subprocess instead of CGO Virtualization.framework bindings
- Subprocess launchers compile cleanly cross-platform with no SDK requirement.
- vfkit/wsl/PowerShell are already-maintained CLI surfaces; we inherit their correctness.
- Trivial to mock in tests by injecting a fake `commandRunner`.
- Trade-off: latency of `exec.Command` vs in-process CGO call is negligible for boot-once workflows.
- Decision recorded in DECISIONS.md ("Desktop launcher transport").