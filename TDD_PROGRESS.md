# TDD Progress — Test Coverage Tracker

## Coverage Table

| Package | Test File | Coverage | Status | Notes |
|---------|-----------|----------|--------|-------|
| internal/runtime | runtime_test.go | ~30% | stable | saveState/loadState tested; exec/kill paths need expansion |
| internal/image | (pending) | 0% | missing | needs image_test.go; Pull/Mount/Unmount/Push all untested |
| internal/secrets | vault_test.go | ~60% | stable | encrypt/decrypt roundtrip verified; edge cases pending |
| internal/telemetry | telemetry_test.go | ~50% | stable | concurrent safety verified via race detector |
| internal/cgroup | cgroup_test.go | ~40% | stable | skips gracefully when not running as root |
| internal/p2p | peer_test.go | ~25% | stable | engine init + select/add/remove peer tested |
| pkg/build | build_test.go | ~45% | stable | CacheKey determinism verified; DAG execution partial |
| pkg/dag | dag_test.go | ~80% | stable | topological sort + cycle detection fully tested |
| pkg/thrivefile | thrivefile_test.go | ~75% | stable | YAML parsing for all directives verified |
| internal/lazypull | (pending) | 0% | missing | needs lazypull_test.go; FUSE fetch path not covered |
| cmd/ | (pending) | 0% | missing | CLI integration tests not yet written |
| internal/vm (desktop) | darwin_launcher_test.go, wsl2_launcher_test.go, hyperv_launcher_test.go, download_test.go | ~70% | stable | 14 tests cover all 6 launcher paths + local image override; subprocess contracts verified via injected mocks (2026-05-17) |

**Overall: ~35% — Target: 70%**

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
