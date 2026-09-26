---
id: AWIT-0P1JVJSH
title: "ops: rendering - ShowFull, DefaultView, BrokenView, RefsBaseDir, ValidateText, AliasWarnLines"
brief: >-
  Moves the show --full and validate text renderers (and their private helpers fullView, refBody, sentenceCount) into internal/ops so the CLI and the TUI detail pane print the same bytes from one implementation; show/validate actions call the exported functions.
status: closed
deps: [AWIT-0P1JVJSY, AWIT-0P1JVJSV]
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

`showFull`, `refsBaseDir`, `defaultView`, `brokenView`, `fullView`, `refBody` (show.go) and `validateText`, `aliasWarnLines`, `sentenceCount` (validate.go) move verbatim into `internal/ops/show.go` and `internal/ops/validate.go`. `showOne`, `fullJSON`, `refsOnlyView`, `showJSON`, `showRefJSON`, `validateAction`, `externalWarnLines`, `staleClaimLines`, `now` stay in cli. The parity test `TestShowFullAndValidateTextMatchCLI` and the two `sentenceCount` tests move with the code.

## Context (read first)

- `docs/superpowers/specs/2026-09-25-two-binary-plan.md` §D.2 rows for show.go/validate.go.
- `internal/cli/show.go:110-257` - the movers. `showOne` (41-108) keeps calling `refsBaseDir` (84), `showFull` (103), `defaultView` (106), `brokenView` (67); `refBody` (229-257) calls `defaultView` and `brokenView`; `fullJSON` (259-284) stays and uses `ops.ToEntry` (TB-1) and `resolver` - do not move it.
- `internal/cli/validate.go:43-65` - `sentenceCount`; `122-190` - `validateText`, `externalWarnLines` (stays), `aliasWarnLines`. `validateAction:98-103` uses `externalWarnLines` and `aliasWarnLines` on the json path; `:110` prints `validateText`.
- `internal/cli/import.go:205` - doc comment mentions `sentenceCount`; reword to "(the same boundary rule as internal/ops' sentence counter)". `deriveImportBrief` does not call it.
- `internal/cli/lazy.go` - `Detail` calls `showFull`, `Validate` calls `validateText`; rewrite to `ops.X`.
- Tests that move: `internal/cli/primitives_test.go:102-122` (`TestShowFullAndValidateTextMatchCLI`), `internal/cli/validate_test.go:14-33` (`TestSentenceCount`) and `195-202` (`TestSentenceCountTerminatorsNeedBoundary`; drop its `unicode` sanity line or keep the import).

## Files

- `internal/ops/show.go` - new: `ShowFull`, `RefsBaseDir`, `DefaultView`, `BrokenView`, `fullView`, `refBody`.
- `internal/ops/validate.go` - new: `ValidateText`, `AliasWarnLines`, `sentenceCount`.
- `internal/ops/render_test.go` (`package ops_test`) - moved parity test.
- `internal/ops/validate_test.go` (`package ops`, internal) - the two sentence tests.
- `internal/cli/show.go`, `validate.go`, `lazy.go`, `import.go` - callers rewritten / comment reworded; moved functions deleted.
- `internal/cli/primitives_test.go` - deleted entirely (only `openTestStore` remains; it is still used by `lazy_test.go:23` until TB-5 → move `openTestStore` into `internal/cli/helpers_test.go`).
- `internal/cli/validate_test.go` - two tests deleted; drop `unicode` import.

## Interfaces

```go
// internal/ops/show.go - doc comments verbatim.
func ShowFull(s *item.Store, g *graph.Graph, n *graph.Node) string
func RefsBaseDir(s *item.Store, n *graph.Node) string
func DefaultView(n *graph.Node) string
func BrokenView(id string, broken []item.Broken) string
func fullView(g *graph.Graph, n *graph.Node, baseDir, itemsDir string) string // unexported
func refBody(g *graph.Graph, itemsDir string, r resolver.Resolved) string      // unexported

## Comments

### 2026-09-25T14:53:06Z jan

implemented
```
