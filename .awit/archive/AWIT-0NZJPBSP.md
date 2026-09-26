---
id: AWIT-0NZJPBSP
title: "lazy: Issues tab - rows, filter grammar, archive toggle, detail viewport"
brief: >-
  The Issues tab renders list-compact rows from graph.Filter plus substring search, accepts awit-list flag grammar in the / prompt, toggles open/archive sources, and mirrors show --full in the detail pane.
status: closed
deps: [AWIT-0NZJPBSV, AWIT-0NZJPBSM]
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

Replace the WI-4 placeholder Issues rows with the real tab: rows are `Ops.Line` over `g.Filter(...)` (WI-1) narrowed by a case-insensitive substring search; `/` opens a prompt that speaks the `awit list` flag vocabulary (`-s`, `-l`, `--ready`, `--blocked`, `--quarantined`, free words = search); `o` switches to the archive (`Ops.LoadArchive`, lazily, cached until `R`); the header shows counts and filter badges; the detail viewport shows `Ops.Detail` (== `show --full`) or the raw archive file. Quarantined rows are unselectable.

## Context (read first)

- Design spec "Issues tab"; plan §D.9 (grammar, rows, header, detail), §D.7, §D.15.
- `pkg/graph/filter.go` (WI-1) - `Filter`, `MatchLabels`.
- `internal/cli/app.go:224-243` - `SplitLabels` rules (trim, drop empty, one `-l` = one OR group); reimplement the same three rules locally in `filter.go` (cannot import `internal/cli`).
- `pkg/item` - `ParseStatus`, `Item.Labels`, `Item.Title`.
- WI-4 test helpers: `newFixture` (L1..L8, archive A1/A2), `press`, `golden`, `resize`.

## Files

- `internal/lazy/filter.go` - `Filter`, `ParseQuery`, `badges`, `applyOpen`, `applyArchive`.
- `internal/lazy/issues.go` - `issuesState`, `issuesRows`, `issuesHeader`, `issuesDetail`, key handling for `/`, `o`.
- `internal/lazy/model.go`, `view.go` - wire the tab (search mode submit, header line 2, hint line for Issues: `j/k move  enter pin  / filter  o open/archive  c close  b block  u unblock  m comment  P check  ? help`).
- `internal/lazy/filter_test.go`, `issues_test.go`; goldens `issues_open`, `issues_archive`, `issues_filtered`, `issues_search_prompt`; `frame_issues` regenerated (rows unchanged, header line 2 now real - review the diff).

## Interfaces

```go
type Filter struct {
	Ready, Blocked, Quarantined bool
	Statuses                    []item.Status
	Labels                      [][]string
	Search                      string // lower-cased; "" = none
}
func ParseQuery(q string) (Filter, error)                      // grammar in plan §D.9; unknown -x → error "unknown flag -x"; bad status → item.ParseStatus error
func (f Filter) badges() string                                // "no filter" | space-joined "[-s open,closed]" "[-l auth|db] [-l p1]" "[--ready]" "[/ text]"
func (f Filter) applyOpen(g *graph.Graph) []*graph.Node        // g.Filter(graph.Filter{...}) then Search over id, title, labels
func (f Filter) applyArchive(items []*item.Item) []*item.Item  // Statuses OR, graph.MatchLabels, Search; state flags ignored

type issuesState struct {
	filter      Filter
	showArchive bool
	archiveN    int // len(archive) after LoadArchive; header shows "(not loaded)" until archiveLoaded
	list        cursorList
}
func (m *Model) issuesRows() []row       // open: Line(n), selectable = !n.Quarantined(); archive: ArchiveLine(it), selectable
func (m *Model) issuesHeader() string    // "open: N  archive: M   filters: <badges>"  (M = "(not loaded)" until loaded) + "   source: open|archive"
func (m *Model) issuesDetail() string    // open → Detail(g,id); archive → ArchiveDetail(id) (error → its text); no selection → "no items match" or "no selectable item; press V for the validate report"

## Comments

### 2026-09-24T20:48:20Z jan

implemented
```
