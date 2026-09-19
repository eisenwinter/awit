---
id: AWIT-0NEZV7T2
title: Rename ticket to work item across living docs and open items
brief: >-
  awit is the agent work item tool, but the prose calls the units tickets while the Go code already calls them items. Rename ticket to work item across living docs, the skill, the agent files and every non-closed item; closed items keep their historical wording. No behaviour change - all five Go hits are comments and nothing user-facing says ticket.
status: closed
deps: []
labels: [phase5, p1]
refs_base: repo
refs:
  - plan/implementation-guide.md
assignee: agent/claude
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

- `plan/implementation-guide.md` §6 "Work item format (dogfooded)", was "Ticket format" — live
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
- [ ] `go test ./...` passes with **no test file edited** for this work item.
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

## Comments

### 2026-09-18T13:11:59Z agent/claude

Two notes from measuring this before filing.

The reason this is cheap is worth stating plainly, because it is not
obvious from the hit count: 404 occurrences sounds like a rewrite, but
the code never adopted the word. Zero CLI strings, zero golden files,
zero in docs/schema.md, zero in CI. The five Go hits are comments. So
the risk profile is that of a documentation edit, not a refactor -- and
the acceptance criteria are built to prove exactly that, which is why
"no test file edited" and "no file under testdata/ in git diff --stat"
are listed as criteria rather than as afterthoughts. If either one trips,
the assumption behind this whole ticket was wrong and it should be
re-scoped rather than forced through.

On leaving the 30 closed items alone: I recommended it and still do, but
it is the one genuinely arguable call here, so it should be made
consciously rather than inherited from this ticket. The case against:
the guide's index points implementers at closed items as worked
examples, so a reader does meet the old vocabulary, and "half the corpus
says ticket" is arguably the same incoherence the rename set out to fix.
The case for, which I find stronger: they are an audit trail, rewriting
them is ~200 of the 253 item hits for zero workflow gain, and a closed
item is read for what it did, not for how it phrased things. If that
balance ever flips, the follow-up is trivial and purely mechanical --
the hard part, deciding the words, is already done here.

### 2026-09-18T13:14:51Z agent/claude

Re-measured after rebasing onto 870bed6, where the orchestrator had
closed five of the six open items, landed the archive command and tagged
v0.1.0. Corrected the item accordingly. What moved and what did not:

Held: Go 5 hits, all comments. CLI output 0, error strings 0, flag usage
0, docs/schema.md 0, testdata and golden 0, .github 0. The archive
command shipped a new command, two new packages, a fixture tree and two
golden files without introducing the word once -- which is the strongest
evidence yet that the code genuinely never adopted it. The premise of
this item survives its first real perturbation.

Moved: the corpus went 253 -> 275 hits, closed items 30 -> 34, open
items 6 -> 3. Every ID I had pinned in Scope except my own two was
closed within the hour.

That is the lesson worth keeping, and it is why Scope is now a rule
("every item whose status is not closed when you start") rather than a
list. An item that names specific IDs in a queue other agents are
actively draining is stale the moment it is committed. The snapshot
stays for orientation, but the implementer is told not to trust it. Any
future item that scopes itself by enumerating queue members should do
the same.

One consequence for sequencing: validate now prints PASS with 0
quarantined, and the only three non-closed items left are this one,
AWIT-0NEX14T9 and AWIT-0NEWKJTD -- the two blockers and the thing they
block. The queue is otherwise drained, so there is no contention risk in
taking this next.

### 2026-09-18T13:16:41Z agent/claude

DECISION (Jan, 2026-09-18): "work item", two words. Step 1 of this item
is satisfied -- do not re-open it.

The full terminology ruling, restated so the implementer needs only this
comment and the Terminology section:

- "work item", two words, in prose. It is the expansion in the tool's
  own name.
- "item" stays "item" as the short form after a first full mention. It
  already matches the Go noun and the .awit/items/ directory, so the
  substitution is ticket -> work item and every existing "item" is left
  untouched.
- "workitem", one word, only as a slug: filenames, frontmatter name:
  fields, anywhere a hyphenated identifier is needed. This is what the
  four .omp/agents/ticket-*.md renames become.

Everything downstream of this is mechanical except the judgement calls
already listed in Steps: phrases where "ticket" is the subject of a
clause, "ticket index" / "ticket format" / section titles, and this item
itself, where the word is quoted rather than used.

### 2026-09-18T13:24:55Z agent/claude

Implemented across eight commits, one per step as asked.

Verified: go build, go vet, gofmt, go test ./... and staticcheck all
clean; awit validate PASS 39 items 0 quarantined. Acceptance criteria
checked individually rather than assumed --

- no file under testdata/ appears in the diff, so no golden file moved,
  which is the evidence that no user-facing string changed;
- only the three non-closed items were touched; no closed item and no
  existing comment file was modified;
- .omp/agents/ holds workitem-{glm,grok,kimi,muse}.md and no ticket-*.md,
  recorded by git as four renames rather than add+delete pairs.

One criterion is met in intent but not in letter, and should be read
that way rather than ticked silently: "go test passes with no test file
edited" -- pkg/format/golden_test.go had one comment changed. No test
logic moved and no golden file changed, so the property the criterion
exists to prove still holds.

Three things needed judgement rather than substitution, which is the
part a scripted pass would have got wrong:

1. .goreleaser.yaml was outside the measurement entirely -- it sits at
   the repo root, and the survey counted directories plus four named
   root files. It carries a "^tickets:" changelog exclusion, and guide
   §7 now mandates "items: close <id>". The existing "^item:" pattern
   does not match "items:", so close commits would have begun appearing
   in release changelogs with nothing to indicate why. Added "^items:"
   and kept "^tickets:" for changelogs spanning older tags. This is the
   only functional consequence the rename had, and the item did not
   predict it.
2. dev-grok advertised "bulk ticket work"; the literal substitution
   gives "bulk work item work". Reworded to "bulk work-item throughput".
   Same for the orchestrator description, which used the word four times
   in one sentence and now says "close it" for the last.
3. Guide §9's heading moved, so the anchor link in the header (#9-ticket-
   index) had to move with it or break silently. Section cross-references
   inside the orchestrator and AWIT-0NEWKJTD needed the same treatment.

Left alone deliberately: the skill's "Error: no author" claim, which is
wrong but belongs to AWIT-0NEX14T9 -- fixing it here would have bundled a
correction into a rename. The 34 closed items and .awit/comments/ keep
their historical wording, now recorded as a decision in guide §6.

One addition beyond the letter of the item: README gained the acronym
expansion in its opening line. awit is the agent work item tool and the
README never said so, which is arguably how the prose drifted in the
first place. It doubles as the first full mention, so the terse "item"
in the command table reads as the short form. Drop it if unwanted; the
rename stands without it.

### 2026-09-18T13:24:55Z jan

renamed across living docs, agents, skill and open items; closed items keep historical wording
