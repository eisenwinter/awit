---
id: AWIT-0P1JVJSR
title: "ops: Lazy - lazy.Ops implementation and parity tests move out of internal/cli"
brief: >-
  Replaces internal/cli's lazyOps with exported ops.Lazy/NewLazy built on the moved primitives, moves the CLI-vs-Ops parity, byte, refusal and archive tests to internal/ops as external tests, and leaves internal/cli/lazy.go as a thin command until the split lands.
status: closed
deps: [AWIT-0P1JVJSV, AWIT-0P1JVJSH, AWIT-0P1JVJSS]
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

`lazyOps` (lazy.go:38-208) becomes `ops.Lazy` with constructor `ops.NewLazy(s, agent, now)`. Exported (deviation from the spec's "unexported" wording): `cmd/lazyawit` is another package and must construct it; the fields stay unexported. `ops` does not import `internal/lazy`, so the `var _ lazy.Ops = (*ops.Lazy)(nil)` assertion lives in the test file now and in `cmd/lazyawit/main.go` from TB-6. Every refusal string is byte-identical. `internal/cli/lazy.go` shrinks to `lazyHumanCmd` + `lazyHumanAction` calling `ops.NewLazy`; `awit lazy-human` still works after this item (TB-7 removes it).

## Context (read first)

- `docs/superpowers/specs/2026-09-25-two-binary-plan.md` §C (deviation), §D.2 Lazy row, §I (package-name shadowing).
- `internal/cli/lazy.go:38-208` - `lazyOps` and its 14 methods. After TB-2..4 every body already calls `ops.X`; this item only relocates the type.
- `internal/lazy/ops.go:22-37` - the `Ops` interface (untouched).
- `internal/cli/lazy_test.go` - the moving tests: `newLazyOps` (20-24), `idsOf`, `idsFromCompact`, `TestLazyListParity`, `TestLazyDetailOverviewQueueParity`, `TestLazyOpsBytesMatchCLI`, `stripLine`, `TestLazyOpsRefusalsMatchCLI`, `TestLazyArchiveOps`. `TestLazyHumanRegisteredAndQuits` (229-248) stays in cli until TB-7.
- Test rewrites while moving: `o.s.ItemPath(...)` / `o.s.Comments(...)` / `o.s.LoadArchive` → the store returned by `newLazyOps`; `o.agent = ""` (line 201) → `o = ops.NewLazy(s, "", fixedNow)`; the local `ops := []op{…}` in `TestLazyOpsBytesMatchCLI` (line 109) must be renamed `cases` - it would shadow the package.
- `internal/ops/helpers_test.go` (TB-1) already provides `run`, `copyFixture`, `openTestStore`, `readItem`.

## Files

- `internal/ops/lazy.go` - new: `Lazy`, `NewLazy`, 14 methods.
- `internal/ops/lazy_test.go` (`package ops_test`) - moved tests.
- `internal/cli/lazy.go` - only `lazyHumanCmd` + `lazyHumanAction` remain (imports: `context`, `time`, bubbletea, `internal/lazy`, `internal/ops`, urfave).
- `internal/cli/lazy_test.go` - only `TestLazyHumanRegisteredAndQuits` remains.
- `internal/lazy/ops.go` - doc comments: "implemented by internal/cli" → "implemented by internal/ops (`ops.Lazy`) with the same functions the CLI commands call"; package doc "awit lazy-human TUI" → "lazyawit TUI".

## Interfaces

```go
// internal/ops/lazy.go
// Lazy implements the TUI's Ops seam (internal/lazy.Ops) over a store with
// the same functions the CLI commands call, so every row, detail and byte
// written matches the corresponding command. Every mutation takes the
// store lock for 5s like the CLI actions do. It never pushes external state
// and never git-commits.
type Lazy struct {
	s     *item.Store
	agent string
	now   func() time.Time
}
// NewLazy binds s, the --agent value (resolved through s.Config.Agent at
// claim time) and a clock (time.Now in production, fixed in tests).
func NewLazy(s *item.Store, agent string, now func() time.Time) *Lazy
func (o *Lazy) Load() (*graph.Graph, error)
func (o *Lazy) LoadArchive() ([]*item.Item, error)
func (o *Lazy) Line(n *graph.Node) string
func (o *Lazy) ArchiveLine(it *item.Item) string
func (o *Lazy) Detail(g *graph.Graph, id string) string
func (o *Lazy) ArchiveDetail(id string) (string, error)
func (o *Lazy) Close(id, reason string) error
func (o *Lazy) Block(id, reason string) error
func (o *Lazy) Unblock(id string) error
func (o *Lazy) Comment(id, text string) error
func (o *Lazy) Claim(id string) error
func (o *Lazy) Release(id string) error
func (o *Lazy) Validate(g *graph.Graph) string
func (o *Lazy) ExternalCheck(ctx context.Context, id string) string

## Comments

### 2026-09-25T15:07:19Z jan

implemented
```
