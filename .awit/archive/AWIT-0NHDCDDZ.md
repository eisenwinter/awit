---
id: AWIT-0NHDCDDZ
title: 'next: explain the selected item on stderr with --why'
brief: >-
  Add an optional one-line explanation of next's selected item, including unblock score, critical-path membership, and the actual tie-break or explicit-selection reason. Preserve existing selection and structured stdout.
status: closed
deps: [AWIT-0ND56X3G, AWIT-0ND56V3G, AWIT-0NFAW5DT]
labels: [phase3, p2]
refs_base: repo
refs: []
---
## Summary

Add `awit next --why`, including exact-item and claim forms. Emit one explanation to stderr only after successful selection/action.

## Context (read first)

- `nextAction`, `pickNext`, `nextNode`, `refuseClaim`.
- `Graph.CriticalPath` is contextual information; it is not a next ranking criterion.
- Existing seed behavior: zero means generate from time; nonzero produces a deterministic PCG tie-break.

## Files

- Modify `internal/cli/next.go`, `next_test.go`.
- Update README next flags, guide §2/command contract, spec next, embedded skill Quick reference and selection guidance.

## Interfaces

- Add bool `--why`.
- Plain stderr form:

```text
why: <id>; unblocks=<N>; critical-path=<yes|no>; selection=max-unblocks; tie-break=<none|pcg(seed=<S>,candidates=<K>)>
```

- Explicit positional lookup uses `selection=explicit; tie-break=none` and still reports actual unblock count/critical-path membership. This applies even when exact non-claim lookup displays blocked/closed work; never falsely say it won the ready ranking.
- K counts the equal-maximum-score group after label filtering, not every ready item. If K=1, tie-break is none. When random choice occurs, print the effective seed actually passed to pickNext, including a time-derived seed, so it can be replayed.
- Membership is in the existing deterministic whole-graph critical path; no extra priority weight. Do not call RNG again or change candidate order to explain a choice.
- No explanation on failed selection, refused claim, failed save/commit, or no-ready exit. Existing errors and quarantine warning remain separate stderr lines.
- Without --why, existing stdout and selection behavior stay byte-for-byte unchanged. With --why, JSON/compact stdout stays byte-for-byte identical to the equivalent seeded invocation without it.

## Steps

- [ ] Add failing tests for unique maximum, seeded tied maximum, label-filtered tie size, critical/noncritical selection, explicit blocked lookup, no candidates, failed claim, and JSON stdout equivalence.
- [ ] Run `go test ./internal/cli -run 'NextWhy|NextSeed|NextExact' -count=1`; observe --why failures.
- [ ] Add the flag and retain already-computed effective seed/candidate-group metadata in nextAction. Format one explanation after the action and entry write succeed; call CriticalPath only when --why needs it.
- [ ] Verify --why does not call pickNext twice or modify ranking. Test observable winner/replay and output separation, not helper wiring.
- [ ] Rerun scoped tests, update documentation, and hand evidence to the orchestrator.

## Acceptance Criteria

- Scoped tests above pass.
- `awit next --seed 42 --why --format json` produces valid JSON stdout identical to `awit next --seed 42 --format json`; stderr contains exactly one why line in addition to any pre-existing applicable advisories.
- The line reports the actual unblock count, whole-graph critical-path membership, and tie group size; replay with its nonzero seed reproduces the chosen item.
- `awit next <blocked-id> --why` says explicit selection, not maximum-unblocks selection. A refused `--claim` produces no successful-pick explanation.

## Out of scope

Changing ranking, making critical-path membership a tie-break, structured explanation fields in JSON, explaining every rejected candidate.


## Comments

### 2026-09-19T15:48:52Z agent/orchestrator

implemented
