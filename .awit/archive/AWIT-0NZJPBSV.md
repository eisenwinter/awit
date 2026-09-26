---
id: AWIT-0NZJPBSV
title: 'graph: add Filter/Narrow/MatchLabels; list uses them'
brief: >-
  `awit list` state/status/label selection lives inline in listAction; extracting it into pkg/graph as Filter/Narrow/MatchLabels gives the lazy-human TUI list parity by construction.
status: closed
deps: []
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---
## Summary

`awit list` decides which nodes to print in ~40 inline lines of `listAction` (state flags → source set, then status OR, then label AND/OR). The lazy-human TUI must show exactly the same rows for the same inputs. Move the rules into `pkg/graph` as `Filter`, `Narrow`, and `MatchLabels`, and make `listAction` call them, so both consumers share one implementation and the existing `list` goldens prove nothing moved.

## Context (read first)

- `docs/design-spec.md` §5 (`list` row), §6 "Label Filter Logic" (AND across flags, OR within), §7.
- `internal/cli/list.go:65-104` — the rules being extracted. Order rules matter: `--ready` alone → `g.Ready()` (ranked order: UnblockCount desc, ID asc); any other state-flag combination → scan `g.Order` (ID asc) keeping nodes where `(ready && n.Ready) || (blocked && n.Blocked) || (quarantined && n.Quarantined())`; no state flag → `g.Order`. Then status OR (comma lists inside one `-s`, repeatable), then `graph.FilterLabels` with `SplitLabels` groups.
- `internal/cli/list.go:52-64` — the `[key]` path keeps its single-node state check; only status/label narrowing is shared.
- `pkg/graph/rank.go:162-187` — `FilterLabels`; it becomes a loop over the new `MatchLabels`.
- `pkg/graph/graph_test.go:17,35` — `buildFixture(t, "clean")`, `nodeIDs`. Clean fixture: 0001 open ready `auth,p1` U2; 0002 open ready `db`; 0003 open blocked (dep 0001); 0004 open blocked `p0`; 0005 closed; 0006 in_progress ready.

## Files

- `pkg/graph/filter.go` — new: `Filter`, `(*Graph).Filter`, `Narrow`, `MatchLabels`.
- `pkg/graph/rank.go` — `FilterLabels` body delegates to `MatchLabels`.
- `pkg/graph/filter_test.go` — new tests.
- `internal/cli/list.go` — `listAction` uses the new API; new `parseStatuses` helper holding lines 81-94.

## Interfaces

```go
// pkg/graph/filter.go
// Filter is the awit list selection: state flags choose the source set
// (Ready alone keeps ranked order; any other combination scans Order in ID
// order; none means Order), then Statuses (OR) and Labels (AND across
// groups, OR within) narrow it.
type Filter struct {
	Ready, Blocked, Quarantined bool
	Statuses                    []item.Status
	Labels                      [][]string
}
func (g *Graph) Filter(f Filter) []*Node
// Narrow keeps nodes whose status is in statuses (empty = any) and that
// match every label group; order is preserved.
func Narrow(nodes []*Node, statuses []item.Status, labels [][]string) []*Node
// MatchLabels reports whether labels satisfy every group (AND), a group
// matching when any of its labels is present (OR). No groups = true.
func MatchLabels(labels []string, groups [][]string) bool

## Comments

### 2026-09-24T20:34:15Z jan

implemented
