---
id: AWIT-0NPYAWT6
title: "docs: document template, body flags and show --unblocks"
brief: >-
  The skill, the guide, the README and the spec all describe a CLI that no longer matches once the template, body and unblocks work lands. Updates all four surfaces, including the skill's now-false body-editing exception and the guide's required signature list.
status: closed
deps:
  [AWIT-0NPYAVTA, AWIT-0NPYAWT8, AWIT-0NPYAWTA, AWIT-0NPYAWT2, AWIT-0NPYAVT1]
labels: [phase7, p2]
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

Four surfaces describe the awit CLI, and all four go stale once `template`, the body flags
and `show --unblocks` land: the driving-awit skill asset, `plan/implementation-guide.md`,
`README.md`, and the command matrix in `plan/awit-implementation-plan.md`. The skill's core
rule becomes actively false - it names the body as the one thing edited by hand.

The guide's own rule ("never rename or change a signature listed here without updating this
guide") makes the §4.6 entry for `ReachableUnblocks` mandatory, not optional.

## Context (read first)

- `internal/skill/assets/driving-awit.body.md:6` - the core rule carving out the body.
- The same file's "Adding work items" section - tells the agent to edit `.awit/items/<new-id>.md` below the closing fence.
- The same file's "Quick reference" table.
- `plan/implementation-guide.md` §2 (additional decisions), §3 (repository layout), §4.6 (`pkg/graph` signatures).
- `plan/awit-implementation-plan.md:171` - "Nineteen commands", plus the CLI command matrix below it.
- `README.md:38` - the Commands table; rows for `create` (43), `show` (49), `update` (51).
- Depends on all five behaviour items being done.

## Files

- `internal/skill/assets/driving-awit.body.md`
- `plan/implementation-guide.md`
- `README.md`
- `plan/awit-implementation-plan.md`

## Interfaces

None. Documentation only.

## Steps

- [ ] In the skill's "Adding work items" section, replace the hand-edit instruction with the template flow:

  > Fetch the template, fill it in, and pass it back - never hand-edit files under `.awit/`:
  > `awit template > body.md`, fill in `body.md`, then
  > `awit create "<imperative title>" --brief "<1-3 sentences>" --body-file body.md -l <phase-or-area> -l p1 -d <dep-id>`.
  > Correct a body after the fact with `awit update <id> --body-file body.md`.

  Do not hardcode `/tmp` - the skill ships to Windows users too.

- [ ] Replace the core rule at `driving-awit.body.md:6`. It currently reads: _"**Core rule: every state change goes through the CLI, never through editing `.awit/` by hand.** The one exception is the work item *body* … and you fill it in."_ Replace the whole line with:

  > **Core rule: every state change goes through the CLI, never through editing `.awit/` by hand.** That includes the work item _body_ (Markdown below the frontmatter): read the shape with `awit template`, fill it in outside `.awit/`, and write it with `awit create --body-file` or `awit update <id> --body-file`.

- [ ] Add two Quick reference rows and extend one:

```
| Body template | `awit template` (exact bytes `create` would use) |
| What this unblocks | `awit show <id> --unblocks` (transitive open items, honours `--format`) |
```

and add `--body`, `--body-file` to the existing "Change fields" row.

- [ ] Add three rows to the `plan/implementation-guide.md` §2 "Additional decisions" table:
  - **`awit template`** - Prints `config.yaml template:` bytes, else `item.DefaultBody`. Builds no graph, so no quarantine warning; ignores `--format`. Shares `readTemplateBody` with `create`.
  - **`create`/`update` body flags** - `--body` (verbatim text) and `--body-file` (`PATH` or `-` for stdin) are mutually exclusive, validated for UTF-8 and conflict markers, and resolved before the lock and mint. Precedence: flags > `config.template` > `item.DefaultBody`. `--body ""` is indistinguishable from unset and falls through to `config.template`; an empty body is reachable only via `--body-file` naming an empty file. `update` adds `body == nil` to its `nothing to update` guard.
  - **`show --unblocks`** - Own view like `--refs-only`; refuses combination with `--full`/`--refs-only` (exit 2). Renders `graph.ReachableUnblocks` through `format.Write`, so `--format` works. Empty set prints nothing, exit 0. Inserted before the JSON branch, so `--format json` emits a bare entry array.

- [ ] Add `template.go` to the guide §3 `internal/cli` file list
- [ ] Add to guide §4.6 `pkg/graph`:

```go
// ReachableUnblocks returns the unique non-closed, non-quarantined nodes reachable
// from start via Unblocks edges, sorted by ID. UnblockCount is its length.
func ReachableUnblocks(start *Node) []*Node
```

- [ ] Update the `README.md` Commands table:
  - New `awit template` row - no flags; prints the body template `create` would use; ignores `--format`; builds no graph, so no quarantine warning.
  - `awit create` row (line 43): add `--body`, `--body-file`; replace the "optional `config.template` body" wording with the precedence rule.
  - `awit update` row (line 51): add `--body`, `--body-file`.
  - `awit show` row (line 49): add `--unblocks`.
  - Optionally: add `.awit/templates/` to the Layout tree, and note in the quarantine-warning paragraph that `template` stays silent.

- [ ] Update `plan/awit-implementation-plan.md`: add a `template` row to the CLI command matrix, extend the `create`, `show` and `update` rows' flag lists, and change "Nineteen commands" (line 171) to twenty. The guide wins on disagreement, but a stale matrix invites the next reader to implement from it
- [ ] Run `go test ./internal/skill/ -v` - PASS. If a test asserts the asset's headings or byte length, update it in the same commit rather than loosening the assertion
- [ ] Run `go test ./... && go vet ./...` - PASS
- [ ] Commit: `docs: document template, body flags and show --unblocks`

## Acceptance Criteria

- `grep -n "one exception" internal/skill/assets/driving-awit.body.md` returns nothing.
- Guide §4.6 lists `func ReachableUnblocks(start *Node) []*Node`.
- Guide §3 lists `template.go` under `internal/cli`.
- Guide §2 carries the three new decision rows, including the `--body ""` limit.
- `README.md` documents `awit template`, `--body`/`--body-file` on both `create` and `update`, and `--unblocks` on `show`.
- `plan/awit-implementation-plan.md` says twenty commands and carries a `template` row.
- `go test ./... && go vet ./...` passes.

## Out of scope

- Any code change; every other item in this batch owns its own behaviour.
- Documenting MCP. That exploration was measured and deliberately not built.

## Comments

### 2026-09-21T14:35:47Z agent/orchestrator

skill, guide, README and plan matrix document template, body flags and show --unblocks; derived omp skill copy regenerated

### 2026-09-21T14:35:47Z agent/orchestrator

Implemented by DocsWorker (all seven acceptance criteria verified; skill asset, guide sections 2/3/4.6, README, plan matrix; derived .omp SKILL.md regenerated as TestDogfoodOmpCopyMatchesRenderer requires; go test ./... and vet clean). Reviewed by reviewer agent: correct, two nits, both folded in: guide section 2 --unblocks empty-set row now per-format precise (compact nothing / table header / json []), and docs/schema.md template row now names create AND awit template as readers with both failure modes (reviewer-flagged stale cross-reference outside the item Files list, corrected with its prescribed one-liner).
