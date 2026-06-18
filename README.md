# Thrive

[![CI](https://github.com/thakurtpr/thrive/actions/workflows/ci.yml/badge.svg)](https://github.com/thakurtpr/thrive/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/thakurtpr/thrive)](https://goreportcard.com/report/github.com/thakurtpr/thrive)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

**THakur Runtime Isolation Virtualization Engine** — a daemonless, rootless, OCI-compliant container runtime written in Go.

Thrive reimplements core container runtime concepts from the ground up: image distribution, namespace isolation, resource limits, and networking — with no central daemon and a fully auditable codebase.

---

## Features

| Capability | Status |
|-----------|--------|
| OCI image pull / push / build | Complete |
| Namespace isolation (PID, mount, UTS, IPC, net) | Complete |
| cgroups v2 resource limits | Complete |
| Rootless (user namespaces) | Complete |
| Lazy-pull via FUSE chunk store | Complete |
| P2P image distribution (DHT) | Complete |
| AES-256-GCM secret store | Complete |
| Ed25519 image signing | Complete |
| CNI network + port forwarding | Complete |
| OpenTelemetry observability | Complete |
| Systemd socket activation | Complete |
| macOS VM desktop (vfkit) | Complete |

## Architecture

```
thrive CLI (cross-platform)
cmd/thrive  ──proxy──▶  cmd/thrived (Linux daemon)
                              │
                    ┌─────────▼─────────┐
                    │     thrived        │
                    │  runtime + ns      │
                    │  network + CNI     │
                    │  cgroup v2         │
                    │  OCI image store   │
                    │  FUSE lazy-pull    │
                    │  DHT p2p           │
                    │  AES-GCM secrets   │
                    │  Ed25519 signing   │
                    └────────────────────┘
```

## Quick Start

### Linux

```bash
git clone https://github.com/thakurtpr/thrive
cd thrive && make install

thrive run alpine:3.19 -- echo hello
thrive ps
thrive images
```

### macOS (Desktop VM via vfkit)

```bash
git clone https://github.com/thakurtpr/thrive
cd thrive
GOOS=darwin go build -o bin/thrive ./cmd/thrive

thrive desktop init
thrive desktop start
thrive run alpine:3.19 -- echo hello
```

## Building from Source

Requires Go 1.22+.

```bash
make build          # CLI for current OS
make build-linux    # linux/amd64 + linux/arm64
make build-darwin   # darwin/arm64 CLI
make test           # full test suite
make test-cover     # with coverage report
make lint           # golangci-lint
```

## Installation

### Debian/Ubuntu

```bash
sudo make install-systemd
sudo systemctl enable --now thrive.socket
```

### Shell Installer

```bash
curl -fsSL https://raw.githubusercontent.com/thakurtpr/thrive/main/scripts/install.sh | sh
```

## Documentation

- [Architecture](ARCHITECTURE.md)
- [Roadmap](ROADMAP.md)
- [Changelog](CHANGELOG.md)
- [Contributing](CONTRIBUTING.md)
- [Security Policy](SECURITY.md)
- [Governance](GOVERNANCE.md)

## Linux Requirements

- Kernel 5.11+, cgroups v2
- `/dev/fuse` for lazy-pull
- Go 1.22+ to build from source

## Community

- Issues: [GitHub Issues](https://github.com/thakurtpr/thrive/issues)
- Discussions: [GitHub Discussions](https://github.com/thakurtpr/thrive/discussions)
- Code of Conduct: [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)

## License

[MIT](LICENSE)
