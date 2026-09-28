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

---

## R2 flag depth (2026-09-15) — logs --since/--until/--timestamps

### Context
`docker logs` supports `--since/--until/--timestamps`. Thrive is daemonless:
detached container stdio streams straight to `/run/thrive/containers/{id}/logs`
with no per-line timestamps recorded.

### Decision
Register all three flags on every platform (CLI parity: scripts passing them
get a clear thrive error instead of `unknown flag`), but honestly refuse at
runtime with an explicit message. The daemon (`handleLogs`) rejects them too,
so VM-proxied calls behave identically. No fake timestamp synthesis.

### If this ever changes
Recording per-line timestamps would require a stdio proxy process between the
container and the log file (a small daemon-shaped component), which contradicts
the daemonless design. Revisit only if the project accepts that tradeoff.

## R2 flag depth (2026-09-15) — diff Changed/Deleted

### Decision
`runtime.Diff` now reports Docker-style kinds: OverlayFS whiteout char devices
(rdev 0/0) and `.wh.` markers → Deleted; opaque dirs (`trusted.overlay.opaque`)
→ Changed; entries also present in a lower image layer (via the container's
recorded image manifest) → Changed; otherwise → Added. No lower-layer access
(e.g. image gone) degrades to Added, never to an error.

## Copy-fallback diff/commit (2026-09-19)

### Context
`image.Mount` falls back to copying layers into `merged/` when neither
kernel overlay nor fuse-overlayfs is available (found via CI e2e: `diff`
on such containers was always empty because it only walked `upper/`).

### Decision
`Mount` records `overlay`/`copy` in `<container>/mount-mode` (absent =
legacy overlay). In copy mode, `Diff` compares merged-vs-lower layers
(Added/Changed by SHA-256 + symlink targets, Deleted for lower-only
files, sorted output) and `Commit` snapshots the full merged rootfs as a
single squashed layer (deletions inherently captured). Overlay behavior
is untouched.

### Tradeoffs
Committed copy-mode images duplicate base files in their layer, and a
squashed layer mounted under a later copy fallback won't apply whiteouts
across generations. Accepted for v1: correctness (no silent empty diffs)
over storage optimality.

## nsenter --root (2026-09-19)

### Context
`thrive exec` entered the container's namespaces via nsenter but never its
root: nsenter changes namespaces, not the caller's chroot. Filesystem
writes (e.g. `touch /x`) landed on the host. Found because copy-mode
`diff` stayed empty even after the merged-vs-lower fix — the file was
never in the container. Read-only probes (`uname -a`) masked it.

### Decision
Both nsenter call sites (Linux CLI, VM daemon) pass
`--root=/proc/<pid>/root`, which always tracks the target's root,
including chroot-only containers. The `=` form is mandatory: nsenter
declares `--root` with an optional argument, so a space-separated path is
misparsed as the command (found live in CI). E2E now asserts the exec exit
code before the diff assertion so a future exec regression is diagnosable
instead of silent.

## R3 (2026-09-19) — cosign verification is host-side

### Context
`thrive pull --verify --verify-key cosign.pub` checks the OCI `.sig`
artifact via sigstore-go. The PEM key file lives on the client machine;
the VM daemon (`thrived`) never sees host files.

### Decision
Verification runs in the client, never in the daemon:
- Linux/darwin pull natively and call shared `verifyPulledImage`, which
  removes the image on failure so untrusted content is never left behind.
- Windows pulls inside the VM via the bridge, then verifies host-side
  (remote registry lookup needs no image bytes) and issues `rmi` over the
  bridge on failure. With `--all-tags` every tag is verified.
- `thrived handlePull` explicitly rejects a `verify` opt with a clear
  error, so no future client can silently skip verification.

### Non-goal
Keyless (Fulcio/Rekor) identities stay out of scope: offline verification
against an explicit public key file only.

## R3 (2026-09-19) — stats streaming keeps one-shot default

### Context
`docker stats` streams by default; thrive historically defaulted to a
one-shot snapshot (`--no-stream=true`, "streaming not yet supported").

### Decision
Streaming is now implemented (2s client-side poll until SIGINT, one-shot
per-container fetch on the daemon), but the default stays one-shot for
back-compat. Pass `--no-stream=false` to stream. Flipping the default is
a deliberate breaking change left for a later release.
