---
id: AWIT-0ND56P3G
title: pkg/graph Tarjan SCC quarantine with example chain
brief: >-
  Replace the detectCycles seam with Tarjan SCC over Node.Deps. Every SCC of
  size greater than 1, or a size-1 self-edge, quarantines every member with
  ReasonCycle, records one graph-level Fault, and prints a deterministic
  example chain plus a dep-rm fix.
status: closed
deps: [AWIT-0ND56N3G]
labels: [phase2, p0]
refs:
  - ../../plan/implementation-guide.md
  - ../../plan/awit-implementation-plan.md
---

## Summary
After this ticket `pkg/graph/scc.go` contains Tarjan's strongly connected components algorithm over `Node.Deps`. `Build` still calls `g.detectCycles()` (the call site in `graph.go` does not change). The empty seam body is deleted from `graph.go` and the method now lives in `scc.go`. An SCC of size > 1, or of size 1 with a self-edge, puts `ReasonCycle` on **every member's** `Node.Faults` and **once** on `g.Faults`. Detail is the example chain starting at the smallest member ID and following `Deps` inside the SCC until that ID repeats: `A -> B -> C -> A`. A self-loop is `A -> A`. Fix is `awit dep rm <A> <B> (break the cycle)` using the first edge of that chain. Classification, unblock counts, `WouldCycle`, and `FilterLabels` stay untouched.

## Context (read first)
- Guide §4.6 `pkg/graph` — `Fault`, `Node`, `Graph`, `Build`. This ticket does not change those signatures.
- Guide §1: stdlib only on top of `pkg/item`; tests use `testing`; determinism (no map iteration order in output). Iterate `g.Order` (already ID-sorted) as Tarjan's outer loop so SCCs and chains are stable.
- Guide §2 / spec §Graph engine: "Cycle detection on load | Tarjan SCC | One report per SCC; the DFS back-edge path is used only to print one example chain."
- Guide §8 `cyclic` fixture (already on disk from `AWIT-0ND56N3G`):
  - `TEST0001` deps `0002`; `TEST0002` deps `0003`; `TEST0003` deps `0001` — one 3-cycle.
  - `TEST0004` deps `0004` — one self-loop.
  - `TEST0005` clean — must **not** be quarantined.
- Spec error contract is about `dep add` (next tickets). This ticket only quarantines on load.
- `pkg/graph/graph.go` already has `Build`, types, dangling/broken faults, `sortFaults` after the three seam calls, and empty `detectCycles` / `classify` / `countUnblocks`. `pkg/graph/graph_test.go` already defines `buildFixture`, `nodeIDs`, `depIDs` in package `graph`. Do **not** redeclare those helpers (same package). Do **not** redeclare `detectCycles` in `graph.go` after you move it.
- Reason constant: `item.ReasonCycle = "CYCLE"`.
- Example chain algorithm (deterministic, no shortest-path search):
  1. Collect member IDs, sort with `sort.Strings`. The start vertex is `ids[0]` (smallest).
  2. Walk: from the current node, take the **first** `Node.Deps` entry whose ID is in the SCC member set. Append it. Stop when that ID equals the start (the closing edge) or no in-SCC dep remains.
  3. Join with `" -> "` (space-arrow-space).
  4. Self-loop: start's first in-SCC dep is itself, so the chain is `A -> A`.
- Fix: split the chain on `" -> "`; `A` is `parts[0]`, `B` is `parts[1]` (for a self-loop both are `A`). Format exactly `awit dep rm %s %s (break the cycle)` including the parenthetical.
- Graph-level Fault `IDs` = all SCC members, sorted. The same `Fault` value is appended to each member's `Node.Faults`. `g.Faults` gets that Fault **once** (not once per member).
- Size-1 SCC **without** a self-edge is not a cycle. Leave those nodes alone.
- `Build` already calls `sortFaults` after `detectCycles`, so you append and let it sort. Do not sort inside `detectCycles`.
- `classify` and `countUnblocks` remain empty seams in `graph.go`.

## Files
- Create: `pkg/graph/scc.go`
- Create: `pkg/graph/scc_test.go`
- Modify: `pkg/graph/graph.go` — **delete** the empty `func (g *Graph) detectCycles() {}` method. Leave the `g.detectCycles()` call in `Build` and leave the empty `classify` / `countUnblocks` methods.

## Interfaces
- Consumes (already in `pkg/graph/graph.go` from `AWIT-0ND56N3G`):
  ```go
  package graph

  type Fault struct {
      Reason item.Reason
      IDs    []string
      Detail string
      Fix    string
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
  type Graph struct {
      Nodes  map[string]*Node
      Order  []*Node
      Broken []item.Broken
      Faults []Fault
  }
  func Build(items []*item.Item, broken []item.Broken) *Graph
  ```
- Consumes (test helpers already in `pkg/graph/graph_test.go`; same package `graph`, do not copy into `scc_test.go`):
  ```go
  func buildFixture(t *testing.T, name string) *Graph
  func nodeIDs(nodes []*Node) []string
  ```
- Produces (package-private, this ticket, in `scc.go`):
  ```go
  func (g *Graph) detectCycles()
  func exampleChain(scc []*Node) string
  func sccMemberIDs(scc []*Node) []string
  func sccHasSelfEdge(n *Node) bool
  ```
- Does not produce: `Ready`, `Blocked`, `WouldCycle`, `FilterLabels`, `CriticalPath`, any CLI.

## Steps

- [ ] **Step 1: Write the failing tests.**

  Create `pkg/graph/scc_test.go`:

  ```go
  package graph

  import (
  	"slices"
  	"testing"

  	"github.com/eisenwinter/awit/pkg/item"
  )

  func mk(id string, deps ...string) *item.Item {
  	return &item.Item{
  		ID:     id,
  		Title:  id,
  		Status: item.StatusOpen,
  		Deps:   deps,
  	}
  }

  func cycleFaults(g *Graph) []Fault {
  	var out []Fault
  	for _, f := range g.Faults {
  		if f.Reason == item.ReasonCycle {
  			out = append(out, f)
  		}
  	}
  	return out
  }

  func TestCycleFaults(t *testing.T) {
  	g := buildFixture(t, "cyclic")

  	cf := cycleFaults(g)
  	if len(cf) != 2 {
  		t.Fatalf("CYCLE faults on graph = %d, want 2", len(cf))
  	}

  	wantTriangle := "AWIT-TEST0001 -> AWIT-TEST0002 -> AWIT-TEST0003 -> AWIT-TEST0001"
  	wantSelf := "AWIT-TEST0004 -> AWIT-TEST0004"
  	wantTriangleFix := "awit dep rm AWIT-TEST0001 AWIT-TEST0002 (break the cycle)"
  	wantSelfFix := "awit dep rm AWIT-TEST0004 AWIT-TEST0004 (break the cycle)"

  	for _, id := range []string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0003"} {
  		n := g.Nodes[id]
  		if n == nil {
  			t.Fatalf("missing node %s", id)
  		}
  		if !n.Quarantined() {
  			t.Fatalf("node %s Quarantined() = false, want true", id)
  		}
  		var found *Fault
  		for i := range n.Faults {
  			if n.Faults[i].Reason == item.ReasonCycle {
  				found = &n.Faults[i]
  				break
  			}
  		}
  		if found == nil {
  			t.Fatalf("node %s has no CYCLE fault", id)
  		}
  		if found.Detail != wantTriangle {
  			t.Fatalf("node %s Detail = %q, want %q", id, found.Detail, wantTriangle)
  		}
  		if found.Fix != wantTriangleFix {
  			t.Fatalf("node %s Fix = %q, want %q", id, found.Fix, wantTriangleFix)
  		}
  		if !slices.Equal(found.IDs, []string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0003"}) {
  			t.Fatalf("node %s IDs = %v, want sorted triangle", id, found.IDs)
  		}
  	}

  	n4 := g.Nodes["AWIT-TEST0004"]
  	if n4 == nil || !n4.Quarantined() {
  		t.Fatal("0004 must exist and be quarantined")
  	}
  	var self *Fault
  	for i := range n4.Faults {
  		if n4.Faults[i].Reason == item.ReasonCycle {
  			self = &n4.Faults[i]
  			break
  		}
  	}
  	if self == nil {
  		t.Fatal("0004 has no CYCLE fault")
  	}
  	if self.Detail != wantSelf {
  		t.Fatalf("0004 Detail = %q, want %q", self.Detail, wantSelf)
  	}
  	if self.Fix != wantSelfFix {
  		t.Fatalf("0004 Fix = %q, want %q", self.Fix, wantSelfFix)
  	}
  	if !slices.Equal(self.IDs, []string{"AWIT-TEST0004"}) {
  		t.Fatalf("0004 IDs = %v, want [AWIT-TEST0004]", self.IDs)
  	}

  	n5 := g.Nodes["AWIT-TEST0005"]
  	if n5 == nil {
  		t.Fatal("missing 0005")
  	}
  	if n5.Quarantined() {
  		t.Fatal("0005 Quarantined() = true, want false")
  	}

  	if cf[0].Detail != wantTriangle {
  		t.Fatalf("g.Faults[0] Detail = %q, want triangle %q", cf[0].Detail, wantTriangle)
  	}
  	if cf[1].Detail != wantSelf {
  		t.Fatalf("g.Faults[1] Detail = %q, want self %q", cf[1].Detail, wantSelf)
  	}
  }

  func TestTarjanUnit(t *testing.T) {
  	// Two disjoint cycles plus a 3-node chain that must stay clean.
  	// Cycle A: AA01 -> AA02 -> AA01
  	// Cycle B: BB01 -> BB02 -> BB03 -> BB01
  	// Chain:   CC01 <- CC02 <- CC03  (CC02 deps CC01, CC03 deps CC02)
  	items := []*item.Item{
  		mk("AWIT-TESTAA02", "AWIT-TESTAA01"),
  		mk("AWIT-TESTAA01", "AWIT-TESTAA02"),
  		mk("AWIT-TESTBB01", "AWIT-TESTBB02"),
  		mk("AWIT-TESTBB02", "AWIT-TESTBB03"),
  		mk("AWIT-TESTBB03", "AWIT-TESTBB01"),
  		mk("AWIT-TESTCC01"),
  		mk("AWIT-TESTCC02", "AWIT-TESTCC01"),
  		mk("AWIT-TESTCC03", "AWIT-TESTCC02"),
  	}
  	g := Build(items, nil)

  	cf := cycleFaults(g)
  	if len(cf) != 2 {
  		t.Fatalf("CYCLE faults = %d, want 2 (one per SCC)", len(cf))
  	}

  	wantAA := "AWIT-TESTAA01 -> AWIT-TESTAA02 -> AWIT-TESTAA01"
  	wantBB := "AWIT-TESTBB01 -> AWIT-TESTBB02 -> AWIT-TESTBB03 -> AWIT-TESTBB01"
  	var gotAA, gotBB bool
  	for _, f := range cf {
  		switch f.Detail {
  		case wantAA:
  			gotAA = true
  			if f.Fix != "awit dep rm AWIT-TESTAA01 AWIT-TESTAA02 (break the cycle)" {
  				t.Fatalf("AA Fix = %q", f.Fix)
  			}
  			if !slices.Equal(f.IDs, []string{"AWIT-TESTAA01", "AWIT-TESTAA02"}) {
  				t.Fatalf("AA IDs = %v", f.IDs)
  			}
  		case wantBB:
  			gotBB = true
  			if f.Fix != "awit dep rm AWIT-TESTBB01 AWIT-TESTBB02 (break the cycle)" {
  				t.Fatalf("BB Fix = %q", f.Fix)
  			}
  			if !slices.Equal(f.IDs, []string{"AWIT-TESTBB01", "AWIT-TESTBB02", "AWIT-TESTBB03"}) {
  				t.Fatalf("BB IDs = %v", f.IDs)
  			}
  		default:
  			t.Fatalf("unexpected CYCLE Detail %q", f.Detail)
  		}
  	}
  	if !gotAA || !gotBB {
  		t.Fatalf("missing cycle detail: AA=%v BB=%v", gotAA, gotBB)
  	}

  	for _, id := range []string{"AWIT-TESTAA01", "AWIT-TESTAA02", "AWIT-TESTBB01", "AWIT-TESTBB02", "AWIT-TESTBB03"} {
  		if !g.Nodes[id].Quarantined() {
  			t.Fatalf("%s not quarantined", id)
  		}
  	}
  	for _, id := range []string{"AWIT-TESTCC01", "AWIT-TESTCC02", "AWIT-TESTCC03"} {
  		if g.Nodes[id].Quarantined() {
  			t.Fatalf("%s quarantined, want chain nodes clean", id)
  		}
  	}
  }

  func TestCycleDetectionDeterministic(t *testing.T) {
  	g1 := buildFixture(t, "cyclic")
  	g2 := buildFixture(t, "cyclic")
  	c1, c2 := cycleFaults(g1), cycleFaults(g2)
  	if len(c1) != len(c2) {
  		t.Fatalf("fault counts %d vs %d", len(c1), len(c2))
  	}
  	for i := range c1 {
  		if c1[i].Reason != c2[i].Reason || c1[i].Detail != c2[i].Detail || c1[i].Fix != c2[i].Fix {
  			t.Fatalf("fault %d differs: %+v vs %+v", i, c1[i], c2[i])
  		}
  		if !slices.Equal(c1[i].IDs, c2[i].IDs) {
  			t.Fatalf("fault %d IDs %v vs %v", i, c1[i].IDs, c2[i].IDs)
  		}
  	}
  	for _, n := range g1.Order {
  		o := g2.Nodes[n.Item.ID]
  		if n.Quarantined() != o.Quarantined() {
  			t.Fatalf("%s quarantined %v vs %v", n.Item.ID, n.Quarantined(), o.Quarantined())
  		}
  	}
  }
  ```

- [ ] **Step 2: Run it, see it fail.**

  ```bash
  go test ./pkg/graph -run 'TestCycleFaults|TestTarjanUnit|TestCycleDetectionDeterministic' -v
  ```

  Expected failure (empty `detectCycles` seam: cyclic items wire as ordinary edges, nobody is quarantined):

  ```text
  === RUN   TestCycleFaults
      scc_test.go: CYCLE faults on graph = 0, want 2
  --- FAIL: TestCycleFaults
  FAIL
  ```

- [ ] **Step 3: Delete the empty seam and implement Tarjan in `scc.go`.**

  In `pkg/graph/graph.go`, delete **only** this method (keep the `g.detectCycles()` call inside `Build`):

  ```go
  func (g *Graph) detectCycles()  {}
  ```

  Create `pkg/graph/scc.go` with this complete file (copy it verbatim):

  ```go
  package graph

  import (
  	"fmt"
  	"sort"
  	"strings"

  	"github.com/eisenwinter/awit/pkg/item"
  )

  // detectCycles runs Tarjan SCC over Node.Deps and quarantines every cyclic
  // component. It is the body of the seam Build already calls.
  func (g *Graph) detectCycles() {
  	index := 0
  	indices := make(map[string]int, len(g.Nodes))
  	lowlink := make(map[string]int, len(g.Nodes))
  	onStack := make(map[string]bool, len(g.Nodes))
  	var stack []*Node

  	var strongconnect func(n *Node)
  	strongconnect = func(n *Node) {
  		id := n.Item.ID
  		indices[id] = index
  		lowlink[id] = index
  		index++
  		stack = append(stack, n)
  		onStack[id] = true

  		for _, dep := range n.Deps {
  			did := dep.Item.ID
  			if _, seen := indices[did]; !seen {
  				strongconnect(dep)
  				if lowlink[did] < lowlink[id] {
  					lowlink[id] = lowlink[did]
  				}
  			} else if onStack[did] {
  				if indices[did] < lowlink[id] {
  					lowlink[id] = indices[did]
  				}
  			}
  		}

  		if lowlink[id] != indices[id] {
  			return
  		}
  		var scc []*Node
  		for {
  			w := stack[len(stack)-1]
  			stack = stack[:len(stack)-1]
  			onStack[w.Item.ID] = false
  			scc = append(scc, w)
  			if w == n {
  				break
  			}
  		}
  		g.quarantineIfCyclic(scc)
  	}

  	for _, n := range g.Order {
  		if _, seen := indices[n.Item.ID]; !seen {
  			strongconnect(n)
  		}
  	}
  }

  func (g *Graph) quarantineIfCyclic(scc []*Node) {
  	if len(scc) > 1 || (len(scc) == 1 && sccHasSelfEdge(scc[0])) {
  		ids := sccMemberIDs(scc)
  		detail := exampleChain(scc)
  		parts := strings.Split(detail, " -> ")
  		b := parts[0]
  		if len(parts) > 1 {
  			b = parts[1]
  		}
  		f := Fault{
  			Reason: item.ReasonCycle,
  			IDs:    ids,
  			Detail: detail,
  			Fix:    fmt.Sprintf("awit dep rm %s %s (break the cycle)", parts[0], b),
  		}
  		for _, n := range scc {
  			n.Faults = append(n.Faults, f)
  		}
  		g.Faults = append(g.Faults, f)
  	}
  }

  func sccHasSelfEdge(n *Node) bool {
  	for _, d := range n.Deps {
  		if d.Item.ID == n.Item.ID {
  			return true
  		}
  	}
  	return false
  }

  func sccMemberIDs(scc []*Node) []string {
  	ids := make([]string, len(scc))
  	for i, n := range scc {
  		ids[i] = n.Item.ID
  	}
  	sort.Strings(ids)
  	return ids
  }

  func exampleChain(scc []*Node) string {
  	members := make(map[string]*Node, len(scc))
  	ids := make([]string, 0, len(scc))
  	for _, n := range scc {
  		members[n.Item.ID] = n
  		ids = append(ids, n.Item.ID)
  	}
  	sort.Strings(ids)
  	start := ids[0]
  	parts := []string{start}
  	cur := start
  	for {
  		n := members[cur]
  		next := ""
  		for _, d := range n.Deps {
  			if _, ok := members[d.Item.ID]; ok {
  				next = d.Item.ID
  				break
  			}
  		}
  		if next == "" {
  			break
  		}
  		parts = append(parts, next)
  		if next == start {
  			break
  		}
  		cur = next
  	}
  	return strings.Join(parts, " -> ")
  }
  ```

  Implementation rules:
  - Walk **`Node.Deps`**, not `Item.Deps`. Dangling IDs are already excluded from `Node.Deps`.
  - Outer Tarjan loop is `g.Order` (ID ascending). That is what makes `TestCycleDetectionDeterministic` hold.
  - `exampleChain` starts at the smallest member ID and follows the first in-SCC `Deps` neighbour. Do not search for a shortest cycle.
  - One graph-level CYCLE fault per cyclic SCC. Every member also receives that same Fault on `Node.Faults` so `Quarantined()` is true.
  - Do not touch `classify` or `countUnblocks`.
  - Do not import anything outside stdlib and `pkg/item`.

- [ ] **Step 4: Run the tests, see them pass, commit.**

  ```bash
  go test ./pkg/graph -run 'TestCycleFaults|TestTarjanUnit|TestCycleDetectionDeterministic' -v
  ```

  Expected:

  ```text
  === RUN   TestCycleFaults
  --- PASS: TestCycleFaults
  === RUN   TestTarjanUnit
  --- PASS: TestTarjanUnit
  === RUN   TestCycleDetectionDeterministic
  --- PASS: TestCycleDetectionDeterministic
  PASS
  ok  	github.com/eisenwinter/awit/pkg/graph
  ```

  Then the whole package, including the Build tests from the previous ticket:

  ```bash
  go test ./pkg/graph -count=1
  gofmt -w pkg/graph/scc.go pkg/graph/scc_test.go pkg/graph/graph.go
  git add pkg/graph/scc.go pkg/graph/scc_test.go pkg/graph/graph.go
  git commit -m "graph: Tarjan SCC quarantine with example chain"
  ```

  `gofmt -l pkg/graph` must print nothing. Confirm `graph.go` no longer contains a `detectCycles` method body (the identifier may appear only as the `g.detectCycles()` call). Two method definitions of `detectCycles` will not compile.

## Acceptance Criteria
- `go test ./pkg/graph -count=1` passes.
- Cyclic fixture: `g.Faults` contains **two** `CYCLE` faults (not six). Triangle nodes `0001`/`0002`/`0003` each have Detail `AWIT-TEST0001 -> AWIT-TEST0002 -> AWIT-TEST0003 -> AWIT-TEST0001` and Fix `awit dep rm AWIT-TEST0001 AWIT-TEST0002 (break the cycle)`. `0004` has Detail `AWIT-TEST0004 -> AWIT-TEST0004` and Fix `awit dep rm AWIT-TEST0004 AWIT-TEST0004 (break the cycle)`. `0005` is not quarantined.
- `TestTarjanUnit`: two disjoint cycles plus a chain; five quarantined nodes; two graph-level CYCLE faults; chain nodes clean.
- Two `Build`s of the cyclic fixture produce identical CYCLE Reason/IDs/Detail/Fix slices.
- `detectCycles` is defined once, in `scc.go`. `Build` still calls `g.detectCycles(); g.classify(); g.countUnblocks()` in that order.

## Out of scope
- Filling `classify` or `countUnblocks`.
- `Ready`, `Blocked`, `Quarantined()`, `Closed`, `WouldCycle`, `FilterLabels` methods on `Graph`.
- `CriticalPath`.
- CLI `dep add` cycle pre-check (that uses `WouldCycle`, next tickets).
- Changing fixture files.
