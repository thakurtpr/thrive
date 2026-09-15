# Deploy Context — THRIVE

## Artifact matrix (`make build-all`)

| Artifact | Target |
|---|---|
| bin/thrive-linux-amd64, bin/thrive-linux-arm64 | CLI (Linux) |
| bin/thrived-linux-amd64, bin/thrived-linux-arm64 | Daemon (Linux) |
| bin/thrive-darwin-arm64 | CLI (macOS) |
| bin/thrive-windows-amd64.exe, bin/thrive-windows-arm64.exe | CLI (Windows) |

Version injected via `-X main.Version` / `-X main.Commit` (see Makefile LDFLAGS).

## Release gates

- `GOOS=linux go build ./...` clean
- `GOOS=linux go vet ./...` clean
- `go test ./...` zero failures (host) + Linux CI green
- `.goreleaser.yml` for archive layout; `scripts/install.sh` for the shell installer

## VM image

Built by `scripts/build-vm-image.sh` (needs linuxkit + docker buildx + linux/arm64).
Published under GitHub releases as `thrive-vm-darwin-*.tar.gz`.
`thrive desktop init` downloads it; `THRIVE_VM_IMAGE_PATH` overrides with a local tarball.
