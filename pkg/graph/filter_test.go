package graph

import (
	"slices"
	"testing"

	"github.com/eisenwinter/awit/pkg/item"
)

// rankedFixture builds a graph whose ready ranking (UnblockCount desc)
// differs from Order (ID asc), so tests can tell g.Ready() apart from an
// Order scan: ready alone must return [B A], any other selection [A B].
func rankedFixture() *Graph {
	a := &Node{Item: &item.Item{ID: "AWIT-A", Status: item.StatusOpen}, Ready: true, UnblockCount: 1}
	b := &Node{Item: &item.Item{ID: "AWIT-B", Status: item.StatusOpen}, Ready: true, UnblockCount: 5}
	return &Graph{
		Nodes: map[string]*Node{"AWIT-A": a, "AWIT-B": b},
		Order: []*Node{a, b},
	}
}

func TestGraphFilterSourceSet(t *testing.T) {
	clean := buildFixture(t, "clean")
	dangling := buildFixture(t, "dangling")

	tests := []struct {
		name string
		g    *Graph
		f    Filter
		want []string
	}{
		{"no flags keeps Order", clean, Filter{}, []string{
			"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0003",
			"AWIT-TEST0004", "AWIT-TEST0005", "AWIT-TEST0006",
		}},
		// rankedFixture: ranked ready order [B A] differs from Order [A B].
		{"ready alone keeps ranked order", rankedFixture(), Filter{Ready: true},
			[]string{"AWIT-B", "AWIT-A"}},
		{"ready plus blocked scans Order", rankedFixture(),
			Filter{Ready: true, Blocked: true}, []string{"AWIT-A", "AWIT-B"}},
		{"ready alone on clean", clean, Filter{Ready: true},
			[]string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0006"}},
		{"blocked alone", clean, Filter{Blocked: true},
			[]string{"AWIT-TEST0003", "AWIT-TEST0004"}},
		{"ready and blocked scans Order", clean,
			Filter{Ready: true, Blocked: true},
			[]string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0003", "AWIT-TEST0004", "AWIT-TEST0006"}},
		{"quarantined alone", dangling, Filter{Quarantined: true},
			[]string{"AWIT-TEST0001"}},
		{"quarantined misses clean graph", clean, Filter{Quarantined: true}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.g.Filter(tt.f)
			if !slices.Equal(nodeIDs(got), tt.want) {
				t.Fatalf("Filter(%+v) = %v, want %v", tt.f, nodeIDs(got), tt.want)
			}
		})
	}
}

func TestGraphFilterNarrowsStatusAndLabels(t *testing.T) {
	g := buildFixture(t, "clean")

	got := g.Filter(Filter{Statuses: []item.Status{item.StatusClosed}})
	if want := []string{"AWIT-TEST0005"}; !slices.Equal(nodeIDs(got), want) {
		t.Fatalf("status closed = %v, want %v", nodeIDs(got), want)
	}

	// Status OR: open or in_progress keeps everything but the closed node.
	got = g.Filter(Filter{Statuses: []item.Status{item.StatusOpen, item.StatusInProgress}})
	want := []string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0003", "AWIT-TEST0004", "AWIT-TEST0006"}
	if !slices.Equal(nodeIDs(got), want) {
		t.Fatalf("status open|in_progress = %v, want %v", nodeIDs(got), want)
	}

	// State flags and statuses compose: ready AND in_progress.
	got = g.Filter(Filter{Ready: true, Statuses: []item.Status{item.StatusInProgress}})
	if want := []string{"AWIT-TEST0006"}; !slices.Equal(nodeIDs(got), want) {
		t.Fatalf("ready + in_progress = %v, want %v", nodeIDs(got), want)
	}

	// Labels AND across groups, OR within; combined with a state flag.
	got = g.Filter(Filter{Blocked: true, Labels: [][]string{{"p0"}}})
	if want := []string{"AWIT-TEST0004"}; !slices.Equal(nodeIDs(got), want) {
		t.Fatalf("blocked + p0 = %v, want %v", nodeIDs(got), want)
	}
	got = g.Filter(Filter{Ready: true, Labels: [][]string{{"auth", "db"}}})
	if want := []string{"AWIT-TEST0001", "AWIT-TEST0002"}; !slices.Equal(nodeIDs(got), want) {
		t.Fatalf("ready + auth|db = %v, want %v", nodeIDs(got), want)
	}
}

func TestNarrow(t *testing.T) {
	g := buildFixture(t, "clean")

	// Empty statuses and labels narrow nothing; the input order (here the
	// ranked ready order) is preserved.
	got := Narrow(g.Ready(), nil, nil)
	if want := []string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0006"}; !slices.Equal(nodeIDs(got), want) {
		t.Fatalf("Narrow(ready, nil, nil) = %v, want %v", nodeIDs(got), want)
	}

	got = Narrow(g.Order, []item.Status{item.StatusClosed}, nil)
	if want := []string{"AWIT-TEST0005"}; !slices.Equal(nodeIDs(got), want) {
		t.Fatalf("Narrow status closed = %v, want %v", nodeIDs(got), want)
	}

	// AND across groups, OR within a group, plus a status restriction.
	got = Narrow(g.Order, []item.Status{item.StatusOpen}, [][]string{{"auth", "db"}, {"p1"}})
	if want := []string{"AWIT-TEST0001"}; !slices.Equal(nodeIDs(got), want) {
		t.Fatalf("Narrow open + (auth|db) AND p1 = %v, want %v", nodeIDs(got), want)
	}
}

func TestMatchLabels(t *testing.T) {
	if !MatchLabels(nil, nil) {
		t.Fatal("no groups must match empty labels")
	}
	if !MatchLabels([]string{"auth", "p1"}, [][]string{{"auth"}, {"p1"}}) {
		t.Fatal("AND across groups: auth AND p1 must match")
	}
	if !MatchLabels([]string{"db"}, [][]string{{"auth", "db"}}) {
		t.Fatal("OR within a group: db must match {auth,db}")
	}
	if MatchLabels([]string{"auth"}, [][]string{{"auth"}, {"p0"}}) {
		t.Fatal("AND across groups: missing p0 must not match")
	}
	if MatchLabels(nil, [][]string{{"auth"}}) {
		t.Fatal("no labels must not match a group")
	}
}
