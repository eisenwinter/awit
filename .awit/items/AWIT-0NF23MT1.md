---
id: AWIT-0NF23MT1
title: 'Safety brake: say which .awit a mutating command found by walking up'
brief: >-
  awit searches ancestor directories for .awit/, so a command run in a subdirectory that is conceptually its own project silently writes into the parent's queue - as the playground harness does into this repo's real backlog. Mutating commands should say on stderr when the root was found by walking up, and AWIT_REPO should let a caller pin the root without passing --repo everywhere.
status: closed
deps: []
labels: [phase5, p1]
refs:
  - ../../plan/implementation-guide.md
  - ../comments/AWIT-0NF23MT1/20260918T135052Z-claude.md
  - ../comments/AWIT-0NF23MT1/20260918T141514Z-orchestrator.md
  - ../comments/AWIT-0NF23MT1/20260918T141516Z-orchestrator.md
assignee: agent/orchestrator
---

## Summary

Without `--repo`, awit walks up from the working directory until it finds a
`.awit/` (guide §2, `--repo` semantics). That is the right default — it is
why `awit next` works from anywhere inside a checkout — and this item does
not change it.

What it changes is the silence. When the walk finds a queue the caller did
not mean, nothing says so. Reproduced while building `playground/`:

    $ cd playground/browserfetch     # its own scratch project, no .awit/
    $ awit list
    [AWIT-0ND5683G] CLI skeleton with urfave/cli v3 and global flags | phase0,p0
    ...

That is this repository's real backlog, printed from a directory that has
nothing to do with it. `list` is harmless; `create` in the same spot files a
practice item into the production queue, and the output looks exactly like
success. This is the failure `AWIT-0NE5H7DZ` was written about, reached by a
different route — and the playground exists to be run by weak models, which
is precisely the population that will not notice.

## Why this cannot be auto-detected

The obvious fix — warn when the root is "suspiciously far" up — does not
work. `playground/browserfetch` is two levels below the root. So is
`pkg/item`, where running `awit close` is entirely legitimate. No threshold
separates them, because nothing in the filesystem distinguishes "a
subdirectory of my project" from "a scratch project I have not `init`ed
yet". Crossing a `.git` boundary is a real signal for nested checkouts, but
it does not fire here either: the playground is inside this very repository.

So the brake is **visibility and control**, not detection. Say what happened
and make it easy to pin; do not attempt to guess intent.

## Design

Two parts.

**1. `AWIT_REPO` environment variable.** Same meaning as `--repo`: the
directory containing `.awit/`. Precedence `--repo` flag → `AWIT_REPO` →
walk up from cwd. This gives harnesses, CI and anyone working from
subdirectories a way to pin the root once instead of threading a flag
through every call, and it matches how `AWIT_AGENT` already works.

**2. A note on stderr when a mutating command walked up.** Exactly when all
of these hold: the command mutates, no `--repo` was passed, `AWIT_REPO` was
not set, and the resolved root is not the working directory. One line:

    Note: no .awit in the current directory; using <root>. Run awit init here, or pass --repo / set AWIT_REPO.

Mutating means it writes an item, comment or archive file: `create`,
`update`, `close`, `release`, `dep`, `comment`, `archive`, and `next` when
`--claim` is given. Read-only commands — `list`, `show`, `prime`,
`validate`, `label`, plain `next` — stay silent. They are the ones that get
piped, diffed and compared against golden files, and a note on a read is
noise on the exact commands designed to be machine-consumed.

**Why stderr, and why this is cheap:** stdout is a contract with golden
files and `--format json`, so it must not change. stderr is free here —
every command test in `internal/cli` passes `--repo` explicitly (guide §5
mandates it), so the condition above can never hold in the suite and no
existing test moves. The note costs nothing to the people who already know
where their queue is, and shows up exactly once for the person who does not.

## Context (read first)

- `plan/implementation-guide.md` §2, `--repo` semantics — the walk-up
  decision this item deliberately preserves.
- `internal/cli/app.go` `openStore` — where the root is resolved for every
  command except `init`; the one place that knows whether a walk happened.
- `pkg/item` `Find` / `Open` / `ErrNotFound` — the walk itself. `Find`
  currently returns only the root, so the caller cannot tell a walk from a
  direct hit; it needs to report that, or `openStore` needs to compare the
  result against cwd.
- `pkg/config/config.go:90` `Agent` — the precedence pattern `AWIT_REPO`
  should mirror.
- `playground/README.md` — the run procedure that exists because of this;
  it should reference `AWIT_REPO` once this lands.

## Files

- Modify: `internal/cli/app.go` — `AWIT_REPO` in root resolution; emit the
  note from the mutating commands' shared path.
- Modify: `pkg/item/store.go` — if `Find` is the right place to report that
  a walk occurred.
- Modify: `internal/cli/app_test.go` (or a new `repo_note_test.go`) — the
  note fires for a mutating command that walked up; does not fire with
  `--repo`, with `AWIT_REPO`, when cwd is the root, or for any read-only
  command.
- Modify: `README.md` — global flags gain `AWIT_REPO`.
- Modify: `plan/implementation-guide.md` §2 `--repo` row — record the
  precedence and the note.
- Modify: `playground/README.md` — `AWIT_REPO` as the alternative to
  copying a prompt directory out.

## Interfaces

No new package. If `Find` grows a second return value, record the new
signature in guide §4 alongside the existing `pkg/item` entries.

## Steps

- [ ] Write the failing tests first: a mutating command run from a
      subdirectory of a temp repo emits the note on stderr; the same command
      with `--repo`, with `AWIT_REPO`, and from the root itself emits
      nothing; `list` and `prime` from the subdirectory emit nothing.
- [ ] Add `AWIT_REPO` to root resolution with `--repo` winning.
- [ ] Emit the note from the mutating path only. Keep it one line and keep
      the wording identical to the Design section — it is user-facing text
      that tests will pin.
- [ ] Confirm the whole existing suite is untouched: no golden file changes
      and no test that asserts `stderr == ""` starts failing.
- [ ] Apply the four documentation changes.
- [ ] `go build ./... && go vet ./... && go test ./... && staticcheck ./...`
      and `awit validate`.

## Acceptance Criteria

- [ ] In a temp repo, `cd sub && awit create "x" --brief "y"` prints the
      note on stderr, still creates the item, and exits 0.
- [ ] The same command with `--repo <root>`, or with `AWIT_REPO=<root>`, or
      run from the root, prints nothing on stderr.
- [ ] `awit list` and `awit prime` from that subdirectory print nothing on
      stderr, and their stdout is byte-identical to the same command run
      from the root.
- [ ] `AWIT_REPO` is honoured by every command that takes `--repo`, with the
      flag winning when both are set.
- [ ] `git diff --stat` lists no file under `testdata/` — no golden changed.
- [ ] `go test ./...` passes with no existing test edited, only new ones
      added.
- [ ] `go build ./...`, `go vet ./...`, `staticcheck ./...` clean on Linux
      and Windows; `awit validate` prints PASS.

## Out of scope

- Changing the walk-up default, or adding a distance limit to it. See **Why
  this cannot be auto-detected** — a limit cannot separate the good case
  from the bad one, and removing the walk would break the ergonomics that
  make awit usable from inside a tree.
- Refusing, or prompting for confirmation, when a walk happened. That
  breaks every script and agent that legitimately relies on the walk. A note
  is the whole intervention.
- Warning on read-only commands.
- Detecting nested git checkouts, or any other heuristic for "this is a
  different project". If one is ever wanted it is a separate item with its
  own argument; this one deliberately guesses nothing.
