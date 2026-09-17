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
