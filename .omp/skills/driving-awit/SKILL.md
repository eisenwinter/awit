---
name: driving-awit
description: Use when working in a repository that has a `.awit/` directory, when asked to pick up / implement / close tickets, run the agent loop, add tickets, or when an `awit` command output is unclear (No ready items, QUARANTINED, GRAPH WARNINGS, FAIL, "no author"). Covers both a single agent working the queue and an orchestrator handing tickets to workers.
---

# Driving awit

`awit` is a zero-daemon CLI: every command rebuilds a dependency graph from `.awit/items/*.md` and writes back at most one file. There is no server and no hidden state — the files and Git **are** the state. You are one of possibly several agents reading the same files, so the discipline below exists to keep the graph truthful for everyone else.

**Core rule: every state change goes through the CLI, never through editing `.awit/` by hand.** The one exception is the ticket _body_ (Markdown below the frontmatter), which `awit create` leaves empty for you to fill.

## Setup (once per session)

```bash
export AWIT_AGENT=<your-name>     # identity for --claim, comment, close; rendered as agent/<name>
awit --repo <dir>                 # only if cwd is not inside the repo
```

Without `AWIT_AGENT`, `next --claim` and `comment` fail with `Error: no author`. Parse output with `--format json`; humans get a table, pipes get compact lines.

## Vocabulary

| Term          | Meaning                                                                                                                                                          |
| ------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `status`      | Stored: `open`, `in_progress`, `closed`. Only you change it.                                                                                                     |
| `state`       | Derived: `ready` (all deps closed), `blocked`, `closed`, `quarantined`. `in_progress (ready)` is normal — it means the graph would let this run; someone has it. |
| `Unblocks: N` | How many open items transitively wait on this one. Higher = more valuable to finish.                                                                             |
| `deps`        | IDs this item waits for. Blocked is derived from deps, never stored.                                                                                             |
| QUARANTINED   | The file or its deps are faulty (cycle, dangling dep, parse error, conflict markers, id mismatch, duplicate id). Excluded from `next`.                           |
| `refs`        | Paths relative to `.awit/items/`. Comments and attachments become refs automatically.                                                                            |
| `assignee`    | Set by `--claim`. **Stays after `close`** on purpose (audit trail); only `release` clears it.                                                                    |

## The loop

Run this per ticket. Do not skip steps; do not reorder.

```bash
awit prime                       # 1. snapshot: warnings, READY, BLOCKED, critical path
awit next --claim                # 2. claim the top ready item; prints its line, commits "awit: claim <id>"
awit show <id> --full            # 3. ticket + every ref inlined; read all of it before touching code
# 4. build your todo list from the ticket (see below)
# 5. do the work
awit comment <id> "<what you did, what you verified, what is left>"   # 6. progress note; repeat as needed
awit close <id> --reason "<one line>"                                  # 7. after EVERY acceptance criterion is verified
awit validate                    # 8. must still print PASS
git add .awit && git commit -m "awit: close <id>"                      # 9. unless the project says the orchestrator commits
```

Decisions the baseline agent had to guess, resolved:

- **Finished = `close`.** `release` means "I give up the claim, someone else take it" and sets it back to `open`.
- **Notes go in `comment`.** `close --reason` is a one-line why, not the report; both create a comment file, so writing the same text in both duplicates it.
- **Only `next --claim` commits** (so a double claim becomes a merge conflict, which quarantine surfaces). `comment`, `close`, `release`, `create`, `update`, `dep` leave the tree dirty — commit `.awit/` together with your code when you are done: `awit: close <id>` after a close, `awit: release <id>` after a release (otherwise Git still shows your claim to everyone else). If an orchestrator owns commits in this project (see `.omp/agents/orchestrator.md`), do not commit; report instead.
- **`.awit/.lock` is never committed.** `awit init` gitignores it; if `git status` shows it untracked, add `.awit/.lock` to `.gitignore` first, then `git add .awit`.
- **One claim at a time.** Finish or `release` before the next `next --claim`.

### Building your todo list from a ticket

The ticket body is the plan. Map it mechanically:

1. Each `- [ ]` under `## Steps` → one todo, verbatim title, in order. Do not merge steps; do not skip a "run, see it fail" step.
2. `## Acceptance Criteria` → one final todo: `Verify acceptance criteria for <id>`. Run each criterion's command and compare against the stated output. `close` is forbidden while this todo is open.
3. Every `## Context (read first)` bullet → read before the first todo. `awit show <dep-id>` every ID in `deps`, whether or not a Context section names it.
4. **Check the ticket against its refs while reading them** — before any acceptance-criteria todo. A criterion can be literally satisfiable and still wrong (spec says 401, ticket says 500). Contradiction → escalation ladder, not a todo.
5. Anything you discover that the ticket does not cover → **new ticket** (below), not silent scope growth. Mention its ID in a comment on the current ticket.

Tickets without `## Steps` (short items): todo list = `Read refs and deps; check ticket against refs`, one todo per acceptance criterion, `Verify acceptance criteria`, `Comment + close`.

## Reading `prime`

```text
=== GRAPH WARNINGS ===          # omitted when empty; anything here is excluded from next
[CYCLE] A -> B -> A (excluded from next)
[DANGLING DEP] C depends on unknown Z
=== READY (3) ===               # unblocks desc, then ID asc; the top line is what next would pick
[ID] Title | label1,label2 | Unblocks: 4
=== BLOCKED (2) ===
[ID] Title <- DEP1, DEP2        # what it waits for
=== CRITICAL PATH (3) ===
A -> B -> C                     # longest open chain; finishing A moves the whole project
```

`prime` is deterministic (same state → same bytes) and safe to paste into a prompt. `--max-tokens N` trims READY then BLOCKED, never warnings. `-l p0` restricts READY/BLOCKED to items with that label; `-l p0 -l auth` = both labels, `-l p0,p1` = either. `awit next -l p0` (without `--claim`) answers "is anything critical ready?" without side effects.

## Adding tickets

```bash
awit create "<imperative title>" \
  --brief "<1-3 sentences: what is wrong / what exists after>" \
  -l <phase-or-area> -l p1 \
  -d <dep-id> -d <dep-id>
```

Then edit `.awit/items/<new-id>.md` **below** the closing `---` — fill `## Summary` and `## Acceptance Criteria` (commands with expected output). Bigger tickets follow the project's ticket template (in this repo: `plan/implementation-guide.md` §6). Rules:

- `--brief` is mandatory and must fit three sentences. If it cannot, the ticket is two tickets.
- Priority is a label (`p0` critical path, `p1`, `p2`), never a field. Check `awit label` for the vocabulary already in use before inventing one.
- Deps later: `awit dep add <id> <dep>` (id _depends on_ dep). It refuses cycles before writing and prints the chain.
- Finish with `awit validate` → `PASS`. Warnings about a missing or long brief are yours to fix now.

## Escalation ladder

Stop and act per row. Do not improvise around the graph.

| Signal                                                                                     | Meaning                                                      | Do                                                                                                                                                             |
| ------------------------------------------------------------------------------------------ | ------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `next` exits 1: `No ready items`                                                           | Queue is drained or fully blocked.                           | Stop the loop. Report `awit prime` output.                                                                                                                     |
| `validate` prints `FAIL` / `prime` shows GRAPH WARNINGS                                    | Graph fault. Each line carries a `fix:` command.             | Run the fix only if the affected item is yours (your ticket, or one you just created). Otherwise report the line verbatim; never `dep rm` to unblock yourself. |
| `[CONFLICT MARKERS]`                                                                       | Two branches touched the same item — usually a double claim. | Do not edit around it. Resolve the Git conflict (or hand to the human), then `validate`.                                                                       |
| Ticket contradicts its refs / spec, or two designs are equally valid and it did not choose | Not your call.                                               | `awit comment <id> "BLOCKED: <exact contradiction and the two options>"`, then `awit release <id>`. Report `BLOCKED`. Do not close, do not pick.               |
| Missing prerequisite (dep marked closed but not really done, tool absent)                  | Upstream truth is wrong.                                     | `awit comment <id> "NEEDS_CONTEXT: …"`, `awit release <id>`, report. Never close the dep yourself.                                                             |
| `Error: no author`                                                                         | Identity not set.                                            | `export AWIT_AGENT=<name>` and rerun.                                                                                                                          |
| `validate --stale-claims` warns about someone else's claim                                 | Another agent may have died.                                 | Report it. Only `release` another agent's item when explicitly told.                                                                                           |
| `Error: another awit process holds .awit/.lock`                                            | Concurrent run in this checkout.                             | Retry after a moment; do not delete the lock.                                                                                                                  |

Report statuses, in this order of preference: `DONE`, `DONE_WITH_CONCERNS`, `BLOCKED`, `NEEDS_CONTEXT`. Always include the ticket ID, the last `awit validate` line, and the ref(s) `awit comment` printed (`close --reason` also writes a comment but prints nothing).

## Orchestrator mode

When you dispatch workers instead of working yourself:

1. `awit next --claim` yourself (you hold the claim; the worker never runs `next`).
2. Spawn one worker per ticket with **only** the ID and the project invariants; the worker runs `awit show <id> --full`. Parallelise only tickets whose `Files` sections do not overlap.
3. Worker reports `DONE` → review the diff → you commit → `awit close <id> --reason "implemented"`.
4. Worker reports `BLOCKED`/`NEEDS_CONTEXT` → read its comment on the ticket, resolve (new ticket, updated ticket via `awit update`, or human), re-dispatch. Never implement it yourself.
5. Loop until `next` exits 1.

## Quick reference

| Need                              | Command                                                                  |
| --------------------------------- | ------------------------------------------------------------------------ | -------------- | ------- | -------- | --- | ---------- |
| Whole picture                     | `awit prime`                                                             |
| What would I get, no side effects | `awit next` / `awit next -l p0`                                          |
| Take work                         | `awit next --claim` (`--no-commit` in tests)                             |
| Read a ticket                     | `awit show <id>` (~200 tokens) / `--full` (refs inlined) / `--refs-only` |
| Record progress                   | `awit comment <id> "…"` / `--file report.log`                            |
| Change fields                     | `awit update <id> --status                                               | --brief        | --title | --assign | -l  | --unlabel` |
| Finish / give back                | `awit close <id> --reason "…"` / `awit release <id>`                     |
| Dependencies                      | `awit dep add                                                            | rm <id> <dep>` |
| Integrity                         | `awit validate [--stale-claims]` (exit 1 on FAIL)                        |
| Everything, filtered              | `awit list -s open -l auth --ready --format json`                        |
| Label vocabulary                  | `awit label [--state open                                                | closed         | all]`   |

## Common mistakes

- Editing frontmatter by hand → id/status drift, quarantine for others. Use `update`.
- Writing the implementation report into `close --reason` → duplicated comment; put it in `comment`.
- Closing before running the acceptance-criteria commands → downstream items become ready on a lie.
- `dep rm` to make your own item ready → hides real blockers; the graph is now wrong for everyone.
- Treating `in_progress (ready)` or a retained `assignee` after close as bugs → both are expected.
- Claiming a second item while holding one → stale claims for everyone else.
- Committing `.awit/` in a project whose orchestrator owns commits → duplicate/misordered history. Check `.omp/agents/` first.
