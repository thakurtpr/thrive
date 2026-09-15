---
name: lint-manifests
description: Validate compose files, Thrivefiles, Dockerfiles, and Go formatting
keywords:
  - manifest
  - k8s
  - kubernetes
  - compose
  - dockerfile
  - thrivefile
  - lint
  - gofmt
---

# Lint-Manifests Skill

Auto-load when the task touches manifests (compose YAML, Thrivefile, Dockerfile)
or Go formatting/linting.

## Checks (in order, stop on first failure only if it blocks the next)

1. `gofmt -l cmd/ internal/ pkg/` — must print nothing for touched files.
2. `GOOS=linux go vet ./...` — must be clean.
3. `golangci-lint run --build-tags linux` — new code clean (repo config is v1-era; CI lint is non-blocking, pre-existing findings stay).
4. Compose/Thrivefile/Dockerfile touched? `thrive compose config -f <file>` must render without errors.
5. `GOOS=linux go test -c -o /dev/null` for every touched linux-only package.

## Definition of done

Builds (linux/darwin/windows) + vet + host tests green before any commit or PR.
