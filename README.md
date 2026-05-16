# THRIVE — THakur Runtime Isolation Virtualization Engine

A daemonless, rootless, OCI-compliant container runtime in Go. Built as a ground-up reimplementation of core Docker/containerd concepts — without the daemon.

## Platform Support

| Platform | Status | Notes |
|----------|--------|-------|
| **Linux** | Full | Native container runtime with all features |
| **macOS** | CLI Only | Build/management CLI; run containers via Docker |
| **Windows** | WSL2 | Use WSL2 for native runtime |

## Quick Start

### Linux (Ubuntu/Debian)

```bash
# Install
curl -fsSL https://raw.githubusercontent.com/thakurprasadrout/thrive/main/scripts/install.sh | sh

# Or from source
git clone https://github.com/thakurprasadrout/thrive
cd thrive && make install

# Run a container
thrive run alpine:3.19 -- echo hello
```

### macOS

**Option 1: Use Docker (Recommended for containers)**
```bash
# Build the Docker image
docker compose up -d

# Use thrive via docker exec
docker exec thrive-dev thrive run alpine:3.19 -- echo hello
docker exec thrive-dev thrive ps
docker exec thrive-dev thrive images
```

**Option 2: Build CLI from source (macOS)**
```bash
git clone https://github.com/thakurprasadrout/thrive
cd thrive
GOOS=linux go build -o bin/thrive ./cmd/thrive
```

Note: Container run commands require Linux; macOS builds are CLI-only.

## Features

| Feature | Linux | macOS (Docker) |
|---------|-------|----------------|
| thrive run | Native | Via Docker |
| thrive ps | Yes | Yes |
| thrive images | Yes | Yes |
| thrive pull/push | Yes | Yes |
| thrive build | Yes | Yes |
| thrive logs | Yes | Yes |
| thrive secret | Yes | Yes |
| Network isolation | Yes | Yes |
| P2P distribution | Yes | Yes |

## Installation Methods

### Shell Installer (Linux)
```bash
curl -fsSL https://raw.githubusercontent.com/thakurprasadrout/thrive/main/scripts/install.sh | sh
```

### Debian Package
```bash
git clone https://github.com/thakurprasadrout/thrive
cd thrive && make deb
sudo dpkg -i ../thrive_*.deb
```

### Docker
```bash
git clone https://github.com/thakurprasadrout/thrive
cd thrive && docker compose up -d
```

## Why THRIVE

| Problem with Docker | THRIVE answer |
|---|---|
| Requires root daemon | Daemonless — every container is a direct child process |
| Needs sudo or docker group | Rootless — user namespaces by default |
| Monolithic daemon is a SPOF | No daemon to crash or hang |
| Sequential layer downloads | Lazy pulling via FUSE |
| Per-image layer duplication | Content-addressed SHA-256 chunk store |
| Sequential Dockerfile builds | Parallel DAG build engine |

## Linux Requirements

- Linux kernel 5.11+
- cgroups v2 (Ubuntu 21.10+, Fedora 31+)
- /dev/fuse access
- Go 1.25+

## Development

```bash
make build        # Build for Linux
make test         # Run tests
make test-cover   # Coverage report
make lint         # Lint
make install      # Install to /usr/local/bin
```

## License

MIT
