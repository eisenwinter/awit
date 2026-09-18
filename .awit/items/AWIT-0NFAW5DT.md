---
id: AWIT-0NFAW5DT
title: next accepts an item ID to claim
brief: >-
  GLM playground run bypassed next --claim (random top pick) for update --status to grab a specific ticket. next --claim with a positional ID claims that exact item when ready, errors otherwise; no-ID behavior unchanged.
status: in_progress
deps: []
labels: []
refs: []
assignee: agent/orchestrator
claimed_at: "2026-09-18T16:20:21Z"
---

## Summary

Evidence: GLM playground run (`/tmp/longcat/mealvoter`, 2026-09-18,
transcript `history://GlmMealvoter`). `next --claim` picks a random ready
ticket (seeded tie-break), so the worker bypassed it and used
`update --status in_progress` to grab the ticket it wanted — losing the
claim commit. Positional ID keeps one path: `awit next --claim <id>`
claims that exact item when ready, errors otherwise; bare `next --claim`
keeps today's pick-top-ready behavior and random ties.

Open behavior decision (guide §2 first): claiming an `<id>` another agent
holds (`in_progress` is Ready per `classify`) would silently overwrite
`assignee`/`claimed_at` — the double-claim the tie-break exists to
prevent. Either refuse (`is claimed by …; awit release …`) or allow
steal; decide before shipping strings.

## Acceptance Criteria

- [ ] `awit next --claim <id>` claims the exact item when ready (same
  file write + commit as bare `--claim`).
- [ ] Refusals: blocked (names deps), closed (points at release),
  quarantined (names reasons, points at validate), unknown ID (existing
  `unknown item` string). Decided behavior for already-claimed.
- [ ] `next --help` and top-level session block cover the ID form.
- [ ] Command tests + `validate` PASS.

## Approved wording (prose-reviewer, 2026-09-18)

Usage line (`next.go`): `ArgsUsage: "[id]"`,
`Usage: "Print the top unblocked item, or [id], optionally claiming it"`.

`--claim` flag (79 cols, zero net width):
`claim [id] or the pick: sets in_progress, commits (needs --agent or AWIT_AGENT)`

Errors (no hardcoded `Error: ` prefix; exit 1 via `Main`):
```
AWIT-0K7M4D4J is blocked by AWIT-0K7M2QX9, AWIT-0K7M4C3H
AWIT-0K7M2QX9 is closed; awit release AWIT-0K7M2QX9 to reopen it
AWIT-0K7M0A1B is quarantined [CYCLE]; run awit validate
unknown item AWIT-0K7M9ZZZ
```
Already-claimed (if refused):
`AWIT-0K7M2QX9 is claimed by agent/other; awit release AWIT-0K7M2QX9`

Session block addition (col 39, 71 cols):
`   awit next --claim <id>              claim that exact item when ready`
