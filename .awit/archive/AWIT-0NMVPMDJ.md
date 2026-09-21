---
id: AWIT-0NMVPMDJ
title: 'skill/docs: teach the manual block mechanic at the failure point'
brief: >-
  Warn when the blocked label is introduced without a manual block, correct help text, and update the driving-awit skill plus agent docs to the block workflow.
status: closed
deps: [AWIT-0NMVNPDQ]
labels: [phase6, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---
## Summary

Make the mechanic discoverable where agents actually fail: a stderr warning when the exact label `blocked` is newly introduced on a non-closed item without a manual block, prose-reviewed help strings everywhere the old behavior is taught, and a corrected `driving-awit` skill (source asset + regenerated copy) plus orchestrator/dev/AGENTS guidance. Labels stay metadata; the warning teaches without assigning scheduling semantics.

## Context (read first)

- `AGENTS.md`; guide `internal/skill` section; `.omp/agents/orchestrator.md`; `.omp/agents/dev.md`.
- MB2 (`AWIT-0NMVNPDQ`): block/unblock commands. Read it via `awit show AWIT-0NMVNPDQ`.
- `internal/cli/create.go` (label diagnostics), `update.go` (introduced-label calculation), `import.go:129-140,153-164` (one-time label snapshot), `app.go` (root help, `typicalSessionBlock` col 39), `list.go`, `next.go`, `prime.go`, `release.go`, `close.go`.
- `internal/skill/assets/driving-awit.body.md` — Escalation ladder tells agents facing a contradiction to comment BLOCKED then `release`, which requeues zero-dep items; must change with this feature. `TestDogfoodOmpCopyMatchesRenderer` pins the generated copy.
- Warning conventions: one lowercase `warning: ` line, clauses joined by `;`, remedy last, no full stops (`create.go:122`, `external_state.go:42,49`, `quarantine_warn.go:25`).
- Full design: `agent://PlanBlockedState` sections D5, F. Reviewed strings: `agent://ProseBlockStrings` (finals verbatim).

## Files

- Modify `internal/cli/create.go` (new `warnBlockedLabel(cmd, it, introduced)` helper beside existing label diagnostics), `update.go`, `import.go`, `app.go`, `list.go`, `next.go`, `prime.go`, `release.go`, `close.go`, plus label/import behavior tests.
- Modify `internal/skill/assets/driving-awit.body.md`; regenerate `.omp/skills/driving-awit/SKILL.md` via the existing renderer (never hand-edit the copy).
- Update `AGENTS.md` (orchestrator records external holds with block, clears only after resolution), `.omp/agents/orchestrator.md` (loop: on BLOCKED/NEEDS_CONTEXT record a hold when an unresolved condition prevents work; do not re-dispatch until resolution + unblock), `.omp/agents/dev.md` (stop/report with concrete obstacle + release condition).
- Update `docs/schema.md`, `plan/implementation-guide.md`, `plan/awit-implementation-plan.md`, `README.md` per plan §F file table.

## Interfaces

Warning (exact, prose-reviewed #28): stderr only, stdout/exit unchanged:

```text
warning: %s has label "blocked", which does not pause work; use awit block %s --reason "..."
```

Trigger: after successful save in create, update, import — only when a non-closed item without a manual block newly receives the exact case-sensitive label `blocked` (reuse update's introduced-label calculation; include create defaults and import snapshots). No recurring query-time warnings, no reserved-label subsystem. Keep unknown-vocabulary warnings independent.

Import help (prose-reviewed #16):

```text
Import copies labels once as metadata. Labels are not synchronised
afterwards, and a label named "blocked" does not pause work. Use
awit block <id> --reason "..." for a local manual block.
```

Label-flag help: create/update `--label`: `metadata label, repeatable; use awit block <id> to pause work`; update `--unlabel`: `remove metadata labels; use awit unblock <id> to clear a manual block`. Root session lines at column 39:

```text
   awit block <id> --reason "..."      pause until a condition clears
   awit unblock <id>                   remove the manual block
```

Skill updates (`driving-awit.body.md`): Vocabulary distinguishes manual reason vs deps vs lifecycle vs labels vs quarantine. Loop/decision bullets: block releases a claim atomically; release does not unblock; close clears the reason; block/unblock never push or commit. Reading prime: manual-only and combined causes + bounded-output caveat (`list --blocked` is the audit surface). Escalation ladder: replace comment-plus-release for unresolved conditions with comment-if-needed then `block`; unblock only after the recorded condition resolves. Anti-patterns: `blocked` labels and blocking comments are not queue controls.

Existing tracker scope statements hold: import copies labels once without auto-holding; `external check` stays body-only; block/unblock/close/release/status updates never reconcile labels.

## Steps

- [ ] Write a focused regression: introducing label `blocked` keeps metadata semantics and warns once on stderr (JSON stdout still parseable); imports copy labels without auto-holding.
- [ ] Run scoped tests; record intended failures.
- [ ] Implement `warnBlockedLabel` using existing introduction detection; wire create/update/import; apply reviewed help strings; update source skill asset; regenerate the committed skill copy; update AGENTS.md, orchestrator.md, dev.md.
- [ ] Rerun scoped tests to green: `go test ./internal/skill -run 'TestDogfoodOmpCopyMatchesRenderer|TestRenderIsDeterministic' -count=1` plus label/import suites. Smoke actual `--help`, create/update/import warning paths in a temp checkout.
- [ ] If an affected existing assertion only pins old wording, remove or replace it with a behavior check — never re-pin new prose. No whole-help snapshots, no prose-wording tests.

## Acceptance Criteria

- An agent adding `blocked` is directed to the real command; label semantics unchanged; imports still copy labels once without auto-holding.
- JSON stdout stays parseable; stderr warning fires exactly once per introduction.
- Orchestrator/dev guidance states who records a hold, who may clear it, and that release is not unblock.
- Source and generated skills match (`TestDogfoodOmpCopyMatchesRenderer` passes).
- Docs updated per plan §F file table.

## Out of scope

Behavior changes to readiness/selection (MB1/MB2); label sync or auto-migration; live-case migration (MB4).

## Comments

### 2026-09-20T19:19:49Z agent/orchestrator

implemented

### 2026-09-20T19:19:49Z agent/orchestrator

MB3 DONE: blocked-label warning + skill/docs teaching; go test ./internal/cli/ ./internal/skill/ pass; review SPEC+QUALITY PASS. Parked non-blocking note: prime Description #15 from MB2 never landed in prime.go; same teaching covered by skill prose — left as future follow-up, not in chain scope.
