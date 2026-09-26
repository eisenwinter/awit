---
id: AWIT-0P21ZYSS
title: lazyawit polish (cosmetic 1-10 + brand)
brief: Design-review cosmetic polish plus lazyawit brand rename; golden-text-only.
status: closed
deps: []
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

Design-review polish, cosmetic half (proposals 1-10) plus the approved brand rename. All changes are golden-text-only; architecture untouched (one root Model, cursorList, toast-until-next-key, NO_COLOR discipline).

## Context

- TuiReview findings P0/P1/P2 (agent transcript history://TuiReview): hints 102>100 wrap, bold-only active tab invisible under NO_COLOR, `enter pin` mislabel, blank `Focused on `, hard-cut truncation, rune-based width math, blank empty-issues pane, help overflow below 30 rows.
- Files: internal/lazy/view.go (hints :186, help :150-178, header :56-72, sub-headers :80-82, quarantine :24), list.go (truncateRunes :121-129, cursor :113-115), keys.go Enter help, queue.go why :38, config detail goldens.

## Files

- internal/lazy/view.go, list.go, keys.go (+ tests, + testdata/\*.golden regen)

## Steps

- [ ] Relabel issues hint `enter pin` -> `enter detail`; keys.go Enter help `select` -> `detail`.
- [ ] Ascii-safe active-tab marker (e.g. `>` prefix or `(1)` style) alongside bold in view.go:52-64; goldens stay ESC-free.
- [ ] Failure toasts prefixed `error: ` (act() model.go:359-368, refusals mutations.go/queue.go); `1 items` -> `1 item(s)` (view.go:24).
- [ ] Graph header with empty root: `Focused (no root - select an item on Issues)` (view.go:80-82).
- [ ] truncateRunes adds `…` ellipsis (cut n-1 cells + `…`).
- [ ] Width math len([]rune) -> lipgloss.Width (already vendored) in list.go:121, view.go:139-144, view.go:70.
- [ ] View() truncates every emitted line to m.width cells (ellipsis content, hard cut hints) (view.go:14-42).
- [ ] Shorten per-tab hints to <= ~78 cells (view.go:180-191).
- [ ] Empty issues source -> unselectable placeholder (`No open items` / `Archive is empty`), mirroring queueRows.
- [ ] Cap helpView to bodyHeight() lines.
- [ ] Brand: header `awit lazy-human` -> `lazyawit` (view.go:56).
- [ ] Regen goldens with -update; assert full golden diff is ONLY the intended lines (header/ellipsis/hints/help/placeholder/brand). RED: new placeholder/ellipsis tests fail first.
- [ ] `go test ./internal/lazy`, gofmt, vet clean.

## Acceptance Criteria

- Hints/help/header/why fit their width at 60x20 and 100x30 (rendered frames asserted in tests).
- Active tab + focus identifiable with zero SGR (NO_COLOR PTY check).
- CJK row keeps `│` aligned (cell math test).
- No behavioral change: key handling, reload, save paths untouched.

## Comments

### 2026-09-25T18:57:13Z jan

implemented
