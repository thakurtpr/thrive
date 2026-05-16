# THRIVE Project Operating System

## Philosophy

Every contributor — human or agent — operates within this OS.
The master-prompt philosophy: read context first, plan before acting, verify after.

## Before Every Session

1. Read HANDOFF.md — understand where the previous session left off
2. Read MEMORY.md — recall key decisions and persistent context
3. Read AGENTS.md — know which agents own which domains

## Coding Mandates

- TDD is mandatory: write the test first, see it fail, then implement
- All code must compile clean: `GOOS=linux go build ./...`
- All tests must pass: `go test ./...`
- Linting must be clean: `golangci-lint run`
- No hardcoded secrets, no console debug artifacts left in committed code

## Agent Responsibilities

- Every agent must update HANDOFF.md after any meaningful work session
- Domain ownership is defined in AGENTS.md — do not cross domains without coordination
- MEMORY.md is the persistent knowledge store; update it when decisions are made

## Production Checklist

Before marking any phase complete:

- [ ] All tests pass (`go test ./...`)
- [ ] Linting is clean (`golangci-lint run`)
- [ ] Coverage is at or above target for the package
- [ ] HANDOFF.md is updated with current state
- [ ] ROADMAP.md phase status is updated
- [ ] TDD_PROGRESS.md coverage table is updated
- [ ] No TODO/FIXME comments left without a tracking issue

## Project Identity

Module: `github.com/thakurprasadrout/thrive`
Runtime: Go 1.22+ | Target OS: Linux | Architecture: amd64/arm64
