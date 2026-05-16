# THRIVE Architecture

## Design Philosophy

THRIVE is a daemonless, rootless, OCI-compliant container runtime. The guiding principles are:

- **No daemon, no SPOF.** Every `thrive run` invocation IS the runtime. There is no long-lived
  privileged process. A crash affects only that container, not all running workloads.
- **Rootless by default.** User namespaces + fuse-overlayfs mean no suid bits and no root required
  on modern kernels (5.11+). On older kernels, the fuse-overlayfs binary provides the fallback.
- **OCI compliance.** Images are pulled and stored per the OCI Image Layout Specification.
  Container execution follows OCI Runtime Specification semantics.
- **Content-addressed storage.** Layers are split into fixed-size SHA-256 chunks. Identical chunks
  across images are stored exactly once.

---

## Layer Diagram

```
┌─────────────────────────────────────────────────────────┐
│                        CLI Layer                         │
│  thrive run | ps | kill | logs | build | pull | push    │
│  thrive secret | metrics | system                        │
└────────────────────────┬────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────┐
│                   Control Plane                          │
│  pkg/build  (DAG executor)                               │
│  pkg/dag    (topological sort)                           │
│  pkg/thrivefile (Thrivefile parser)                      │
└──────┬──────────────────────────┬───────────────────────┘
       │                          │
┌──────▼──────┐          ┌────────▼────────────────────┐
│   Runtime   │          │      Storage                 │
│ internal/   │          │ internal/image  (OverlayFS)  │
│   runtime   │          │ internal/lazypull (FUSE)     │
│ internal/   │          │ /var/lib/thrive/             │
│   cgroup    │          │   images/{ref}/layers/       │
└──────┬──────┘          │   chunks/{xx}/{rest}         │
       │                 └────────────┬─────────────────┘
       │                              │
┌──────▼──────────────────────────────▼──────────────────┐
│                     Network / P2P                        │
│  internal/p2p  (Kademlia DHT, chunk distribution)       │
│  [Phase 10] CNI, veth, NAT — PENDING                    │
└──────────────────────────┬──────────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────────┐
│                   Observability                          │
│  internal/telemetry  (Prometheus metrics)                │
│  internal/otel       (OTLP gRPC traces)                  │
└─────────────────────────────────────────────────────────┘
```

---

## Key Subsystems

### runtime.Start

Located at `internal/runtime/runtime.go`.

Responsibilities:
1. Resolves the container rootfs path from OverlayFS mergeddir
2. Sets `cmd.SysProcAttr` with namespace clone flags and chroot path
3. Wires stdout/stderr to `/run/thrive/containers/{id}/logs`
4. Applies cgroup limits via `internal/cgroup` before exec
5. Saves `state.json` with PID and status

### image.Mount

Located at `internal/image/image.go`.

Responsibilities:
1. Assembles lowerdir string from reversed image layers (bottom-most layer last)
2. Creates per-container upperdir, workdir, and mergeddir under `/var/lib/thrive/containers/{id}/`
3. Attempts `syscall.Mount("overlay", mergeddir, "overlay", 0, opts)`
4. On EPERM or ENODEV, falls back to `exec.Command("fuse-overlayfs", ...)`

### lazypull (FUSE)

Located at `internal/lazypull/`.

On first file access within the FUSE mount, `fetchChunk` performs an HTTP GET to the OCI registry
blob endpoint for the corresponding digest. The response body is written to the local chunk store
and served from cache on all subsequent reads.

### p2p (Kademlia DHT)

Located at `internal/p2p/`.

Each THRIVE node maintains a Kademlia routing table. When a chunk is not available locally or from
the OCI registry, `RequestChunk` broadcasts a FIND_VALUE to k-nearest peers. The first peer to
respond with the chunk wins. A 30-second timeout prevents indefinite blocking.

### secrets (AES-256-GCM)

Located at `internal/secrets/`.

The master key is auto-generated on first run using `crypto/rand` and persisted at
`/var/lib/thrive/secrets/.master` with mode 0600. Secrets are encrypted with AES-256-GCM
(random 12-byte nonce prepended to ciphertext). At container start, secrets are mounted via a
tmpfs into the container's `/run/secrets/` directory — never passed as environment variables.

---

## Data Flow: `thrive run alpine:3.19 -- /bin/sh`

```
1. CLI parses flags → constructs RunConfig{Image: "alpine:3.19", Cmd: ["/bin/sh"]}

2. image.Pull("alpine:3.19")
   └─ go-containerregistry fetches manifest from registry
   └─ Downloads each layer tarball
   └─ Extracts tar into /var/lib/thrive/images/alpine:3.19/layers/{digest}/
   └─ Splits blobs into SHA-256 chunks → /var/lib/thrive/chunks/{xx}/{rest}

3. image.Mount(imageRef, containerID)
   └─ lowerdir = reversed layer paths joined by ":"
   └─ upperdir = /var/lib/thrive/containers/{id}/upper
   └─ workdir  = /var/lib/thrive/containers/{id}/work
   └─ mergeddir = /var/lib/thrive/containers/{id}/merged
   └─ syscall.Mount("overlay", mergeddir, "overlay", 0, opts)

4. runtime.Start(containerID, mergeddir, ["/bin/sh"])
   └─ cmd.SysProcAttr = {
         Cloneflags: CLONE_NEWNS|CLONE_NEWPID|CLONE_NEWUTS|CLONE_NEWIPC|CLONE_NEWNET|CLONE_NEWUSER,
         Chroot: mergeddir,
         UidMappings: [{0, hostUID, 1}],
         GidMappings: [{0, hostGID, 1}],
      }
   └─ cgroup.Apply(containerID, limits)   // PID, memory, CPU
   └─ cmd.Start()
   └─ writes state.json {id, pid, status:"running", ...}

5. On exit:
   └─ status updated to "stopped" in state.json
   └─ image.Unmount(containerID) → syscall.Unmount(mergeddir, MNT_DETACH)
   └─ If --rm: remove /run/thrive/containers/{id}/ and /var/lib/thrive/containers/{id}/
```

---

## Key Linux Syscalls

| Syscall | Purpose |
|---------|---------|
| `clone(2)` | Create child process with new namespaces (PID, mount, UTS, IPC, net, user) |
| `mount(2)` | Assemble OverlayFS mergeddir from layer stack |
| `umount2(2)` | Detach OverlayFS on container removal (MNT_DETACH for lazy unmount) |
| `chroot(2)` | Confine container process to OverlayFS mergeddir as its `/` |
| `unshare(2)` | Used in rootless path to create user namespace before clone |
| `prctl(2)` | Set child subreaper so orphaned processes are reparented correctly |
| `write(2)` to cgroupfs | Apply PID/memory/CPU limits via cgroup v2 file interface |

---

## State Storage Layout

```
/run/thrive/
  containers/
    {containerID}/
      state.json          # {id, pid, status, image, cmd, createdAt}
      logs                # stdout+stderr of container process

/var/lib/thrive/
  images/
    {ref}/                # e.g. alpine:3.19/
      layers/
        {digest}/         # extracted tar contents of each OCI layer
  chunks/
    {xx}/
      {rest}              # SHA-256 content-addressed chunk blobs
  containers/
    {containerID}/
      upper/              # OverlayFS upperdir (container writes)
      work/               # OverlayFS workdir (kernel internal)
      merged/             # OverlayFS mergeddir (container rootfs)
  secrets/
    .master               # AES-256 master key (chmod 0600)
    {name}.enc            # Encrypted secret ciphertext
```

---

## Why Daemonless

Docker's architecture requires `dockerd` (root daemon) as the single point of contact for all
container operations. If `dockerd` crashes, all containers are unreachable. THRIVE eliminates this:

- `thrive run` calls `clone(2)` directly in the calling process
- The parent process monitors the child and updates state.json
- No IPC socket is needed; state is read from the filesystem
- Multiple `thrive` invocations are fully independent processes

---

## OverlayFS Design

```
Image layers (read-only):
  layer-0/ <- bottom (base OS)
  layer-1/ <- package additions
  layer-2/ <- app files

OverlayFS mount:
  lowerdir  = layer-2:layer-1:layer-0   (reversed, top-first)
  upperdir  = containers/{id}/upper/    (read-write, container writes go here)
  workdir   = containers/{id}/work/     (kernel internal scratch space)
  mergeddir = containers/{id}/merged/   (unified view = container's rootfs)

Reads:  satisfied from upperdir if present, else walks lowerdir stack
Writes: copy-on-write into upperdir; original layers untouched
Delete: whiteout file created in upperdir masking the lower file
```

---

## Secrets Architecture

```
At init:
  crypto/rand -> 32 bytes -> /var/lib/thrive/secrets/.master (0600)

Encrypt(name, plaintext):
  key       = read(.master)
  nonce     = crypto/rand 12 bytes
  ciphertext = AES-256-GCM.Seal(nonce + ciphertext)
  write to  /var/lib/thrive/secrets/{name}.enc

At container start (--secret name):
  mount tmpfs -> /proc/{pid}/root/run/secrets/
  Decrypt({name}.enc) -> plaintext
  write plaintext -> /proc/{pid}/root/run/secrets/{name}
  chmod 0400

Secret is NEVER passed via env var or process args (visible in /proc/{pid}/environ)
```

---

## P2P Architecture

```
Node startup:
  Generate 160-bit node ID (SHA-1 of public key or random)
  Bootstrap -> contact known bootstrap nodes -> populate k-buckets

Chunk lookup (RequestChunk):
  1. Check local chunk store -> return if found
  2. FIND_VALUE(chunkDigest) -> k-nearest peers
  3. Peers respond with chunk data or closer peers
  4. Parallel fetch from all responding peers (BitTorrent-style)
  5. First complete chunk wins; verify SHA-256 digest
  6. Store in local chunk store; serve to requestor
  7. Timeout: 30 seconds

Chunk announcement (after Pull):
  STORE(chunkDigest, localAddr) -> k-nearest peers
  Peers update their routing table to know this node has the chunk
```
