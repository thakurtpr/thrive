# /review — Request a code review of uncommitted changes

Run the `code-reviewer` sub-agent against `git status --short` / `git diff`.

Scope: Go code under `cmd/`, `internal/`, `pkg/` only. Docs-only diffs get a light pass.

Ask the reviewer for a verdict (`approve` / `request-changes`) with file:line findings.
Do not apply fixes in this command — report them and stop.
