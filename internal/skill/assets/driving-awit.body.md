
# Driving awit

`awit` is a zero-daemon CLI: every command rebuilds a dependency graph from `.awit/items/*.md` and writes back at most one file. There is no server and no hidden state — the files and Git **are** the state. You are one of possibly several agents reading the same files, so the discipline below exists to keep the graph truthful for everyone else.

**Core rule: every state change goes through the CLI, never through editing `.awit/` by hand.** The one exception is the work item _body_ (Markdown below the frontmatter), which `awit create` leaves empty for you to fill.

## Setup (once per session)

```bash
export AWIT_AGENT=<your-name>     # identity for --claim, comment, close; rendered as agent/<name>
awit --repo <dir>                 # only if cwd is not inside the repo
```

`AWIT_AGENT` is not optional in practice, and the two commands that need an identity fail **differently** without it:

- `next --claim` refuses: `Error: no agent identity; pass --agent or set AWIT_AGENT`. It tries `--agent` → `AWIT_AGENT` → `config.agent_id` and stops there.
- `comment` and `close` do **not** refuse. They keep going to git `user.name` (lowercased, spaces hyphenated) and use it **verbatim, with no `agent/` prefix** — so your notes are signed with whoever owns the checkout, and nothing in the output says so. You only see `Error: no author; pass --author or set AWIT_AGENT` when git has no `user.name` either, which in practice means a bare CI checkout.

So setting `AWIT_AGENT` is not what makes those commands work — it is what makes the audit trail true. Unset, the work still happens and is attributed to a human who did not do it.

Parse output with `--format json`; humans get a table, pipes get compact lines.

## Vocabulary

| Term          | Meaning                                                                                                                                                          |
| ------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `status`      | Stored: `open`, `in_progress`, `closed`. Only you change it.                                                                                                     |
| `state`       | Derived: `ready` (all deps closed), `blocked`, `closed`, `quarantined`. `in_progress (ready)` is normal — it means the graph would let this run; someone has it. |
| `Unblocks: N` | How many open items transitively wait on this one. Higher = more valuable to finish.                                                                             |
| `deps`        | IDs this item waits for. Blocked is derived from deps, never stored.                                                                                             |
| QUARANTINED   | The file or its deps are faulty (cycle, dangling dep, parse error, conflict markers, id mismatch, duplicate id). Excluded from `next`.                           |
| `refs`        | Paths relative to the repo root when `refs_base: repo` (new writes). Omitted marker means historical `.awit/items/` base until first mutation. Comments and attachments become refs automatically. |
| `assignee`    | Set by `--claim`. **Stays after `close`** on purpose (audit trail); only `release` clears it.                                                                    |

When a command loads a graph holding quarantined items or broken files (`list`, `next`, `prime`, `show`, `validate`, `dep`, `archive`), it prints one stderr line first: `warning: N items quarantined, run awit validate`. That is a pointer, not a failure: exit codes and stdout (including `--format json`) are unchanged.

## The loop

Run this per work item. Do not skip steps; do not reorder.

```bash
awit prime                       # 1. snapshot: warnings, READY, BLOCKED, critical path
awit next --claim                # 2. claim the top ready item; prints its line, commits "awit: claim <id>" unless the commit policy says no
awit show <id> --full            # 3. work item + every ref inlined; read all of it before touching code
# 4. build your todo list from the work item (see below)
# 5. do the work
awit comment <id> "<what you did, what you verified, what is left>"   # 6. progress note; repeat as needed
awit close <id> --reason "<one line>"                                  # 7. after EVERY acceptance criterion is verified
awit validate                    # 8. must still print PASS
git add .awit && git commit -m "awit: close <id>"                      # 9. unless the project says the orchestrator commits
```

Decisions the baseline agent had to guess, resolved:

- **Finished = `close`.** `release` means "I give up the claim, someone else take it": it returns an in-progress **or closed** item to `open`, clears `assignee` and `claimed_at`, and confirms with a plain `reopened <id>` line (never JSON, even under `--format json`).
- **Notes go in `comment`.** `close --reason` is a one-line why, not the report; both create a comment file, so writing the same text in both duplicates it.
- **Only `next --claim` commits** (so a double claim becomes a merge conflict, which quarantine surfaces). Whether it does follows the commit policy: `--commit=true|false` for one run, `commit: false` in `config.yaml` as the repository default (default `true`); `--no-commit` is the deprecated spelling of `--commit=false`. `comment`, `close`, `release`, `create`, `update`, `dep`, `ref` leave the tree dirty — commit `.awit/` together with your code when you are done: `awit: close <id>` after a close, `awit: release <id>` after a release (otherwise Git still shows your claim to everyone else). If an orchestrator owns commits in this project (see `.omp/agents/orchestrator.md`), do not commit; report instead.
- **`.awit/.lock` is never committed.** `awit init` gitignores it; if `git status` shows it untracked, add `.awit/.lock` to `.gitignore` first, then `git add .awit`. `init` can also seed this skill into `.claude`, `.omp`, `.opencode`, `.agents` and `.pi` (`--skills` to skip the prompts); unlike the lock, those files are project config and belong in the commit.
- **One claim at a time.** Finish or `release` before the next `next --claim`.

### Building your todo list from a work item

The work item body is the plan. Map it mechanically:

1. Each `- [ ]` under `## Steps` → one todo, verbatim title, in order. Do not merge steps; do not skip a "run, see it fail" step.
2. `## Acceptance Criteria` → one final todo: `Verify acceptance criteria for <id>`. Run each criterion's command and compare against the stated output. `close` is forbidden while this todo is open.
3. Every `## Context (read first)` bullet → read before the first todo. `awit show <dep-id>` every ID in `deps`, whether or not a Context section names it.
4. **Check the work item against its refs while reading them** — before any acceptance-criteria todo. A criterion can be literally satisfiable and still wrong (spec says 401, work item says 500). Contradiction → escalation ladder, not a todo.
5. Anything you discover that the work item does not cover → **new work item** (below), not silent scope growth. Mention its ID in a comment on the current work item.

Work items without `## Steps` (short items): todo list = `Read refs and deps; check work item against refs`, one todo per acceptance criterion, `Verify acceptance criteria`, `Comment + close`.

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

`prime` is deterministic (same state → same bytes) and safe to paste into a prompt. `--max-tokens N` is a soft budget: it sheds BLOCKED rows from the end, then READY rows from the end (never the top line), then the critical path, then headings — warning details and the top READY line always survive, even over budget. `-l p0` restricts READY/BLOCKED to items with that label; `-l p0 -l auth` = both labels, `-l p0,p1` = either. `awit next -l p0` (without `--claim`) answers "is anything critical ready?" without side effects.

## Adding work items

```bash
awit create "<imperative title>" \
  --brief "<1-3 sentences: what is wrong / what exists after>" \
  -l <phase-or-area> -l p1 \
  -d <dep-id> -d <dep-id>
```

Then edit `.awit/items/<new-id>.md` **below** the closing `---` — fill `## Summary` and `## Acceptance Criteria` (commands with expected output). Bigger work items follow the project's work item template (in this repo: `plan/implementation-guide.md` §6). Optional `.awit/config.yaml` `template:` (repo-root-relative, forward slashes) replaces the default body with that file's exact bytes; looked up from the repository root even when cwd is nested; absent keeps the skeleton; `import` never reads it. Rules:

- `--brief` is mandatory and must fit three sentences. If it cannot, the work item is two work items.
- Priority is a label (`p0` critical path, `p1`, `p2`), never a field. Check `awit label` for the vocabulary already in use before inventing one.
- Deps later: `awit dep add <id> <dep>` (id _depends on_ dep). It refuses cycles before writing and prints the chain.
- Finish with `awit validate` → `PASS`. Warnings about a missing or long brief are yours to fix now.

## Escalation ladder

Stop and act per row. Do not improvise around the graph.

| Signal                                                                                     | Meaning                                                      | Do                                                                                                                                                             |
| ------------------------------------------------------------------------------------------ | ------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `next` exits 1: `No ready items`                                                           | Queue is drained or fully blocked.                           | Stop the loop. Report `awit prime` output.                                                                                                                     |
| `validate` prints `FAIL` / `prime` shows GRAPH WARNINGS                                    | Graph fault. Each line carries a `fix:` command.             | Run the fix only if the affected item is yours (your work item, or one you just created). Otherwise report the line verbatim; never `dep rm` to unblock yourself. |
| `[CONFLICT MARKERS]`                                                                       | Two branches touched the same item — usually a double claim. | Do not edit around it. Resolve the Git conflict (or hand to the human), then `validate`.                                                                       |
| Work item contradicts its refs / spec, or two designs are equally valid and it did not choose | Not your call.                                               | `awit comment <id> "BLOCKED: <exact contradiction and the two options>"`, then `awit release <id>`. Report `BLOCKED`. Do not close, do not pick.               |
| Missing prerequisite (dep marked closed but not really done, tool absent)                  | Upstream truth is wrong.                                     | `awit comment <id> "NEEDS_CONTEXT: …"`, `awit release <id>`, report. Never close the dep yourself.                                                             |
| `Error: no agent identity; pass --agent or set AWIT_AGENT`                                 | `next --claim` has no identity to claim with.                | `export AWIT_AGENT=<name>` and rerun.                                                                                                                          |
| `Error: no author; pass --author or set AWIT_AGENT`                                        | `comment`/`close` found no identity **and** no git `user.name` — usually a bare CI checkout. | `export AWIT_AGENT=<name>`, or pass `--author` for a one-off.                                                                    |
| A comment or close is signed with a human name you did not expect                          | Not an error. `AWIT_AGENT` was unset, so git `user.name` was used verbatim and your work is attributed to them. | Set `AWIT_AGENT` now, and say in a comment which notes were misattributed. Do not rewrite the comment files.                       |
| `validate --stale-claims` warns about someone else's claim                                 | Another agent may have died.                                 | Report it. Only `release` another agent's item when explicitly told.                                                                                           |
| `Error: another awit process holds .awit/.lock`                                            | Concurrent run in this checkout.                             | Retry after a moment; do not delete the lock.                                                                                                                  |

Report statuses, in this order of preference: `DONE`, `DONE_WITH_CONCERNS`, `BLOCKED`, `NEEDS_CONTEXT`. Always include the work item ID, the last `awit validate` line, and the ref(s) `awit comment` printed (`close --reason` also writes a comment but prints nothing).

## Orchestrator mode

When you dispatch workers instead of working yourself:

1. `awit next --claim` yourself (you hold the claim; the worker never runs `next`).
2. Spawn one worker per work item with **only** the ID and the project invariants; the worker runs `awit show <id> --full`. Parallelise only work items whose `Files` sections do not overlap.
3. Worker reports `DONE` → review the diff → you commit → `awit close <id> --reason "implemented"`.
4. Worker reports `BLOCKED`/`NEEDS_CONTEXT` → read its comment on the work item, resolve (new work item, updated work item via `awit update`, or human), re-dispatch. Never implement it yourself.
5. Loop until `next` exits 1.

## Quick reference

| Need                              | Command                                                                  |
| --------------------------------- | ------------------------------------------------------------------------ | -------------- | ------- | -------- | --- | ---------- |
| Whole picture                     | `awit prime`                                                             |
| What would I get, no side effects | `awit next` / `awit next -l p0`                                          |
| Take work                         | `awit next --claim` (`--commit=false` in tests; `--no-commit` also works) |
| Look up an item                   | any `<id>` argument also accepts an `alias` (case-insensitive) or `owner/repo#127` / `#127`; ambiguity lists the canonical IDs |
| Import a Gitea issue              | `awit import <issue-url> --brief "…" [--alias DTRM-F21 --tea-login name]` (one-time snapshot via `tea`; duplicates refused) |
| Read a work item                     | `awit show <id>` (~200 tokens) / `--full` (refs inlined) / `--refs-only` |
| Record progress                   | `awit comment <id> "…"` / `--file report.log`                            |
| Change fields                     | `awit update <id> --status                                               | --brief        | --title | --assign | -l  | --unlabel | --external-* | --clear-external` |
| Finish / give back / reopen     | `awit close <id> --reason "…"` / `awit release <id>` (prints `reopened <id>`) |
| Dependencies                      | `awit dep add                                                            | rm <id> <dep>` |
| File references                   | `awit ref add                                                            | rm <id> <path>` |
| Integrity                         | `awit validate [--stale-claims]` (exit 1 on FAIL; invalid external is WARN)                        |
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

---

_This file is generated by `awit init`. In the awit repository itself the source is `internal/skill/assets/driving-awit.body.md` plus the target's frontmatter block; edit there and regenerate, because a test pins the committed copy to the renderer. In any other repository it is yours to edit._
