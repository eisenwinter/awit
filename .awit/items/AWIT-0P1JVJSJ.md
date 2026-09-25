---
id: AWIT-0P1JVJSJ
title: 'awit: drop lazy-human, add dependency-graph gate, document lazyawit'
brief: >-
  Removes the lazy-human command and internal/cli/lazy.go so cmd/awit no longer links internal/lazy or the charm modules, adds the go list -deps test that keeps it that way, and updates design-spec, README and usage.md for the two-binary layout.
status: closed
deps: [AWIT-0P1JVJSQ]
labels: [tui, p1]
refs_base: repo
refs: [.awit/comments/AWIT-0P1JVJSJ/20260925T153739Z-jan.md]
assignee: agent/orchestrator
---
## Summary

The final cut: `lazyHumanCmd` leaves `newRoot`, `internal/cli/lazy.go` and `lazy_test.go` are deleted, and `awit lazy-human` becomes `unknown command "lazy-human" (run "awit --help")` with exit 2. A test in `cmd/awit` runs `go list -deps .` and fails on any `charmbracelet` or `/internal/lazy` line — the machine-checked rule that agents holding only `awit` cannot open a TUI. Docs move the TUI from the awit command tables to a `lazyawit` binary description.

## Context (read first)

- `docs/superpowers/specs/2026-09-25-two-binary-design.md` — Goal, Acceptance 1, Docs bullet.
- `docs/superpowers/specs/2026-09-25-two-binary-plan.md` §F (doc plan, file-by-file), §G (final acceptance), §I (staticcheck U1000, Windows CI).
- `internal/cli/app.go:129-151` — `Commands` slice (`lazyHumanCmd` at line 142); `:202-208` `rootAction` produces the unknown-command message; `app_test.go:105-116` `TestMainUnknownCommand` is the shape for the new test.
- `internal/cli/lazy.go`, `internal/cli/lazy_test.go` — deleted whole.
- `cmd/awit/main.go` — unchanged; the gate test lives beside it.
- `go list -deps ./cmd/awit` today prints `github.com/charmbracelet/…` and `github.com/eisenwinter/awit/internal/lazy` lines (that is what must disappear); it prints `github.com/eisenwinter/awit/internal/ops` after TB-1 (sanity anchor for the test).
- Docs: `docs/design-spec.md:194` (`lazy-human` row), `:196` (Global flags), `:257-278` (§7 map incl. `internal/lazy` line 266 and closing sentence 278); `README.md:25-33` (install from source), `:77` (row), `:79-85` (global flags paragraph); `docs/usage.md:26-34` (install from source), `:160-168` (Browsing interactively intro), `:215` (row). `site/` is generated — untouched. `.github/workflows/*` need no change.
- Ensure nothing becomes unused after the deletion: `graphItems`, `externalWarnLines`, `staleClaimLines`, `now` remain used by cli actions.

## Files

- `internal/cli/app.go` — remove `lazyHumanCmd` from `Commands`.
- `internal/cli/lazy.go`, `internal/cli/lazy_test.go` — deleted.
- `internal/cli/app_test.go` — `TestLazyHumanIsUnknown`.
- `cmd/awit/main_test.go` — new: `TestNoTUIInDependencyGraph`.
- `docs/design-spec.md`, `README.md`, `docs/usage.md` — per §F of the plan (exact text below).

## Interfaces

```go
// cmd/awit/main_test.go (package main)
// TestNoTUIInDependencyGraph is the machine-checked rule behind the two-binary
// split: the classic binary must never link the TUI. It shells out to
// `go list -deps .` and fails on any charmbracelet or internal/lazy package.
func TestNoTUIInDependencyGraph(t *testing.T)
