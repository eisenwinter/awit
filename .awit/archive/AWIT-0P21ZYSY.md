---
id: AWIT-0P21ZYSY
title: lazyawit polish (behavioral 11-15)
brief: Layout/focus/gate/wrap/isatty improvements; incremental, no rewrites.
status: closed
deps: [AWIT-0P21ZYSS]
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---
## Summary
Design-review polish, behavioral half (proposals 11-15). Incremental rendering/layout changes; no rewrites, architecture preserved.

## Context
- TuiReview P1/P2: footer flap, focus ambiguity, no min-size gate, clipped detail unreachable, raw epoll error on non-TTY.
- Files: internal/lazy/model.go (footerLines :416-428, toast clear :183, body clamp :397-414), view.go (:14-42, :114-136), list.go (:98-119), cmd/lazyawit/main.go (:65-81).

## Files
- internal/lazy/model.go, view.go, list.go, cmd/lazyawit/main.go (+ tests, + goldens)

## Steps
- [ ] Fixed footer slots: always render toast line (blank when empty); Queue always renders why line.
- [ ] cursorList.view takes focused flag: focus==detail renders cursor row plain (no `>`/reverse); add detail scroll cue (`focus: detail 12%` or separator row).
- [ ] View() min-size gate below ~50x12: `lazyawit needs >= 50x12 (have WxH)` only.
- [ ] Wrap detail content on SetContent to detail width via vendored reflow/wordwrap; re-wrap on resize.
- [ ] cmd/lazyawit: isatty(stdin)&&isatty(stdout) check before tea.NewProgram; else stderr `lazyawit requires an interactive terminal`, exit 1.
- [ ] RED-first tests for each (gate output, wrapped detail, focus rendering, stable footer line counts); regen goldens; pin diffs.
- [ ] Full suite + vet + gofmt clean.

## Acceptance Criteria
- Frame line counts stable across toast/tab switches at fixed size.
- Focus unambiguous without color; detail position visible.
- Undersized terminal shows only the gate message; pipes show the designed refusal with exit 1.

## Comments

### 2026-09-25T19:17:27Z jan

implemented
