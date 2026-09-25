package cli

import (
	"strings"
	"testing"

	"github.com/eisenwinter/awit/internal/ops"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

func openTestStore(t *testing.T, dir string) *item.Store {
	t.Helper()
	s, err := item.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestShowFullAndValidateTextMatchCLI(t *testing.T) {
	dir := copyFixture(t, "clean")
	s := openTestStore(t, dir)
	g, err := ops.LoadGraph(s)
	if err != nil {
		t.Fatal(err)
	}
	for id := range g.Nodes {
		_, stdout, _ := run(t, "--repo", dir, "show", id, "--full")
		if got := showFull(s, g, g.Nodes[id]); got != stdout {
			t.Fatalf("showFull(%s) =\n%s\nwant\n%s", id, got, stdout)
		}
	}
	_, stdout, _ := run(t, "--repo", dir, "validate")
	if got := validateText(g); got != stdout {
		t.Fatalf("validateText =\n%s\nwant\n%s", got, stdout)
	}
	if !strings.HasPrefix(validateText(graph.Build(nil, nil)), "PASS  0 items, 0 quarantined\n") {
		t.Fatal("validateText on empty graph")
	}
}
