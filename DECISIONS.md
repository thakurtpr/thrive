# Architecture Decision Records (ADRs)

Each ADR captures a significant architectural decision: the context that prompted it,
the decision made, and the tradeoffs accepted.

---

## ADR-001: Daemonless architecture (no thrive daemon)

**Status:** Accepted
**Date:** 2026-05-16

### Context

Docker requires a root daemon (`dockerd`) running at all times. Every container operation
goes through a Unix socket to that daemon. If dockerd crashes, all containers lose their
management plane. Running dockerd requires root or careful privilege escalation setup.

### Decision

Every `thrive run` invocation calls `clone(2)` directly. The calling process IS the runtime.
There is no long-lived `thrived` daemon, no Unix socket, no privilege escalation via a setuid
helper. State is tracked via files on disk (`state.json`) that any `thrive` invocation can read.

### Tradeoffs

**Pros:**
- No SPOF: a crash affects only the single container whose parent process died
- No socket: no privilege escalation, no IPC complexity
- Simple mental model: `thrive run` = fork + exec inside namespaces

**Cons:**
- No central event bus: subscribing to container lifecycle events requires polling state files
- No streaming logs push: `thrive logs --follow` tails a file rather than receiving events
- Reconciliation complexity: if the parent process dies unexpectedly, orphaned container
  processes must be detected via PID file staleness checks

---

## ADR-002: SysProcAttr.Chroot vs full pivot_root re-exec

**Status:** Accepted (v0.x); Revisit for v1.0
**Date:** 2026-05-16

### Context

OCI Runtime Specification compliance requires full mount namespace isolation. The canonical
approach (used by runc) is a two-phase re-exec: the runtime re-execs itself into the new mount
namespace, calls `pivot_root(2)` to make the container rootfs the new `/`, then unmounts the
old root. This is complex: it requires the binary to detect it is in "init" mode, parse config
from a pipe, and execute the pivot before any user process starts.

### Decision

For v0.x, use `SysProcAttr.Chroot` which Go's `os/exec` sets via `chroot(2)` before exec.
This is simpler, avoids re-exec complexity, and is sufficient for trusted workloads.

### Tradeoffs

**Pros:**
- ~20 lines of code vs ~200 lines for full re-exec
- Easier to audit and debug
- Sufficient isolation for development and CI workloads

**Cons:**
- Not strict OCI Runtime Spec compliant (`pivot_root` is required by spec)
- Old root is theoretically accessible via `..` traversal without additional bind-mount hardening
- Not suitable for multi-tenant or untrusted workload hosting

**Resolution path:** See RISKS.md RISK-004. Implement pivot_root re-exec for v1.0.

---

## ADR-003: AES-256-GCM for secrets, not age/gpg

**Status:** Accepted
**Date:** 2026-05-16

### Context

Container secrets need file-level encryption at rest. Options considered:
- `age`: modern, simple CLI tool; adds external binary dependency
- `gpg`: widely available; complex key management, not programmatic-friendly
- Native Go `crypto/aes` + `crypto/cipher` (GCM mode): no external deps, programmatic

### Decision

Use AES-256-GCM implemented directly with Go's `crypto` stdlib. Auto-generate a 32-byte
master key on first run using `crypto/rand`. Persist at `/var/lib/thrive/secrets/.master`
(chmod 0600). Each secret encrypted with a random 12-byte nonce prepended to ciphertext.

### Tradeoffs

**Pros:**
- Zero external dependencies
- Programmatic: encrypt/decrypt within the same binary, no subprocess
- GCM provides authenticated encryption (integrity + confidentiality)
- Nonce-per-secret prevents IV reuse attacks across secrets

**Cons:**
- Key is on the same filesystem as the secrets (acceptable for single-host; see RISKS.md RISK-005)
- No key rotation mechanism implemented
- No envelope encryption (key-encrypting-key hierarchy)

---

## ADR-004: fuse-overlayfs fallback vs kernel overlay only

**Status:** Accepted
**Date:** 2026-05-16

### Context

Kernel OverlayFS requires root or Linux 5.11+ for unprivileged use. On older kernels or CI
environments without kernel overlay support, `mount("overlay", ...)` returns EPERM or ENODEV.
`fuse-overlayfs` is a FUSE-based userspace implementation that works rootless on any kernel
with FUSE support, but requires the `fuse-overlayfs` binary on the host PATH.

### Decision

Attempt kernel overlay first. On EPERM or ENODEV, fall back to `exec.Command("fuse-overlayfs", ...)`.
Emit a clear log line indicating which path was taken.

### Tradeoffs

**Pros:**
- Best performance path (kernel overlay) used when available
- Rootless compatibility on older kernels via fallback
- No bundled binary required in the common case

**Cons:**
- fuse-overlayfs must be installed on the host for fallback to work
- FUSE path has higher latency (~10-15% overhead for metadata-heavy workloads)
- Two code paths to test and maintain

**Resolution path:** See RISKS.md RISK-001. Add preflight check and clear error message when
fuse-overlayfs is absent.

---

## ADR-005: Content-addressed chunk store (SHA-256)

**Status:** Accepted
**Date:** 2026-05-16

### Context

Docker deduplicates at the layer level: if two images share an identical layer (same digest),
that layer is stored once in `/var/lib/docker/overlay2/`. However, if two images have slightly
different layers that share most content, all bytes are duplicated.

THRIVE targets a fleet of servers pulling similar images. Deduplication at the chunk level
(fixed-size blocks, content-addressed by SHA-256) achieves much higher deduplication ratios.
This is also the foundation for P2P chunk distribution: peers can serve individual chunks
regardless of which image they came from.

### Decision

Split OCI image layers into fixed-size chunks during `Pull()`. Address each chunk by its
SHA-256 digest. Store at `/var/lib/thrive/chunks/{xx}/{rest}` where `{xx}` is the first two
hex characters of the digest (directory sharding to avoid inode limits).

### Tradeoffs

**Pros:**
- Sub-layer deduplication: images sharing common files (e.g. libc) share chunks
- Foundation for P2P: any node that has a chunk can serve it to peers
- Foundation for lazypull: FUSE can fetch individual chunks on demand

**Cons:**
- Higher implementation complexity than layer-level storage
- Chunk boundary alignment means the last chunk of a layer is smaller (wasted space is minimal)
- Rebuilding a full layer for `Push()` requires re-reading all chunks in order
- No whiteout awareness at chunk level (layer-level concern; see RISKS.md RISK-006)

---

## ADR-006: Desktop VM launchers via subprocess, not CGO

**Status:** Accepted
**Date:** 2026-05-17

### Context

The "Thrive Desktop" feature must launch a Linux VM on macOS, Windows-Hyper-V, and
Windows-WSL2 so that container workloads can run on developer machines without
native Linux. Options considered:

- **A. CGO bindings to Virtualization.framework (macOS) and hcsshim (Windows).** Use
  `Code-Hex/vz` for macOS, `Microsoft/hcsshim` for Hyper-V. Most direct API surface.
- **B. Embed QEMU.** Single hypervisor backend across all three OSes; large binary,
  slower than native hypervisors.
- **C. Shell out to existing CLI tools** — `vfkit` on macOS, `wsl.exe` and PowerShell
  on Windows. Each tool already wraps the native hypervisor and is independently
  maintained.

The previous session left six "not implemented" stubs in `lifecycle_stubs.go` because
options A/B looked like multi-day commitments.

### Decision

Subprocess (Option C). The desktop launchers — `darwinLauncher`, `wsl2Launcher`,
`hyperVLauncher` — each shell out to the relevant CLI. All external interactions are
abstracted behind three function-typed fields (`commandRunner`, `processStarter`,
`pathLookup`) so tests inject mocks instead of executing real processes.

### Tradeoffs

**Pros:**
- Cross-compiles cleanly without an SDK (no CGO, no platform-specific headers).
- Inherits correctness from already-shipping tools maintained by Apple/Microsoft/Red Hat.
- Tests can verify the full subprocess contract (argv, error wrapping, idempotency)
  without ever spawning a real process.
- Single-session implementation across all three platforms.

**Cons:**
- Requires users to install `vfkit` on macOS (the launcher returns a clear
  `brew install` hint when missing).
- Less surface to optimize — can't pipeline syscalls or hold a live hypervisor handle.
- Argument-parsing brittleness if upstream CLIs change flags (mitigated by tests
  asserting the exact argv we send).

### Future migration path

If the subprocess approach becomes limiting (e.g. need for low-latency VM device
hotplug), the `launcher` interface in `lifecycle.go` makes it straightforward to
add an alternative implementation behind the same dispatch.
