// Package graph builds the in-memory dependency graph from parsed items.
package graph

import (
	"sort"
	"strings"

	"github.com/eisenwinter/awit/pkg/item"
)

type Fault struct {
	Reason item.Reason
	IDs    []string
	Detail string
	Fix    string
}

type Node struct {
	Item         *item.Item
	Deps         []*Node
	Unblocks     []*Node
	Faults       []Fault
	Ready        bool
	Blocked      bool
	UnblockCount int
}

func (n *Node) Quarantined() bool {
	return len(n.Faults) > 0
}

func (n *Node) DepIDs() []string {
	out := append([]string(nil), n.Item.Deps...)
	sort.Strings(out)
	return out
}

func (n *Node) OpenDepIDs() []string {
	resolved := make(map[string]*Node, len(n.Deps))
	for _, d := range n.Deps {
		resolved[d.Item.ID] = d
	}
	var out []string
	for _, id := range n.Item.Deps {
		d, ok := resolved[id]
		if !ok {
			out = append(out, id)
			continue
		}
		if d.Quarantined() || d.Item.Status != item.StatusClosed {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

type Graph struct {
	Nodes  map[string]*Node
	Order  []*Node
	Broken []item.Broken
	Faults []Fault
}

// detectCycles, classify, and countUnblocks are seams. Build always calls
// them in this order. Later tickets replace the bodies; do not change the
// call site.
func (g *Graph) classify()      {}
func (g *Graph) countUnblocks() {}

func reasonFix(b item.Broken) string {
	switch b.Reason {
	case item.ReasonParse:
		return "edit the frontmatter until `awit validate` passes"
	case item.ReasonConflict:
		return "resolve the git conflict in " + b.Path
	case item.ReasonIDMismatch:
		return "rename the file or fix the id: key"
	case item.ReasonDuplicate:
		return "rename one of the files"
	default:
		return ""
	}
}

func sortFaults(fs []Fault) {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Reason != fs[j].Reason {
			return fs[i].Reason < fs[j].Reason
		}
		return strings.Join(fs[i].IDs, ",") < strings.Join(fs[j].IDs, ",")
	})
}

func Build(items []*item.Item, broken []item.Broken) *Graph {
	g := &Graph{
		Nodes:  make(map[string]*Node, len(items)),
		Broken: append([]item.Broken(nil), broken...),
	}
	for _, it := range items {
		n := &Node{Item: it}
		g.Nodes[it.ID] = n
		g.Order = append(g.Order, n)
	}
	sort.Slice(g.Order, func(i, j int) bool {
		return g.Order[i].Item.ID < g.Order[j].Item.ID
	})

	brokenByID := make(map[string]item.Broken, len(broken))
	for _, b := range broken {
		brokenByID[b.ID] = b
		g.Faults = append(g.Faults, Fault{
			Reason: b.Reason,
			IDs:    []string{b.ID},
			Detail: b.Detail,
			Fix:    reasonFix(b),
		})
	}

	for _, n := range g.Order {
		for _, depID := range n.Item.Deps {
			if dep, ok := g.Nodes[depID]; ok {
				n.Deps = append(n.Deps, dep)
				dep.Unblocks = append(dep.Unblocks, n)
				continue
			}
			f := Fault{
				Reason: item.ReasonDangling,
				IDs:    []string{n.Item.ID},
			}
			if _, ok := brokenByID[depID]; ok {
				f.Detail = "depends on quarantined " + depID
				f.Fix = "fix " + depID + " first"
			} else {
				f.Detail = n.Item.ID + " depends on unknown " + depID
				f.Fix = "awit dep rm " + n.Item.ID + " " + depID
			}
			n.Faults = append(n.Faults, f)
			g.Faults = append(g.Faults, f)
		}
	}

	for _, n := range g.Order {
		sort.Slice(n.Unblocks, func(i, j int) bool {
			return n.Unblocks[i].Item.ID < n.Unblocks[j].Item.ID
		})
	}

	g.detectCycles()
	g.classify()
	g.countUnblocks()
	sortFaults(g.Faults)
	return g
}
