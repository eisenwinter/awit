---
id: AWIT-0NHDBZDH
title: 'release: confirm reopening and describe both supported source states'
brief: >-
  Make release visibly confirm successful persistence and clarify that it reopens closed as well as in-progress items while clearing the claim.
status: closed
deps: [AWIT-0ND56M3G, AWIT-0NF68SDS, AWIT-0NF3RZDP]
labels: [phase1, p1]
refs: [../comments/AWIT-0NHDBZDH/20260919T114205Z-jan.md]
---
## Summary

After a successful release save, print exactly `reopened <canonical-id>\n` on stdout. Explicitly describe release as returning in-progress or closed work to open and clearing assignee/claimed_at.

## Context (read first)

- `internal/cli/release.go:releaseAction` already sets open and clears both claim fields.
- `closeAction` and archive use plain confirmation output regardless of global format.
- `next.go:refuseClaim` already recommends release for a closed item.

## Files

- Modify `internal/cli/release.go`, release coverage in current CLI tests; add `release_test.go` only if keeping those tests there is clearer.
- Update README Commands, guide release contract, spec command matrix, embedded skill vocabulary/quick reference.

## Interfaces

- Keep command spelling and exit behavior.
- Usage: `Return an in-progress or closed item to open and clear its claim`.
- All source states, including already-open, result in open with no assignee/claimed_at and the same successful confirmation. This gives an idempotent visible action.
- As with close/archive, `--format json|compact|table` does not change this mutation confirmation into an entry or JSON object. Document that exception explicitly.
- Print only after `Store.Save` succeeds. No success line on load/write failure. External-push warnings from Draft 4 do not erase local success confirmation.

## Steps

- [ ] Add a failing test invoking release on closed and claimed in-progress items, checking persisted state and exact confirmation; include already-open, global JSON format, and failed-save cases.
- [ ] Run `go test ./internal/cli -run Release -count=1`; observe missing-output failures.
- [ ] Replace the tail return in `releaseAction` with save-error handling followed by `fmt.Fprintf(cmd.Root().Writer, "reopened %s\n", it.ID)`; update command help.
- [ ] Rerun tests; remove any test that merely pins stale help wording and test discoverable behavior rather than implementation text.
- [ ] Update the named docs and provide evidence for orchestrator commit.

## Acceptance Criteria

- `go test ./internal/cli -run Release -count=1` passes.
- In a temporary repo, `awit release <closed-id>` prints `reopened <id>`, exits 0, and clears both claim fields.
- `awit --format json release <id>` prints the same documented plain confirmation.
- `awit release --help` describes reopening; no Git commit is made.

## Out of scope

A separate reopen command, new state values, changing close’s audit assignee retention, changing all mutation outputs to JSON.

