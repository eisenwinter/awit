package ops_test

import (
	"strings"
	"testing"

	"github.com/eisenwinter/awit/internal/ops"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

func TestShowFullAndValidateTextMatchCLI(t *testing.T) {
	dir := copyFixture(t, "clean")
	s := openTestStore(t, dir)
	g, err := ops.LoadGraph(s)
	if err != nil {
		t.Fatal(err)
	}
	for id := range g.Nodes {
		_, stdout, _ := run(t, "--repo", dir, "show", id, "--full")
		if got := ops.ShowFull(s, g, g.Nodes[id]); got != stdout {
			t.Fatalf("ShowFull(%s) =\n%s\nwant\n%s", id, got, stdout)
		}
	}
	_, stdout, _ := run(t, "--repo", dir, "validate")
	if got := ops.ValidateText(g); got != stdout {
		t.Fatalf("ValidateText =\n%s\nwant\n%s", got, stdout)
	}
	if !strings.HasPrefix(ops.ValidateText(graph.Build(nil, nil)), "PASS  0 items, 0 quarantined\n") {
		t.Fatal("ValidateText on empty graph")
	}
}

func TestBrokenViewMatchesShow(t *testing.T) {
	dir := copyFixture(t, "parse-error")
	g, _ := ops.LoadGraph(openTestStore(t, dir))
	for _, br := range g.Broken {
		_, stdout, _ := run(t, "--repo", dir, "show", br.ID)
		var matches []item.Broken
		for _, b := range g.Broken {
			if b.ID == br.ID {
				matches = append(matches, b)
			}
		}
		if got := ops.BrokenView(br.ID, matches); got != stdout {
			t.Fatalf("BrokenView(%s) =\n%s\nwant\n%s", br.ID, got, stdout)
		}
	}
	for id, n := range g.Nodes {
		_, stdout, _ := run(t, "--repo", dir, "show", id)
		if got := ops.DefaultView(n); got != stdout {
			t.Fatalf("DefaultView(%s) differs from show", id)
		}
	}
}
