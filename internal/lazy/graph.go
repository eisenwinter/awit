package lazy

import (
	"bytes"
	"fmt"
	"sort"
	"strings"

	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/prime"
)

// treeDepth caps each direction of the focused trees; deeper subtrees
// collapse into one "(+N more)" stub row.
const treeDepth = 5

// graphState is the Graph tab. Overview (focused=false) shows awit prime's
// snapshot verbatim as selectable rows; Focused shows the root's upstream
// and downstream trees. rootID is retargeted at the Issues selection on tab
// entry.
type graphState struct {
	focused bool
	rootID  string
	list    cursorList
}

// overviewRows turns prime.Render's output into rows. A line becomes a
// selectable row with id X when it starts with "[X]" and the graph knows X
// (the §6 contract: only READY/BLOCKED node rows qualify; section headers,
// warnings and the critical-path line never do).
func overviewRows(g *graph.Graph) []row {
	var buf bytes.Buffer
	if err := prime.Render(&buf, g, prime.Options{}); err != nil {
		return nil
	}
	out := buf.String()
	if out == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	rows := make([]row, 0, len(lines))
	for _, line := range lines {
		clean := sanitize(line, false)
		r := row{text: clean}
		if strings.HasPrefix(clean, "===") {
			r.head = true
			switch {
			case strings.Contains(clean, "READY"):
				r.hstatus = "ready"
			case strings.Contains(clean, "CRITICAL"):
				r.hstatus = "critical"
			default:
				r.hstatus = "blocked" // BLOCKED and warnings share warn
			}
			rows = append(rows, r)
			continue
		}
		if rest, ok := strings.CutPrefix(clean, "["); ok {
			if i := strings.IndexByte(rest, ']'); i >= 0 {
				if id := rest[:i]; g.Nodes[id] != nil {
					r.id, r.selectable = id, true
					r.status = string(g.Nodes[id].Item.Status)
				}
			}
		}
		rows = append(rows, r)
	}
	return rows
}

// treeRows renders root at depth 0 and its children - via next, ID
// ascending - recursively. Last children draw "└─ ", others "├─ ";
// ancestors contribute "│  " or "   ". A node at depth treeDepth with k>0
// children emits one unselectable "(+k more)" child row instead of
// recursing; children already on the current path are skipped, so
// quarantined cycles terminate.
func treeRows(root *graph.Node, next func(*graph.Node) []*graph.Node) []row {
	var rows []row
	var walk func(n *graph.Node, depth int, prefix string, last bool, path map[string]bool)
	walk = func(n *graph.Node, depth int, prefix string, last bool, path map[string]bool) {
		conn := ""
		if depth > 0 {
			conn = "├─ "
			if last {
				conn = "└─ "
			}
		}
		rows = append(rows, row{id: n.Item.ID, text: sanitize(prefix+conn+nodeText(n), false), status: string(n.Item.Status), selectable: !n.Quarantined()})
		var children []*graph.Node
		for _, c := range next(n) {
			if !path[c.Item.ID] {
				children = append(children, c)
			}
		}
		sort.Slice(children, func(i, j int) bool { return children[i].Item.ID < children[j].Item.ID })
		// The children's inherited prefix: the root contributes nothing,
		// every deeper node its branch continuation.
		childPrefix := prefix
		if depth > 0 {
			if last {
				childPrefix += "   "
			} else {
				childPrefix += "│  "
			}
		}
		if depth == treeDepth {
			if len(children) > 0 {
				rows = append(rows, row{text: childPrefix + "└─ " + fmt.Sprintf("(+%d more)", len(children))})
			}
			return
		}
		for i, c := range children {
			cp := make(map[string]bool, len(path)+1)
			for id := range path {
				cp[id] = true
			}
			cp[c.Item.ID] = true
			walk(c, depth+1, childPrefix, i == len(children)-1, cp)
		}
	}
	walk(root, 0, "", false, map[string]bool{root.Item.ID: true})
	return rows
}

// focusedRows renders the focused-mode body: the root's upstream Deps tree,
// then its downstream Unblocks tree, each under its own header. A missing
// root (empty id or unknown to the graph) yields one unselectable hint row.
func focusedRows(g *graph.Graph, rootID string) []row {
	root := g.Nodes[rootID]
	if rootID == "" || root == nil {
		return []row{{text: "select an item on the Work items tab first"}}
	}
	deps := func(n *graph.Node) []*graph.Node { return n.Deps }
	unblocks := func(n *graph.Node) []*graph.Node { return n.Unblocks }
	rows := []row{{text: fmt.Sprintf("depends on (upstream, depth <= %d)", treeDepth), head: true}}
	rows = append(rows, treeRows(root, deps)...)
	rows = append(rows, row{})
	rows = append(rows, row{text: fmt.Sprintf("unblocks (downstream, depth <= %d)", treeDepth), head: true})
	return append(rows, treeRows(root, unblocks)...)
}

// nodeText is one tree node's label: "[ID] status title", plus the
// quarantine marker when the node is faulted.
func nodeText(n *graph.Node) string {
	s := "[" + n.Item.ID + "] " + string(n.Item.Status) + " " + n.Item.Title
	if n.Quarantined() {
		s += " [quarantined]"
	}
	return s
}

// graphRows builds the Graph list for the current mode. A graph with no
// nodes gets one placeholder row (prime itself would print bare scaffolding).
func (m *Model) graphRows() []row {
	if m.g == nil {
		return nil
	}
	if m.graphTab.focused {
		return focusedRows(m.g, m.graphTab.rootID)
	}
	if len(m.g.Order) == 0 {
		return []row{{text: "graph is empty"}}
	}
	return overviewRows(m.g)
}

// rebuildGraphRows refills the Graph list for the current mode, keeping the
// cursor on the selected node when possible.
func (m *Model) rebuildGraphRows() {
	keep := ""
	if r, ok := m.graphTab.list.selected(); ok {
		keep = r.id
	}
	m.graphTab.list.setRows(m.graphRows(), keep)
}

// enterGraphTab switches to the Graph tab in its current mode, retargeting
// the focused root at the Issues selection when there is one.
func (m *Model) enterGraphTab() {
	m.tab = tabGraph
	m.focus = focusList // the Graph tab has no detail pane
	if r, ok := m.issues.list.selected(); ok && r.id != "" {
		m.graphTab.rootID = r.id
	}
	m.rebuildGraphRows()
	m.refreshDetail()
}

// toggleGraphMode flips Overview/Focused, keeping the cursor id where the
// new mode still has it.
func (m *Model) toggleGraphMode() {
	m.graphTab.focused = !m.graphTab.focused
	m.rebuildGraphRows()
	m.refreshDetail()
}

// jumpToIssues lands on the Issues tab, open source, cursor pinned on the
// selected graph node.
func (m *Model) jumpToIssues() {
	r, ok := m.graphTab.list.selected()
	if !ok || r.id == "" {
		return
	}
	m.tab = tabIssues
	m.issues.showArchive = false
	m.issues.list.setRows(m.issuesRows(), r.id)
	m.focus = focusList
	m.refreshDetail()
}
