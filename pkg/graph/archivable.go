package graph

import (
	"github.com/eisenwinter/awit/pkg/item"
)

// Archivable returns the fixed-point set of closed, non-quarantined
// nodes with no Unblocks neighbour outside the set (guide §2 "Archive
// eligibility"). Sorted ID asc; empty when none.
func (g *Graph) Archivable() []*Node {
	in := make(map[*Node]bool, len(g.Order))
	for _, n := range g.Order {
		if n.Item.Status == item.StatusClosed && !n.Quarantined() {
			in[n] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for n := range in {
			for _, u := range n.Unblocks {
				if !in[u] {
					delete(in, n)
					changed = true
					break
				}
			}
		}
	}
	out := make([]*Node, 0, len(in))
	for _, n := range g.Order { // g.Order is ID asc → deterministic
		if in[n] {
			out = append(out, n)
		}
	}
	return out
}
