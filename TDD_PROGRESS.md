# TDD Progress — Test Coverage Tracker

## Coverage Table

| Package | Test File | Coverage | Status | Notes |
|---------|-----------|----------|--------|-------|
| internal/runtime | runtime_test.go, lifecycle_test.go, runtime_extra_test.go | ~55% | improved | state roundtrip + 13 lifecycle error-path tests; stats streaming is client-side ticker (unit: flag registration); live exec needs root |
| internal/image | image_test.go | ~60% | improved | 31 tests: extractTar, chunk store, List, Mount/Unmount error paths, SafeRef |
| internal/secrets | vault_test.go, vault_extra_test.go | ~70% | improved | roundtrip + wrong-key/corrupt/truncated edge cases + concurrency |
| internal/telemetry | telemetry_test.go | ~50% | stable | concurrent safety verified via race detector |
| internal/cgroup | cgroup_test.go, cgroup_extra_test.go | ~65% | improved | hermetic Manager tests (freeze/shares/pids/stats) via temp dirs |
| internal/p2p | peer_test.go, peer_extra_test.go, torrent_extra_test.go | ~55% | improved | DHT ops, peers, chunk request paths |
| pkg/build | build_test.go | ~45% | stable | CacheKey determinism verified; DAG execution partial |
| pkg/dag | dag_test.go | ~80% | stable | topological sort + cycle detection fully tested |
| pkg/thrivefile | thrivefile_test.go | ~75% | stable | YAML parsing for all directives verified |
| internal/lazypull | lazypull_test.go | ~55% | improved | 11 tests: fetch paths, cache hits, HTTP errors |
| cmd/thrive/commands | *_test.go | ~22% | improved | structure/flag tests for all Phase A–E commands (logic is thin by design) |
| internal/vm (desktop) | darwin_launcher_test.go, wsl2_launcher_test.go, hyperv_launcher_test.go, download_test.go, vsock_darwin_test.go | ~80% | stable | 28 tests, all PASS |

**Overall: ~60% (measured on portable packages; Linux-only suites run in CI) — Target: 70%**

### New packages (added in Phase A–E push, 2026-09-15)

| Package | Test File | Coverage | Status | Notes |
|---------|-----------|----------|--------|-------|
| internal/events | events_test.go | 55% | new | log/query/filter/format (measured) |
| internal/registry | registry_test.go | 55% | new | auth, save/load roundtrip, import, history, manifest CRUD (measured) |
| internal/volume | volume_test.go | 82% | new | full CRUD + prune + classifier (measured) |
| internal/network | nets_test.go, network_test.go | 61% | new | store/IPAM/validation (measured on darwin; bridge/veth need Linux) |
| internal/buildx | buildx_test.go | 76% | new | builder CRUD + cache accounting (measured) |
| internal/contextstore | contextstore_test.go | 78% | new | context CRUD + current (measured) |
| internal/system | system_test.go | — | new | smoke tests (Linux CI) |
| internal/swarm | swarm_test.go | — | new | init/leave/token/validation (Linux CI) |
| internal/plugin | plugin_test.go | — | new | install/enable/disable/remove (Linux CI) |
| internal/checkpoint | checkpoint_test.go | — | new | list/validation incl. CRIU gate (Linux CI) |
| pkg/dockerfile | dockerfile_test.go | — | new | 6/6 PASS verified via portable scratch module |
| pkg/compose | compose_extra_test.go | — | new | filter/scale/config/build-skips (Linux CI) |

### How to complete the last mile (Linux runner)
- `GOOS=linux go test -coverprofile=coverage.txt ./...` gives the authoritative total.
- Biggest remaining lever: `cmd/thrive/commands` (22%) — add golden-output tests for ps/images/inspect formatting.
- Live-path tests needing root: OverlayFS mount, veth/iptables, FUSE serve, criu dump.

---

## Gap Analysis — What Is Needed to Reach 70%

### Priority 1 — High Impact, Low Effort

**internal/image (0% -> target 60%)**

Write `image_test.go` covering:
- `Pull()` — mock the OCI registry HTTP endpoint, verify layers extracted to correct paths
- `Mount()` — verify OverlayFS option string construction; test fuse-overlayfs fallback path
- `Unmount()` — verify syscall.Unmount is called with MNT_DETACH
- `Push()` — mock remote.Write; verify layer tarballs are reconstructed from extracted dirs

**internal/lazypull (0% -> target 50%)**

Write `lazypull_test.go` covering:
- `fetchChunk()` — mock HTTP server returning chunk bytes; verify chunk is written to store
- Cache hit path — verify second call reads from disk without HTTP request
- HTTP error handling — 404, 500, timeout

### Priority 2 — Medium Impact

**internal/runtime (30% -> target 55%)**

Expand `runtime_test.go` covering:
- `Kill()` — verify signal delivery to container PID
- `Logs()` — write synthetic log file; verify streaming output
- `Delete()` — verify state directory is removed

**internal/p2p (25% -> target 50%)**

Expand `peer_test.go` covering:
- `Bootstrap()` — mock peer responding to FIND_NODE
- `RequestChunk()` — mock peer returning chunk bytes within timeout
- Timeout path — verify RequestChunk returns error after 30 seconds with no response

**internal/secrets (60% -> target 80%)**

Expand `vault_test.go` covering:
- Wrong master key — decrypt with different key, verify error
- Corrupt ciphertext — truncated nonce, verify error
- Missing master key file — verify auto-generation on first call

### Priority 3 — CLI Integration

**cmd/ (0% -> target 40%)**

Write golden-output integration tests:
- `thrive ps` — populate fake state directory; verify tabular output format
- `thrive images` — populate fake image directory; verify listing
- `thrive secret list` — populate fake secrets dir; verify names listed

---

## TDD Workflow Reminder

1. Write the test (RED — it must fail before implementation)
2. Run `go test ./... -run TestFunctionName` — confirm failure
3. Write minimal implementation (GREEN)
4. Run `go test ./... -run TestFunctionName` — confirm pass
5. Refactor if needed, re-run to confirm still GREEN
6. Update this table with new coverage estimate
7. Update HANDOFF.md with session progress

---

## Running Coverage Locally

```bash
# Full coverage report
GOOS=linux go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html

# Per-package coverage
GOOS=linux go test ./internal/runtime/... -cover
GOOS=linux go test ./internal/secrets/... -cover

# Race detector (run always)
GOOS=linux go test -race ./...
```
