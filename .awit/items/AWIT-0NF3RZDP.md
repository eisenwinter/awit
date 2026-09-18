---
id: AWIT-0NF3RZDP
title: CLI teaches its own workflow in --help
brief: >-
  Top-level --help gains a Typical session block, --status names valid values with did-you-mean, list hints open items are owed transitions. Source: LongCat-2.0 playground run filed 6 tickets but never transitioned one.
status: closed
deps: []
labels: []
refs: [../comments/AWIT-0NF3RZDP/20260918T144451Z-orchestrator.md, ../comments/AWIT-0NF3RZDP/20260918T144453Z-orchestrator.md]
assignee: agent/orchestrator
---

## Summary

`awit --help` names commands but teaches no workflow. Evidence: LongCat-2.0
playground run (`/tmp/longcat/browserfetch`, 2026-09-18) ran `init`, filed 6
tickets via `create`, built the site — and never transitioned one ticket (all
stayed open). A default-model run guessed `update --status doing` before
finding `in_progress`: `--status string` names no valid values.
Three boring fixes, one help surface, no new flag (direction revised per
reviews: teach `next --claim`, not `update --status` — a claim sets
`in_progress` by itself, so the weak model never needs `update`):
1. Top-level `--help` gains a `TYPICAL SESSION` block in urfave's house shape
   (uppercase header, 3-space indent, ≤80 cols): create → prime →
   next --claim → show --full → comment → close. Header carries
   `set AWIT_AGENT first` (`--claim` refuses without identity).
2. `--status` usage names valid values only:
   ``set status: `open`, in_progress or closed`` (usage column starts ~col 59).
3. `list` prints a `Note: `-prefixed footer to stderr while non-closed items
   exist, naming open and in_progress counts, teaching next --claim → close.

## Acceptance Criteria

- [ ] `--help` shows the session block; `--status` help names valid values.
- [ ] `update --status bogus <id>` fails with the valid-values hint.
- [ ] `list` with open items shows the footer; with none open, no footer.
- [ ] Unchanged `playground/browserfetch` rerun transitions tickets.

## Proposed help prose (Kimi K3 draft, 2026-09-18)

Statuses verified against `pkg/item/reason.go`: `open`, `in_progress`,
`closed`. Note `ParseStatus` already errors with
`unknown status %q (open|in_progress|closed)` — the gap is flag help and
workflow, not parsing.

Top-level `--help` session block:
```
TYPICAL SESSION:
   awit create "Title"        file tickets first, before writing code
   awit next                  pick the next ready ticket
   awit update <id> --status in_progress   mark it while you work on it
   awit close <id>            close it when the work is done
```

`--status` flag description:
`set status (open, in_progress, closed); use in_progress while working`

`list` footer, shown only while open items exist:
```
Note: open items remain. Run "awit next" for a ready one, then
"awit update <id> --status in_progress" while working and "awit close <id>" when done.
```

## Revised help prose (muse review, 2026-09-18)

Review changed the direction: teach `next --claim`, not
`update --status in_progress`. A claim sets `in_progress` by itself, so the
weak model never needs the `update` vocabulary at all. Session block now
follows the plan's agent loop (prime → next --claim → show --full →
comment → close).

Top-level `--help` session block:
```
Typical session:
  awit create "Title" --brief "..."   file work items first
  awit prime                          see ready and blocked items
  awit next --claim                   claim a ready item (sets status to in_progress)
  awit show <id> --full               show the full item
  awit comment <id> "note"            comment while you work
  awit close <id>                     close when done
```

`--status` flag description:
`status: open, in_progress, closed; claim sets in_progress; close sets closed`

`list` footer, shown only while open items exist:
`N open items: claim a ready item with "awit next --claim", close with "awit close <id>" when done.`

Change-notes: (1) "tickets" → "work items"/"item", block rewritten as the
real agent loop; old block taught `update --status` and hid it. (2) flag line
names the claim/close → status mapping instead of "use in_progress while
working". (3) footer teaches `next --claim`, drops harder words.

## Final help prose (prose-reviewer, 2026-09-18)

Session block (`TYPICAL SESSION (set AWIT_AGENT first):`, 3-space indent,
max 75 cols):
```
TYPICAL SESSION (set AWIT_AGENT first):
   awit create "Title" --brief "..."   file work items before you code
   awit prime                          see ready and blocked items
   awit next --claim                   claim a ready item, sets in_progress
   awit show <id> --full               read the item and its refs
   awit comment <id> "note"            add a progress note while you work
   awit close <id>                     close it when the work is done
```

`--status` usage: ``set status: `open`, in_progress or closed``

`list` footer to stderr (bare commands, house `Note: ` prefix):
```
Note: 3 open, 1 in_progress. Claim a ready item with awit next --claim;
close it with awit close <id> when the work is done.
```
Drop zero counts; count only printed rows (so `list -s closed` stays
footer-free).

Advisories for implementer: footer to stderr keeps `--format json`
parseable and golden files untouched; `next.go:26` `--claim` usage should
gain `(needs --agent or AWIT_AGENT)`; did-you-mean needs only the
suggestion — `ParseStatus` already lists values.
