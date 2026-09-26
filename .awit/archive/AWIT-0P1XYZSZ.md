---
id: AWIT-0P1XYZSZ
title: "lazy: Config tab shell - tab 4, rows, detail, header/help/hints, reload from disk"
brief: >-
  Adds tabConfig with one row per config.yaml key (schema order, "(unset)" for empties), per-key rule/default/current detail text, the 4 and e bindings, header "[4] Config", help and hint rows, config loaded in New and re-read in reload; mutation keys are inert on the tab. The only ticket that regenerates existing goldens - header line and two help rows, nothing else.
status: closed
deps: [AWIT-0P1XYZSP, AWIT-0P1XYZSS]
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

Read-only half of the Config tab. `4` switches to a list of the eight `config.yaml` keys rendered from the config read through `Ops.Config()`; the detail pane shows the key's rule (from `docs/schema.md`), default, current value and the save note. `New` loads the config (error → fatal screen, like the graph); `reload()` re-reads it so `R` and every post-mutation reload reflect the file. `c/b/u/m/space/r/P/V/o//` do nothing on the tab. The `e` binding exists (keymap + help + hints) but is wired in CB-4. The header gains `[4] Config` and the help text two rows, so every existing frame golden changes on exactly its first line and `frame-help.golden` on exactly those two rows - regenerate with `-update` and verify the diff by eye; nothing else may differ.

## Context (read first)

- `docs/superpowers/specs/2026-09-26-config-tab-plan.md` §D.3 (row/detail texts verbatim, wiring list), §G, §H.1 (golden decision).
- `internal/lazy/model.go:15-21` (`tab` consts), `:47-72` (`queueState`, `Model`), `:79-108` (`New`), `:135-257` (`updateKey`: `Tab1/2/3` cases 179-189), `:272-287` (`rebuildRows`), `:289-304` (`refreshDetail`), `:308-324` (`reload`), `:337-356` (`selectedID`, `activeList`), `:359-376` (`layout` list loop).
- `internal/lazy/mutations.go:92-122` - `mutationKey` (add the tab guard as its first statement).
- `internal/lazy/view.go:44-72` (`headerView` tabs slice), `:76-92` (`tabHeaderView`), `:147-174` (`helpView` rows 151 and 156), `:176-185` (`hintsView`).
- `internal/lazy/keys.go` - `keymap` struct + table; `internal/lazy/keys_test.go:10-63` (binding table) and `:65-83` (`TestKeymapTabs`).
- `internal/lazy/queue.go` - the smallest tab (rows function + model helpers) to mirror; `internal/lazy/issues.go:5-10` - per-tab state struct shape.
- `internal/lazy/list.go` - `row`, `cursorList.setRows/selected/view`.
- `internal/lazy/lazy_test.go` - `fakeOps` (now with `cfg/cfgErr/saved`, CB-2), `newFixture` (135-162), `newModel`, `press`, `golden`, `countCalls`; `view_test.go:12-24` - fatal pattern.
- `pkg/config` - `Config`, `Duration.String` (CB-1).
- `docs/schema.md:49-58` - source of the rule sentences.

## Files

- `internal/lazy/config.go` - new: `configState`, `configKeys`, `configValue`, `configRows`, `configDetail`.
- `internal/lazy/model.go` - `tabConfig`; `Model.config`; `New`, `updateKey` (`Tab4`), `rebuildRows`, `refreshDetail`, `reload`, `selectedID`, `activeList`, `layout`.
- `internal/lazy/mutations.go` - `mutationKey` guard.
- `internal/lazy/view.go` - header tabs slice, `tabHeaderView`, `helpView`, `hintsView`.
- `internal/lazy/keys.go` - `Tab4`, `Edit`.
- `internal/lazy/config_test.go` - new tests; `keys_test.go` - two table rows + `TestKeymapTabs` step; `lazy_test.go` - `newFixture` sets `cfg`.
- `internal/lazy/testdata/frame-config.golden` - new; every other frame golden regenerated (header line); `frame-help.golden` regenerated (two rows).

## Interfaces

```go
// internal/lazy/keys.go
Tab4: key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "config tab")),
Edit: key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
// keymap struct: add Tab4 after Tab3 and Edit after Enter; doc comment: "4 opens config, e edits a config value".

## Comments

### 2026-09-25T17:56:08Z jan

implemented
```
