# Runtime Subsystem — Deep Dive

Package: `internal/runtime`
Owner: RUNTIME agent
See also: `internal/cgroup`, ARCHITECTURE.md, DECISIONS.md ADR-001, ADR-002

---

## Container Lifecycle

```
                  thrive run
                      |
                      v
              runtime.Create()
                      |
              state.json written
              status: "created"
                      |
                      v
              runtime.Start()
                      |
              cgroup.Apply()
                      |
              cmd.Start() -> clone(2)
                      |
              state.json updated
              status: "running"
                      |
            (process runs)
                      |
              child exits / thrive kill
                      |
              state.json updated
              status: "stopped"
                      |
                      v
              runtime.Delete()  (explicit or --rm)
                      |
              remove /run/thrive/containers/{id}/
              remove /var/lib/thrive/containers/{id}/
```

---

## Namespace Flags

`runtime.Start` sets the following `SysProcAttr.Cloneflags`:

| Flag | Syscall Constant | Purpose |
|------|-----------------|---------|
| Mount namespace | `syscall.CLONE_NEWNS` | Container gets its own mount table; OverlayFS mount is invisible to host |
| PID namespace | `syscall.CLONE_NEWPID` | Container process sees itself as PID 1; host PIDs are invisible |
| UTS namespace | `syscall.CLONE_NEWUTS` | Container can have its own hostname without affecting host |
| IPC namespace | `syscall.CLONE_NEWIPC` | Isolates System V IPC and POSIX message queues |
| Network namespace | `syscall.CLONE_NEWNET` | Container gets its own network stack (lo only until Phase 10 CNI) |
| User namespace | `syscall.CLONE_NEWUSER` | Maps container root (UID 0) to an unprivileged host UID |

The user namespace mapping (`UidMappings`, `GidMappings`) allows the container to appear to
run as root inside its namespace while being an unprivileged user on the host.

---

## Cgroup v2 Hierarchy

When a container starts, `cgroup.Apply(containerID, limits)` creates and populates:

```
/sys/fs/cgroup/thrive/
  {containerID}/
    cgroup.procs       <- container PID written here to join the cgroup
    pids.max           <- PID limit (default: 256)
    memory.max         <- memory limit in bytes (default: 512MB)
    cpu.max            <- CPU quota in format "quota period" (default: "100000 100000" = 100%)
```

Writing the container PID to `cgroup.procs` atomically moves the process into the cgroup.
All child processes it spawns inherit the cgroup. Limits are enforced by the kernel.

On container removal, the cgroup directory is removed via `os.Remove`.

Prerequisite: the calling user must have write access to `/sys/fs/cgroup/thrive/`.
On systemd systems this requires a delegated slice. See RISKS.md RISK-002.

---

## Log File Location and Format

- **Path:** `/run/thrive/containers/{containerID}/logs`
- **Format:** Raw stdout+stderr interleaved, no timestamps, no framing
- **Access:** `thrive logs {id}` reads and streams this file
- **Follow mode:** `thrive logs --follow {id}` polls the file in a loop, sleeping briefly
  between reads when at EOF to simulate tail-f behavior

Log files are not rotated. See RISKS.md RISK-010 for the resolution path.

---

## State JSON Schema

`/run/thrive/containers/{id}/state.json`:

```json
{
  "id":        "string  -- container ID (UUID or user-provided name)",
  "pid":       "int     -- host PID of the container process (0 if not running)",
  "status":    "string  -- one of: created | running | stopped",
  "image":     "string  -- image reference used to start this container",
  "cmd":       ["string", "..."],
  "env":       ["KEY=VALUE", "..."],
  "createdAt": "string  -- RFC3339 timestamp of container creation",
  "name":      "string  -- optional human-readable name (from --name flag)"
}
```

State is written atomically: content is written to a temp file then renamed into place to
prevent partial reads by concurrent `thrive ps` invocations.

---

## Rootless Isolation: How Chroot + User Namespace Work Together

```
Host (uid=1000)
  |
  +-- thrive run (uid=1000, no capabilities)
        |
        +-- clone(CLONE_NEWUSER | CLONE_NEWNS | ...)
              |
              +-- child process
                    user namespace: uid 0 (root) maps to host uid 1000
                    mount namespace: sees OverlayFS mergeddir as /
                    SysProcAttr.Chroot: kernel calls chroot(mergeddir) before exec
                    |
                    exec /bin/sh   <- runs as "root" inside container
                                      but is uid 1000 on the host
```

Key points:
- `CLONE_NEWUSER` grants the child a new user namespace where it has a full capability set
- The UID mapping (`0 -> 1000`) means files owned by uid 0 inside the container are
  stored as uid 1000 on disk — no actual root ownership on the host filesystem
- `SysProcAttr.Chroot` confines the process to the OverlayFS mergeddir before exec,
  so the container cannot see the host filesystem at paths outside the chroot
- The combination achieves rootless isolation without any setuid binaries or elevated privileges

Limitation: this uses `chroot(2)` not `pivot_root(2)`. See DECISIONS.md ADR-002 and
RISKS.md RISK-004 for the known limitation and v1.0 resolution path.
