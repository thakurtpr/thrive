# THRIVE Desktop — Cross-Platform VM Architecture

**Date:** 2026-05-17
**Author:** Staff/Principal Systems Architect
**Status:** Approved for implementation

---

## 1. Problem Statement

Thrive is a daemonless, rootless, OCI-compliant container runtime that runs natively on Linux via direct `clone(2)` syscalls. On macOS and Windows, Thrive cannot run containers natively because neither platform has a Linux kernel. To provide a Docker Desktop-like experience on these platforms, Thrive Desktop embeds a lightweight Linux VM and exposes the same CLI that Linux users get — but proxies commands to the VM's Thrive daemon instead of calling the kernel directly.

---

## 2. Platform Strategy

| Platform | Container execution | VM layer |
|----------|--------------------|----------|
| **Linux** | Direct native (`clone(2)`, namespaces, cgroups v2) | No VM |
| **macOS** | Hypervisor.framework Linux VM + VSOCK bridge | `thrive-vm` via `github.com/mist64/hv` |
| **Windows Pro/Enterprise** | Hyper-V Linux VM + named pipe | `thrive-vm` via Hyper-V |
| **Windows Home / WSL fallback** | WSL2 Linux VM + named pipe | `thrive-wsl` |

---

## 3. Non-Functional Requirements

- **Latency:** `thrive run` < 2s after VM is warm
- **Memory:** VM default 2GB, configurable 512MB–8GB
- **Startup:** Desktop app ready in < 3s after user login
- **Storage:** VM image < 150MB compressed; images in `~/.thrive/images/`
- **Security:** VM isolated by Hypervisor.framework (macOS) / Hyper-V (Windows Pro) / WSL2 (Windows Home)
- **Scale:** 1 user, 1–10 containers concurrent

---

## 4. Core Design Principle

```
CLI = brain
Tray = observer
VM = execution environment
```

**Everything must work headlessly.** The tray and any GUI are read-only observers of state that the CLI already manages. If the tray process dies, CLI still works perfectly. If the GUI crashes, containers keep running.

---

## 5. Binary Boundaries

```
cmd/
├── thrive/                     # Host CLI binary (all platforms: darwin/win/linux)
│   ├── main.go                 # Entry point — NO platform gate here
│   └── commands/
│       ├── run.go              # Linux: runtime.Create/Start; //go:build linux
│       ├── run_proxy.go        # Non-Linux: Bridge.Exec; //go:build !linux
│       ├── ps.go               # Linux: direct fs read; //go:build linux
│       ├── ps_proxy.go         # Non-Linux: Bridge.Exec("ps"); //go:build !linux
│       ├── logs.go             # Linux: streaming file read; //go:build linux
│       ├── logs_proxy.go       # Non-Linux: Bridge.ExecStream; //go:build !linux
│       ├── kill.go             # Linux: syscall.Kill; //go:build linux
│       ├── kill_proxy.go       # Non-Linux: Bridge.Exec("kill"); //go:build !linux
│       ├── rm.go               # Linux: runtime.Delete; //go:build linux
│       ├── rm_proxy.go         # Non-Linux: Bridge.Exec("rm"); //go:build !linux
│       ├── desktop.go          # init/start/stop/restart/status (all platforms)
│       ├── images.go           # Works natively (shared ~/.thrive/images/) — no proxy
│       ├── buildpushpull.go    # Works natively (OCI registry ops) — no proxy
│       ├── secret.go           # Works natively (shared ~/.thrive/secrets/) — no proxy
│       ├── system.go              # Linux: reads /proc/, cgroupfs; //go:build linux
│       └── system_proxy.go        # Non-Linux: Bridge.Exec("system"); //go:build !linux
│
└── thrived/                    # VM daemon binary (linux/amd64 only, embedded in rootfs.img)
    ├── main.go                 # Entry: runit supervision + socket server start
    ├── socket.go               # newline-delimited JSON server (jsonrpc2-style)
    └── exec.go                 # Dispatches JSON cmd → runtime.Create/Start/Stop/Delete

internal/
├── runtime/
│   ├── platform/
│   │   ├── linux.go            # IsSupported()=true, capability detection
│   │   └── darwin.go           # IsSupported()=false, detection only
│   └── runtime.go              # Linux only runtime operations
│
└── vm/                         # //go:build !linux — transport + lifecycle
    ├── bridge.go               # Bridge interface + Dial() factory
    ├── vsock_darwin.go         # VSOCK (macOS)
    ├── hyperv_windows.go       # Named pipe (Windows Hyper-V)
    ├── wsl2_windows.go         # Named pipe (Windows WSL2)
    ├── lifecycle.go            # Start/Stop/WaitForBoot/HealthCheck
    ├── config.go               # config.json + vm.json read/write
    └── download.go             # Fetch VM image from GitHub Release

desktop/                        # //go:build darwin || windows
    └── tray.go                     # getlantern/systray — platform diffs handled by library
```

---

## 6. Data Layout

```
~/.thrive/                           (macOS / Linux)
%LOCALAPPDATA%\Thrive\              (Windows)

├── vm/
│   ├── kernel                      (downloaded on init, versioned)
│   ├── initrd                      (downloaded on init)
│   ├── rootfs.img                  (pre-built squashfs, downloaded from releases)
│   ├── vm.json                     (VM runtime state — renamed from state.json)
│   └── downloaded-version.json     (tracks which release was fetched)
├── images/                         (OCI store — shared with Linux path)
├── chunks/                         (P2P chunk store)
└── config.json                     (memory_mb, cpu_count, vm_type)
```

### vm.json schema
```json
{
  "version": "1.0",
  "running": false,
  "pid": 0,
  "cid": 0,
  "wsl_instance": "",
  "last_start": ""
}
```

### config.json schema
```json
{
  "memory_mb": 2048,
  "cpu_count": 2,
  "vm_type": "darwin-hv"
}
```

**vm_type values (canonical, used in Dial()):**
| Platform | vm_type |
|----------|---------|
| macOS | `darwin-hv` |
| Windows + Hyper-V | `hyperv` |
| Windows + WSL fallback | `wsl2` |

---

## 7. Bridge Interface

```go
// internal/vm/bridge.go

//go:build !linux

package vm

import (
    "context"
    "io"
)

// Bridge is the transport abstraction for CLI→VM daemon communication.
// Implemented per-platform: VSOCK (macOS), named pipe (Windows Hyper-V), WSL2 interop (Windows).
type Bridge interface {
    // Exec runs a Thrive CLI command inside the VM and returns structured output.
    // opts carries additional parameters (e.g., {"follow": true} for logs).
    Exec(ctx context.Context, cmd string, args []string, opts map[string]any) ([]byte, error)

    // ExecStream runs a streaming command (logs --follow) writing output to out as lines arrive.
    // Returns when {"eof": true} is received or ctx is cancelled.
    ExecStream(ctx context.Context, cmd string, args []string, opts map[string]any, out io.Writer) error

    Close() error
}

// Dial returns a platform-appropriate Bridge.
// vm_type values: "darwin-hv", "hyperv", "wsl2"
func Dial(ctx context.Context, vmType string) (Bridge, error) {
    switch vmType {
    case "darwin-hv":
        return newVSOCKBridge()
    case "hyperv":
        return newHyperVBridge()
    case "wsl2":
        return newWSL2Bridge()
    default:
        return nil, fmt.Errorf("unknown vm type: %s", vmType)
    }
}
```

---

## 8. Daemon Communication Protocol

**Format: newline-delimited JSON (no new dependencies, stdlib only)**

### Request schema (host → VM daemon)
```go
type Request struct {
    ID   int          `json:"id"`
    Cmd  string       `json:"cmd"`
    Args []string     `json:"args"`
    Opts map[string]any `json:"opts,omitempty"`
}
```

### Examples
```json
{"cmd": "run", "args": ["-it", "alpine:3.19", "/bin/sh"], "opts": {"detach": false}, "id": 1}
{"cmd": "ps", "args": [], "opts": {}, "id": 2}
{"cmd": "logs", "args": ["abc123"], "opts": {"follow": true}, "id": 3}
{"cmd": "kill", "args": ["abc123"], "opts": {}, "id": 4}
{"cmd": "rm", "args": ["abc123"], "opts": {}, "id": 5}
```

### Response schema (VM daemon → host)
```go
type Response struct {
    ID     int         `json:"id"`
    Result interface{} `json:"result,omitempty"`
    Stream string      `json:"stream,omitempty"`
    EOF    bool        `json:"eof,omitempty"`
    Error  *ErrorInfo  `json:"error,omitempty"`
}

type ErrorInfo struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
}
```

### Examples
```json
{"id": 1, "result": {"container_id": "xyz789"}}
{"id": 2, "result": {"containers": [{"id": "abc123", "image": "alpine:3.19", "status": "running", "pid": 12345}]}}
{"id": 3, "stream": "stdout line from container"}
{"id": 3, "stream": "stderr line from container"}
{"id": 3, "eof": true}
{"id": 4, "result": {}}
{"id": 5, "result": {}}
{"id": 1, "error": {"code": 1, "message": "image not found: alpine:3.19"}}
```

---

## 9. Platform Gate Refactor

**Problem:** Current `main.go:16-29` calls `os.Exit(1)` on non-Linux before cobra parses subcommands. This kills all non-runtime commands (`desktop`, `images`, `build`, etc.) before they can execute.

**Fix:** Remove the `os.Exit(1)` block from `main.go` entirely. Move platform enforcement to individual command files via `//go:build` tags.

```go
// cmd/thrive/main.go — REMOVE the os.Exit block
// Platform check happens at command level, not at main() level
func main() {
    rootCmd := NewRootCmd()
    if err := rootCmd.Execute(); err != nil {
        fmt.Fprintf(os.Stderr, "thrive: %v\n", err)
        os.Exit(1)
    }
}
```

### Build-tagged command pairs

Each runtime-requiring command gets a Linux native version and a non-Linux proxy version:

| Command | Linux (//go:build linux) | Non-Linux (//go:build !linux) |
|---------|-------------------------|-------------------------------|
| run | `runtime.Create()` + `runtime.Start()` | `Bridge.Exec("run", args, opts)` |
| ps | Read `/run/thrive/containers/` | `Bridge.Exec("ps", [], {})` |
| logs | `io.Copy(os.Stdout, logFile)` | `Bridge.ExecStream("logs", args, opts, os.Stdout)` |
| kill | `syscall.Kill(pid, sig)` | `Bridge.Exec("kill", args, opts)` |
| rm | `runtime.Delete()` | `Bridge.Exec("rm", args, opts)` |
| system | Read `/proc/` + cgroupfs directly | `Bridge.Exec("system", args, opts)` |

### Commands with NO proxy (work natively on all platforms)
- `images` — reads `~/.thrive/images/` (shared store)
- `buildpushpull` — OCI registry ops via go-containerregistry
- `secret` — reads `~/.thrive/secrets/` (shared store)
- `metrics` — reads Prometheus endpoint (daemon provides it)
- `desktop` — VM lifecycle commands (own package)

---

## 10. VM Image Sourcing

### Decision: Pre-built image downloaded from GitHub Releases

**Why not build locally on `thrive desktop init`:**
- Buildroot toolchain adds complexity for users
- VM image is a release artifact — same binary on all hosts
- Network fetch is faster than local compile
- Version-pinned via `downloaded-version.json`

### VM image contents (per platform binary)
- Alpine Linux base (5MB kernel + 50MB rootfs)
- Static `thrived` binary (`linux/amd64`, built from `cmd/thrived/`)
- `runit` as init (PID 1, supervises `thrived`)
- Minimal tooling: `sh`, `ip`, `mount` — enough for container runtime

### Release artifact naming
```
thrive-vm-darwin-amd64.tar.gz   # macOS x86_64
thrive-vm-darwin-arm64.tar.gz   # macOS Apple Silicon
thrive-vm-windows-amd64.tar.gz # Windows x86_64 (Hyper-V)
thrive-vm-windows-amd64.tar.gz # Windows (WSL2 uses same image)
```

### `thrive desktop init` flow
```
thrive desktop init
↓
Check ~/.thrive/vm/downloaded-version.json
↓
If missing or stale:
  → GET https://github.com/thakurprasadrout/thrive/releases/latest/vm-{os}-{arch}.tar.gz
  → SHA-256 verify against published checksum
  → Extract to ~/.thrive/vm/
  → Write downloaded-version.json with release tag
↓
Write ~/.thrive/config.json (memory_mb=2048, cpu_count=2, vm_type=<detected value>)
↓
Note: `"auto"` is the name of the detection logic, NOT a persisted vm_type.
The detection result ("darwin-hv", "hyperv", or "wsl2") is what gets written to config.json.
`Dial()` never sees "auto".
Write ~/.thrive/vm/vm.json (version: "1.0", running: false)
↓
Done. Show: "Thrive VM ready. Run `thrive desktop start` to begin."
```

### Auto-detection table for vm_type
| Platform | vm_type to write |
|----------|-----------------|
| macOS (any) | `darwin-hv` |
| Windows + Hyper-V available | `hyperv` |
| Windows Home / Hyper-V unavailable | `wsl2` |
| Linux | (no VM, not applicable) |

`--vm-type` flag overrides auto-detection.

---

## 11. Tray Architecture (Optional)

**Technology:** Pure Go via `getlantern/systray` — zero Rust, single Go toolchain.

**Design:** Tray is a pure observer. It reads `vm.json` and shells out to `thrive desktop status` for container details. It never controls runtime.

### Menu items
```
● Thrive Running

Containers: 4
CPU: 12%
Memory: 2.1GB

──────────────

Restart VM
Stop VM
──────────────

Preferences    → opens config.json editor
Quit Thrive
```

**Memory target:** <10MB idle.

**Key behavior:**
- Tray icon color: green (running), red (stopped), orange (starting)
- On click: opens terminal with `thrive ps` output
- No heavy dashboard — keep it minimal

---

## 12. Failure Modes

| Component | Mode | Detection | Recovery |
|-----------|------|-----------|----------|
| VM kernel panic | macOS/Windows | Health check via VSOCK/pipe ping every 5s | Auto-restart via Hypervisor.framework / Hyper-V |
| Thrive daemon crash | Inside VM | CLI connect fails | Supervised restart by runit inside VM |
| Disk full | Host | Check before pull | Error: "disk full, cannot pull image" |
| Network failure | `thrive desktop init` | Fetch fails | Retry 3x with backoff, then error with manual URL |
| VM image corrupt | On start | SHA-256 mismatch | Re-download from release |
| CLI can't reach daemon | VM not running | connect() returns error | Clear error: "Run `thrive desktop start` first" |

---

## 13. Implementation Phases

| Phase | What | Files | Gate |
|-------|------|-------|------|
| 0 | Linux native (unchanged) | `runtime.go`, `run.go`, etc. | Already works |
| 1 | `main.go` platform gate refactor | `cmd/thrive/main.go` | Remove `os.Exit(1)` block |
| 2 | `cmd/thrived/` daemon binary | `cmd/thrived/{main,socket,exec}.go` | `GOOS=linux GOARCH=amd64` build |
| 3 | `internal/vm/` bridge + lifecycle | `internal/vm/{bridge,lifecycle,config,download}.go` | `//go:build !linux` |
| 4 | macOS: VSOCK bridge | `internal/vm/vsock_darwin.go` | `//go:build darwin` |
| 5 | `thrive desktop init/start/stop/status` | `cmd/thrive/commands/desktop.go` | All platforms |
| 6 | Command proxy pairs | `run_proxy.go`, `ps_proxy.go`, `logs_proxy.go`, `kill_proxy.go`, `rm_proxy.go` | `//go:build !linux` |
| 7 | macOS tray | `desktop/tray.go` | `//go:build darwin || windows` |
| 8 | Windows: Hyper-V named pipe | `internal/vm/hyperv_windows.go` | `//go:build windows` |
| 9 | Windows: WSL2 interop socket | `internal/vm/wsl2_windows.go` | `//go:build windows` |
| 10 | Windows tray + verify | `desktop/tray.go` (same file, verify on Windows) | `//go:build darwin || windows` |

---

## 14. Open Questions

- **Apple Silicon verification:** VSOCK via Rosetta needs testing on actual Apple Silicon hardware
- **Windows WSL2 named pipe:** WSL2 interop behavior on Windows 11 needs verification
- **VM image update mechanism:** `thrive desktop upgrade` command to re-download newer VM images

---

## 15. Rollback Procedures

| Phase | If it fails | Rollback |
|-------|-------------|----------|
| Phase 1 (main.go gate) | CLI crashes on Linux | Revert main.go change — platform gate back at startup |
| Phase 2 (thrived binary) | VM won't boot | Use previous release's VM image, rebuild binary |
| Phase 3 (vm package) | Build errors | Disable VM commands, CLI falls back to error message |
| Phase 5 (desktop commands) | Commands error | User falls back to direct `thrive` on Linux |

---

## 16. Key Files Summary

| File | Purpose |
|------|---------|
| `cmd/thrive/main.go` | Entry point — no platform gate |
| `cmd/thrive/commands/run.go` | Linux: direct runtime |
| `cmd/thrive/commands/run_proxy.go` | Non-Linux: bridge proxy |
| `cmd/thrive/commands/desktop.go` | init/start/stop/restart/status |
| `cmd/thrived/main.go` | VM daemon entrypoint |
| `cmd/thrived/socket.go` | JSON-RPC over Unix socket |
| `cmd/thrived/exec.go` | Dispatch to runtime |
| `internal/vm/bridge.go` | Bridge interface + Dial |
| `internal/vm/vsock_darwin.go` | macOS VSOCK |
| `internal/vm/hyperv_windows.go` | Windows Hyper-V pipe |
| `internal/vm/wsl2_windows.go` | Windows WSL2 interop |
| `internal/vm/lifecycle.go` | Start/Stop/HealthCheck |
| `internal/vm/config.go` | config.json + vm.json |
| `internal/vm/download.go` | GitHub Release fetch |
| `desktop/tray.go` | getlantern/systray menu (all non-Linux platforms) |