---
id: AWIT-0NZJPBSH
title: "cli: extract closeItem/blockItem/unblockItem/releaseItem/claimItem/showFull/validateText"
brief: >-
  The setter-and-save sequences of close, block, unblock, release and next --claim, plus the show --full and validate text renderers, become plain functions so the TUI can call the same code the CLI actions run.
status: closed
deps: []
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

Pure refactor. Each CLI action keeps its flag parsing, store open, `noteWalkedUp`, lock, stdout line, external push and git commit. Only the part between "item loaded" and "item saved" moves into a function of `(*item.Store, *item.Item, …)`; `show --full` and `validate` text rendering become string-returning functions. The lazy-human TUI (WI-9) calls these seven functions; nothing else changes, and every existing golden and test must stay byte-identical.

## Context (read first)

- `docs/design-spec.md` §5 rows `close`, `release`, `block`, `unblock`, `next`, `show`, `validate`.
- `internal/cli/close.go:47-62` - close sequence: `SetStatus(closed)`, `SetClaimedAt(nil)`, `SetBlockedReason("")`, then `AddComment` when a reason is given (AddComment saves), else `Save`.
- `internal/cli/block.go:61-72` - closed refusal (`cli.Exit(..., 1)`), `SetBlockedReason` (invalid → `cli.Exit(..., 2)`), `SetStatus(open)`, `SetAssignee("")`, `SetClaimedAt(nil)`, `Save`.
- `internal/cli/block.go:95-100` - unblock: `SetBlockedReason("")`, `Save`.
- `internal/cli/release.go:45-50` - release: `SetStatus(open)`, `SetAssignee("")`, `SetClaimedAt(nil)`, `Save`.
- `internal/cli/next.go:181-197` - claim: agent from `s.Config.Agent(flag)`, `now.UTC().Truncate(time.Second)`, `SetStatus(in_progress)`, `SetAssignee(withAgentPrefix(agent))`, `SetClaimedAt(&now)`, `Save`. The identity check stays in `nextAction`.
- `internal/cli/show.go:83-87,105-108` - baseDir selection + `fullView`.
- `internal/cli/validate.go:112-133` - text report (status line, faults with fix, WARN lines, alias warnings). Stale-claim lines (134-138) and the exit code stay in `validateAction`.
- `internal/cli/helpers_test.go` - `initRepo`, `run`, `readItem`, `copyFixture`.

## Files

- `internal/cli/close.go`, `block.go`, `release.go`, `next.go`, `show.go`, `validate.go` - extraction.
- `internal/cli/primitives_test.go` - new.

## Interfaces

```go
func closeItem(s *item.Store, it *item.Item, reason, author string, now time.Time) error
func blockItem(s *item.Store, it *item.Item, reason string) error   // same cli.Exit errors as blockAction
func unblockItem(s *item.Store, it *item.Item) error
func releaseItem(s *item.Store, it *item.Item) error
func claimItem(s *item.Store, it *item.Item, agent string, now time.Time) error // applies withAgentPrefix and Truncate(time.Second)
func showFull(s *item.Store, g *graph.Graph, n *graph.Node) string   // == show <id> --full text
func validateText(g *graph.Graph) string                             // == validate text output minus --stale-claims lines

## Comments

### 2026-09-24T21:17:42Z jan

implemented
```
