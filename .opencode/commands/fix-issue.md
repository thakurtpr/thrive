# /fix-issue — Fix a GitHub issue end to end

Arguments: `$ISSUE` (issue number or URL).

1. `gh issue view $ISSUE` — read reproduction, labels, linked code.
2. Reproduce locally first (`GOOS=linux go build ./...`, minimal repro, no guessing).
3. Fix on a branch named `fix/<issue>-<slug>`, following AGENTS.md conventions and build tags.
4. Verify: builds (linux/darwin/windows), `go test ./...`, `GOOS=linux go vet ./...`, new regression test.
5. `gh pr create` with reproduction + fix + verification in the body. Never push to main.
