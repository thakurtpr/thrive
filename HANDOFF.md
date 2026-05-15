# THRIVE — HANDOFF

## Last updated
2026-05-16T02:20:00Z

## What was just completed
- Phase 1 initial implementation: internal/runtime/, internal/namespace/, internal/cgroup/, internal/supervisor/ packages
- All packages build with linux build tags
- Config types: ContainerConfig, ContainerState, Mount, ResourceLimits, RestartPolicy
- runtime.Create() creates container state file at /run/thrive/containers/{id}/state.json
- namespace.CreateUserNamespace() handles user namespace creation + UID/GID mapping
- namespace.CloneFlags() returns proper clone flags for container
- supervisor.Watch() handles process exit with restart policy
- cgroup.Manager handles cgroup v2 setup for containers

## What is in progress (incomplete)
- Phase 1 runtime.Start() not implemented (stub returns "not implemented")
- Phase 2 image/storages not started

## What to do next (exact next step)
Implement runtime.Start() — use clone(2) with namespace flags, pivot_root to switch rootfs. This is the core container creation. Then wire CLI run command to call runtime.Create + runtime.Start.

## Files modified in last session
- internal/runtime/config.go — ContainerConfig, ContainerState, Mount, ResourceLimits, RestartPolicy types
- internal/runtime/runtime.go — Create, Start, Kill, Delete, State functions (Start is stub)
- internal/namespace/namespace.go — CreateUserNamespace, CloneFlags (linux-only)
- internal/supervisor/supervisor.go — Watch, ExecInContainer (linux-only)
- internal/cgroup/cgroup.go — Manager with Apply, SetMemoryLimit, SetCPUQuota (linux-only)

## Tests passing / failing
go build ./... ✓ (linux build tags on darwin = package not compiled, but no errors)
go test ./... — no test files yet

## Known broken things
- runtime.Start() returns "not implemented" — needs clone(2) + pivot_root(2) implementation
- supervisor.ExecInContainer is stub — needs real clone implementation
- cgroup operations are stubs on darwin (expected with linux build tag)

## Decisions made this session
- Added //go:build linux build tags to all internal/ packages to prevent darwin build errors
- All runtime functions take context.Context as first parameter
- State persisted to /run/thrive/containers/{id}/state.json as JSON

## Open questions
- Should we add tests that skip on darwin or only run on linux?
