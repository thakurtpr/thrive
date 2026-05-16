# Risk Register

Severity levels: HIGH (blocks production use) | MED (degrades functionality) | LOW (minor / cosmetic)

---

## HIGH Severity

### RISK-001: OverlayFS requires root on kernels older than 5.11
**Severity:** HIGH
**Area:** internal/image
**Description:** Kernel OverlayFS (`mount("overlay", ...)`) requires CAP_SYS_ADMIN or a kernel
with unprivileged overlay support (Linux 5.11+). On older kernels, non-root users cannot mount
overlay filesystems.
**Mitigation:** fuse-overlayfs fallback is implemented — but it requires the `fuse-overlayfs`
binary to be installed on the host system. If the binary is absent, container start fails.
**Resolution path:** Document minimum kernel version (5.11+) in README. Add preflight check that
verifies fuse-overlayfs is available when kernel overlay fails.

---

### RISK-002: cgroup v2 directory creation requires systemd delegation
**Severity:** HIGH
**Area:** internal/cgroup
**Description:** Creating `/sys/fs/cgroup/thrive/{containerID}/` requires write permission to the
cgroup v2 hierarchy. On systemd-managed systems without a delegated slice, non-root processes
cannot create cgroup directories.
**Mitigation:** Tests skip when not running as root. Production use requires either root, a
systemd slice with `Delegate=yes`, or a system with rootless cgroup delegation enabled.
**Resolution path:** Phase 12 (systemd integration) will add a `thrive.slice` with delegation.
Until then, document the requirement explicitly.

---

### RISK-003: P2P chunk distribution has no authentication
**Severity:** HIGH
**Area:** internal/p2p
**Description:** A malicious peer can respond to `RequestChunk` with corrupt or adversarial data.
There is currently no signature verification on received chunks — only a SHA-256 digest check
against the expected digest.
**Mitigation:** SHA-256 digest verification prevents silent corruption. Does not prevent a peer
from serving a crafted payload if the expected digest is not independently verified.
**Resolution path:** Phase 11 (image signing) will add cosign-compatible chunk signing. Until
then, only use P2P on trusted internal networks.

---

## MED Severity

### RISK-004: SysProcAttr.Chroot vs full pivot_root re-exec
**Severity:** MED
**Area:** internal/runtime
**Description:** THRIVE uses `SysProcAttr.Chroot` for rootfs confinement, which is simpler but
does not achieve full mount namespace isolation that `pivot_root(2)` provides. The old root
remains accessible via `..` traversal in some kernel configurations.
**Mitigation:** Acceptable for v0.x development. Not suitable for untrusted workloads.
**Resolution path:** ADR-002 tracks this. Implement full pivot_root re-exec pattern (as used by
runc) for v1.0 strict OCI Runtime Spec compliance.

---

### RISK-005: AES-256 master key stored on same filesystem as secrets
**Severity:** MED
**Area:** internal/secrets
**Description:** The master key lives at `/var/lib/thrive/secrets/.master` (chmod 0600). An
attacker with read access to the filesystem can decrypt all secrets stored in the same directory.
**Mitigation:** chmod 0600 limits access to the owning user. Acceptable for single-host
development and CI use.
**Resolution path:** Future: TPM-backed key sealing or integration with a hardware security
module or secrets manager (Vault, AWS KMS). Out of scope for v0.x.

---

### RISK-006: Layer extraction does not handle whiteout files
**Severity:** MED
**Area:** internal/image
**Description:** OCI image layers use Docker-style whiteout files (`.wh.` prefix) to signal file
deletions from lower layers. The current `Pull()` extraction does not process whiteouts, so files
deleted in upper layers may still appear in the merged rootfs.
**Mitigation:** Affects images with multi-layer delete operations (e.g. slim/distroless builds).
Alpine base images are largely unaffected.
**Resolution path:** Add whiteout processing in `extractLayer()`: detect `.wh.` prefix, create
corresponding whiteout files so OverlayFS handles deletions correctly.

---

### RISK-007: lazypull.fetchChunk does not retry on transient HTTP errors
**Severity:** MED
**Area:** internal/lazypull
**Description:** A single HTTP 5xx or network timeout from the OCI registry causes the FUSE read
to fail immediately with no retry. Applications inside the container see an I/O error.
**Resolution path:** Add exponential backoff retry (3 attempts, 1s/2s/4s) in `fetchChunk`.

---

### RISK-008: p2p.Bootstrap does not verify remote node identity
**Severity:** MED
**Area:** internal/p2p
**Description:** Bootstrap contacts a known node address without any identity verification. A
DNS-hijack or ARP-poisoning attack could redirect bootstrap to a malicious node that populates
the routing table with adversarial peers.
**Resolution path:** Add TLS mutual authentication for bootstrap connections. Tracked under
Phase 11 security hardening.

---

## LOW Severity

### RISK-009: OTLP exporter uses insecure gRPC (no TLS)
**Severity:** LOW
**Area:** internal/otel
**Description:** `otel.Init` dials the OTLP endpoint with `grpc.WithInsecure()`. Trace data
transmitted over untrusted networks is observable in plaintext.
**Mitigation:** Acceptable for internal cluster observability pipelines (same-datacenter).
**Resolution path:** Add `OTEL_EXPORTER_OTLP_INSECURE=false` config option that enables TLS
dial when set.

---

### RISK-010: Log files are not rotated
**Severity:** LOW
**Area:** internal/runtime
**Description:** Container stdout/stderr is appended to `/run/thrive/containers/{id}/logs`
indefinitely. Long-lived containers or verbose workloads can fill the filesystem.
**Resolution path:** Implement log rotation (max size + max files) using a rolling writer.
Candidate: `gopkg.in/natefinish/lumberjack.v2`.

---

### RISK-011: No image garbage collection
**Severity:** LOW
**Area:** internal/image
**Description:** Pulled images accumulate in `/var/lib/thrive/images/` and chunk blobs in
`/var/lib/thrive/chunks/` indefinitely. No reference counting or GC is implemented.
**Resolution path:** Implement `thrive system prune` that removes images not referenced by any
container state, and orphaned chunks not referenced by any image manifest.
