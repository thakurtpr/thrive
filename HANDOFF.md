# THRIVE — HANDOFF

> This file is updated BEFORE every context compaction.
> When starting a new session, read this file FIRST.
> It tells you exactly where work stopped and what to do next.

## Last updated
[TIMESTAMP — update this before every compaction]

## What was just completed
[FILL THIS IN before compacting — be specific about files, functions, line numbers]

## What is in progress (incomplete)
[FILL THIS IN — partial implementations, half-written functions, TODOs]

## What to do next (exact next step)
[FILL THIS IN — the single most important next action]

## Files modified in last session
[LIST every file touched with a one-line description of what changed]

## Tests passing / failing
[LIST current test status — go test ./... output summary]

## Known broken things
[LIST anything that compiles but doesn't work correctly]

## Decisions made this session
[LIST any architectural or implementation decisions made — so they aren't re-debated]

## Open questions
[LIST anything unresolved that needs a decision]

---

## HOW TO USE THIS FILE

### Before compacting context:
1. Fill in every section above accurately
2. Run `go build ./...` and `go test ./...` — record results
3. Commit everything: `git add -A && git commit -m "checkpoint: [description]"`
4. Tell the user: "HANDOFF.md updated. Safe to compact and start new session."

### When starting a new session:
1. Read MEMORY.md first (architecture, decisions, gotchas)
2. Read HANDOFF.md (current state, next step)
3. Read AGENTS.md (your role and other agents' roles)
4. Run `go build ./...` to confirm compile state
5. Run `go test ./...` to confirm test state
6. Pick up exactly from "What to do next"