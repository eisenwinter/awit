package graph

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/eisenwinter/awit/pkg/item"
)

func fixtureRoot(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "fixtures", name)
}

func buildFixture(t *testing.T, name string) *Graph {
	t.Helper()
	st, err := item.Open(fixtureRoot(t, name))
	if err != nil {
		t.Fatalf("item.Open(%s): %v", name, err)
	}
	items, broken, err := st.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll(%s): %v", name, err)
	}
	return Build(items, broken)
}

func nodeIDs(nodes []*Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Item.ID
	}
	return out
}

func unblockIDs(n *Node) []string {
	out := make([]string, len(n.Unblocks))
	for i, u := range n.Unblocks {
		out[i] = u.Item.ID
	}
	return out
}

func depIDs(n *Node) []string {
	out := make([]string, len(n.Deps))
	for i, d := range n.Deps {
		out[i] = d.Item.ID
	}
	return out
}

func TestBuildClean(t *testing.T) {
	g := buildFixture(t, "clean")
	if len(g.Nodes) != 6 {
		t.Fatalf("len(Nodes) = %d, want 6", len(g.Nodes))
	}
	wantOrder := []string{
		"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0003",
		"AWIT-TEST0004", "AWIT-TEST0005", "AWIT-TEST0006",
	}
	if got := nodeIDs(g.Order); !slices.Equal(got, wantOrder) {
		t.Fatalf("Order IDs = %v, want %v", got, wantOrder)
	}
	n1 := g.Nodes["AWIT-TEST0001"]
	n3 := g.Nodes["AWIT-TEST0003"]
	n4 := g.Nodes["AWIT-TEST0004"]
	n5 := g.Nodes["AWIT-TEST0005"]
	n6 := g.Nodes["AWIT-TEST0006"]
	if n1 == nil || n3 == nil || n4 == nil || n5 == nil || n6 == nil {
		t.Fatalf("missing node in clean graph: %+v", nodeIDs(g.Order))
	}
	if got := depIDs(n3); !slices.Equal(got, []string{"AWIT-TEST0001"}) {
		t.Fatalf("0003.Deps = %v, want [AWIT-TEST0001]", got)
	}
	if got := depIDs(n4); !slices.Equal(got, []string{"AWIT-TEST0001", "AWIT-TEST0003"}) {
		t.Fatalf("0004.Deps = %v, want [AWIT-TEST0001 AWIT-TEST0003]", got)
	}
	gotUnblocks := unblockIDs(n1)
	if !slices.Contains(gotUnblocks, "AWIT-TEST0003") || !slices.Contains(gotUnblocks, "AWIT-TEST0004") {
		t.Fatalf("0001.Unblocks = %v, want to contain 0003 and 0004", gotUnblocks)
	}
	if n1.Quarantined() {
		t.Fatal("0001 Quarantined() = true, want false")
	}
	if n5.Item.Status != item.StatusClosed {
		t.Fatalf("0005 status = %q, want closed", n5.Item.Status)
	}
	if n6.Item.Assignee != "agent/claude" {
		t.Fatalf("0006 assignee = %q, want agent/claude", n6.Item.Assignee)
	}
	if got := n4.DepIDs(); !slices.Equal(got, []string{"AWIT-TEST0001", "AWIT-TEST0003"}) {
		t.Fatalf("0004.DepIDs() = %v, want sorted [0001 0003]", got)
	}
	if got := n4.OpenDepIDs(); !slices.Equal(got, []string{"AWIT-TEST0001", "AWIT-TEST0003"}) {
		t.Fatalf("0004.OpenDepIDs() = %v, want [0001 0003] (both open)", got)
	}
	if got := n6.OpenDepIDs(); len(got) != 0 {
		t.Fatalf("0006.OpenDepIDs() = %v, want empty (0005 is closed)", got)
	}
	if len(g.Broken) != 0 {
		t.Fatalf("Broken = %d, want 0", len(g.Broken))
	}
	if len(g.Faults) != 0 {
		t.Fatalf("Faults = %d, want 0", len(g.Faults))
	}
}

func TestBuildDangling(t *testing.T) {
	g := buildFixture(t, "dangling")
	n1 := g.Nodes["AWIT-TEST0001"]
	n2 := g.Nodes["AWIT-TEST0002"]
	if n1 == nil || n2 == nil {
		t.Fatal("dangling fixture missing 0001 or 0002")
	}
	if !n1.Quarantined() {
		t.Fatal("0001 Quarantined() = false, want true")
	}
	if n2.Quarantined() {
		t.Fatal("0002 Quarantined() = true, want false")
	}
	if len(n1.Faults) != 1 {
		t.Fatalf("0001 Faults = %d, want 1", len(n1.Faults))
	}
	f := n1.Faults[0]
	if f.Reason != item.ReasonDangling {
		t.Fatalf("reason = %q, want %q", f.Reason, item.ReasonDangling)
	}
	wantDetail := "AWIT-TEST0001 depends on unknown AWIT-TEST9999"
	if f.Detail != wantDetail {
		t.Fatalf("Detail = %q, want %q", f.Detail, wantDetail)
	}
	wantFix := "awit dep rm AWIT-TEST0001 AWIT-TEST9999"
	if f.Fix != wantFix {
		t.Fatalf("Fix = %q, want %q", f.Fix, wantFix)
	}
	if !slices.Equal(f.IDs, []string{"AWIT-TEST0001"}) {
		t.Fatalf("IDs = %v, want [AWIT-TEST0001]", f.IDs)
	}
	if len(n1.Deps) != 0 {
		t.Fatalf("0001.Deps = %v, want empty (dangling excluded)", depIDs(n1))
	}
	if got := n1.OpenDepIDs(); !slices.Equal(got, []string{"AWIT-TEST9999"}) {
		t.Fatalf("0001.OpenDepIDs() = %v, want [AWIT-TEST9999]", got)
	}
	if got := depIDs(n2); !slices.Equal(got, []string{"AWIT-TEST0001"}) {
		t.Fatalf("0002.Deps = %v, want [AWIT-TEST0001]", got)
	}
	var dangling []Fault
	for _, ff := range g.Faults {
		if ff.Reason == item.ReasonDangling {
			dangling = append(dangling, ff)
		}
	}
	if len(dangling) != 1 {
		t.Fatalf("g.Faults dangling count = %d, want 1", len(dangling))
	}
	if dangling[0].Fix != wantFix {
		t.Fatalf("g.Faults Fix = %q, want %q", dangling[0].Fix, wantFix)
	}
}

func TestBuildDependsOnBroken(t *testing.T) {
	items := []*item.Item{{
		ID:     "AWIT-TEST0002",
		Title:  "ok",
		Status: item.StatusOpen,
		Deps:   []string{"AWIT-TEST0001"},
	}}
	broken := []item.Broken{{
		ID:     "AWIT-TEST0001",
		Path:   filepath.Join("x", "AWIT-TEST0001.md"),
		Reason: item.ReasonParse,
		Detail: "yaml: boom",
	}}
	g := Build(items, broken)
	n := g.Nodes["AWIT-TEST0002"]
	if n == nil {
		t.Fatal("missing depender node")
	}
	if !n.Quarantined() {
		t.Fatal("depender Quarantined() = false, want true")
	}
	if len(n.Deps) != 0 {
		t.Fatalf("Deps = %v, want empty (broken ID is not a node)", depIDs(n))
	}
	if len(n.Faults) != 1 {
		t.Fatalf("Faults = %d, want 1", len(n.Faults))
	}
	f := n.Faults[0]
	if f.Reason != item.ReasonDangling {
		t.Fatalf("reason = %q, want %q", f.Reason, item.ReasonDangling)
	}
	wantDetail := "depends on quarantined AWIT-TEST0001"
	if f.Detail != wantDetail {
		t.Fatalf("Detail = %q, want %q", f.Detail, wantDetail)
	}
	wantFix := "fix AWIT-TEST0001 first"
	if f.Fix != wantFix {
		t.Fatalf("Fix = %q, want %q", f.Fix, wantFix)
	}
}

func TestBuildCarriesBroken(t *testing.T) {
	tests := []struct {
		fixture string
		reason  item.Reason
		id      string
		fix     func(b item.Broken) string
		healthy []string
	}{
		{
			fixture: "conflicted",
			reason:  item.ReasonConflict,
			id:      "AWIT-TEST0001",
			fix: func(b item.Broken) string {
				return "resolve the git conflict in " + b.Path
			},
			healthy: []string{"AWIT-TEST0002"},
		},
		{
			fixture: "id-mismatch",
			reason:  item.ReasonIDMismatch,
			id:      "AWIT-TEST0001",
			fix: func(item.Broken) string {
				return "rename the file or fix the id: key"
			},
		},
		{
			fixture: "parse-error",
			reason:  item.ReasonParse,
			id:      "AWIT-TEST0001",
			fix: func(item.Broken) string {
				return "edit the frontmatter until `awit validate` passes"
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			g := buildFixture(t, tt.fixture)
			if len(g.Broken) != 1 {
				t.Fatalf("len(Broken) = %d, want 1", len(g.Broken))
			}
			b := g.Broken[0]
			if b.ID != tt.id {
				t.Fatalf("Broken.ID = %q, want %q", b.ID, tt.id)
			}
			if b.Reason != tt.reason {
				t.Fatalf("Broken.Reason = %q, want %q", b.Reason, tt.reason)
			}
			var found *Fault
			for i := range g.Faults {
				if g.Faults[i].Reason == tt.reason {
					found = &g.Faults[i]
					break
				}
			}
			if found == nil {
				t.Fatalf("no %q fault in g.Faults (%v)", tt.reason, g.Faults)
			}
			if !slices.Equal(found.IDs, []string{tt.id}) {
				t.Fatalf("Fault.IDs = %v, want [%s]", found.IDs, tt.id)
			}
			if found.Detail != b.Detail {
				t.Fatalf("Fault.Detail = %q, want Broken.Detail %q", found.Detail, b.Detail)
			}
			wantFix := tt.fix(b)
			if found.Fix != wantFix {
				t.Fatalf("Fault.Fix = %q, want %q", found.Fix, wantFix)
			}
			if g.Nodes[tt.id] != nil {
				t.Fatalf("Nodes[%s] exists; Broken files must not become nodes", tt.id)
			}
			if got := nodeIDs(g.Order); !slices.Equal(got, tt.healthy) {
				t.Fatalf("healthy Order = %v, want %v", got, tt.healthy)
			}
		})
	}
}

func TestBuildCarriesDuplicate(t *testing.T) {
	root := fixtureRoot(t, "duplicate-id")
	ents, err := os.ReadDir(filepath.Join(root, ".awit", "items"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) < 2 {
		t.Skip("duplicate-id fixture collapsed on a case-insensitive filesystem")
	}
	g := buildFixture(t, "duplicate-id")
	if len(g.Broken) != 2 {
		t.Fatalf("len(Broken) = %d, want 2", len(g.Broken))
	}
	for _, b := range g.Broken {
		if b.Reason != item.ReasonDuplicate {
			t.Fatalf("Broken %s reason = %q, want %q", b.ID, b.Reason, item.ReasonDuplicate)
		}
	}
	var n int
	for _, f := range g.Faults {
		if f.Reason != item.ReasonDuplicate {
			continue
		}
		n++
		if f.Fix != "rename one of the files" {
			t.Fatalf("Fix = %q, want %q", f.Fix, "rename one of the files")
		}
	}
	if n != 2 {
		t.Fatalf("duplicate faults = %d, want 2", n)
	}
	if len(g.Nodes) != 0 {
		t.Fatalf("len(Nodes) = %d, want 0", len(g.Nodes))
	}
}

func TestFaultsSorted(t *testing.T) {
	items := []*item.Item{{
		ID:     "AWIT-TEST0002",
		Title:  "dangling",
		Status: item.StatusOpen,
		Deps:   []string{"AWIT-MISSING"},
	}}
	broken := []item.Broken{
		{ID: "AWIT-TEST0005", Reason: item.ReasonConflict, Detail: "c5", Path: "p5"},
		{ID: "AWIT-TEST0001", Reason: item.ReasonConflict, Detail: "c1", Path: "p1"},
		{ID: "AWIT-TEST0004", Reason: item.ReasonIDMismatch, Detail: "m", Path: "p4"},
		{ID: "AWIT-TEST0003", Reason: item.ReasonParse, Detail: "p", Path: "p3"},
	}
	g := Build(items, broken)
	got := make([]string, len(g.Faults))
	for i, f := range g.Faults {
		got[i] = string(f.Reason) + ":" + f.IDs[0]
	}
	want := []string{
		"CONFLICT MARKERS:AWIT-TEST0001",
		"CONFLICT MARKERS:AWIT-TEST0005",
		"DANGLING DEP:AWIT-TEST0002",
		"ID MISMATCH:AWIT-TEST0004",
		"PARSE ERROR:AWIT-TEST0003",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Faults = %v, want %v", got, want)
	}
}

func TestNodeDepIDsSorted(t *testing.T) {
	a := &item.Item{ID: "AWIT-TEST0001", Title: "a", Status: item.StatusOpen, Deps: []string{"AWIT-TEST0003", "AWIT-TEST0002"}}
	b := &item.Item{ID: "AWIT-TEST0002", Title: "b", Status: item.StatusOpen}
	c := &item.Item{ID: "AWIT-TEST0003", Title: "c", Status: item.StatusClosed}
	g := Build([]*item.Item{a, b, c}, nil)
	n := g.Nodes["AWIT-TEST0001"]
	if got := n.DepIDs(); !slices.Equal(got, []string{"AWIT-TEST0002", "AWIT-TEST0003"}) {
		t.Fatalf("DepIDs() = %v, want sorted", got)
	}
	if got := n.OpenDepIDs(); !slices.Equal(got, []string{"AWIT-TEST0002"}) {
		t.Fatalf("OpenDepIDs() = %v, want [0002] (0003 is closed)", got)
	}
	if got := depIDs(n); !slices.Equal(got, []string{"AWIT-TEST0003", "AWIT-TEST0002"}) {
		t.Fatalf("Node.Deps order = %v, want Item.Deps order", got)
	}
}

func TestLoopFixtureLoads(t *testing.T) {
	g := buildFixture(t, "loop")
	if got := nodeIDs(g.Order); !slices.Equal(got, []string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0003"}) {
		t.Fatalf("Order = %v", got)
	}
	n1 := g.Nodes["AWIT-TEST0001"]
	if !slices.Equal(n1.Item.Refs, []string{"../../docs/spec.md"}) {
		t.Fatalf("0001.Refs = %v, want [../../docs/spec.md]", n1.Item.Refs)
	}
	if got := depIDs(g.Nodes["AWIT-TEST0002"]); !slices.Equal(got, []string{"AWIT-TEST0001"}) {
		t.Fatalf("0002.Deps = %v", got)
	}
	if got := depIDs(g.Nodes["AWIT-TEST0003"]); !slices.Equal(got, []string{"AWIT-TEST0002"}) {
		t.Fatalf("0003.Deps = %v", got)
	}
}
