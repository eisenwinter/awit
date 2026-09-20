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

func TestManualBlockZeroDepHeld(t *testing.T) {
	held := &item.Item{ID: "AWIT-TEST0001", Title: "A", Status: item.StatusOpen, BlockedReason: "waiting on vendor"}
	labelOnly := &item.Item{ID: "AWIT-TEST0002", Title: "B", Status: item.StatusOpen, Labels: []string{"blocked"}}
	g := Build([]*item.Item{held, labelOnly}, nil)
	n1 := g.Nodes["AWIT-TEST0001"]
	if n1.Quarantined() {
		t.Fatal("held item Quarantined() = true, want false")
	}
	if n1.Ready || !n1.Blocked {
		t.Fatalf("held Ready/Blocked = %v/%v, want false/true", n1.Ready, n1.Blocked)
	}
	n2 := g.Nodes["AWIT-TEST0002"]
	if !n2.Ready || n2.Blocked {
		t.Fatalf("label-only Ready/Blocked = %v/%v, want true/false", n2.Ready, n2.Blocked)
	}
	if got := nodeIDs(g.Ready()); !slices.Equal(got, []string{"AWIT-TEST0002"}) {
		t.Fatalf("Ready() = %v, want [AWIT-TEST0002]", got)
	}
	if got := nodeIDs(g.Blocked()); !slices.Equal(got, []string{"AWIT-TEST0001"}) {
		t.Fatalf("Blocked() = %v, want [AWIT-TEST0001]", got)
	}
}

func TestManualBlockClearRestoresReadiness(t *testing.T) {
	closed, err := item.Parse("/abs/AWIT-TEST0000.md", []byte("---\nid: AWIT-TEST0000\ntitle: Base\nstatus: closed\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	free, err := item.Parse("/abs/AWIT-TEST0001.md", []byte("---\nid: AWIT-TEST0001\ntitle: Free\nstatus: open\ndeps: [AWIT-TEST0000]\nblocked_reason: waiting\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := item.Parse("/abs/AWIT-TEST0002.md", []byte("---\nid: AWIT-TEST0002\ntitle: Pinned\nstatus: open\ndeps: [AWIT-TEST0001]\nblocked_reason: waiting\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	g := Build([]*item.Item{closed, free, pinned}, nil)
	if g.Nodes["AWIT-TEST0001"].Ready {
		t.Fatal("held item with satisfied deps must not be ready before the reason is cleared")
	}
	if err := free.SetBlockedReason(""); err != nil {
		t.Fatal(err)
	}
	if err := pinned.SetBlockedReason(""); err != nil {
		t.Fatal(err)
	}
	g = Build([]*item.Item{closed, free, pinned}, nil)
	if !g.Nodes["AWIT-TEST0001"].Ready {
		t.Fatal("clearing the reason with satisfied deps must restore readiness")
	}
	if g.Nodes["AWIT-TEST0002"].Ready || !g.Nodes["AWIT-TEST0002"].Blocked {
		t.Fatal("clearing the reason with an open dep must leave the item blocked")
	}
}

func TestManualBlockHeldMiddleNodePaths(t *testing.T) {
	a := &item.Item{ID: "AWIT-TEST0001", Title: "A", Status: item.StatusOpen}
	b := &item.Item{ID: "AWIT-TEST0002", Title: "B", Status: item.StatusOpen, Deps: []string{"AWIT-TEST0001"}, BlockedReason: "held"}
	c := &item.Item{ID: "AWIT-TEST0003", Title: "C", Status: item.StatusOpen, Deps: []string{"AWIT-TEST0002"}}
	d := &item.Item{ID: "AWIT-TEST0004", Title: "D", Status: item.StatusClosed, BlockedReason: "held while closed"}
	g := Build([]*item.Item{a, b, c, d}, nil)
	if g.Nodes["AWIT-TEST0002"].Ready || !g.Nodes["AWIT-TEST0002"].Blocked {
		t.Fatal("held middle node must be blocked, not ready")
	}
	if g.Nodes["AWIT-TEST0002"].Quarantined() {
		t.Fatal("held middle node must not be quarantined")
	}
	if got := g.Nodes["AWIT-TEST0001"].UnblockCount; got != 2 {
		t.Fatalf("A UnblockCount = %d, want 2 (held B and open C still count)", got)
	}
	if got := nodeIDs(g.CriticalPath()); !slices.Equal(got, []string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0003"}) {
		t.Fatalf("CriticalPath() = %v, want [0001 0002 0003] (held nodes stay structural)", got)
	}
	if got := nodeIDs(g.Archivable()); !slices.Equal(got, []string{"AWIT-TEST0004"}) {
		t.Fatalf("Archivable() = %v, want [AWIT-TEST0004] (closed held node still archives)", got)
	}
}
