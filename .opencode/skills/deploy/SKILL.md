---
name: deploy
description: Cut a Thrive release — cross-compile, VM image, GitHub release
keywords:
  - release
  - deploy
  - goreleaser
  - vm image
  - publish
---

# Deploy Skill

Triggered by: release, deploy, publish, cut a release, ship it.

## Flow

1. Read `deploy-config.md` (same dir) for artifact matrix and gates.
2. Pre-flight: `git status` clean, `GOOS=linux go build ./...`, `GOOS=linux go vet ./...`.
3. `make build-all` — expect 7 binaries, verify with `file bin/*`.
4. Tag: `git tag vX.Y.Z && git push origin vX.Y.Z` (release workflow builds on tag).
5. VM image (only if `cmd/thrived` or `scripts/build-vm-image.sh` changed): `make build-vm-image`, upload artifact, update release notes.
6. Post: smoke `thrive pull alpine && thrive run alpine -- echo ok` on Linux; update CHANGELOG.md.

## Never

- Push tags from a dirty tree. Skip the VM rebuild when only CLI/docs changed.
