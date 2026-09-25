---
id: AWIT-0P1XYZSK
title: 'lazy: Config tab editing — prefilled prompt, validation mirroring Load, save through Ops'
brief: >-
  Wires e/enter on the Config tab to the existing prompt prefilled with the current value; enter parses the field (prefix grammar, duration, bools, comma lists), runs config.Normalize, and on success saves the whole config through Ops.SaveConfig with toast "saved <key>"; any failure toasts the Load/init message, writes nothing and keeps the selection.
status: in_progress
deps: [AWIT-0P1XYZSZ]
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
claimed_at: "2026-09-25T17:56:19Z"
---
## Summary

Second half of the Config tab. On `tabConfig`, `e` or `enter` opens the shared `textinput` prompt in a new `inputConfig` kind, prefilled with `configValue` of the selected key. `esc` discards (existing branch). `enter` runs `setConfigField` (per-key parse) then `Normalize`; a failure is a toast and nothing is written; success calls `act(func() error { return ops.SaveConfig(next) }, "saved "+key)` so rows re-render from the re-read file and the cursor stays on the key. One edit path for all eight keys; no draft state, no new mode.

## Context (read first)

- `docs/superpowers/specs/2026-09-26-config-tab-plan.md` §D.4 (flow, exact error strings, edge cases), §E (why per-field save).
- `internal/lazy/model.go:39-45` (`inputKind`), `:149-170` (modeInput branch: `esc` → Blur+Reset; `enter` → `submitInput`), `:208-217` (Claim/Release cases — insert the config case after them), `:228-234` (`Enter` case keeps its meaning elsewhere).
- `internal/lazy/mutations.go:10-37` — `openInput` (uses `Reset`; the config edit needs `SetValue`, so a sibling helper) and `submitInput` switch.
- `internal/lazy/view.go:187-203` — `promptView` (label + ` [target]` suffix; config uses `<key>: ` with no suffix).
- `internal/lazy/config.go` (CB-3) — `configValue`, `configRows`; `internal/lazy/config_test.go` — `configRowTexts`.
- `pkg/config` — `ValidPrefix`, `(Config).Normalize`, `Duration` (CB-1). `internal/cli/create.go:69-80` `parseIDList` is the comma-split rule to mirror (split, trim, drop empties) — copy the rule, not the function.
- `internal/lazy/lazy_test.go` — `fakeOps.SaveConfig` records `"SaveConfig"` in `calls`, honours `fail`, appends to `saved` (CB-2); `testdata/prompt_block.golden` last line shows the prompt shape.
- bubbles `textinput`: `SetValue`, `CursorEnd`, `Value`, `Focus() tea.Cmd`.

## Files

- `internal/lazy/config.go` — `setConfigField`, `splitList`, `(*Model).beginConfigEdit`, `(*Model).saveConfigField`.
- `internal/lazy/model.go` — `inputConfig` const; `updateKey` config edit case.
- `internal/lazy/mutations.go` — `submitInput` case `inputConfig`.
- `internal/lazy/view.go` — `promptView` case `inputConfig`.
- `internal/lazy/config_test.go` — tests below; `internal/lazy/testdata/prompt_config.golden` — new.

## Interfaces

```go
// internal/lazy/model.go
const ( inputClose inputKind = iota; inputBlock; inputComment; inputConfig )
