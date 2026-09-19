---
id: AWIT-0NE5JZDX
title: Validate trips on conflict markers quoted in ticket bodies
brief: >-
  awit validate fails on our own queue because a closed ticket quotes load-bearing conflict-marker fixture bytes; decide fence-aware detection or ticket-text convention.
status: closed
deps: []
labels: [phase5, p2]
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

Dogfood find: `bin/awit validate` on this repo reports `FAIL ... [CONFLICT MARKERS]` for `.awit/items/AWIT-0ND56N3G.md`, because that closed ticket's body quotes the load-bearing `<<<<<<< HEAD` / `=======` / `>>>>>>> branch-b` bytes of the `conflicted` test fixture (ticket lines ~465-476). `HasConflictMarkers` (`pkg/item/frontmatter.go`) trims space then matches, so fenced/indented quotes trip it. The queue is healthy; the detector cannot tell quoted bytes from a real conflict.

Decide one: (a) make the detector fence-aware (ignore markers inside fenced code blocks) — changes quarantine semantics, needs guide §4-adjacent design care; or (b) establish a ticket-text convention (e.g. break markers with zero-width/interpolated text when quoting fixtures) and scrub 56N3G's body. Either way, `awit validate` on this repo must end PASS.

## Acceptance Criteria

- [ ] `bin/awit validate` on this repo exits 0 with no FAIL lines
- [ ] `go test ./pkg/item ./internal/cli` still pass (no quarantine regression)

## Comments

### 2026-09-18T06:45:10Z agent/orchestrator

implemented
