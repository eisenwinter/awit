---
id: AWIT-0P4XRXDX
title: Extend dogfood coverage for stale .omp skill synchronization
brief: >-
  Prove the real sync command restores a stale .omp skill copy byte-exactly and reports current on re-run.
status: closed
deps: [AWIT-0P4XRXDJ]
labels: []
refs_base: repo
refs: [.awit/comments/AWIT-0P4XRXDX/20260926T210105Z-jan.md]
assignee: agent/orchestrator
---
## Summary

Extend the existing dogfood scenario (`TestDogfoodOmpCopyMatchesRenderer` area, `internal/skill/skill_test.go`) to drive the real `awit skill sync` against a stale `.omp` copy.

## Context (read first)

- `internal/skill/skill_test.go` — existing dogfood flow, fixture isolation, CLI launch convention.
- SK-1's command + `skill.Render` as expected-content source (never a copied skill body).

## Files

- `internal/skill/skill_test.go` (extend) + helpers only as needed.

## Interfaces

- Real `awit skill sync` via the harness; expected bytes from `skill.Render`; actual `.omp` destination path.

## Steps

- [ ] RED: stale the `.omp` copy in-scenario, assert it differs from rendered bytes pre-sync; add post-sync exact-content/status assertions that fail without sync.
- [ ] GREEN: invoke sync, assert `updated` + byte-for-byte equality with current render; invoke again, assert `current` with identical bytes.
- [ ] Focused scenario green; unrelated dogfood coverage preserved. No project-wide validation.

## Acceptance Criteria

- Fixture proven stale pre-command; `updated` repair byte-exact; second run `current`.
- Test stays valid when the embedded source changes (SK-3).
- No duplicate harness, copied skill body, or non-isolated mutation.

## Out of scope

New targets, harness refactoring, benchmarks, project-wide validation.
