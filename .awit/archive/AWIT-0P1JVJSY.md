---
id: AWIT-0P1JVJSY
title: 'ops: new package — Open, WalkedUpNote, LoadGraph, LoadItem, ResolveItemID, ToEntry, ArchiveEntry, Version'
brief: >-
  Creates internal/ops with the store-open precedence, graph/item loaders, key resolution and entry converters that every CLI command and the future lazyawit main share; internal/cli delegates to them so the classic binary's output stays byte-identical.
status: closed
deps: []
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---
## Summary

First slice of the service layer. `openStore`, `noteWalkedUp`, `toEntry`, `resolveItemID`, `parseExternalKey`, `errUnknownItem` (app.go), `loadGraph` (validate.go), `loadItem` (author.go) and `archiveEntry` (lazy.go) move to a new package `internal/ops` under exported names. `internal/cli` keeps `openStore(cmd)` and `noteWalkedUp(cmd, s)` as two-line delegates (every command keeps calling them) and rewrites every other caller to `ops.X`. `ops.Version` is added now so the ldflag symbol exists before `cmd/lazyawit` does. No behaviour changes; every golden stays green.

## Context (read first)

- `docs/superpowers/specs/2026-09-25-two-binary-design.md` — Interfaces, Rules ("`internal/ops` never pushes, never commits"), Version stamps.
- `docs/superpowers/specs/2026-09-25-two-binary-plan.md` §D.1 (package rules), §D.2 (move table), §I (risks).
- `internal/cli/app.go:155-200` — `openStore`, `noteWalkedUp`: precedence `--repo` flag → `AWIT_REPO` → `item.Find(cwd)`; the note string at line 199 must stay byte-identical (`internal/cli/repo_note_test.go:10-11` pins prefix/suffix).
- `internal/cli/app.go:246-361` — `errUnknownItem`, `graphItems` (stays in cli), `resolveItemID`, `parseExternalKey`, `toEntry`.
- `internal/cli/validate.go:35-41` — `loadGraph`.
- `internal/cli/author.go:38-71` — `loadItem` (uses `resolveItemID`, `errUnknownItem`, `id.Valid`, `item.BrokenError`).
- `internal/cli/lazy.go:210-234` — `archiveEntry`.
- `internal/cli/helpers_test.go` — `run`, `repoRoot`, `copyFixture`, `readItem`, `initRepo`; `internal/cli/primitives_test.go:12-19` — `openTestStore`. These are `package cli` test helpers and cannot be imported: copy them into `internal/ops/helpers_test.go` (`package ops_test`).
- `internal/cli/init_test.go:103-123` — `TestOpenStoreWithoutRepo` (stays; still exercises the delegate through a `*cli.Command`).
- Call sites to rewrite (from `grep`): `loadGraph` — archive.go:32, dep.go:55,99,134, list.go:37, next.go:134, prime.go:32, show.go:46, validate.go:72; `loadItem` — block.go:57,82, close.go:43, comment.go:54, ref.go:81,128, release.go:41, update.go:93; `resolveItemID` — create.go:202, dep.go:62,66,106,110, external.go:68,195, list.go:61, next.go:234, show.go:55; `errUnknownItem` — create.go:204, next.go:238; `toEntry` — dep.go:142, list.go:88, next.go:204, show.go:79,128,260.

## Files

- `internal/ops/ops.go` — new: package doc only.
- `internal/ops/version.go` — new: `Version`.
- `internal/ops/store.go` — new: `Open`, `WalkedUpNote`.
- `internal/ops/graph.go` — new: `LoadGraph`, `LoadItem`, `ResolveItemID`, `ErrUnknownItem`, `parseExternalKey`.
- `internal/ops/entry.go` — new: `ToEntry`, `ArchiveEntry`.
- `internal/ops/helpers_test.go`, `internal/ops/store_test.go`, `internal/ops/graph_test.go` — new tests.
- `internal/cli/app.go` — `openStore`/`noteWalkedUp` become delegates; `toEntry`, `resolveItemID`, `parseExternalKey`, `errUnknownItem` deleted; package doc gains one sentence.
- `internal/cli/validate.go`, `author.go`, `lazy.go` — moved functions deleted.
- `internal/cli/{archive,block,close,comment,create,dep,external,list,next,prime,ref,release,show,update,validate}.go` — callers rewritten.

## Interfaces

```go
// internal/ops/ops.go
// Package ops is the store-level service layer shared by the awit CLI
// (internal/cli) and the lazyawit TUI (cmd/lazyawit). It opens the store,
// loads graphs and items, mutates items and renders the show/validate
// texts. It never pushes external state, never git-commits, and never
// imports internal/cli or internal/lazy: internal/cli delegates here, and
// cmd/awit's dependency graph must stay free of TUI code.
package ops

## Comments

### 2026-09-25T14:34:26Z jan

implemented
