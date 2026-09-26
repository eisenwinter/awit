---
id: AWIT-0P4XRXDJ
title: Add deterministic skill sync command using existing skill primitives
brief: >-
  `awit skill sync` refreshes every detected skill target with deterministic current/updated/created output, reusing Detect/Render and init's atomic writer.
status: in_progress
deps: []
labels: [tui]
refs_base: repo
refs: []
assignee: agent/orchestrator
claimed_at: "2026-09-26T20:50:43Z"
---
## Summary

Add `awit skill sync` to refresh every detected skill target, with deterministic `current`, `updated`, and `created` output.

## Context (read first)

- `internal/skill/skill.go` — `Targets()` (5 dirs, fixed order), `Detect(root)`, `Render(t)` (infallible: frontmatter + body).
- `internal/cli/init.go` — `seedSkills`, prompt policy, `config.WriteAtomic` writer, root resolution; policy stays untouched.
- `internal/cli/app.go` — command-group registration pattern (e.g. `external`); global `--repo` inherited.
- `docs/superpowers/plans/2026-09-26-skill-sync.md` §D.

## Files

- `internal/cli/app.go` — register the `skill` command group.
- `internal/cli/skill.go` — NEW: group + `sync` action (follow existing file conventions).
- `internal/cli/skill_sync_test.go` — NEW: focused behavior tests via the `run()` harness.
- Existing help goldens — update only directly affected contracts.

## Interfaces

- CLI: `awit skill sync` (existing `--repo` mechanism).
- Output: `<status> <repo-relative-slash-path>` per detected target, Targets order; statuses `current` (bytes match, no write), `updated` (replaced), `created` (missing written).
- Exit 0 on full success, including zero detected targets. Failures: contextual nonzero CLI error, stop at first failure, no success line for failed writes.

## Steps

- [ ] RED: behavior tests through the CLI harness — stale, missing, current destinations; mixed-target order; missing destination dirs; `--repo` isolation; no detected targets; read/write error paths (portable deterministic cases, no chmod-only). Assert byte-exact content, statuses, and non-rewrite of current files. See them FAIL (no `skill` command).
- [ ] GREEN: wire group/subcommand; resolve root; Detect; render once per target; read+byte-compare; atomic-write only stale/missing; report after success.
- [ ] Non-not-exist read errors stay errors. Concise group/subcommand help stating refresh + hand-edit overwrite.
- [ ] Exercise the real command in an isolated repo: stale/missing/current transitions + second-run `current` + `--repo` from another cwd.
- [ ] Focused tests + existing init/skill tests green. No project-wide validation.

## Acceptance Criteria

- `skill sync` exits 0; every detected target processed exactly once in deterministic order.
- Missing dirs/files created; stale/hand-edited files become byte-identical to `skill.Render`; current files untouched (prove non-rewrite).
- Exact `<status> <path>` output; errors nonzero without success lines.
- `--repo` works; init flag behavior unchanged; no dry-run/JSON/schema/new dep.

## Out of scope

Detection redesign, new targets, merge/backups, per-target selection, locks, retries, rollback, new config, project-wide validation.
