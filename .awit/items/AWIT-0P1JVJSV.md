---
id: AWIT-0P1JVJSV
title: 'ops: mutations and author — CloseItem, BlockItem, UnblockItem, ReleaseItem, ClaimItem, RefuseClaim, ResolveAuthor'
brief: >-
  Moves the WI-3 write primitives and author resolution from internal/cli to internal/ops with exported names; CLI actions keep their locks, stdout lines, pushes and commits and call the moved functions, and the primitive tests move with them as external ops tests.
status: closed
deps: [AWIT-0P1JVJSY]
labels: [tui, p1]
refs_base: repo
refs: [.awit/comments/AWIT-0P1JVJSV/20260925T144134Z-jan.md]
assignee: agent/orchestrator
---
## Summary

The "between item loaded and item saved" primitives — `closeItem`, `blockItem`, `unblockItem`, `releaseItem`, `claimItem`, `refuseClaim` — and `resolveAuthor`/`withAgentPrefix` move verbatim into `internal/ops/mutate.go` and `internal/ops/author.go`. `RefuseClaim` and `BlockItem` keep returning `cli.Exit(...)` values (exit 1 / 2) because `internal/cli.report` and the goldens depend on those exact bodies and codes; that is the only urfave use in `internal/ops`. Locks, `noteWalkedUp`, external push and git commit stay in the CLI actions exactly as today.

## Context (read first)

- `docs/superpowers/specs/2026-09-25-two-binary-plan.md` §C (cli.Exit rationale), §D.2 rows for mutate.go/author.go.
- `internal/cli/close.go:66-80` — `closeItem`; caller `closeAction` line 54 (and `resolveAuthor` at line 50).
- `internal/cli/block.go:93-115` — `blockItem`, `unblockItem`; callers lines 61, 86.
- `internal/cli/release.go:57-64` — `releaseItem`; caller line 45.
- `internal/cli/next.go:254-294` — `refuseClaim`, `claimItem`; callers lines 153, 190. `claimItem` calls `withAgentPrefix`.
- `internal/cli/author.go:15-36` — `withAgentPrefix`, `resolveAuthor`; `loadItem` already left in TB-1, so the file becomes empty and is deleted.
- `internal/cli/comment.go:58` — `resolveAuthor` caller.
- `internal/cli/lazy.go` — `lazyOps` methods call all of these; rewrite to `ops.X` (the type itself moves in TB-5).
- Tests that move: `internal/cli/primitives_test.go:21-100` (`TestClaimItemFields`, `TestCloseItemWithAndWithoutReason`, `TestBlockItemRefusesClosed`); `internal/cli/update_test.go:238-285` (`TestResolveAuthorPrecedence`, needs `gitx.UserName`, `config.Config`, `os/exec`). `TestShowFullAndValidateTextMatchCLI` stays in cli until TB-3.

## Files

- `internal/ops/mutate.go` — new: six functions.
- `internal/ops/author.go` — new: `ResolveAuthor`, `WithAgentPrefix`.
- `internal/ops/primitives_test.go`, `internal/ops/author_test.go` — moved tests (`package ops_test`).
- `internal/cli/close.go`, `block.go`, `release.go`, `next.go`, `comment.go`, `lazy.go` — callers rewritten; moved functions deleted.
- `internal/cli/author.go` — deleted.
- `internal/cli/primitives_test.go` — the three moved tests deleted (file keeps `openTestStore` + `TestShowFullAndValidateTextMatchCLI` until TB-3).
- `internal/cli/update_test.go` — `TestResolveAuthorPrecedence` deleted (drop now-unused imports).

## Interfaces

```go
// internal/ops/mutate.go — doc comments verbatim from today's functions.
func CloseItem(s *item.Store, it *item.Item, reason, author string, now time.Time) error
func BlockItem(s *item.Store, it *item.Item, reason string) error   // returns cli.Exit(...,1) closed / cli.Exit(...,2) bad reason
func UnblockItem(s *item.Store, it *item.Item) error
func ReleaseItem(s *item.Store, it *item.Item) error
func ClaimItem(s *item.Store, it *item.Item, agent string, now time.Time) error
func RefuseClaim(n *graph.Node) error                                // returns cli.Exit(...,1) values, messages verbatim
