---
name: driving-awit
description: Use when working in a repository that has a `.awit/` directory, when asked to pick up / implement / close work items, run the agent loop, add work items, or when an `awit` command output is unclear (No ready items, QUARANTINED, GRAPH WARNINGS, FAIL, "no author", "no agent identity"). Covers both a single agent working the queue and an orchestrator handing work items to workers.
---

# Driving awit

`awit` is a zero-daemon CLI: each command rebuilds the dependency graph from `.awit/items/*.md` and writes at most one file. Files and Git are the state; there is no server or hidden state. Other agents share these files: keep the graph truthful.

**Core rule: every state change goes through the CLI, never through editing `.awit/` by hand.** That includes the work item _body_ (Markdown below the frontmatter): read the shape with `awit template`, fill it in outside `.awit/`, and write it with `awit create --body-file` or `awit update <id> --body-file`.

## Setup (once per session)

```bash
export AWIT_AGENT=<your-name>     # identity for --claim, comment, close; rendered as agent/<name>
awit --repo <dir>                 # only if cwd is not inside the repo
```

Set `AWIT_AGENT` to keep the audit trail accurate:

- `next --claim`: `--agent` → `AWIT_AGENT` → `config.agent_id`; no identity is an error.
- `comment`/`close`: `--author` → `AWIT_AGENT` → `config.agent_id` → git `user.name`. The git fallback is lowercased with spaces hyphenated, used without `agent/`, and silent: your work is attributed to the checkout owner with no warning.

Authenticate the external tracker's CLI yourself; awit never logs in, selects logins or reads tokens:

- Gitea: `tea` (`tea login add`; `--tea-login name` selects a login).
- GitLab: `glab` 1.118.0 (`glab auth login --hostname <host>`; check with `glab auth status --hostname <host>`). Missing or rejected glab reports the problem; repair that setup, not a workaround.

Parse output with `--format json`; humans get a table, pipes get compact lines.

## Vocabulary

| Term             | Meaning                                                                                                                                                                                                                                                                                                                          |
| ---------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `status`         | Stored: `open`, `in_progress`, `closed`. Only you change it.                                                                                                                                                                                                                                                                     |
| `state`          | Derived: `ready` (no manual hold, all deps closed), `blocked` (open deps or `blocked_reason`), `closed`, `quarantined`. `in_progress (ready)` is normal: runnable, but claimed.                                                                                                                                                  |
| `Unblocks: N`    | Open items transitively waiting on this one. Higher = more valuable to finish.                                                                                                                                                                                                                                                   |
| `deps`           | IDs this item waits for; open deps block it.                                                                                                                                                                                                                                                                                     |
| `blocked_reason` | Non-empty manual hold, regardless of deps. Set only by `awit block`; cleared only by `awit unblock` or `close`.                                                                                                                                                                                                                  |
| QUARANTINED      | Faulty file or deps: cycle, dangling dep, parse error, conflict markers, id mismatch, duplicate id. Excluded from `next`.                                                                                                                                                                                                        |
| `refs`           | Repo-root-relative when `refs_base: repo` (new writes); no marker means historical `.awit/items/` base until first mutation. Comments and attachments become refs automatically. `ref add` requires an existing target unless `--allow-missing`.                                                                                 |
| `assignee`       | Set by `--claim`; retained after `close` for audit. Only `release` clears it.                                                                                                                                                                                                                                                    |
| `labels`         | Free-form. `.awit/config.yaml` `labels` is advisory: `create`/`update` warn on unknown names they introduce but store them. `awit label` counts actual use. A `blocked` label or a BLOCKED comment never holds work; `create`/`update`/`import` warn on stderr when introducing the label. Use `awit block <id> --reason "..."`. |

`list`, `next`, `prime`, `show`, `validate`, `dep` and `archive` print `warning: N items quarantined, run awit validate` on stderr first when the graph contains quarantined items or broken files. This warning changes neither exit codes nor stdout, including `--format json`.

## The loop

For each work item, run every step in order.

```bash
awit prime                       # 1. snapshot: warnings, READY, BLOCKED, critical path
awit next --claim                # 2. claim the top ready item and print its line
awit show <id> --full            # 3. work item + every ref inlined; read all of it before touching code
# 4. build your todo list from the work item (see below)
# 5. do the work
awit comment <id> "<what you did, what you verified, what is left>"   # 6. progress note; repeat as needed
awit close <id> --reason "<one line>"                                  # 7. only after EVERY acceptance criterion is verified
awit validate                    # 8. must still print PASS
git add .awit && git commit -m "awit: close <id>"                      # 9. unless the orchestrator commits
```

- **Finished = `close`.** `release` means "I give up the claim, someone else take it": it returns an in-progress **or closed** item to `open`, clears `assignee` and `claimed_at`, and confirms with a plain `reopened <id>` line (never JSON, even under `--format json`). For linked items `close` pushes `closed` (and `release` pushes `open`) to the linked issue (Gitea via `tea`, GitLab via `glab`) after the local save, under the store lock; an explicit `update --status` pushes the mapped state the same way (`closed`→closed, `open`/`in_progress`→open) and a non-status update never pushes. Automatic pushes follow `external_push` in `.awit/config.yaml` (omitted means true); `--push=true|false` overrides one invocation (a true `--no-push` equals `--push=false`; `--no-push=false` is neutral), and `--no-push`/`--push=false` is the silent offline path: no tool, auth or network even with malformed metadata. `--tea-login` chooses the Gitea login (ignored for GitLab). Local state stays canonical: a remote failure keeps the local work, prints `warning: <id> saved locally; external state push failed: <reason>; retry with awit update <id> --status <status>` on stderr and still exits 0 - run that retry, never a duplicate close reason. A config-disabled skip prints `warning: <id> saved locally; external state push skipped by config external_push: false; push with awit update <id> --status <status> --push=true`.
- **Reports go in `comment`; `close --reason` is a one-line why.** Both create comment files; repeating text duplicates it.
- **Only `next --claim` commits**, exposing double claims as merge conflicts and quarantine. Policy: `--commit=true|false` overrides the repository default `commit:` in `config.yaml` (omitted means true); `--no-commit` is deprecated `--commit=false`. `comment`, `close`, `release`, `create`, `update`, `dep`, `ref` leave changes uncommitted. Commit `.awit/` with your code: `awit: close <id>` after close, `awit: release <id>` after release, or others still see your claim. Check `.omp/agents/`, especially `.omp/agents/orchestrator.md`: if the orchestrator owns commits, report instead of committing.
- **One claim at a time** while you do the work yourself; orchestrators hold one claim per dispatched worker. Finish or `release` before the next `next --claim`.
- **Holds go in `block`.** `awit block <id> --reason "<obstacle and release condition>"` sets status `open` and clears the claim in one save, leaving deps, labels and refs unchanged. `release` preserves a hold; `close` clears it. `block` and `unblock` never push, commit or touch the tracker.

### Building your todo list from a work item

Map the work item body to todos:

1. Each `- [ ]` under `## Steps` → one todo, verbatim and in order. Never merge or skip steps, including "run, see it fail".
2. `## Acceptance Criteria` → final todo: `Verify acceptance criteria for <id>`. Run every criterion's command and compare its stated output. Do not `close` while this todo is open.
3. Read every `## Context (read first)` bullet before the first todo. Run `awit show <dep-id>` for every `deps` ID, even if Context omits it.
4. While reading refs, check the work item against them before any acceptance-criteria todo. A satisfiable criterion can still contradict the spec (401 versus 500): use the escalation ladder, not a todo.
5. Uncovered work → **new work item**, not scope growth. Comment its ID on the current item.

Work items without `## Steps` (short items): todo list = `Read refs and deps; check work item against refs`, one todo per acceptance criterion, `Verify acceptance criteria`, `Comment + close`.

## Reading `prime`

```text
=== GRAPH WARNINGS ===          # omitted when empty; anything here is excluded from next
[CYCLE] A -> B -> A (excluded from next)
[DANGLING DEP] C depends on unknown Z
=== READY (3) ===               # unblocks desc, then ID asc; the top line is what next would pick
[ID] Title | label1,label2 | Unblocks: 4
=== BLOCKED (3) ===
[ID] Title <- DEP1, DEP2        # what it waits for
[ID] Title | Blocked reason: waiting on vendor (awit unblock <ID>)        # held manually, no open deps
[ID] Title <- DEP1 | Blocked reason: waiting on vendor (awit unblock <ID>)  # both causes at once
=== CRITICAL PATH (3) ===
A -> B -> C                     # longest open chain; finishing A moves the whole project
```

Manual holds never appear under READY. Use `awit list --blocked` for the full queue when `--max-tokens` drops rows.

`prime` is deterministic (same state → same bytes) and prompt-safe. `--max-tokens N` is a soft budget: drop BLOCKED rows from the end, then READY rows from the end except the top line, then the critical path, then headings. Warning details and the top READY line survive even over budget. `-l p0` filters READY/BLOCKED; `-l p0 -l auth` means both labels, `-l p0,p1` either. `awit next -l p0` without `--claim` checks critical readiness without side effects. `awit next --why` adds a one-line stderr explanation (unblocks, critical-path membership, selection, tie-break); stdout is unchanged.

## Adding work items

Fetch the template, fill it in, and pass it back - never hand-edit files under `.awit/`:
`awit template > body.md`, fill in `body.md`, then
`awit create "<imperative title>" --brief "<1-3 sentences>" --body-file body.md -l <phase-or-area> -l p1 -d <dep-id>`.
Correct a body after the fact with `awit update <id> --body-file body.md`.

The built-in skeleton is `## Summary` + `## Acceptance Criteria`. Anything bigger carries the sections the loop reads, in this order: `## Summary`, `## Context (read first)`, `## Files`, `## Interfaces`, `## Steps`, `## Acceptance Criteria`, `## Out of scope`. Fix that shape for a repo with `.awit/config.yaml` `template:`, which replaces the default skeleton with the file's exact bytes; use a repo-root-relative path with forward slashes, resolved from the root even in nested cwd. `--body`/`--body-file` overrides it; with neither, the template applies, else the built-in skeleton. `import` never reads it. Rules:

- `## Steps` are `- [ ]` checkboxes in execution order, one action each, with real code in fenced blocks. For code, that is TDD order: write failing test → run, see it fail → implement → run, see it pass → commit. The implementer turns each box into a todo verbatim, so a box that needs guesswork is not a step.
- `## Acceptance Criteria` are commands with their expected output, checkable by someone who never read your reasoning. `## Context (read first)` says what to read and why, `## Files` names the paths the item touches (orchestrators parallelise on non-overlapping `Files`), `## Out of scope` fences scope growth. Attach the sources with `awit ref add <id> <path>`; an item nobody can review without asking you is not ready.
- Never paste raw Git conflict markers (seven `<`, a line of seven or more `=`, seven `>`) into a body: the item quarantines on its next load. Break the marker - insert U+200B after the first character - when quoting one.

- `create` requires `--brief`, at most three sentences; otherwise split the item. Only `import` derives an omitted brief from the remote title or body's first sentence.
- Priority is a label (`p0` critical path, `p1`, `p2`), never a field. Prefer `.awit/config.yaml` `labels`; check `awit label` before inventing names.
- Add deps with `awit dep add <id> <dep>` (id depends on dep). Cycles are refused before writing, with the chain printed.
- Finish with `awit validate` → `PASS`; fix missing or long brief warnings now.

## Escalation ladder

Stop and follow the matching row; never bypass the graph.

| Signal                                                                         | Action                                                                                                                                                                                                                                                          |
| ------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `next` exits 1: `No ready items`                                               | Queue drained or fully blocked: stop the loop and report `awit prime` output.                                                                                                                                                                                   |
| `validate` prints `FAIL` / `prime` shows GRAPH WARNINGS                        | Each graph-fault line carries a `fix:` command. Run it only for your work item or one you just created; otherwise report the line verbatim. Never `dep rm` to unblock yourself.                                                                                 |
| `[CONFLICT MARKERS]`                                                           | Two branches touched the item, usually a double claim. Resolve the Git conflict or hand it to the human; then `validate`. Do not edit around it.                                                                                                                |
| Work item contradicts refs/spec, or leaves two equally valid designs undecided | `awit comment <id> "BLOCKED: <exact contradiction and the two options>"` (skip if recorded), then `awit block <id> --reason "<what must resolve>"`. Report `BLOCKED`; neither close nor choose. `awit unblock <id>` only after the recorded condition resolves. |
| Missing prerequisite: closed dep not actually done, or absent tool             | Upstream truth is wrong: `awit comment <id> "NEEDS_CONTEXT: …"`, then `awit block <id> --reason "<missing prerequisite and what provides it>"`; report. `release` preserves the hold. Never close the dep yourself.                                             |
| `Error: no agent identity; pass --agent or set AWIT_AGENT`                     | `next --claim` lacks identity: `export AWIT_AGENT=<name>` and rerun.                                                                                                                                                                                            |
| `Error: no author; pass --author or set AWIT_AGENT`                            | `comment`/`close` found neither identity nor git `user.name`, typically in bare CI. `export AWIT_AGENT=<name>` or pass `--author` once.                                                                                                                         |
| Comment or close signed with an unexpected human name                          | Silent git `user.name` fallback, not an error. Set `AWIT_AGENT`; comment which notes were misattributed. Never rewrite comment files.                                                                                                                           |
| `validate --stale-claims` warns about someone else's claim                     | The agent may have died: report it. `release` another agent's item only when explicitly told.                                                                                                                                                                   |
| `Error: another awit process holds .awit/.lock`                                | Concurrent run in this checkout: retry after a moment. Never delete the lock.                                                                                                                                                                                   |

Report status preference: `DONE`, `DONE_WITH_CONCERNS`, `BLOCKED`, `NEEDS_CONTEXT`. Include the work item ID, last `awit validate` line and refs printed by `awit comment` (`close --reason` writes a comment but prints nothing).

## Orchestrator mode

When dispatching workers:

1. `awit next --claim` yourself (you hold the claim; the worker never runs `next`).
2. Spawn one worker per work item with **only** the ID and the project invariants; the worker runs `awit show <id> --full`. Parallelise only work items whose `Files` sections do not overlap.
3. Worker reports `DONE` → review the diff → you commit → `awit close <id> --reason "implemented"`.
4. Worker reports `BLOCKED`/`NEEDS_CONTEXT` → read its comment on the work item, resolve (new work item, updated work item via `awit update`, or human), then `awit unblock <id>` and re-dispatch. Never implement it yourself.
5. Loop until `next` exits 1.

## Quick reference

| Need                        | Command                                                                                                                                                                                                                                                                                                                                           |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Whole picture               | `awit prime`                                                                                                                                                                                                                                                                                                                                      |
| Preview work                | `awit next` / `awit next -l p0` (`--why`: stderr explanation)                                                                                                                                                                                                                                                                                     |
| Take work                   | `awit next --claim` (`--commit=false` in tests; `--no-commit` also works)                                                                                                                                                                                                                                                                         |
| Resolve an item             | Any `<id>` accepts a case-insensitive `alias`, `owner/repo#127`, `group/sub/project#127` or `#127`; ambiguity lists canonical IDs.                                                                                                                                                                                                                |
| Import an issue             | `awit import <issue-url> [--brief "…"] [--alias DTRM-F21 --tea-login name]`: one-time Gitea/GitLab snapshot via `tea`/`glab`; omitted `--brief` uses the remote title, else the body's first sentence capped at 240 code points. Accepts GitLab issue and work_items URLs; `--tea-login` is ignored for GitLab. Tracker-aware duplicates refused. |
| Linked-body drift           | `awit external check [--tea-login name]` (read-only; exit 1 names every drifted item) / `awit external push-body <id> [--tea-login name]` (explicit local-canonical repair; verifies exact bytes)                                                                                                                                                 |
| Read an item                | `awit show <id>` (~200 tokens) / `--full` (refs inlined) / `--refs-only`                                                                                                                                                                                                                                                                          |
| What this unblocks          | `awit show <id> --unblocks` (transitive open items, honours `--format`)                                                                                                                                                                                                                                                                           |
| Body template               | `awit template` (exact bytes `create` would use)                                                                                                                                                                                                                                                                                                  |
| Record progress             | `awit comment <id> "…"` / `--file report.log`                                                                                                                                                                                                                                                                                                     |
| Change fields               | `awit update <id>` with `--status`, `--brief`, `--body`, `--body-file`, `--title`, `--assign`, `-l`, `--unlabel`, `--external-*`, `--clear-external` `[--push --no-push --tea-login]` (same-status `--status` re-pushes; `--push=true` overrides config)                                                                                          |
| Finish / give back / reopen | `awit close <id> --reason "…"` / `awit release <id>` `[--push --no-push --tea-login]`                                                                                                                                                                                                                                                             |
| Pause / resume              | `awit block <id> --reason "<obstacle and release condition>"` / `awit unblock <id>` (only after the recorded condition resolves; never claims or pushes)                                                                                                                                                                                          |
| Dependencies                | `awit dep add/rm <id> <dep>`                                                                                                                                                                                                                                                                                                                      |
| File references             | `awit ref add/rm <id> <path>` (`add` requires an existing target unless `--allow-missing`)                                                                                                                                                                                                                                                        |
| Integrity                   | `awit validate [--stale-claims]` (exit 1 on FAIL; invalid external is WARN)                                                                                                                                                                                                                                                                       |
| Filter items                | `awit list -s open -l auth --ready --format json`                                                                                                                                                                                                                                                                                                 |
| Label vocabulary            | `awit label [--state open/closed/all]`                                                                                                                                                                                                                                                                                                            |

## Common mistakes

- GitLab executes `/command` lines at column zero as quick actions, even inside fenced code blocks. Body pushes containing them are refused before any mutation. Move the slash line away from column zero and retry; awit never rewrites the body for safety.
- Linked-body drift: never hand-edit either side to match; repair with `awit external push-body <id>`.
