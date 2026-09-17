package graph

import (
	"slices"
	"sort"

	"github.com/eisenwinter/awit/pkg/item"
)

// CriticalPath returns the longest chain (in node count) over non-closed,
// non-quarantined nodes, following Unblocks edges in topological order.
// Every tie breaks to the smaller ID: the ready queue is ID-sorted, an
// equal-distance predecessor keeps the smaller ID, and the end node is the
// smaller ID on equal distance. The path runs from a root (a candidate with
// no candidate deps) downstream. It returns an empty non-nil slice when no
// candidates exist. The graph is not mutated.
func (g *Graph) CriticalPath() []*Node {
	path := []*Node{}

	candidate := make(map[string]bool, len(g.Order))
	for _, n := range g.Order {
		if n.Item.Status != item.StatusClosed && !n.Quarantined() {
			candidate[n.Item.ID] = true
		}
	}
	if len(candidate) == 0 {
		return path
	}

	// Kahn topological walk over Unblocks restricted to candidates.
	// indegree counts candidate deps only, so a closed or quarantined dep
	// never holds a candidate back.
	indegree := make(map[string]int, len(candidate))
	dist := make(map[string]int, len(candidate))
	prev := make(map[string]*Node, len(candidate))
	var ready []*Node
	for _, n := range g.Order {
		if !candidate[n.Item.ID] {
			continue
		}
		deg := 0
		for _, d := range n.Deps {
			if candidate[d.Item.ID] {
				deg++
			}
		}
		indegree[n.Item.ID] = deg
		dist[n.Item.ID] = 1 // every candidate is a path of one node
		if deg == 0 {
			ready = append(ready, n)
		}
	}
	sort.Slice(ready, func(i, j int) bool { return ready[i].Item.ID < ready[j].Item.ID })

	for len(ready) > 0 {
		cur := ready[0]
		ready = ready[1:]
		for _, next := range cur.Unblocks {
			if !candidate[next.Item.ID] {
				continue
			}
			d := dist[cur.Item.ID] + 1
			if p := prev[next.Item.ID]; d > dist[next.Item.ID] ||
				(d == dist[next.Item.ID] && (p == nil || cur.Item.ID < p.Item.ID)) {
				dist[next.Item.ID] = d
				prev[next.Item.ID] = cur
			}
			indegree[next.Item.ID]--
			if indegree[next.Item.ID] == 0 {
				i := sort.Search(len(ready), func(i int) bool { return ready[i].Item.ID > next.Item.ID })
				ready = append(ready, nil)
				copy(ready[i+1:], ready[i:])
				ready[i] = next
			}
		}
	}

	// End node: maximum distance; g.Order is ID-ascending and a strictly
	// greater distance is required to replace end, so ties keep the smaller ID.
	var end *Node
	for _, n := range g.Order {
		if !candidate[n.Item.ID] {
			continue
		}
		if end == nil || dist[n.Item.ID] > dist[end.Item.ID] {
			end = n
		}
	}

	for n := end; n != nil; n = prev[n.Item.ID] {
		path = append(path, n)
	}
	slices.Reverse(path)
	return path
}
