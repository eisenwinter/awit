package ops_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/internal/ops"
	"github.com/eisenwinter/awit/pkg/item"
)

func TestResolveItemID(t *testing.T) {
	items := []*item.Item{
		{ID: "AWIT-A", Alias: "DTRM-F21"},
		{ID: "AWIT-B", Alias: "AWIT-A"}, // alias equal to another canonical id: canonical wins
		{ID: "AWIT-C", External: &item.External{Tracker: "gitea", Repo: "o/r", ID: 7}},
		{ID: "AWIT-D", External: &item.External{Tracker: "gitlab", Repo: "g/sub/p", ID: 7}},
		{ID: "AWIT-E", Alias: "dup"},
		{ID: "AWIT-F", Alias: "DUP"},
	}
	cases := []struct{ key, want, errSub string }{
		{"AWIT-A", "AWIT-A", ""},
		{"dtrm-f21", "AWIT-A", ""},
		{"o/r#7", "AWIT-C", ""},
		{"g/sub/p#7", "AWIT-D", ""},
		{"#7", "", `ambiguous external key "#7" matches AWIT-C, AWIT-D`},
		{"dup", "", `ambiguous alias "dup" matches AWIT-E, AWIT-F`},
		{"nope", "", "unknown item nope"},
		{"7", "", "unknown item 7"},
	}
	for _, c := range cases {
		got, err := ops.ResolveItemID(items, c.key)
		if c.errSub == "" {
			if err != nil || got != c.want {
				t.Errorf("%q = %q, %v; want %q", c.key, got, err, c.want)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), c.errSub) {
			t.Errorf("%q err = %v, want %q", c.key, err, c.errSub)
		}
		if strings.HasPrefix(c.errSub, "unknown") && !errors.Is(err, ops.ErrUnknownItem) {
			t.Errorf("%q: not ErrUnknownItem", c.key)
		}
	}
}

func TestLoadGraphAndLoadItem(t *testing.T) {
	dir := copyFixture(t, "clean")
	s := openTestStore(t, dir)
	g, err := ops.LoadGraph(s)
	if err != nil || len(g.Order) != 6 {
		t.Fatalf("LoadGraph = %d nodes, %v", len(g.Order), err)
	}
	if it, err := ops.LoadItem(s, "AWIT-TEST0001"); err != nil || it.ID != "AWIT-TEST0001" {
		t.Fatalf("LoadItem = %v, %v", it, err)
	}
	if _, err := ops.LoadItem(s, "AWIT-TEST9999"); err == nil || err.Error() != "unknown item AWIT-TEST9999" {
		t.Fatalf("unknown err = %v", err)
	}
}

func TestToEntryAndArchiveEntry(t *testing.T) {
	dir := copyFixture(t, "clean")
	g, _ := ops.LoadGraph(openTestStore(t, dir))
	e := ops.ToEntry(g.Nodes["AWIT-TEST0003"])
	if e.State != "blocked" || e.Labels == nil || e.Deps == nil {
		t.Fatalf("entry = %+v", e)
	}
	a := ops.ArchiveEntry(&item.Item{ID: "X", Status: item.StatusClosed})
	if a.State != "closed" || a.Unblocks != 0 || a.Labels == nil || a.Deps == nil {
		t.Fatalf("archive entry = %+v", a)
	}
}
