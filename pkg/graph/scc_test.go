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
