---
id: AWIT-0P1XYZSS
title: 'ops: Config/SaveConfig on the lazy.Ops seam — read and write .awit/config.yaml'
brief: >-
  Extends the lazy.Ops interface with Config() (re-read from disk, adopted into the store) and SaveConfig() (Config.Write under the store lock, adopted into the store), implements both on ops.Lazy with tests, and stubs them on the TUI's fakeOps so nothing else changes yet.
status: in_progress
deps: []
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
claimed_at: "2026-09-25T17:35:53Z"
---
## Summary

The Config tab reads and writes `.awit/config.yaml` only through the `Ops` seam. Two methods are added to `internal/lazy.Ops`: `Config() (config.Config, error)` re-reads the file with `config.Load` and adopts the result as `Store.Config` (so a later `Claim` / comment author sees the new `agent_id`); `SaveConfig(c)` takes the store lock, writes `c` via `Config.Write` (temp-then-rename) and adopts `c`. `ops.Lazy` implements them; `fakeOps` in `internal/lazy/lazy_test.go` gets in-memory versions so the package keeps compiling. No TUI behaviour changes; zero golden churn.

## Context (read first)

- `docs/superpowers/specs/2026-09-26-config-tab-plan.md` §C (`Store.Config` consumers), §D.2 (this item).
- `internal/lazy/ops.go:18-37` — `Ops` interface and doc comment.
- `internal/ops/lazy.go:16-31` — `Lazy` struct/doc, `NewLazy`; `:132-171` — `Claim`/`Release` show the lock pattern (`o.s.Lock(5 * time.Second)`, `defer rel()`).
- `pkg/item/store.go:20-24` — `Store{Root, Dir, Config}`; `Dir` is the `.awit` directory.
- `pkg/config/config.go` — `Load(awitDir)`, `(Config).Write(awitDir)`.
- `internal/ops/lazy_test.go:17-26` — `var _ lazy.Ops = (*ops.Lazy)(nil)`, `newLazyOps(t, "clean")` returns `(*ops.Lazy, *item.Store, dir)`; `internal/ops/helpers_test.go` — `copyFixture`, `openTestStore`. Fixture `testdata/fixtures/clean/.awit/config.yaml` is `prefix: AWIT\nstale_claim: 2h\n`.
- `internal/lazy/lazy_test.go:29-124` — `fakeOps` (fields `items, archive, loadErr, fail, calls, external`) and its mutators (`calls` append + `fail`).
- `cmd/lazyawit/main.go:19` — compile-time assertion; it will fail to build until `ops.Lazy` has both methods.

## Files

- `internal/lazy/ops.go` — two interface methods + doc sentence; new import `pkg/config`.
- `internal/ops/lazy.go` — `Config`, `SaveConfig`; `Lazy` doc gains one sentence; new import `pkg/config`.
- `internal/ops/lazy_test.go` — `TestLazyConfigSaveAndReread`.
- `internal/lazy/lazy_test.go` — `fakeOps` fields `cfg config.Config`, `cfgErr error`, `saved []config.Config`; methods `Config`, `SaveConfig`; import `pkg/config`.

## Interfaces

```go
// internal/lazy/ops.go — appended to Ops; doc comment gains:
// The Config tab reads and writes .awit/config.yaml through Config and
// SaveConfig; nothing else in the TUI touches the file.
Config() (config.Config, error)
SaveConfig(c config.Config) error
