---
id: AWIT-0ND56Q3G
title: 'pkg/graph ready/blocked, unblock counts, WouldCycle, FilterLabels'
brief: >-
  Fill the classify and countUnblocks seams and add Ready, Blocked,
  Quarantined, Closed, WouldCycle, and FilterLabels. Ready is every
  non-closed non-quarantined node whose Item.Deps all resolve to closed
  non-quarantined nodes; unblock counts are BFS over Unblocks; WouldCycle
  DFS-prechecks dep add.
status: open
deps: [AWIT-0ND56P3G]
labels: [phase2, p0]
refs:
  - ../../plan/implementation-guide.md
  - ../../plan/awit-implementation-plan.md
---

## Summary
After this ticket `pkg/graph/rank.go` holds `classify`, `countUnblocks`, `Graph.Ready` / `Blocked` / `Quarantined` / `Closed`, `WouldCycle`, and `FilterLabels`. `classify` sets `Node.Ready` / `Node.Blocked` and never both; closed and quarantined nodes get both flags false. `countUnblocks` writes the number of unique non-closed non-quarantined nodes reachable via `Unblocks` (BFS, including through closed nodes); quarantined nodes get `-1`. `Ready()` sorts by UnblockCount descending then ID ascending; the other three lists are ID ascending. `WouldCycle(from, to)` DFS-walks `to`'s `Deps` looking for `from`. `FilterLabels` is AND across groups and OR within a group. `CriticalPath` is not implemented.

## Context (read first)
- Guide §4.6 — exact signatures for `Ready`, `Blocked`, `Quarantined`, `Closed`, `WouldCycle`, `FilterLabels`. Copy them. Do not add `CriticalPath` here.
- Guide §2 Unblock count: "Number of **unique, non-closed, non-quarantined** nodes reachable via `Unblocks` edges (transitive). Quarantined nodes have `UnblockCount == -1`." Computed once per build, cached on the node, never inside a sort comparator.
- Guide §2 decision 1 (labels): AND across groups, OR within a group. Empty `groups` (nil or `len==0`) leaves the input slice unchanged (same backing array).
- Spec §Graph engine Ready / Blocked: Ready = not closed and every dep closed. Blocked = not closed and any dep open, dangling, or quarantined. A quarantined dep makes the depender Blocked (if the depender itself is not quarantined). Dangling deps already quarantined the **depender** in `Build`, so that node is neither Ready nor Blocked.
- Guide §8 `clean` fixture (already on disk):
  - `0001` open, no deps, labels `auth,p1` — unblocks `0003` and `0004` → UnblockCount 2, Ready.
  - `0002` open, labels `db` — UnblockCount 0, Ready.
  - `0003` open, deps `0001` — Blocked, UnblockCount 1 (`0004`).
  - `0004` open, deps `0001,0003`, labels `p0` — Blocked, UnblockCount 0.
  - `0005` closed — Closed list, not Ready/Blocked.
  - `0006` in_progress, deps `0005` (closed) — Ready, UnblockCount 0.
  - Ready order: `0001` then `0002` then `0006` (count 2, then 0/0 by ID).
  - Blocked order: `0003`, `0004`.
- Guide §8 `dangling` fixture: `0001` deps unknown `AWIT-TEST9999` (quarantined); `0002` deps `0001` (Blocked, `OpenDepIDs` = `[AWIT-TEST0001]`).
- `WouldCycle` (guide §4.6): DFS from `to` over `Node.Deps` looking for `from`. Result starts with `from` and ends with `from`: `[from, to, ..., from]`. `from == to` → `[from, from]` without walking. Unknown IDs (either argument missing from `g.Nodes`) → nil. On `clean`, `WouldCycle(0001, 0004)` is `[AWIT-TEST0001, AWIT-TEST0004, AWIT-TEST0001]` because `0004.Deps` includes `0001` as a direct edge (Item.Deps order is `0001` then `0003`, so the walk hits `0001` first). `WouldCycle(0002, 0001)` is nil (`0001` has no path to `0002`).
- `pkg/graph/graph.go` still has empty `classify` and `countUnblocks` methods. Delete those empty methods; define them in `rank.go`. Keep the `Build` call site `g.detectCycles(); g.classify(); g.countUnblocks()` unchanged. `detectCycles` already lives in `scc.go`.
- `pkg/graph/graph_test.go` already defines `buildFixture`, `nodeIDs` in package `graph`. Do not redeclare them.
- `item.Item.HasLabel(l string) bool` exists; use it in `FilterLabels`. Parsed fixture items have it working. Hand-built items with a `Labels` slice also work if `HasLabel` ranges `Item.Labels`.
- Partition: every node is in exactly one of Ready / Blocked / Quarantined / Closed, except a closed+quarantined node which is Quarantined only (`Closed()` skips quarantined). `in_progress` is not closed.

## Files
- Create: `pkg/graph/rank.go`
- Create: `pkg/graph/rank_test.go`
- Modify: `pkg/graph/graph.go` — delete the empty `func (g *Graph) classify() {}` and `func (g *Graph) countUnblocks() {}` methods. Leave the two calls in `Build`.

## Interfaces
- Consumes (already in the package):
  ```go
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
  func (n *Node) OpenDepIDs() []string
  func Build(items []*item.Item, broken []item.Broken) *Graph
  func (it *item.Item) HasLabel(l string) bool
  ```
- Produces (verbatim from guide §4.6, this ticket):
  ```go
  func (g *Graph) Ready() []*Node
  func (g *Graph) Blocked() []*Node
  func (g *Graph) Quarantined() []*Node
  func (g *Graph) Closed() []*Node
  func (g *Graph) WouldCycle(from, to string) []string
  func FilterLabels(nodes []*Node, groups [][]string) []*Node
  ```
- Produces (package-private, this ticket, in `rank.go`):
  ```go
  func (g *Graph) classify()
  func (g *Graph) countUnblocks()
  func depSatisfied(g *Graph, depID string) bool
  func reachableUnblocks(start *Node) int
  ```
- Not produced: `CriticalPath`. Do not declare it.

## Steps

- [ ] **Step 1: Write the failing tests.**

  Create `pkg/graph/rank_test.go`:

  ```go
  package graph

  import (
  	"slices"
  	"testing"

  	"github.com/eisenwinter/awit/pkg/item"
  )

  func TestClassifyClean(t *testing.T) {
  	g := buildFixture(t, "clean")
  	gotReady := nodeIDs(g.Ready())
  	wantReady := []string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0006"}
  	if !slices.Equal(gotReady, wantReady) {
  		t.Fatalf("Ready() = %v, want %v", gotReady, wantReady)
  	}
  	gotBlocked := nodeIDs(g.Blocked())
  	wantBlocked := []string{"AWIT-TEST0003", "AWIT-TEST0004"}
  	if !slices.Equal(gotBlocked, wantBlocked) {
  		t.Fatalf("Blocked() = %v, want %v", gotBlocked, wantBlocked)
  	}
  	gotClosed := nodeIDs(g.Closed())
  	if !slices.Equal(gotClosed, []string{"AWIT-TEST0005"}) {
  		t.Fatalf("Closed() = %v, want [AWIT-TEST0005]", gotClosed)
  	}
  	if len(g.Quarantined()) != 0 {
  		t.Fatalf("Quarantined() = %v, want empty", nodeIDs(g.Quarantined()))
  	}
  	n1 := g.Nodes["AWIT-TEST0001"]
  	n5 := g.Nodes["AWIT-TEST0005"]
  	n6 := g.Nodes["AWIT-TEST0006"]
  	if !n1.Ready || n1.Blocked {
  		t.Fatalf("0001 Ready/Blocked = %v/%v, want true/false", n1.Ready, n1.Blocked)
  	}
  	if n5.Ready || n5.Blocked {
  		t.Fatalf("0005 Ready/Blocked = %v/%v, want false/false (closed)", n5.Ready, n5.Blocked)
  	}
  	if !n6.Ready || n6.Blocked {
  		t.Fatalf("0006 Ready/Blocked = %v/%v, want true/false (in_progress, dep closed)", n6.Ready, n6.Blocked)
  	}
  }

  func TestUnblockCounts(t *testing.T) {
  	g := buildFixture(t, "clean")
  	cases := map[string]int{
  		"AWIT-TEST0001": 2,
  		"AWIT-TEST0003": 1,
  		"AWIT-TEST0004": 0,
  		"AWIT-TEST0002": 0,
  		"AWIT-TEST0006": 0,
  	}
  	for id, want := range cases {
  		got := g.Nodes[id].UnblockCount
  		if got != want {
  			t.Fatalf("%s UnblockCount = %d, want %d", id, got, want)
  		}
  	}
  }

  func TestUnblockCountsIgnoreClosedDownstream(t *testing.T) {
  	// Unblocks edges: A → B(closed) → C(open). Count from A is 1 (C only).
  	// Deps are the reverse: B deps A, C deps B.
  	a := &item.Item{ID: "AWIT-TEST0001", Title: "A", Status: item.StatusOpen}
  	b := &item.Item{ID: "AWIT-TEST0002", Title: "B", Status: item.StatusClosed, Deps: []string{"AWIT-TEST0001"}}
  	c := &item.Item{ID: "AWIT-TEST0003", Title: "C", Status: item.StatusOpen, Deps: []string{"AWIT-TEST0002"}}
  	g := Build([]*item.Item{a, b, c}, nil)
  	got := g.Nodes["AWIT-TEST0001"].UnblockCount
  	if got != 1 {
  		t.Fatalf("A UnblockCount = %d, want 1 (closed B is skipped, open C is counted)", got)
  	}
  	if g.Nodes["AWIT-TEST0002"].UnblockCount != 1 {
  		t.Fatalf("B UnblockCount = %d, want 1 (reaches C)", g.Nodes["AWIT-TEST0002"].UnblockCount)
  	}
  	if g.Nodes["AWIT-TEST0003"].UnblockCount != 0 {
  		t.Fatalf("C UnblockCount = %d, want 0", g.Nodes["AWIT-TEST0003"].UnblockCount)
  	}
  }

  func TestWouldCycle(t *testing.T) {
  	g := buildFixture(t, "clean")
  	got := g.WouldCycle("AWIT-TEST0001", "AWIT-TEST0004")
  	want := []string{"AWIT-TEST0001", "AWIT-TEST0004", "AWIT-TEST0001"}
  	if !slices.Equal(got, want) {
  		t.Fatalf("WouldCycle(0001,0004) = %v, want %v", got, want)
  	}
  	if got := g.WouldCycle("AWIT-TEST0002", "AWIT-TEST0001"); got != nil {
  		t.Fatalf("WouldCycle(0002,0001) = %v, want nil", got)
  	}
  	self := g.WouldCycle("AWIT-TEST0001", "AWIT-TEST0001")
  	if !slices.Equal(self, []string{"AWIT-TEST0001", "AWIT-TEST0001"}) {
  		t.Fatalf("WouldCycle(0001,0001) = %v, want [0001 0001]", self)
  	}
  	if got := g.WouldCycle("AWIT-NOPE0001", "AWIT-TEST0001"); got != nil {
  		t.Fatalf("WouldCycle(unknown, 0001) = %v, want nil", got)
  	}
  	if got := g.WouldCycle("AWIT-TEST0001", "AWIT-NOPE0001"); got != nil {
  		t.Fatalf("WouldCycle(0001, unknown) = %v, want nil", got)
  	}
  }

  func TestFilterLabels(t *testing.T) {
  	g := buildFixture(t, "clean")

  	p1 := FilterLabels(g.Order, [][]string{{"p1"}})
  	if !slices.Equal(nodeIDs(p1), []string{"AWIT-TEST0001"}) {
  		t.Fatalf("filter p1 = %v, want [0001]", nodeIDs(p1))
  	}

  	// AND across groups: auth AND p0 — 0001 has auth, 0004 has p0, nobody has both.
  	both := FilterLabels(g.Order, [][]string{{"auth"}, {"p0"}})
  	if len(both) != 0 {
  		t.Fatalf("filter auth AND p0 = %v, want empty", nodeIDs(both))
  	}

  	// OR within a group: auth OR db — 0001 (auth) and 0002 (db).
  	or := FilterLabels(g.Order, [][]string{{"auth", "db"}})
  	if !slices.Equal(nodeIDs(or), []string{"AWIT-TEST0001", "AWIT-TEST0002"}) {
  		t.Fatalf("filter auth OR db = %v, want [0001 0002]", nodeIDs(or))
  	}

  	// AND auth AND p1 — 0001 has both.
  	and := FilterLabels(g.Order, [][]string{{"auth"}, {"p1"}})
  	if !slices.Equal(nodeIDs(and), []string{"AWIT-TEST0001"}) {
  		t.Fatalf("filter auth AND p1 = %v, want [0001]", nodeIDs(and))
  	}

  	unchangedNil := FilterLabels(g.Order, nil)
  	if len(unchangedNil) != len(g.Order) {
  		t.Fatalf("nil groups len = %d, want %d", len(unchangedNil), len(g.Order))
  	}
  	if len(g.Order) > 0 && &unchangedNil[0] != &g.Order[0] {
  		t.Fatal("nil groups must return the input slice unchanged")
  	}
  	unchangedEmpty := FilterLabels(g.Order, [][]string{})
  	if len(g.Order) > 0 && &unchangedEmpty[0] != &g.Order[0] {
  		t.Fatal("empty groups must return the input slice unchanged")
  	}
  }

  func TestDanglingIsQuarantinedNotBlocked(t *testing.T) {
  	g := buildFixture(t, "dangling")
  	n1 := g.Nodes["AWIT-TEST0001"]
  	n2 := g.Nodes["AWIT-TEST0002"]
  	if !n1.Quarantined() {
  		t.Fatal("0001 Quarantined() = false, want true")
  	}
  	if n1.Ready || n1.Blocked {
  		t.Fatalf("0001 Ready/Blocked = %v/%v, want false/false", n1.Ready, n1.Blocked)
  	}
  	if n1.UnblockCount != -1 {
  		t.Fatalf("0001 UnblockCount = %d, want -1", n1.UnblockCount)
  	}
  	if got := nodeIDs(g.Quarantined()); !slices.Equal(got, []string{"AWIT-TEST0001"}) {
  		t.Fatalf("Quarantined() = %v, want [0001]", got)
  	}
  	if n2.Quarantined() {
  		t.Fatal("0002 Quarantined() = true, want false")
  	}
  	if !n2.Blocked || n2.Ready {
  		t.Fatalf("0002 Ready/Blocked = %v/%v, want false/true", n2.Ready, n2.Blocked)
  	}
  	if got := n2.OpenDepIDs(); !slices.Equal(got, []string{"AWIT-TEST0001"}) {
  		t.Fatalf("0002 OpenDepIDs() = %v, want [AWIT-TEST0001]", got)
  	}
  	if got := nodeIDs(g.Blocked()); !slices.Equal(got, []string{"AWIT-TEST0002"}) {
  		t.Fatalf("Blocked() = %v, want [0002]", got)
  	}
  	if len(g.Ready()) != 0 {
  		t.Fatalf("Ready() = %v, want empty", nodeIDs(g.Ready()))
  	}
  }
  ```

- [ ] **Step 2: Run it, see it fail.**

  ```bash
  go test ./pkg/graph -run 'TestClassifyClean|TestUnblockCounts|TestWouldCycle|TestFilterLabels|TestDanglingIsQuarantinedNotBlocked' -v
  ```

  Expected failure (empty `classify` / `countUnblocks` seams: `Ready()` is empty, `UnblockCount` is 0, `WouldCycle` is undefined):

  ```text
  # github.com/eisenwinter/awit/pkg/graph [github.com/eisenwinter/awit/pkg/graph.test]
  pkg/graph/rank_test.go: undefined: FilterLabels
  FAIL	github.com/eisenwinter/awit/pkg/graph [build failed]
  ```

  If you declare the methods as empty returns first, the compile error becomes an assertion failure instead:

  ```text
  === RUN   TestClassifyClean
      rank_test.go: Ready() = [], want [AWIT-TEST0001 AWIT-TEST0002 AWIT-TEST0006]
  --- FAIL: TestClassifyClean
  ```

  Either form is a valid red; do not skip it.

- [ ] **Step 3: Delete the empty seams and implement `rank.go`.**

  In `pkg/graph/graph.go`, delete **only** these two methods (keep the calls in `Build`):

  ```go
  func (g *Graph) classify()      {}
  func (g *Graph) countUnblocks() {}
  ```

  Create `pkg/graph/rank.go`:

  ```go
  package graph

  import (
  	"sort"

  	"github.com/eisenwinter/awit/pkg/item"
  )

  // classify sets Ready / Blocked on every node. Closed and quarantined nodes
  // get both flags false. A node is Ready iff every Item.Deps ID resolves to a
  // closed, non-quarantined node; otherwise it is Blocked.
  func (g *Graph) classify() {
  	for _, n := range g.Order {
  		n.Ready = false
  		n.Blocked = false
  		if n.Item.Status == item.StatusClosed || n.Quarantined() {
  			continue
  		}
  		ready := true
  		for _, depID := range n.Item.Deps {
  			if !depSatisfied(g, depID) {
  				ready = false
  				break
  			}
  		}
  		n.Ready = ready
  		n.Blocked = !ready
  	}
  }

  func depSatisfied(g *Graph, depID string) bool {
  	d, ok := g.Nodes[depID]
  	if !ok {
  		return false
  	}
  	return d.Item.Status == item.StatusClosed && !d.Quarantined()
  }

  // countUnblocks writes UnblockCount. Quarantined nodes get -1. Others get the
  // number of unique non-closed non-quarantined nodes reachable via Unblocks,
  // walking through closed and quarantined nodes so a closed middle node does
  // not hide an open descendant.
  func (g *Graph) countUnblocks() {
  	for _, n := range g.Order {
  		if n.Quarantined() {
  			n.UnblockCount = -1
  			continue
  		}
  		n.UnblockCount = reachableUnblocks(n)
  	}
  }

  func reachableUnblocks(start *Node) int {
  	seen := map[string]bool{start.Item.ID: true}
  	count := 0
  	queue := append([]*Node(nil), start.Unblocks...)
  	for len(queue) > 0 {
  		n := queue[0]
  		queue = queue[1:]
  		if seen[n.Item.ID] {
  			continue
  		}
  		seen[n.Item.ID] = true
  		if !n.Quarantined() && n.Item.Status != item.StatusClosed {
  			count++
  		}
  		queue = append(queue, n.Unblocks...)
  	}
  	return count
  }

  func (g *Graph) Ready() []*Node {
  	var out []*Node
  	for _, n := range g.Order {
  		if n.Ready {
  			out = append(out, n)
  		}
  	}
  	sort.Slice(out, func(i, j int) bool {
  		if out[i].UnblockCount != out[j].UnblockCount {
  			return out[i].UnblockCount > out[j].UnblockCount
  		}
  		return out[i].Item.ID < out[j].Item.ID
  	})
  	return out
  }

  func (g *Graph) Blocked() []*Node {
  	var out []*Node
  	for _, n := range g.Order {
  		if n.Blocked {
  			out = append(out, n)
  		}
  	}
  	return out
  }

  func (g *Graph) Quarantined() []*Node {
  	var out []*Node
  	for _, n := range g.Order {
  		if n.Quarantined() {
  			out = append(out, n)
  		}
  	}
  	return out
  }

  func (g *Graph) Closed() []*Node {
  	var out []*Node
  	for _, n := range g.Order {
  		if n.Item.Status == item.StatusClosed && !n.Quarantined() {
  			out = append(out, n)
  		}
  	}
  	return out
  }

  // WouldCycle returns the dependency chain that adding "from depends on to"
  // would close, or nil. DFS from `to` over Node.Deps looking for `from`.
  func (g *Graph) WouldCycle(from, to string) []string {
  	if g.Nodes[from] == nil || g.Nodes[to] == nil {
  		return nil
  	}
  	if from == to {
  		return []string{from, from}
  	}
  	seen := make(map[string]bool)
  	var dfs func(cur string) []string
  	dfs = func(cur string) []string {
  		if cur == from {
  			return []string{cur}
  		}
  		if seen[cur] {
  			return nil
  		}
  		seen[cur] = true
  		n := g.Nodes[cur]
  		if n == nil {
  			return nil
  		}
  		for _, d := range n.Deps {
  			if rest := dfs(d.Item.ID); rest != nil {
  				return append([]string{cur}, rest...)
  			}
  		}
  		return nil
  	}
  	rest := dfs(to)
  	if rest == nil {
  		return nil
  	}
  	return append([]string{from}, rest...)
  }

  // FilterLabels keeps nodes matching every group (AND) where a group matches
  // if any label in it is present (OR). Empty groups (nil or length 0) returns
  // nodes unchanged.
  func FilterLabels(nodes []*Node, groups [][]string) []*Node {
  	if len(groups) == 0 {
  		return nodes
  	}
  	var out []*Node
  next:
  	for _, n := range nodes {
  		for _, group := range groups {
  			matched := false
  			for _, label := range group {
  				if n.Item.HasLabel(label) {
  					matched = true
  					break
  				}
  			}
  			if !matched {
  				continue next
  			}
  		}
  		out = append(out, n)
  	}
  	return out
  }
  ```

  Implementation rules:
  - `classify` uses `Item.Deps` (the original ID list), not `Node.Deps`, so a dangling ID fails `depSatisfied` the same way a missing node does. The dangling **depender** is already quarantined and is skipped before that loop.
  - A dep that exists but is quarantined (even if `status: closed`) is not satisfied → Blocked.
  - `countUnblocks` **traverses** closed and quarantined descendants but only **counts** unique non-closed non-quarantined nodes. Never count `start` itself (`seen` starts with start).
  - `Ready()` sorts after filtering; do not sort inside `classify`. `Blocked` / `Quarantined` / `Closed` walk `g.Order` so they are already ID-ascending.
  - `WouldCycle` walks `Node.Deps` (resolved). Check `cur == from` before `seen`, so the target can be found even if it was queued. Mark `to` when the walk starts at `to`.
  - `FilterLabels`: `len(groups)==0` returns the **same** slice header, not a copy. An empty inner group matches nothing (AND with an empty OR-set fails). Preserve input order.
  - Two definitions of `classify` or `countUnblocks` will not compile — they must exist only in `rank.go`.

- [ ] **Step 4: Run the tests, see them pass, commit.**

  ```bash
  go test ./pkg/graph -run 'TestClassifyClean|TestUnblockCounts|TestUnblockCountsIgnoreClosedDownstream|TestWouldCycle|TestFilterLabels|TestDanglingIsQuarantinedNotBlocked' -v
  ```

  Expected:

  ```text
  === RUN   TestClassifyClean
  --- PASS: TestClassifyClean
  === RUN   TestUnblockCounts
  --- PASS: TestUnblockCounts
  === RUN   TestUnblockCountsIgnoreClosedDownstream
  --- PASS: TestUnblockCountsIgnoreClosedDownstream
  === RUN   TestWouldCycle
  --- PASS: TestWouldCycle
  === RUN   TestFilterLabels
  --- PASS: TestFilterLabels
  === RUN   TestDanglingIsQuarantinedNotBlocked
  --- PASS: TestDanglingIsQuarantinedNotBlocked
  PASS
  ok  	github.com/eisenwinter/awit/pkg/graph
  ```

  Full package (Build + Tarjan + rank):

  ```bash
  go test ./pkg/graph -count=1
  gofmt -w pkg/graph/rank.go pkg/graph/rank_test.go pkg/graph/graph.go
  git add pkg/graph/rank.go pkg/graph/rank_test.go pkg/graph/graph.go
  git commit -m "graph: classify, unblock counts, WouldCycle, FilterLabels"
  ```

  `gofmt -l pkg/graph` must print nothing.

## Acceptance Criteria
- `go test ./pkg/graph -count=1` passes.
- Clean fixture: `Ready()` is `0001`, `0002`, `0006` in that order; `Blocked()` is `0003`, `0004`; `Closed()` is `0005`; `Quarantined()` is empty.
- Clean fixture unblock counts: `0001→2`, `0003→1`, `0004→0`.
- Hand-built A(open)→B(closed)→C(open) over Unblocks: A count 1.
- `WouldCycle(0001,0004)` = `[AWIT-TEST0001, AWIT-TEST0004, AWIT-TEST0001]`; `WouldCycle(0002,0001)` = nil; self = `[x, x]`; unknown = nil.
- `FilterLabels`: AND across groups, OR within a group; nil/empty groups return the input slice unchanged.
- Dangling fixture: `0001` quarantined (not blocked) with `UnblockCount == -1`; `0002` Blocked with `OpenDepIDs() == [AWIT-TEST0001]`.
- `classify` and `countUnblocks` are defined once, in `rank.go`. `Build` still calls `g.detectCycles(); g.classify(); g.countUnblocks()` in that order.

## Out of scope
- `CriticalPath` — `AWIT-0ND56V3G`.
- CLI `dep add` / `dep rm` / `validate` / `prime` / `next` / `list`.
- Changing Tarjan or fixture files.
- Re-implementing `Build`, dangling detection, or Broken carry-through.
