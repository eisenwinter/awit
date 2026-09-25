---
id: AWIT-0P21ZYSM
title: 'lazyawit: async loads with loading status (deferred)'
brief: 'Optional design-review follow-up: tea.Cmd loads + loading line.'
status: in_progress
deps: []
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
claimed_at: "2026-09-25T18:57:23Z"
---
## Summary
Design-review proposal 16 (deferred, optional): move LoadArchive and reload's Load into tea.Cmds with a `loading…` status line, mirroring externalCmd.

## Context
- TuiReview P2: synchronous file I/O in Update freezes input for the load duration (issues.go:67-96 toggleArchive, model.go:336-357 reload). Harmless on tiny repos, visible on big ones. Pattern already established (mutations.go:42-46 externalCmd).

## Files
- internal/lazy/issues.go, model.go (+ tests)

## Steps
- [ ] Archive toggle dispatches tea.Cmd; result message swaps rows; status line shows `loading…` meanwhile.
- [ ] reload()'s Load goes through a tea.Cmd with the same treatment; errors keep old graph + toast (existing semantics).
- [ ] Initial New() shows first frame immediately with loading status instead of blocking (if feasible without restructuring init flow; otherwise document and keep blocking start).
- [ ] RED-first interaction tests (headless key-sequence → loading → rows); goldens for the loading line.

## Acceptance Criteria
- Input never blocks on file I/O; loading state visible; error semantics unchanged.
- Full suite green; no golden churn outside the loading line.
