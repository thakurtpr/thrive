# Integration Tests

End-to-end integration tests for THRIVE.

**Requirements:** Linux kernel 5.11+, optionally fuse-overlayfs for rootless OverlayFS.

## Running

```bash
# Must run on Linux with appropriate permissions
sudo go test -v ./tests/...
```

## Planned test scenarios

- `thrive run alpine:3.19 -- /bin/echo hello` — full pull + mount + run + exit cycle
- `thrive build` with a minimal Thrivefile — DAG execution + layer commit
- `thrive secret set foo bar && thrive run --secret foo ...` — secret injection
- `thrive push` — round-trip push to a local registry (localhost:5000)

Integration tests are not yet implemented. They require a live Linux kernel with cgroup v2 delegation and are excluded from CI (which runs only unit tests).
