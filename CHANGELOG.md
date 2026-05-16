# Changelog

All notable changes to THRIVE are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.3.0] — 2026-05-17

### Added

- `internal/vm/darwin_launcher.go`: spawns vfkit (Apple Virtualization.framework
  wrapper) on macOS with `--memory`, `--cpus`, `--bootloader linux,...`,
  virtio-blk + virtio-rng + virtio-vsock device flags
- `internal/vm/wsl2_launcher.go`: registers and launches the `Thrive` WSL2
  distro via `wsl --import` + `wsl -d`; idempotent if distro already exists
- `internal/vm/hyperv_launcher.go`: creates a Hyper-V Generation 2 VM via
  PowerShell `New-VM` + `Set-VMProcessor` + `Start-VM`; idempotent
- `internal/vm/exec.go`: `commandRunner` / `processStarter` / `pathLookup`
  function type indirection so launcher tests inject mocks
- `internal/vm/download.go`: `THRIVE_VM_IMAGE_PATH` env var bypasses HTTP
  download (extracts a local `.tar.gz` instead) — unblocks dev / air-gapped
  installs / the current missing-GitHub-release situation
- Makefile: `build-all`, `build-linux`, `build-darwin`, `build-windows`,
  `test-desktop` targets for cross-platform release artifacts
- Tests: `darwin_launcher_test.go`, `wsl2_launcher_test.go`,
  `hyperv_launcher_test.go`, `download_test.go` (14 new tests, all subprocess
  contracts covered via injected mocks)

### Changed

- `internal/vm/lifecycle.go`: replaced switch-on-vmType with a `launcher`
  interface + `selectLauncher`; `Start`/`Stop` now persist state via
  `WriteVMState` automatically
- `thrive desktop start` no longer returns `"not implemented"` on any
  platform — emits an actionable error if the required hypervisor tool
  (vfkit / wsl.exe / powershell.exe) is missing

### Removed

- `internal/vm/lifecycle_stubs.go` (dead code; replaced by real launchers)

### Fixed

- macOS `thrive desktop init` no longer hard-fails on a 404 from GitHub
  releases; users can supply a local tarball via `THRIVE_VM_IMAGE_PATH`

---

## [0.2.0] — 2026-05-16

### Added

- `image.Pull`: real tar extraction of OCI layers into per-digest lowerdir directories
- `image.Mount`: real OverlayFS mount with fuse-overlayfs rootless fallback on EPERM/ENODEV
- `image.Unmount`: `syscall.Unmount(mergeddir, MNT_DETACH)` implementation replacing stub
- `image.Push`: re-tar layer dirs + `remote.Write` via go-containerregistry
- `runtime.Start`: chroot into OverlayFS mergeddir for container rootfs isolation
- `runtime.Start`: cgroup v2 wiring — Apply PID limit, SetMemoryLimit, SetCPUQuota
- `runtime.Start`: log file redirect (stdout/stderr to `/run/thrive/containers/{id}/logs`)
- `thrive logs`: real log file streaming with `--follow`/`-f` flag support
- `thrive run`: `--detach`/`-d`, `--rm`, `--name`, `--env`/`-e`, `--secret` flags
- `thrive pull`/`push`: real registry implementation replacing no-op stubs
- `build.Execute`: real container-per-step execution with poll-until-stopped loop
- `secrets/vault`: auto-generate AES-256 master key on first run, persist to disk
- `otel.Init`: OTLP gRPC trace exporter wired when `OTEL_EXPORTER_OTLP_ENDPOINT` is set
- `lazypull.fetchChunk`: real HTTP GET to OCI registry blob endpoint
- `p2p.RequestChunk`: blocking 30-second timeout replacing silent non-blocking default
- Test files: `runtime_test.go`, `vault_test.go`, `telemetry_test.go`, `cgroup_test.go`,
  `build_test.go`, `peer_test.go` — all compile clean and pass

### Fixed

- `p2p.RequestChunk` previously returned immediately without a result when no peer responded;
  now blocks up to 30 seconds before returning an error
- `image.Mount` now correctly reverses layer order for lowerdir (top layer first)
- `secrets/vault` no longer panics when master key file does not yet exist

### Changed

- Build target restricted to `GOOS=linux`; CI matrix updated accordingly
- `internal/otel` package renamed from `internal/observability` for clarity

---

## [0.1.0] — 2026-05-16

### Added

- Initial project skeleton: all packages present, CI wired, MIT license
- `runtime.Create`/`Kill`/`Delete`/`State` with `state.json` persistence
- `image.Pull` with registry download (tar stored to chunk path — stub extraction)
- Thrivefile YAML parser + DAG topological sort with cycle detection
- Secrets store with AES-256-GCM encrypt/decrypt (manual key required)
- FUSE lazy-pull skeleton (go-fuse mount, no-op read handlers)
- Kademlia DHT skeleton (routing table, peer management, no chunk transfer)
- OTEL Prometheus metrics endpoint (`/metrics`)
- CLI subcommands: `run`, `ps`, `kill`, `logs`, `rm`, `images`, `build`, `pull`, `push`,
  `secret`, `metrics`, `system`
- `go.mod` with module `github.com/thakurprasadrout/thrive`, Go 1.22+
- `.github/workflows/ci.yml`: lint + build + test on push/PR
- `.golangci.yml`: golangci-lint configuration
- `Makefile`: build, test, lint, clean targets
- `LICENSE`: MIT
