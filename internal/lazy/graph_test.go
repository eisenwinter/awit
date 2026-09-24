package lazy

import (
	"bytes"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/eisenwinter/awit/pkg/prime"
)

func TestOverviewRowsMatchPrime(t *testing.T) {
	f := newFixture()
	g, _ := f.Load()
	var want bytes.Buffer
	if err := prime.Render(&want, g, prime.Options{}); err != nil {
		t.Fatal(err)
	}
	rows := overviewRows(g)
	var got strings.Builder
	var ids []string
	for _, r := range rows {
		got.WriteString(r.text + "\n")
		if r.selectable {
			ids = append(ids, r.id)
		}
	}
	if got.String() != want.String() {
		t.Fatalf("overview text differs from prime:\n%s\nwant:\n%s", got.String(), want.String())
	}
	wantIDs := []string{"AWIT-LAZY0001", "AWIT-LAZY0002", "AWIT-LAZY0006", "AWIT-LAZY0003", "AWIT-LAZY0004", "AWIT-LAZY0007"}
	if strings.Join(ids, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("selectable ids = %v, want %v", ids, wantIDs)
	}
}

func chain(n int) *graph.Graph {
	var its []*item.Item
	for i := 1; i <= n; i++ {
		var deps []string
		if i > 1 {
			deps = []string{id(i - 1)}
		}
		its = append(its, item.New(id(i), "N", "b.", deps, nil))
	}
	return graph.Build(its, nil)
}
func id(i int) string { return "AWIT-CHAIN000" + string(rune('0'+i)) }

func TestTreeRowsDepthCap(t *testing.T) {
	g := chain(8) // 1 <- 2 <- ... <- 8 (each depends on the previous)
	rows := focusedRows(g, id(1))
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(r.text + "\n")
	}
	golden(t, "tree_depth_cap", b.String())
	if !strings.Contains(b.String(), "(+1 more)") {
		t.Fatalf("missing depth stub:\n%s", b.String())
	}
	depth := 0
	for _, r := range rows {
		if strings.Contains(r.text, id(7)) {
			depth++
		}
	}
	if depth != 0 {
		t.Fatal("node beyond depth 5 rendered")
	}
}

func TestTreeRowsCycleGuard(t *testing.T) {
	a := item.New("AWIT-CYC00001", "A", "b.", []string{"AWIT-CYC00002"}, nil)
	b := item.New("AWIT-CYC00002", "B", "b.", []string{"AWIT-CYC00001"}, nil)
	g := graph.Build([]*item.Item{a, b}, nil)
	rows := focusedRows(g, "AWIT-CYC00001")
	if len(rows) > 12 {
		t.Fatalf("cycle not guarded: %d rows", len(rows))
	}
	for _, r := range rows {
		if r.selectable {
			t.Fatalf("quarantined cycle node selectable: %+v", r)
		}
	}
}

func TestGraphTabToggleAndJump(t *testing.T) {
	m := newModel(t, newFixture())
	m, _ = press(m, "j", "j", "2") // Issues cursor on L3, then Graph
	if m.tab != tabGraph || m.graphTab.focused || m.graphTab.rootID != "AWIT-LAZY0003" {
		t.Fatalf("graph entry: tab=%d focused=%v root=%s", m.tab, m.graphTab.focused, m.graphTab.rootID)
	}
	golden(t, "graph_overview", m.View())
	m, _ = press(m, "tab")
	if !m.graphTab.focused {
		t.Fatal("tab did not switch to focused")
	}
	golden(t, "graph_focused", m.View())
	if !contains(m.View(), "AWIT-LAZY0001") || !contains(m.View(), "AWIT-LAZY0004") {
		t.Fatalf("focused tree lacks upstream/downstream:\n%s", m.View())
	}
	m, _ = press(m, "j") // move somewhere in the tree
	sel, ok := m.graphTab.list.selected()
	if !ok || sel.id == "" {
		t.Fatalf("no node row selected after j: %+v", sel)
	}
	m, _ = press(m, "enter")
	if m.tab != tabIssues || m.issues.showArchive || m.focus != focusList {
		t.Fatalf("enter did not jump to Issues open source: tab=%d", m.tab)
	}
	if m.selectedID() != sel.id {
		t.Fatalf("jump pinned %s, want %s", m.selectedID(), sel.id)
	}
}
