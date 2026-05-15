# THRIVE — AGENTS

> THRIVE is built by specialized agents working in parallel.
> Each agent owns a domain. Agents communicate via interfaces, not internals.
> Read your assignment. Stay in your lane. Document your interfaces.

---

## Agent Roster

### Agent 0 — ORCHESTRATOR
**Role:** Project coordinator. Reads all agent outputs. Resolves conflicts.
Writes integration tests. Updates HANDOFF.md before every compaction.
**Owns:** cmd/thrive/, MEMORY.md, HANDOFF.md, AGENTS.md, Makefile, go.mod
**Never touches:** Any pkg/ or internal/ package internals
**Key responsibility:** Keep all agents in sync. If two agents produce
conflicting interfaces, ORCHESTRATOR decides which wins.

---

### Agent 1 — RUNTIME AGENT
**Role:** The heart. Implements container lifecycle using Linux primitives.
**Owns:** internal/runtime/, internal/namespace/, internal/cgroup/, internal/supervisor/
**Delivers:**
- `runtime.Create(config ContainerConfig) (*Container, error)`
- `runtime.Start(id string) error`
- `runtime.Kill(id string, signal syscall.Signal) error`
- `runtime.Delete(id string) error`
- `runtime.State(id string) (*ContainerState, error)`
**Key syscalls:** clone(2), pivot_root(2), unshare(2), mount(2), prctl(2)
**Key packages:** golang.org/x/sys/unix, github.com/containerd/cgroups/v3
**Constraints:**
- ALL containers must run rootless via user namespaces
- cgroup v2 only (no v1 fallback)
- Supervisor must handle SIGCHLD and restart on failure per policy
- State must be persisted to /run/thrive/containers/{id}/state.json

---

### Agent 2 — IMAGE AGENT
**Role:** Everything about images — pull, store, layer management, lazy loading.
**Owns:** internal/image/, internal/storage/, internal/lazypull/
**Delivers:**
- `image.Pull(ref string, opts PullOptions) (*Image, error)`
- `image.Mount(imageID string, containerID string) (string, error)` — returns rootfs path
- `image.Unmount(containerID string) error`
- `image.List() ([]*Image, error)`
- `image.Remove(id string) error`
**Key packages:** github.com/google/go-containerregistry, github.com/hanwen/go-fuse/v2
**Constraints:**
- Content-addressed chunk store: every layer chunk stored by SHA-256
- OverlayFS for writable container layer (fuse-overlayfs fallback for rootless)
- Lazy pull: FUSE filesystem serves chunks on-demand, background goroutine prefetches
- Image metadata in /var/lib/thrive/images/{digest}/manifest.json

---

### Agent 3 — BUILD AGENT
**Role:** The Thrivefile parser and parallel DAG build engine.
**Owns:** pkg/build/, pkg/dag/, pkg/thrivefile/
**Delivers:**
- `build.Parse(path string) (*BuildGraph, error)` — parse Thrivefile into DAG
- `build.Execute(graph *BuildGraph, opts BuildOptions) error` — parallel build
- `build.CacheKey(step *Step) (string, error)` — content-addressed cache key
**Thrivefile format:** YAML-based, steps declare deps explicitly
**Key packages:** gopkg.in/yaml.v3, golang.org/x/sync/errgroup
**Constraints:**
- Steps with no declared dependency run in parallel
- Cache key = SHA-256 of (base image digest + command + input file hashes + env vars)
- Never invalidate a step's cache unless its direct inputs changed
- Build context sent to build step as tar stream, not bind mount

---

### Agent 4 — NETWORK AGENT
**Role:** Container networking — bridge, veth pairs, NAT, DNS.
**Owns:** internal/network/
**Delivers:**
- `network.Setup(containerID string, config NetworkConfig) (*NetworkInfo, error)`
- `network.Teardown(containerID string) error`
- `network.Connect(containerID string, networkName string) error`
**Key packages:** github.com/vishvananda/netlink, github.com/containernetworking/plugins
**Constraints:**
- Default bridge network: thrive0 (172.20.0.0/16)
- Each container gets a veth pair: vethXXXXXX (host) + eth0 (container)
- NAT via iptables/nftables for outbound connectivity
- Embedded DNS resolver for container name resolution on same network
- Must work rootless via slirp4netns as fallback

---

### Agent 5 — REGISTRY AGENT
**Role:** OCI registry client, image push/pull, P2P chunk sharing.
**Owns:** internal/registry/
**Delivers:**
- `registry.Pull(ref string) ([]Chunk, error)`
- `registry.Push(imageID string, ref string) error`
- `registry.PeerFetch(chunkDigest string) ([]byte, error)` — P2P chunk fetch
**Key packages:** github.com/google/go-containerregistry
**Constraints:**
- OCI Distribution Spec v1.1 compliant
- Pull order: local chunk store → cluster peers → configured mirrors → origin registry
- Peer discovery via UDP broadcast on local subnet (simple, no external dependency)
- Image signing via cosign-compatible signature verification on pull

---

### Agent 6 — SECRETS AGENT
**Role:** Secrets lifecycle — store, encrypt, inject, revoke.
**Owns:** pkg/secrets/
**Delivers:**
- `secrets.Create(name string, value []byte) error`
- `secrets.Inject(containerID string, secretNames []string) (string, error)` — returns tmpfs mount path
- `secrets.Revoke(containerID string) error`
**Constraints:**
- Secrets encrypted at rest using AES-256-GCM
- Master key derived from machine-unique entropy (TPM if available, /dev/urandom fallback)
- Injected as individual files at /run/secrets/{name} inside container via tmpfs
- tmpfs unmounted on container exit — secrets never touch disk inside container
- NEVER inject as environment variables

---

### Agent 7 — OBSERVABILITY AGENT
**Role:** Built-in metrics, logs, traces via OpenTelemetry.
**Owns:** internal/telemetry/
**Delivers:**
- `telemetry.Init(config TelemetryConfig) error`
- `telemetry.ContainerMetrics(id string) *ContainerMetrics` — CPU, mem, net, disk
- `telemetry.Span(ctx context.Context, name string) (context.Context, trace.Span)`
- `telemetry.Log(level, msg string, fields ...zap.Field)`
**Key packages:** go.opentelemetry.io/otel, go.uber.org/zap
**Constraints:**
- Zero-config: default export to stdout (structured JSON) with no backend configured
- OTLP export when THRIVE_OTEL_ENDPOINT env var set
- Container CPU/memory metrics via cgroup v2 files (no procfs polling)
- Trace context propagated via W3C TraceContext headers
- Every CLI command automatically creates a root span

---

## Agent Communication Rules

1. **Agents talk through interfaces, never internals.**
   Import `internal/runtime` → use only exported functions. Never reach into structs.

2. **Interface changes need ORCHESTRATOR sign-off.**
   If Agent 2 needs to change the `Mount()` signature, Agent 0 must approve
   and notify Agent 1 (who calls Mount).

3. **Each agent writes its own unit tests.**
   Coverage target: 70% minimum per package.

4. **Integration tests are ORCHESTRATOR's job.**
   Agents write unit tests for their own domain only.

5. **Agents document their packages.**
   Every exported function needs a godoc comment.

6. **Error handling contract:**
   All errors must be wrapped with context: `fmt.Errorf("runtime.Create: %w", err)`
   Never swallow errors. Never panic in library code.

---

## Build Order (dependency graph)

```
Agent 7 (Telemetry)   ← no deps, build first
Agent 6 (Secrets)     ← no deps, build alongside
Agent 4 (Network)     ← no deps, build alongside
Agent 5 (Registry)   ← no deps, build alongside
Agent 2 (Image)       ← depends on Registry (Agent 5)
Agent 1 (Runtime)     ← depends on Image (Agent 2), Network (Agent 4), Secrets (Agent 6)
Agent 3 (Build)       ← depends on Image (Agent 2), Registry (Agent 5)
Agent 0 (Orchestrator) ← depends on all agents — wires CLI to everything
```

**In a single-developer flow:**
Build agents in exactly this order. Each phase must compile and
have basic tests passing before moving to the next.