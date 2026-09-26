package lazy

import (
	"slices"
	"testing"

	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

func TestParseQuery(t *testing.T) {
	cases := []struct {
		q     string
		want  Filter
		isErr bool
	}{
		{"", Filter{}, false},
		{"--ready", Filter{Ready: true}, false},
		{"-s open,closed", Filter{Statuses: []item.Status{item.StatusOpen, item.StatusClosed}}, false},
		{"-s open -s closed", Filter{Statuses: []item.Status{item.StatusOpen, item.StatusClosed}}, false},
		{"-l auth,db -l p1", Filter{Labels: [][]string{{"auth", "db"}, {"p1"}}}, false},
		{"--label auth --status in_progress --blocked token thing", Filter{Blocked: true, Statuses: []item.Status{item.StatusInProgress}, Labels: [][]string{{"auth"}}, Search: "token thing"}, false},
		{"-l ,", Filter{}, false},
		{"-s bogus", Filter{}, true},
		{"-x", Filter{}, true},
		{"-s", Filter{}, true},
	}
	for _, c := range cases {
		got, err := ParseQuery(c.q)
		if (err != nil) != c.isErr {
			t.Errorf("ParseQuery(%q) err = %v, want err=%v", c.q, err, c.isErr)
			continue
		}
		if err == nil && !filtersEqual(got, c.want) {
			t.Errorf("ParseQuery(%q) = %+v, want %+v", c.q, got, c.want)
		}
	}
}

func filtersEqual(a, b Filter) bool {
	return a.Ready == b.Ready && a.Blocked == b.Blocked && a.Quarantined == b.Quarantined &&
		slices.Equal(a.Statuses, b.Statuses) && a.Search == b.Search &&
		slices.EqualFunc(a.Labels, b.Labels, slices.Equal)
}

func TestBadges(t *testing.T) {
	if got := (Filter{}).badges(); got != "no filter" {
		t.Fatalf("empty badges = %q", got)
	}
	f := Filter{Ready: true, Statuses: []item.Status{item.StatusOpen}, Labels: [][]string{{"auth", "db"}}, Search: "tok"}
	if got := f.badges(); got != "[-s open] [-l auth|db] [--ready] [/ tok]" {
		t.Fatalf("badges = %q", got)
	}
}

func TestApplyOpenAndArchive(t *testing.T) {
	f := newFixture()
	g, _ := f.Load()
	ids := func(ns []*graph.Node) []string {
		var out []string
		for _, n := range ns {
			out = append(out, n.Item.ID)
		}
		return out
	}
	if got := ids(Filter{Ready: true}.applyOpen(g)); !slices.Equal(got, []string{"AWIT-LAZY0001", "AWIT-LAZY0002", "AWIT-LAZY0006"}) {
		t.Fatalf("ready = %v", got)
	}
	if got := ids(Filter{Search: "rotate"}.applyOpen(g)); !slices.Equal(got, []string{"AWIT-LAZY0004", "AWIT-LAZY0006"}) {
		t.Fatalf("search rotate = %v", got)
	}
	if got := ids(Filter{Search: "lazy0007"}.applyOpen(g)); !slices.Equal(got, []string{"AWIT-LAZY0007"}) {
		t.Fatalf("search by id = %v", got)
	}
	if got := ids(Filter{Search: "p0", Blocked: true}.applyOpen(g)); !slices.Equal(got, []string{"AWIT-LAZY0004", "AWIT-LAZY0007"}) {
		t.Fatalf("search label + blocked = %v", got)
	}
	arch := Filter{Ready: true, Labels: [][]string{{"auth"}}}.applyArchive(f.archive)
	if len(arch) != 1 || arch[0].ID != "AWIT-LAZY0101" {
		t.Fatalf("archive filter = %v", arch)
	}
}
