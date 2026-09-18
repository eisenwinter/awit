---
id: AWIT-0NEZV7T2
title: Rename ticket to work item across living docs and open items
brief: >-
  awit is the agent work item tool, but the prose calls the units tickets while the Go code already calls them items. Rename ticket to work item across living docs, the skill, the agent files and every non-closed item; closed items keep their historical wording. No behaviour change - all five Go hits are comments and nothing user-facing says ticket.
status: open
deps: []
labels: [phase5, p1]
refs:
  - ../../plan/implementation-guide.md
  - ../comments/AWIT-0NEZV7T2/20260918T131159Z-claude.md
  - ../comments/AWIT-0NEZV7T2/20260918T131451Z-claude.md
  - ../comments/AWIT-0NEZV7T2/20260918T131641Z-claude.md
---

## Summary

`awit` expands to **agent work item tool**. The Go code agrees — the domain
noun is `Item` (`pkg/item`, `item.Item`, `.awit/items/`). Only the prose
drifted, and it settled on "ticket", a word that appears nowhere in the name
and nowhere in the code.

This is a vocabulary change, not a behaviour change. Measured at
`870bed6` (after the v0.1.0 tag and the `archive` command landed):

| Surface                                   | Hits |
| ----------------------------------------- | ---- |
| Go code                                   | 5 (all comments) |
| CLI output, error strings, flag usage     | 0 |
| `docs/schema.md`                          | 0 |
| `testdata/`, golden files                 | 0 |
| `.github/` CI                             | 0 |
| Living prose (plan 28, .omp 112, README 5, AGENTS 2) | 147 |
| `.awit/items` bodies                      | 275, across 34 closed and 3 open files |
| `.awit/comments/`                         | 5 |

Because there is no user-facing string and no golden file, nothing needs
regenerating and no test should change. That is what makes a rename of this
size safe; confirm it stays true (Acceptance Criteria).

**Re-measure before starting.** These counts have already moved once: this
item was filed against 6 open items and by the time it was pushed the
orchestrator had closed five of them and added `archive`, taking the corpus
from 253 hits to 275. The zero rows held throughout, which is the load-
bearing part — but the item list did not, so Scope below is written as a
rule rather than a list of IDs.

## Terminology decision

Resolve this first; everything else follows from it.

- **"work item"**, two words, in prose. It is the expansion in the tool's
  own name, and "workitem" is not English.
- **"item" stays "item".** It is already the code's noun and the directory
  name, and it is the natural short form after a first full mention. So the
  substitution is `ticket -> work item`, and every existing "item" is left
  alone — which removes a large share of the edits before they start.
- **`workitem`**, one word, only as a slug: filenames, frontmatter `name:`
  fields, anywhere a hyphenated identifier is required.

## Context (read first)

- `plan/implementation-guide.md` §6 "Ticket format (dogfooded)" — live
  contract, not history. The section title and body both change.
- `.omp/skills/driving-awit/SKILL.md` — 19 hits, the largest single file.
- `.omp/agents/orchestrator.md` — 16 hits; it is the file that describes
  handing work to workers, so it carries the vocabulary most densely.
- `.omp/agents/ticket-{glm,grok,kimi,muse}.md` — four **file renames**, not
  just content edits. Verified: nothing outside each file references its
  name; the only match is its own `name:` frontmatter line.

## Why this blocks AWIT-0NEWKJTD

AWIT-0NEWKJTD embeds `driving-awit` into the binary and seeds it into every
`.claude`, `.omp`, `.opencode`, `.agents` and `.pi` directory it finds. The
skill is 19 of these hits. Renaming after that ships means every seeded repo
carries the old vocabulary and needs re-seeding; renaming before costs
nothing extra. Same argument as AWIT-0NEX14T9, which blocks it for the same
reason — get the skill's words right once, then distribute.

## Scope

**In:** `README.md`, `AGENTS.md`, `plan/awit-implementation-plan.md`,
`plan/implementation-guide.md`, all of `.omp/`, and **every item whose
status is not `closed` when you start** — determine that list yourself, do
not trust the snapshot. At `870bed6` that is three items:
`AWIT-0NEWKJTD`, `AWIT-0NEX14T9`, and this one. Optionally the 5 Go
comments — cheap, no risk, do it in the same pass.

This item is itself the one place where "ticket" legitimately survives: it
is the term being retired, so its title and body quote it throughout. Do
not substitute blindly here — a pass that turns "rename ticket to work
item" into "rename work item to work item" has destroyed the record of why
the change happened. Treat every occurrence in this file as a quotation
unless it is plainly being used as the live noun.

**Out:** every `closed` item (34 of them at `870bed6`, carrying ~270 of the
275 item hits), and every file under `.awit/comments/`. They
record what was written at the time; rewriting them changes an audit trail,
balloons the diff by roughly 270 hits, and helps nobody, because no one
works from a closed item. Accept the inconsistency deliberately rather than
by omission — and say so in the guide so the next reader knows it was a
decision.

## Files

- Modify: `README.md` (5), `AGENTS.md` (2).
- Modify: `plan/implementation-guide.md`, `plan/awit-implementation-plan.md`
  (28 between them), including the §6 section title.
- Modify: `.omp/skills/driving-awit/SKILL.md` (19).
- Modify: `.omp/agents/orchestrator.md` (16), `dev.md`, `dev-glm.md`,
  `dev-grok.md`, `dev-kimi.md`, `dev-muse.md` (7 each).
- Rename + modify: `.omp/agents/ticket-{glm,grok,kimi,muse}.md` to
  `workitem-{glm,grok,kimi,muse}.md`, updating the `name:` field in each.
- Modify: every non-closed item (3 at `870bed6`, including this one).
- Modify (optional): `internal/cli/app.go:71`, `internal/cli/show.go:93`,
  `pkg/format/golden_test.go:15`, `pkg/graph/graph.go:66`,
  `pkg/item/item.go:12` — comments only.

## Interfaces

None. No exported identifier, flag, output string or on-disk key contains
"ticket", so nothing in `plan/implementation-guide.md` §4 changes.

## Steps

- [ ] Settle the terminology decision above with Jan before editing, since
      every later step is a mechanical consequence of it.
- [ ] Record the baseline so the end state is checkable:
      `grep -ric ticket <each in-scope file>` into a scratch file, plus
      `git rev-parse HEAD` for the golden-file comparison.
- [ ] Rename the four `.omp/agents/ticket-*.md` files with `git mv`, and
      fix the `name:` field inside each. Do this first and alone — it is
      the only step that moves files, and keeping it in its own commit
      keeps the content diff readable.
- [ ] Pass over living prose file by file. **Not a blind `sed`**: phrases
      like "ticket index", "ticket format", "Ticket Ops" and any sentence
      where "ticket" is the subject of a clause need reading, not
      substitution. Where "work item" reads clumsily after a first
      mention, use "item".
- [ ] Pass over every non-closed item the same way, this one last and by hand.
- [ ] Optionally fix the 5 Go comments.
- [ ] Add one line to `plan/implementation-guide.md` recording that closed
      items deliberately keep the old wording, so the inconsistency reads
      as a decision rather than a missed file.
- [ ] `go build ./... && go vet ./... && go test ./... && staticcheck ./...`
      and `awit validate`.

## Acceptance Criteria

- [ ] `grep -ric ticket` over `README.md`, `AGENTS.md`, `plan/`, `.omp/`
      and every non-closed item returns 0, except where a hit is a deliberate
      quotation of the old term.
- [ ] `.omp/agents/` contains `workitem-{glm,grok,kimi,muse}.md` and no
      `ticket-*.md`; each file's `name:` matches its new filename.
- [ ] `git status` shows the four renames as renames (`R`), not as
      add+delete pairs.
- [ ] Every closed item and everything under `.awit/comments/` are
      byte-identical to before: `git diff --stat` lists none of them.
- [ ] `git diff --stat` lists no file under `testdata/` — no golden file
      changed, which is the proof that no user-facing string moved.
- [ ] `go test ./...` passes with **no test file edited** for this ticket.
- [ ] `go build ./...`, `go vet ./...`, `staticcheck ./...` clean on Linux
      and Windows; `awit validate` prints PASS.

## Out of scope

- Renaming anything in Go beyond the five comments. `item.Item`, `pkg/item`
  and `.awit/items/` are already correct and must not move.
- Closed items and `.awit/comments/` — see Scope.
- Renaming the `.awit/items/` directory or any on-disk key. That would be a
  format break requiring migration, and the current name is already right.
- Re-seeding skills into other repos. That is AWIT-0NEWKJTD's job, and it
  runs after this by construction.
