# Contributing to Thrive

Thank you for your interest in contributing!

## Developer Certificate of Origin

By contributing, you agree to the [Developer Certificate of Origin (DCO)](https://developercertificate.org/). Sign your commits:

```
git commit -s -m "feat: add my feature"
```

## Development Setup

**Requirements:** Go 1.22+, Linux kernel 5.11+ (for runtime tests), or macOS (for CLI-only tests).

```bash
git clone https://github.com/thakurtpr/thrive
cd thrive
go mod download
make build       # build for current platform
make test        # run all tests (GOOS=linux)
make test-cover  # coverage report
make lint        # run golangci-lint
```

## Project Structure

```
cmd/thrive/      CLI (cross-platform)
cmd/thrived/     Daemon (linux only)
internal/
  cgroup/        cgroups v2 resource limits
  image/         OCI image store
  lazypull/      FUSE lazy-pull chunks
  network/       bridge, veth, CNI, port-forward
  p2p/           DHT-based chunk distribution
  runtime/       container lifecycle
  secrets/       AES-256-GCM secret store
  signing/       Ed25519 image signing
  telemetry/     OpenTelemetry observability
```

## Making Changes

1. Fork the repo and create a feature branch from `main`
2. Write tests first (TDD); minimum 80% coverage
3. Run `make vet lint test` before pushing
4. Open a pull request against `main`; fill in the PR template

## Commit Format

```
<type>: <short description>

<optional body>
```

Types: `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `perf`, `ci`

## Code Review

All PRs require at least one maintainer approval. See [SECURITY.md](SECURITY.md) for the responsible disclosure process.

## Questions

Open a [GitHub Discussion](https://github.com/thakurtpr/thrive/discussions) or file an issue.
