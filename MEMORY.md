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
- Phase 1: Core runtime (namespaces + cgroups + pivot_root) — FOUNDATION
- Phase 2: Image management (pull, layers, OverlayFS) — STORAGE
- Phase 3: CLI (thrive run, ps, kill, logs) — USABILITY
- Phase 4: Thrivefile + DAG build engine — BUILD SYSTEM
- Phase 5: Secrets manager (tmpfs vault) — SECURITY
- Phase 6: Lazy pulling via FUSE — PERFORMANCE
- Phase 7: Built-in OTEL observability — OBSERVABILITY
- Phase 8: P2P registry + chunk store — DISTRIBUTION

## Current phase
[x] Phase 1 — Core runtime (namespace isolation, cgroups, supervisor) ✓

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