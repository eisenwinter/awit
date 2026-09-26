---
id: AWIT-0P26FMSK
title: lazyawit premium visual treatment (theme, borders, sticky errors, sanitization)
brief: >-
  Rounded pane chrome, ANSI-slot palette, render-time toast pairing, ANSI sanitization and sticky error toasts.
status: closed
deps: []
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---
## Summary
Premium visual treatment for lazyawit per the approved design proposal (full text in agent transcript history://TuiVisual): ANSI-named 4-hue palette, rounded-light pane chrome, render-time ok:/error: toasts, new theme.go, F1 ANSI/OSC sanitization at the lazy boundary, F2 sticky error toasts.

## Context
- Proposal: history://TuiVisual (palette slots muted/accent/ok/warn/err; rounded borders, focus by border color; 45/55 joined no-gap; tab bar/footer treatment; theme.go sketch; findings F1/F2).
- Constraints: lipgloss only, no new deps (x/ansi.Strip already vendored); NO_COLOR goldens stay clean (box glyphs remain, colors strip); lipgloss.Width for all measurement; one root Model, no timers/overlays/nested models; toast ok: prefix render-time only (m.toast bytes unchanged).

## Files
- NEW internal/lazy/theme.go; view.go, list.go, model.go, issues/queue/graph/config row construction; goldens regen.

## Steps
- [ ] theme.go: slots, styles, sanitize (x/ansi.Strip at row/detail construction), styleToast/styleRow/styleDetail.
- [ ] Rounded borders on both panes (>=80, joined no-gap, 45/55 outer); single bordered pane <80; focus = border color accent/muted.
- [ ] Tab bar, footer/hints/why/quarantine styling per proposal; ok:/error: render-time toast pairing.
- [ ] F1: strip ANSI/OSC in titles/bodies at lazy boundary. F2: error: toasts sticky until esc/tab-switch/next outcome.
- [ ] RED-first tests (palette fallback plain-text, border geometry, sticky errors, sanitization); regen goldens; pin diff to chrome + markers.
- [ ] Full suite + vet + gofmt clean; PTY check (color + NO_COLOR + exit restore).

## Acceptance Criteria
- Live terminal shows bordered panes, status colors, readable on dark and light themes; NO_COLOR output clean and aligned.
- No color-only signaling; focus unambiguous unstyled.
- Suite green; golden diff contains only chrome/marker lines.

## Comments

### 2026-09-25T20:28:42Z jan

implemented
