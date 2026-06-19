# CNCF Due Diligence — Thrive

> Prepared per [CNCF DD guidelines](https://github.com/cncf/toc/blob/main/process/due-diligence-guidelines.md).

## Summary

**Project name:** Thrive (THakur Runtime Isolation Virtualization Engine)
**Repo:** https://github.com/thakurtpr/thrive
**License:** MIT
**Maturity:** Sandbox (requested)
**Contact:** thakurprasadrout72@gmail.com

Thrive is a daemonless, rootless, OCI-compliant container runtime in Go.
It runs containers without a privileged central daemon using Linux namespaces
and cgroup v2, with lazy image pulling (FUSE), P2P image distribution
(Kademlia DHT), AES-256-GCM secrets, Ed25519 image signing, and an Apple
Silicon macOS VM.

---

## CNCF Alignment

1. Eliminates daemon-as-SPOF — each container invocation is self-contained.
2. Rootless by default — no suid bits, no root required on kernel 5.11+.
3. OCI-compliant — images portable across all OCI-conformant runtimes.
4. P2P distribution reduces registry bandwidth for air-gapped deployments.

---

## Feature Matrix

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

## Architecture

```
thrive CLI (darwin/linux/windows)
    │
    └──proxy──▶ thrived (linux daemon) or vfkit VM (macOS)
                     │
        ┌────────────┴───────────────┐
        │  runtime + cgroup          │
        │  image (OverlayFS/FUSE)   │
        │  network (CNI + NAT)      │
        │  P2P DHT + lazy pull      │
        │  secrets (AES-GCM)        │
        │  signing (Ed25519)        │
        │  OTLP + Prometheus        │
        └───────────────────────────┘
```

---

## Governance

See [GOVERNANCE.md](../GOVERNANCE.md) — lazy consensus, CNCF CoC v1.3.
Maintainers in [MAINTAINERS](../MAINTAINERS).

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
| Test coverage | ≥ 80% (CI enforced) |
| CI | GitHub Actions — Linux amd64/arm64, macOS arm64, race detector, lint |
| Releases | goreleaser, semantic versioning |
| Install | `curl -fsSL .../scripts/install.sh \| sh` |

## Near-term Roadmap (v0.2+)

- Windows Hyper-V native integration
- CRI plugin (Kubernetes node)
- Rootless nesting

## Known Limitations

- Requires Linux kernel 5.11+ with cgroup v2 mounted
- `/dev/fuse` required for lazy-pull on Linux
- macOS support requires Apple Silicon (vfkit is arm64)

## Sponsors

- Sponsor 1: TBD (CNCF TOC member)
- Sponsor 2: TBD
