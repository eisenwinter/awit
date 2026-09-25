package lazy

import (
	"fmt"

	"github.com/eisenwinter/awit/pkg/graph"
)

// queueRows lists g.Ready() in its deterministic prime order via line. Every
// row is selectable; an empty queue yields one unselectable placeholder row.
func queueRows(g *graph.Graph, line func(*graph.Node) string) []row {
	if g == nil {
		return []row{{text: "No ready items"}}
	}
	ready := g.Ready()
	if len(ready) == 0 {
		return []row{{text: "No ready items"}}
	}
	rows := make([]row, 0, len(ready))
	for _, n := range ready {
		rows = append(rows, row{id: n.Item.ID, text: sanitize(line(n), false), status: string(n.Item.Status), selectable: true})
	}
	return rows
}

// whyLine explains n in next --why terms. selection is max-unblocks when n
// shares the top UnblockCount, otherwise ranked; the TUI never randomises,
// so tie-break is always none.
func whyLine(g *graph.Graph, n *graph.Node, onCritical bool) string {
	cp := "no"
	if onCritical {
		cp = "yes"
	}
	sel := "ranked"
	if ready := g.Ready(); len(ready) > 0 && n.UnblockCount == ready[0].UnblockCount {
		sel = "max-unblocks"
	}
	return fmt.Sprintf("why: %s; unblocks=%d; critical-path=%s; selection=%s; tie-break=none",
		n.Item.ID, n.UnblockCount, cp, sel)
}

// claimSelected claims the queue selection through Ops; failures arrive as
// errors and become toasts with the selection kept.
func (m *Model) claimSelected() {
	id := m.selectedID()
	if id == "" {
		m.toast = "error: no item selected"
		return
	}
	m.act(func() error { return m.ops.Claim(id) }, "claimed "+id)
}

// releaseSelected releases the queue selection back to open.
func (m *Model) releaseSelected() {
	id := m.selectedID()
	if id == "" {
		m.toast = "error: no item selected"
		return
	}
	m.act(func() error { return m.ops.Release(id) }, "reopened "+id)
}
