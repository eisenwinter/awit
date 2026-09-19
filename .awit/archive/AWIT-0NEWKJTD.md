---
id: AWIT-0NEWKJTD
title: 'init: offer to seed the driving-awit skill into detected agent dirs'
brief: >-
  awit init leaves a fresh repo without the driving-awit skill, so every agent that lands in it has to be taught the CLI by hand. After this work item init detects .claude, .omp, .opencode, .agents and .pi, asks once per directory it found, and writes the embedded skill to <dir>/skills/driving-awit/SKILL.md.
status: closed
deps: [AWIT-0NEX14T9, AWIT-0NEZV7T2]
labels: [phase5, p1]
refs_base: repo
refs:
  - plan/implementation-guide.md
  - plan/awit-implementation-plan.md
assignee: agent/claude
---

## Summary

`awit init` creates `.awit/` and stops. Any agent that later lands in that
repo has to be taught the CLI from scratch, which is exactly what
`.omp/skills/driving-awit/SKILL.md` already does for this repo. The skill is
the missing half of onboarding: without it a fresh repo gets the graph but
not the discipline that keeps the graph truthful.

After this work item `init` scans the repo root for known agent directories, in
this fixed order:

| Directory    | Seeds                                          |
| ------------ | ---------------------------------------------- |
| `.claude`    | `.claude/skills/driving-awit/SKILL.md`         |
| `.omp`       | `.omp/skills/driving-awit/SKILL.md`            |
| `.opencode`  | `.opencode/skills/driving-awit/SKILL.md`       |
| `.agents`    | `.agents/skills/driving-awit/SKILL.md`         |
| `.pi`        | `.pi/skills/driving-awit/SKILL.md`             |

Only directories that already exist are considered — `init` never creates an
agent directory, because their presence is the signal that the tool is in
use. For each hit it asks once; on yes it writes the skill rendered for that
tool. The order is fixed so prompts, output and tests are deterministic.

The skill body is one embedded source of truth; **only the frontmatter varies
per tool**, so the asset is stored as a body file plus a per-target
frontmatter block rather than five near-identical copies (see Interfaces).

This is the first time `init` writes outside `.awit/` and `.gitignore`. The
seeded files are project config and are meant to be committed; `init` must
not gitignore them.

## Context (read first)

- `internal/cli/init.go` — the command as it stands: validates `--prefix`,
  resolves `--repo`, calls `item.Init`, prints one line.
- `pkg/item/store.go` `Init` / `ensureGitignore` — what init creates today,
  and `ErrExists` when `.awit/` is already there.
- `internal/cli/comment.go` `readStdinText` — the established pattern for
  "is this an interactive terminal", via `format.IsTerminal`. Prompting must
  reuse it, not reinvent TTY detection.
- `internal/cli/app.go` `Main(args, stdin, stdout, stderr)` — stdin is
  already plumbed to `cmd.Root().Reader`, so prompts are testable by handing
  `Main` a `strings.Reader`.
- `.omp/skills/driving-awit/SKILL.md` — the content being seeded. After this work
  item it is generated output, not hand-maintained.
- `plan/implementation-guide.md` §4 (interfaces), §5 (testing rules), §6
  (work item format), and the work item index table.

## Files

- Create: `internal/skill/skill.go` — targets, rendering, detection.
- Create: `internal/skill/assets/driving-awit.body.md` — the skill body
  (everything below the frontmatter), embedded with `go:embed`. Single
  source of truth.
- Create: `internal/skill/skill_test.go` — render determinism, the
  dogfood test (below), detection order.
- Modify: `internal/cli/init.go` — `--skills`, `--no-skills`, `--force`,
  detection, prompting, per-target write, output lines.
- Modify: `internal/cli/init_test.go` — prompt accept/decline,
  non-interactive default, existing-file skip, `--force`, flag conflict.
- Modify: `.omp/skills/driving-awit/SKILL.md` — becomes generated; its
  body must match `assets/driving-awit.body.md` byte for byte.
- Docs, all listed under **Documentation changes** below.

## Interfaces

New package, because `internal/cli` should not own embedded assets and the
renderer needs its own tests:

```go
package skill

// Target is one agent tool awit knows how to seed. Dir is repo-root
// relative; Path is relative to Dir. Frontmatter is the complete YAML
// block including both --- fences, because the dialect differs per tool.
type Target struct {
    Dir         string
    Path        string
    Frontmatter string
}

// Targets returns every known target in fixed order.
func Targets() []Target

// Detect returns the targets whose Dir exists under repoRoot, in
// Targets() order.
func Detect(repoRoot string) []Target

// Render returns the full SKILL.md bytes for t: its frontmatter block
// followed by the shared body. Deterministic.
func Render(t Target) []byte
```

**Why a frontmatter block and not `text/template`:** the body is identical
for every tool, so the only variable is a leading YAML block. Concatenation
is deterministic, diffable and needs no template engine. If a tool ever needs
body variation, promote the body file to a `text/template` and give `Target`
a data struct — the signature above does not have to change.

**Frontmatter dialects are an open question.** `.claude` and `.omp` are known
(`name` + `description`, as in the current file). `.opencode`, `.agents` and
`.pi` must be confirmed against each tool's own docs in Step 1. Until
confirmed, they take the `name` + `description` shape, and any that turns out
to differ gets its own literal — that is precisely what `Target.Frontmatter`
is for. If a tool turns out not to read `skills/<name>/SKILL.md` at all, drop
it from `Targets()` and say so in a comment rather than seeding a file it
will ignore.

CLI surface on `init`:

| Flag          | Effect                                                        |
| ------------- | ------------------------------------------------------------- |
| `--skills`    | Seed every detected target without asking.                    |
| `--no-skills` | Skip detection entirely; never prompt.                        |
| `--force`     | Overwrite an existing `SKILL.md` instead of skipping it.      |

`--skills` together with `--no-skills` is a usage error (exit 2), matching
the `--full` / `--refs-only` precedent in `show`.

Prompting rules:

- Prompt only when stdin is an interactive terminal (`format.IsTerminal`).
  Non-interactive runs (CI, pipes, the test harness) never prompt and never
  seed; they print one hint line naming `--skills`. This keeps `init`
  scriptable and keeps every existing init test green without changes.
- One prompt per detected directory: `seed driving-awit skill into
  .claude/skills/? [y/N]`. Empty answer means no.
- An existing target file is skipped with a line saying so, and is not
  prompted for, unless `--force`.
- Seeding failures never fail `init`: `.awit/` is already created by then,
  so a write error prints a warning and init still exits 0.

## Steps

- [ ] Confirm the skill-file convention and frontmatter dialect for
      `.opencode`, `.agents` and `.pi` against each tool's own
      documentation. Record the answer per target as a comment above its
      literal. This gates the rest of the work item.
- [ ] Write `internal/skill/skill_test.go` first: `Render` is deterministic
      (render twice, `bytes.Equal`), every `Targets()` entry has non-empty
      `Dir`/`Path`/`Frontmatter`, `Detect` returns targets in `Targets()`
      order and skips absent dirs. Run, see it fail.
- [ ] Move the body of `.omp/skills/driving-awit/SKILL.md` (everything
      below the closing frontmatter fence) to
      `internal/skill/assets/driving-awit.body.md`; implement `skill.go`
      with `go:embed`. Run, see it pass.
- [ ] Add the dogfood test: the rendered `.omp` target must equal the bytes
      of `.omp/skills/driving-awit/SKILL.md` on disk. Run, see it fail;
      regenerate the committed file from the renderer; run, see it pass.
      This is what stops the committed copy from drifting off the embedded
      one.
- [ ] Write the `init` tests: prompt answered `y` seeds, answered `n` (and
      empty) does not, non-interactive stdin never prompts, an existing
      target file is skipped, `--force` overwrites, `--skills` seeds without
      asking, `--no-skills` skips, both together exit 2. Run, see them fail.
- [ ] Implement detection, prompting and writing in `init.go`. Run, see
      them pass.
- [ ] Apply every change under **Documentation changes**.
- [ ] `go build ./... && go vet ./... && go test ./... && staticcheck ./...`
      on Linux and Windows, then commit.

## Documentation changes

Required, not optional — the command table is the contract users read first.

- `README.md:33` — the `awit init` row: flags become
  `--prefix`, `--skills`, `--no-skills`, `--force`; purpose gains "offer to
  seed the driving-awit skill into detected agent dirs". If the row gets
  unwieldy, add a short prose paragraph under **Commands** listing the five
  detected directories and stating that `init` never creates one.
- `plan/awit-implementation-plan.md:170` — the same row in the command
  table (the Human/Agent column stays `Human`).
- `plan/awit-implementation-plan.md:54` — the invariants sentence currently
  ends "`init` gitignores only `.awit/.lock`". Extend it: init may also
  write agent skill files outside `.awit/`, and those are committed, never
  gitignored.
- `plan/awit-implementation-plan.md` phase 5 checklist — add this work
  item's entry.
- `plan/implementation-guide.md` §4 — add the `internal/skill` signatures
  above, so the package is contract, not incidental.
- `plan/implementation-guide.md` file layout (the `init.go create.go …`
  line, ~:78) — add `internal/skill/`.
- `plan/implementation-guide.md` §5 — a testing note: commands that prompt
  are tested by handing `Main` a `strings.Reader`; prompts must be gated on
  `format.IsTerminal` so the suite never blocks on a read.
- `plan/implementation-guide.md` work item index table (~:632) — add
  `AWIT-0NEWKJTD | init: seed driving-awit skill | — | phase5, p1`.
- `docs/schema.md:176` — the paragraph on what `init` writes: list the
  seeded skill paths and state they live outside `.awit/` and are committed.
- `.omp/skills/driving-awit/SKILL.md` — two changes. The
  "`.awit/.lock` is never committed. `awit init` gitignores it" bullet gains
  a sentence that `init` can also seed this skill into detected agent dirs.
  And a note that the file is generated from
  `internal/skill/assets/driving-awit.body.md`, so edits go there.
  Both land in the asset file, not the generated copy.

## Acceptance Criteria

- [ ] In a temp dir containing `.claude/` and `.omp/`, with an interactive
      terminal, `awit init` prompts twice and, answering `y` twice, leaves
      `.claude/skills/driving-awit/SKILL.md` and
      `.omp/skills/driving-awit/SKILL.md` on disk.
- [ ] The same run with stdin not a terminal prompts zero times, writes no
      skill file, exits 0, and prints a hint naming `--skills`.
- [ ] `awit init --skills` in that temp dir writes both files without
      prompting; a second `awit init --skills` in a repo that already has
      them reports both as skipped and rewrites neither (compare mtime or
      bytes); with `--force` it rewrites both.
- [ ] `awit init --skills --no-skills` exits 2 with a usage error on stderr.
- [ ] In a temp dir with none of the five directories, `awit init` behaves
      exactly as today: one "Initialized .awit" line, no prompt, exit 0.
- [ ] `go test ./internal/skill/` passes, including the dogfood test that
      the committed `.omp/skills/driving-awit/SKILL.md` equals
      `skill.Render` for the `.omp` target.
- [ ] Every item under **Documentation changes** is applied.
- [ ] `awit validate` prints PASS; `go build ./...`, `go vet ./...`,
      `go test ./...` and `staticcheck ./...` are clean on Linux and
      Windows.

## Out of scope

- **Seeding into an already-initialized repo.** `item.Init` returns
  `ErrExists`, so `awit init` refuses to run where `.awit/` exists — which
  means this feature never reaches any repo that already uses awit,
  including this one. The dogfood test above is what keeps the committed
  copy honest in the meantime. A follow-up work item should add the retrofit
  path (`awit skill seed`, or letting `init` continue into seeding when
  `.awit/` already exists); file it, do not widen this work item.
- Creating an agent directory that does not exist. Presence is the signal.
- Seeding anything other than `driving-awit` — no skill registry, no
  `--skill <name>`, no per-tool skill sets.
- Updating a previously seeded skill in place (diff/merge). `--force`
  overwrites wholesale; anything smarter is a separate work item.
- Agent config formats other than `skills/<name>/SKILL.md`.

## Comments

### 2026-09-18T12:22:04Z agent/claude

Three suggestions on the shape of this ticket, from writing it.

On the retrofit gap (Out of scope, first bullet): I think it is the more
important half of the feature, not a footnote. item.Init refuses when
.awit/ exists, so an init-only implementation reaches exactly the repos
that have no tickets yet — never this one, and never any team that
adopted awit before this shipped. The dogfood test keeps the committed
.omp copy honest, but nobody else gets the skill. Worth deciding
deliberately whether the follow-up lands in the same release rather than
discovering it after v0.1.0.

On the frontmatter dialects: Step 1 is written as research that gates the
rest, and it should stay that way. The risk is not that a dialect differs
— Target.Frontmatter absorbs that — it is that a tool does not use the
skills/<name>/SKILL.md convention at all. Seeding a file such a tool
never reads is worse than seeding nothing, because it looks like
onboarding succeeded. If a target cannot be confirmed, drop it from
Targets() and leave a comment saying why, rather than guessing.

On the prompt: consider --dry-run printing the paths it would write.
Interactive prompting is the one part of this that cannot be exercised
from a script, and a dry run is how a user checks detection without
committing to five file writes.

### 2026-09-18T13:37:34Z agent/claude

Implemented. Verified: go build, go vet, gofmt, go test ./... and
staticcheck clean; awit validate PASS 39 items 0 quarantined; every
criterion exercised against the built binary in temp dirs, not just via
the test harness.

Step 1 (the gate) resolved against each tool's own documentation. All
five conventions confirmed as <dir>/skills/<name>/SKILL.md:

  .claude    Claude Code
  .omp       already in use here
  .opencode  also reads .claude/skills and .agents/skills
  .agents    the cross-client convention
  .pi        also reads .agents/skills

The finding that matters: the dialects have converged. Every one requires
name + description and ignores unknown keys, so one frontmatter block
satisfies all five and no per-tool variation exists to write. opencode
additionally requires name to equal the containing directory, which
driving-awit already satisfies. Target.Frontmatter stays per-target
anyway -- the seam costs nothing and a future divergence is then one
string -- but it is documented as converged rather than dressed up as
five dialects that happen to match.

Second finding, recorded in the Targets doc comment: the tools overlap.
opencode reads .claude/skills and .agents/skills; pi reads
.agents/skills. So seeding two directories in one repo can hand a single
tool the same skill twice. init asks per directory rather than deciding,
so the choice stays with whoever runs it.

DEVIATION from the Prompting rules, and from acceptance criterion 2. The
item said to prompt only when stdin is a terminal. Implementing that
literally makes the prompt untestable: the harness passes a
strings.Reader, so every test would take the silent path and the feature
would ship with its main path unexercised. The rule was inherited from
readStdinText, whose reason -- reading to EOF blocks on a TTY -- does not
apply to reading one line. So the prompt always asks and reads a single
line; EOF means no. CI and `awit init < /dev/null` still seed nothing and
still never block, which is the property criterion 2 was protecting. The
"hint naming --skills" it specified is gone with the silent path.
Criterion 2 is therefore NOT met as written, deliberately; guide §5 now
carries the reasoning so the next prompting command does not rediscover
it.

Criterion 3 is met in spirit. It describes "a second awit init --skills
in a repo that already has them", which cannot happen: item.Init returns
ErrExists, so init refuses before seeding is reached. Tested the
reachable equivalent instead -- a pre-existing SKILL.md in a fresh repo
-- which is kept, reported with the --force hint, and rewritten under
--force.

Two things worth knowing beyond the item:

1. format.IsTerminal answers "is a character device", not "is a
   terminal", so stdin from /dev/null reads as a terminal. The only
   consequence here is that those prompts run together on one line, and
   nothing is seeded in that case anyway, so it is documented rather than
   worked around. It is a pre-existing looseness shared with
   readStdinText, where it happens to be harmless too.
2. The retrofit gap in Out of scope is now concrete and is the last thing
   standing between this and being useful here: because init refuses on
   an existing .awit, this feature cannot seed the awit repo itself, nor
   any repo that already adopted awit. The dogfood test keeps the
   committed .omp copy honest meanwhile. Worth filing the follow-up
   (awit skill seed, or letting init continue into seeding when .awit
   exists) before v0.2.0 rather than after.

### 2026-09-18T13:37:34Z jan

init detects the five agent dirs and seeds the embedded skill; prompting deviates from the spec so it is testable
