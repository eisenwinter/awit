---
id: AWIT-0P2890SR
title: "lazyawit: coherent Work-items wording and top-bar spacing"
brief: >-
  Rename the Issues tab to Work items in all user-facing strings and widen top-bar spacing.
status: closed
deps: []
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

Coherent work-item wording in lazyawit: the first tab and all user-facing strings say "Issues"; the tool's unit is the work item. Rename to "Work items" everywhere users read it, and widen top-bar tab spacing.

## Context

- Tab bar (view.go header), `?` help rows, per-tab hint lines referencing the Issues tab (e.g. graph jump hint), empty-pane placeholder for the tab.
- Goldens embed `[1] Issues` - regen and pin diff to the rename + spacing only.

## Files

- internal/lazy/view.go, keys.go, issues.go, graph.go (+ tests, + goldens); docs touch-ups where user-facing docs name the "Issues tab" (README, usage.md, design-spec lazyawit paragraph).

## Steps

- [ ] Rename display label `[1] Issues` -> `[1] Work items` (key `1` unchanged); adjust help rows, hint lines, placeholder, graph-jump hint to match.
- [ ] Widen spacing between top-bar tab options (single source, e.g. two spaces / padded cells); keep total within width guard.
- [ ] Keep internal identifiers (tabIssues, file names) - display strings only, unless a rename is trivially safe.
- [ ] Regen goldens; assert diff is rename + spacing only. Full suite + vet + gofmt clean.

## Acceptance Criteria

- No user-visible "Issues" remains in the TUI (grep); help/hints/placeholder coherent.
- Top bar breathes; 80-col layout still fits.
- Suite green; golden diff pinned.

## Comments

### 2026-09-25T20:38:33Z jan

implemented
