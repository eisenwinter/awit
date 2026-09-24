---
id: AWIT-0NZJPBSZ
title: 'lazy: Graph tab — prime overview rows, focused tree (depth 5, +N more), enter jump'
brief: >-
  The Graph tab shows awit prime's READY/BLOCKED/CRITICAL PATH text verbatim with selectable node rows, and a focused ASCII tree of deps above and unblocks below the selected id capped at depth 5, with tab toggling modes and enter jumping to the Issues tab.
status: in_progress
deps: [AWIT-0NZJPBSM]
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
claimed_at: "2026-09-24T20:55:14Z"
---
## Summary

Two render modes over the shared graph snapshot. Overview = `prime.Render(&buf, g, prime.Options{})` split into lines; a line becomes a selectable row with id X when it starts with `[X]` and `g.Nodes[X]` exists (the §6 output contract) — byte-identical to `awit prime` apart from the two-column cursor gutter. Focused = the selected id's upstream `Deps` tree and downstream `Unblocks` tree, children sorted by ID, each direction capped at depth 5 with `(+N more)` stubs. `tab` toggles modes, `enter` jumps to the Issues tab pinned on the row's id. The Graph tab has no detail pane; the tree/overview fills the body.

## Context (read first)

- Design spec "Graph tab"; plan §D.10.
- `docs/design-spec.md` §6 "`awit prime` Output Contract" — every node row starts with `[<ID>]`; CRITICAL PATH is one `A -> B -> C` line (not selectable).
- `pkg/prime` — `Render(w io.Writer, g *graph.Graph, opts Options) error`, `Options{MaxTokens, Labels}`.
- `pkg/graph` — `Node.Deps`, `Node.Unblocks` (both sorted by ID in `Build`), `Node.Quarantined()`.
- WI-4: `cursorList`, `row`, `press`, `golden`, `newFixture`.

## Files

- `internal/lazy/graph.go` — `graphState`, `overviewRows`, `treeRows`, `focusedRows`, key handling (`tab`, `enter`).
- `internal/lazy/model.go`, `view.go` — Graph tab wiring; on entering the tab (`2`) `rootID` = current Issues selection when non-empty; header line 2 = `Overview` | `Focused on <id>`; hint line `j/k move  tab overview/focused  enter open in issues  c close  b block  u unblock  m comment  ? help`; body = list only, full width.
- `internal/lazy/graph_test.go`; goldens `graph_overview`, `graph_focused`, `tree_depth_cap`.

## Interfaces

```go
type graphState struct {
	focused bool
	rootID  string
	list    cursorList
}
const treeDepth = 5
func overviewRows(g *graph.Graph) []row // prime.Render into a buffer; lines = strings.Split(strings.TrimSuffix(out, "\n"), "\n") (none when out == ""); selectable iff line starts with "[X]" and g.Nodes[X] != nil
// treeRows renders root at depth 0 then next(n) recursively; child order ID
// asc; prefixes "├─ ", "└─ ", "│  ", "   "; a node at depth treeDepth with k
// children emits one unselectable "(+k more)" child row instead of recursing;
// nodes already on the current path are skipped (quarantined cycle guard).
func treeRows(root *graph.Node, next func(*graph.Node) []*graph.Node) []row
func focusedRows(g *graph.Graph, rootID string) []row
// = header "depends on (upstream, depth <= 5)", treeRows(root, deps), "", header "unblocks (downstream, depth <= 5)", treeRows(root, unblocks)
// rootID missing → single unselectable row "select an item on the Issues tab first"
func nodeText(n *graph.Node) string // "[ID] status title" + " [quarantined]" when quarantined
