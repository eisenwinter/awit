---
id: AWIT-0ND56V3G
title: pkg/graph critical path
brief: >-
  Add Graph.CriticalPath: the longest chain (in node count) over non-closed,
  non-quarantined nodes, following Unblocks edges in Kahn topological order
  with smaller-ID tie-breaks at the ready queue, the predecessor choice, and
  the end node. Returns an empty non-nil slice when no candidates exist.
status: open
deps: [AWIT-0ND56Q3G]
labels: [phase3, p1]
refs:
  - ../../plan/implementation-guide.md
  - ../../plan/awit-implementation-plan.md
---

## Summary
After this ticket `pkg/graph/critical.go` exists with `func (g *Graph) CriticalPath() []*Node`. Candidates are nodes that are not closed and not quarantined. A Kahn topological walk over `Unblocks` edges restricted to candidates (ready queue kept sorted by ID) computes `dist[n]`, the longest path to `n` measured in nodes, and `prev[n]`, the predecessor that achieved it. On equal distance the smaller-ID predecessor wins; the end node is the maximum distance, ties by smaller ID; the path is walked back through `prev` and reversed. No candidates → an empty **non-nil** slice. `pkg/prime` (`AWIT-0ND56W3G`) consumes this; nothing else changes.

## Context (read first)
- Guide §4.6 `pkg/graph` — the exact signature `func (g *Graph) CriticalPath() []*Node` with comment "see §2; empty when no open nodes". Copy it. Do not change any existing signature.
- Guide §2 decision table, row "Critical path": "Longest path (by node count) over non-closed, non-quarantined nodes following `Unblocks` edges in topological order; ties by smaller ID at each DP step. Printed from the root (item with no open deps) downstream." This ticket implements the path computation only; printing is `AWIT-0ND56W3G`.
- Guide §8 fixture rows this ticket asserts:
  - `clean`: Critical = `0001 → 0003 → 0004`. `0005` is closed (not a candidate); `0006` is `in_progress` with dep `0005` closed, so it is a candidate root of distance 1 and never wins.
  - `cyclic`: `0001`–`0004` are quarantined (two CYCLE faults from `AWIT-0ND56P3G`); only `0005` is a candidate → path `[AWIT-TEST0005]`.
  - `dangling`: `0001` is quarantined (DANGLING DEP); `0002` depends on it but is itself clean, so `0002` is a candidate whose **candidate** deps are empty → path `[AWIT-TEST0002]`.
- Dep ticket `AWIT-0ND56Q3G` must be `status: closed` before you start (classify/countUnblocks/Ready/Blocked live in `pkg/graph/rank.go`; quarantine via `Node.Quarantined()` comes from `AWIT-0ND56P3G`).
- `pkg/graph/graph_test.go` already defines `buildFixture(t, name) *Graph` and `nodeIDs(nodes []*Node) []string` in package `graph`. Reuse them; do not redeclare them.
- The candidate subgraph is guaranteed acyclic: every cycle member is quarantined by Tarjan in `Build` (`AWIT-0ND56P3G`), and quarantined nodes are excluded. No cycle guard is needed in `CriticalPath`.
- Guide §1: stdlib plus `pkg/item` only in this package; table-driven tests with the `testing` stdlib; `sort` and `slices` are stdlib and allowed. Commit scope for this package is `graph`.

## Files
- Create: `pkg/graph/critical.go`
- Create: `pkg/graph/critical_test.go`

## Interfaces
- Consumes (already in the package, do not reimplement):
  ```go
  type Graph struct {
      Nodes  map[string]*Node
      Order  []*Node // all nodes sorted by ID asc
      Broken []item.Broken
      Faults []Fault
  }
  type Node struct {
      Item         *item.Item
      Deps         []*Node
      Unblocks     []*Node
      Faults       []Fault
      Ready        bool
      Blocked      bool
      UnblockCount int
  }
  func (n *Node) Quarantined() bool
  const item.StatusClosed Status = "closed"
  ```
- Produces (verbatim from guide §4.6, this ticket):
  ```go
  func (g *Graph) CriticalPath() []*Node
  ```
- Not produced here: nothing else. Do not touch `graph.go`, `scc.go`, `rank.go`, or any fixture. `CriticalPath` has no seam in `Build`; it is a pure query over the built graph.

## Steps

- [ ] **Step 1: Write the failing tests.**

  Create `pkg/graph/critical_test.go` with exactly this content:

  ```go
  package graph

  import (
  	"slices"
  	"testing"

  	"github.com/eisenwinter/awit/pkg/item"
  )

  func TestCriticalPathClean(t *testing.T) {
  	g := buildFixture(t, "clean")
  	got := nodeIDs(g.CriticalPath())
  	want := []string{"AWIT-TEST0001", "AWIT-TEST0003", "AWIT-TEST0004"}
  	if !slices.Equal(got, want) {
  		t.Fatalf("CriticalPath() = %v, want %v", got, want)
  	}
  }

  func TestCriticalPathSkipsClosedAndQuarantined(t *testing.T) {
  	// cyclic: 0001-0004 are quarantined by CYCLE faults; 0005 is the only
  	// candidate. dangling: 0001 is quarantined (DANGLING DEP); 0002 depends
  	// on it but is clean, and its only dep is not a candidate.
  	cases := []struct {
  		fixture string
  		want    []string
  	}{
  		{"cyclic", []string{"AWIT-TEST0005"}},
  		{"dangling", []string{"AWIT-TEST0002"}},
  	}
  	for _, tt := range cases {
  		t.Run(tt.fixture, func(t *testing.T) {
  			g := buildFixture(t, tt.fixture)
  			got := nodeIDs(g.CriticalPath())
  			if !slices.Equal(got, tt.want) {
  				t.Fatalf("CriticalPath() = %v, want %v", got, tt.want)
  			}
  		})
  	}
  }

  func TestCriticalPathTieBreak(t *testing.T) {
  	// Diamond: A unblocks B and C; B and C both unblock D. A→B→D and A→C→D
  	// both have length 3; on equal distance the smaller-ID predecessor (B)
  	// wins, so the path is [A B D] regardless of dep declaration order.
  	build := func(dDeps []string) *Graph {
  		a := &item.Item{ID: "AWIT-TEST0001", Title: "A", Status: item.StatusOpen}
  		b := &item.Item{ID: "AWIT-TEST0002", Title: "B", Status: item.StatusOpen, Deps: []string{"AWIT-TEST0001"}}
  		c := &item.Item{ID: "AWIT-TEST0003", Title: "C", Status: item.StatusOpen, Deps: []string{"AWIT-TEST0001"}}
  		d := &item.Item{ID: "AWIT-TEST0004", Title: "D", Status: item.StatusOpen, Deps: dDeps}
  		return Build([]*item.Item{a, b, c, d}, nil)
  	}
  	want := []string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0004"}
  	for _, dDeps := range [][]string{
  		{"AWIT-TEST0002", "AWIT-TEST0003"},
  		{"AWIT-TEST0003", "AWIT-TEST0002"},
  	} {
  		if got := nodeIDs(build(dDeps).CriticalPath()); !slices.Equal(got, want) {
  			t.Fatalf("D deps %v: CriticalPath() = %v, want %v", dDeps, got, want)
  		}
  	}

  	// End tie: chains A→B and C→D both have length 2. The end node tie
  	// breaks to the smaller ID (B), so the path is [A B], not [C D].
  	a := &item.Item{ID: "AWIT-TEST0001", Title: "A", Status: item.StatusOpen}
  	b := &item.Item{ID: "AWIT-TEST0002", Title: "B", Status: item.StatusOpen, Deps: []string{"AWIT-TEST0001"}}
  	c := &item.Item{ID: "AWIT-TEST0003", Title: "C", Status: item.StatusOpen}
  	d := &item.Item{ID: "AWIT-TEST0004", Title: "D", Status: item.StatusOpen, Deps: []string{"AWIT-TEST0003"}}
  	g := Build([]*item.Item{a, b, c, d}, nil)
  	wantEnd := []string{"AWIT-TEST0001", "AWIT-TEST0002"}
  	if got := nodeIDs(g.CriticalPath()); !slices.Equal(got, wantEnd) {
  		t.Fatalf("end tie: CriticalPath() = %v, want %v", got, wantEnd)
  	}
  }

  func TestCriticalPathEmpty(t *testing.T) {
  	closed := &item.Item{ID: "AWIT-TEST0001", Title: "done", Status: item.StatusClosed}
  	closedDep := &item.Item{ID: "AWIT-TEST0002", Title: "done too", Status: item.StatusClosed, Deps: []string{"AWIT-TEST0001"}}
  	g := Build([]*item.Item{closed, closedDep}, nil)
  	got := g.CriticalPath()
  	if got == nil {
  		t.Fatal("CriticalPath() = nil, want empty non-nil slice")
  	}
  	if len(got) != 0 {
  		t.Fatalf("CriticalPath() = %v, want empty (all nodes closed)", nodeIDs(got))
  	}

  	empty := Build(nil, nil)
  	if got := empty.CriticalPath(); got == nil || len(got) != 0 {
  		t.Fatalf("empty graph CriticalPath() = %v (nil=%v), want empty non-nil slice", got, got == nil)
  	}
  }

  func TestCriticalPathDeterministic(t *testing.T) {
  	g := buildFixture(t, "clean")
  	first := nodeIDs(g.CriticalPath())
  	for i := 0; i < 10; i++ {
  		if got := nodeIDs(g.CriticalPath()); !slices.Equal(got, first) {
  			t.Fatalf("run %d: CriticalPath() = %v, want %v", i, got, first)
  		}
  	}
  	// A freshly rebuilt graph over the same fixture must agree.
  	again := buildFixture(t, "clean")
  	if got := nodeIDs(again.CriticalPath()); !slices.Equal(got, first) {
  		t.Fatalf("rebuilt graph CriticalPath() = %v, want %v", got, first)
  	}
  }
  ```

- [ ] **Step 2: Run it, see it fail.**

  ```bash
  go test ./pkg/graph -run 'TestCriticalPath' -v
  ```

  Expected failure — `CriticalPath` does not exist yet, so the package does not compile:

  ```text
  # github.com/eisenwinter/awit/pkg/graph [github.com/eisenwinter/awit/pkg/graph.test]
  pkg/graph/critical_test.go:12:20: g.CriticalPath undefined (type *Graph has no field or method CriticalPath)
  FAIL	github.com/eisenwinter/awit/pkg/graph [build failed]
  ```

  Do not skip this step; the compile error is the red state.

- [ ] **Step 3: Implement `critical.go`.**

  Create `pkg/graph/critical.go` with exactly this content:

  ```go
  package graph

  import (
  	"slices"
  	"sort"

  	"github.com/eisenwinter/awit/pkg/item"
  )

  // CriticalPath returns the longest chain (in node count) over non-closed,
  // non-quarantined nodes, following Unblocks edges in topological order.
  // Every tie breaks to the smaller ID: the ready queue is ID-sorted, an
  // equal-distance predecessor keeps the smaller ID, and the end node is the
  // smaller ID on equal distance. The path runs from a root (a candidate with
  // no candidate deps) downstream. It returns an empty non-nil slice when no
  // candidates exist. The graph is not mutated.
  func (g *Graph) CriticalPath() []*Node {
  	path := []*Node{}

  	candidate := make(map[string]bool, len(g.Order))
  	for _, n := range g.Order {
  		if n.Item.Status != item.StatusClosed && !n.Quarantined() {
  			candidate[n.Item.ID] = true
  		}
  	}
  	if len(candidate) == 0 {
  		return path
  	}

  	// Kahn topological walk over Unblocks restricted to candidates.
  	// indegree counts candidate deps only, so a closed or quarantined dep
  	// never holds a candidate back.
  	indegree := make(map[string]int, len(candidate))
  	dist := make(map[string]int, len(candidate))
  	prev := make(map[string]*Node, len(candidate))
  	var ready []*Node
  	for _, n := range g.Order {
  		if !candidate[n.Item.ID] {
  			continue
  		}
  		deg := 0
  		for _, d := range n.Deps {
  			if candidate[d.Item.ID] {
  				deg++
  			}
  		}
  		indegree[n.Item.ID] = deg
  		dist[n.Item.ID] = 1 // every candidate is a path of one node
  		if deg == 0 {
  			ready = append(ready, n)
  		}
  	}
  	sort.Slice(ready, func(i, j int) bool { return ready[i].Item.ID < ready[j].Item.ID })

  	for len(ready) > 0 {
  		cur := ready[0]
  		ready = ready[1:]
  		for _, next := range cur.Unblocks {
  			if !candidate[next.Item.ID] {
  				continue
  			}
  			d := dist[cur.Item.ID] + 1
  			if p := prev[next.Item.ID]; d > dist[next.Item.ID] ||
  				(d == dist[next.Item.ID] && (p == nil || cur.Item.ID < p.Item.ID)) {
  				dist[next.Item.ID] = d
  				prev[next.Item.ID] = cur
  			}
  			indegree[next.Item.ID]--
  			if indegree[next.Item.ID] == 0 {
  				i := sort.Search(len(ready), func(i int) bool { return ready[i].Item.ID > next.Item.ID })
  				ready = append(ready, nil)
  				copy(ready[i+1:], ready[i:])
  				ready[i] = next
  			}
  		}
  	}

  	// End node: maximum distance; g.Order is ID-ascending and a strictly
  	// greater distance is required to replace end, so ties keep the smaller ID.
  	var end *Node
  	for _, n := range g.Order {
  		if !candidate[n.Item.ID] {
  			continue
  		}
  		if end == nil || dist[n.Item.ID] > dist[end.Item.ID] {
  			end = n
  		}
  	}

  	for n := end; n != nil; n = prev[n.Item.ID] {
  		path = append(path, n)
  	}
  	slices.Reverse(path)
  	return path
  }
  ```

  Implementation rules:
  - Candidacy is `Status != item.StatusClosed && !Quarantined()`. `in_progress` counts as open. Quarantined nodes are excluded even if their status is `closed`.
  - `indegree` walks `Node.Deps` (resolved edges) and counts only candidate deps. A dangling dep is not in `Node.Deps` at all, and its depender is quarantined anyway, so neither needs special-casing.
  - A node's `dist`/`prev` are final before it is enqueued: the last candidate predecessor to be processed drops `indegree` to 0, and all relaxations from predecessors happen at predecessor-pop time. The equal-distance comparison `cur.Item.ID < p.Item.ID` therefore resolves the predecessor tie deterministically regardless of `Unblocks` edge order.
  - The ready queue stays ID-sorted via `sort.Search` insertion; popping `ready[0]` is always the smallest-ID available node.
  - `dist` is measured in **nodes**, so roots have `dist == 1`, not 0. The returned slice contains every node on the path, root first.
  - `path` starts as `[]*Node{}` so the no-candidate return is empty but non-nil; `prime` will range over it without a nil check.

- [ ] **Step 4: Run the tests, see them pass, commit.**

  ```bash
  go test ./pkg/graph -run 'TestCriticalPath' -v
  ```

  Expected:

  ```text
  === RUN   TestCriticalPathClean
  --- PASS: TestCriticalPathClean
  === RUN   TestCriticalPathSkipsClosedAndQuarantined
  === RUN   TestCriticalPathSkipsClosedAndQuarantined/cyclic
  === RUN   TestCriticalPathSkipsClosedAndQuarantined/dangling
  --- PASS: TestCriticalPathSkipsClosedAndQuarantined
  === RUN   TestCriticalPathTieBreak
  --- PASS: TestCriticalPathTieBreak
  === RUN   TestCriticalPathEmpty
  --- PASS: TestCriticalPathEmpty
  === RUN   TestCriticalPathDeterministic
  --- PASS: TestCriticalPathDeterministic
  PASS
  ok  	github.com/eisenwinter/awit/pkg/graph
  ```

  Then the full package (Build + Tarjan + rank + critical) and commit:

  ```bash
  go test ./pkg/graph -count=1
  gofmt -w pkg/graph/critical.go pkg/graph/critical_test.go
  git add pkg/graph/critical.go pkg/graph/critical_test.go
  git commit -m "graph: critical path with ID tie-breaks"
  ```

  `gofmt -l pkg/graph` must print nothing.

- [ ] **Step 5: Close ticket.**

  1. Run `go build ./... && go vet ./pkg/graph && go test ./pkg/graph -count=1` and copy the output.
  2. Create `.awit/comments/AWIT-0ND56V3G/<YYYYMMDDTHHMMSSZ>-<author>.md` (UTC stamp, author sanitised to `[a-z0-9._-]`, e.g. `20260917T153000Z-claude.md`) with this shape:

     ```markdown
     ---
     author: <author>
     created: <RFC3339 UTC, e.g. 2026-09-17T15:30:00Z>
     ---

     Acceptance evidence for AWIT-0ND56V3G (pkg/graph critical path):

     <paste the real test/vet output from step 1>
     ```
  3. In `.awit/items/AWIT-0ND56V3G.md` append `  - ../comments/AWIT-0ND56V3G/<that filename>` to the `refs:` block and change `status: open` to `status: closed`. Touch nothing else in the frontmatter.
  4. Commit:

     ```bash
     git add .awit/items/AWIT-0ND56V3G.md .awit/comments/AWIT-0ND56V3G/
     git commit -m "tickets: close AWIT-0ND56V3G"
     ```

## Acceptance Criteria
- `go test ./pkg/graph -count=1` passes, including the five new tests.
- `TestCriticalPathClean`: clean fixture path is exactly `[AWIT-TEST0001, AWIT-TEST0003, AWIT-TEST0004]` — root first, measured in nodes.
- `TestCriticalPathSkipsClosedAndQuarantined`: cyclic fixture → `[AWIT-TEST0005]`; dangling fixture → `[AWIT-TEST0002]`.
- `TestCriticalPathTieBreak`: diamond A→B→D / A→C→D yields `[A B D]` under both dep declaration orders; equal-length chains A→B and C→D yield `[A B]` (end tie → smaller ID).
- `TestCriticalPathEmpty`: all-closed graph and empty graph both return an empty **non-nil** slice.
- `TestCriticalPathDeterministic`: ten calls on one graph and one call on a rebuilt graph all return identical IDs.
- `go vet ./pkg/graph` is clean; `gofmt -l pkg/graph` prints nothing.
- Only `pkg/graph/critical.go` and `pkg/graph/critical_test.go` are added; no other file in the package changes.

## Out of scope
- `pkg/prime` rendering of the CRITICAL PATH section and the `awit prime` command — `AWIT-0ND56W3G`.
- `awit next`, `awit list`, `awit validate`, `awit dep`.
- Changing `Build`, Tarjan, `classify`, `countUnblocks`, `Ready`/`Blocked`/`Quarantined`/`Closed`, `WouldCycle`, `FilterLabels`, or any fixture file.
- Weighted critical paths (effort estimates); length is always node count.
