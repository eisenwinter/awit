---
id: AWIT-0NZJPBSQ
title: 'lazy: Queue tab — Ready() rows, why line, space/r via Ops'
brief: >-
  The Queue tab lists g.Ready() in its deterministic prime order, shows the next --why explanation for the highlighted row, and claims or releases the selection through Ops with a toast and synchronous reload.
status: closed
deps: [AWIT-0NZJPBSM]
labels: [tui, p1]
refs_base: repo
refs: [.awit/comments/AWIT-0NZJPBSQ/20260924T205501Z-jan.md]
assignee: agent/orchestrator
---
## Summary

`g.Ready()` is already ready-minus-quarantined sorted UnblockCount desc, ID asc — the exact `prime` READY order and what `next` ranks before its PCG tie-break. The Queue tab renders those rows with `Ops.Line`, pins the shared detail viewport to the selection, prints a `why:` footer line built like `next --why` (with `tie-break=none` always, since the TUI never randomises), and maps `space` → `Ops.Claim`, `r` → `Ops.Release` through `act` (toast + reload). Refusals (held, claimed, unresolved identity) arrive as errors from `Ops` and become toasts; selection is kept.

## Context (read first)

- Design spec "Queue tab"; plan §D.11.
- `internal/cli/next.go:139-179,211-228` — `why:` line format `why: <id>; unblocks=<n>; critical-path=yes|no; selection=<s>; tie-break=<t>`; `selection` is `max-unblocks` for a ranked pick. In the TUI: `max-unblocks` when the row shares the top UnblockCount, otherwise `ranked`; `tie-break=none`.
- `pkg/graph` — `Ready()`, `CriticalPath()`, `UnblockCount`.
- WI-4: `act`, `reload`, `onCritical`, `press`, `golden`, `newFixture` (ready: L1 U2, L2 U0, L6 U0 in_progress).

## Files

- `internal/lazy/queue.go` — `queueState`, `queueRows`, `whyLine`, key handling for `space`/`r`.
- `internal/lazy/model.go`, `view.go` — Queue wiring; header line 2 `ready: N`; footer gains the `why:` line on the Queue tab; hint line `j/k move  space claim  r release  c close  b block  u unblock  m comment  P check  ? help`.
- `internal/lazy/queue_test.go`; goldens `queue`, `queue_after_claim`.

## Interfaces

```go
type queueState struct{ list cursorList }
func queueRows(g *graph.Graph, line func(*graph.Node) string) []row // over g.Ready(); all selectable
func whyLine(g *graph.Graph, n *graph.Node, onCritical bool) string
// "why: <id>; unblocks=<n>; critical-path=yes|no; selection=max-unblocks|ranked; tie-break=none"
