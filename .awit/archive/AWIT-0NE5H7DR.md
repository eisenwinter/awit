---
id: AWIT-0NE5H7DR
title: 'Guide section 5: record the urfave/cli v3 in-process lessons'
brief: >-
  Fold the four urfave/cli v3 findings re-derived across tickets into plan/implementation-guide.md section 5.
status: closed
deps: []
labels: [phase5, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary
Four behaviors of `github.com/urfave/cli/v3@v3.12.0` were proven (source-read + failing tests) while implementing the CLI tickets, each forcing a deviation from ticket snippets. Record all five rules in guide §5 (testing conventions / command patterns). Do not touch §4 signatures.
Rules: (1) output via `cmd.Root().Writer`/`ErrWriter`, never `cmd.Writer` (stale Writer across `Main` calls, `didSetupDefaults` gate); (2) `Required: true` fires only on the first `Main` call per process — Action-level guard with exact usage error, exit 2; (3) set-flag detection via values, not `cmd.IsSet` (`hasBeenSet` retention); (4) package-level `sync.Mutex` around `root.Run()` in `Main` (shared command tree race); (5) `-l` flags need `DisableSliceFlagSeparator: true` plus manual comma-split (guide §2 decision 1).
Evidence in repo: `internal/cli/create.go` (brief-guard comment), `update.go` (value-based set detection), `app.go` (Main mutex), `list.go` (slice-flag separator).
## Acceptance Criteria
- [ ] §5 contains all five rules in the guide's voice; no other section changed
- [ ] `go build ./...` unaffected (no Go changes)

## Comments

### 2026-09-18T06:47:13Z agent/orchestrator

implemented
