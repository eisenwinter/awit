package graph

import (
	"sort"

	"github.com/eisenwinter/awit/pkg/item"
)

// classify sets Ready / Blocked on every node. Closed and quarantined nodes
// get both flags false. A healthy non-closed node is Ready iff it carries no
// manual block (Item.BlockedReason empty) and every Item.Deps ID resolves to
// a closed, non-quarantined node; otherwise it is Blocked.
func (g *Graph) classify() {
	for _, n := range g.Order {
		n.Ready = false
		n.Blocked = false
		if n.Item.Status == item.StatusClosed || n.Quarantined() {
			continue
		}
		ready := n.Item.BlockedReason == ""
		for _, depID := range n.Item.Deps {
			if !depSatisfied(g, depID) {
				ready = false
				break
			}
		}
		n.Ready = ready
		n.Blocked = !ready
	}
}

func depSatisfied(g *Graph, depID string) bool {
	d, ok := g.Nodes[depID]
	if !ok {
		return false
	}
	return d.Item.Status == item.StatusClosed && !d.Quarantined()
}

// countUnblocks writes UnblockCount. Quarantined nodes get -1. Others get the
// number of unique non-closed non-quarantined nodes reachable via Unblocks,
// walking through closed and quarantined nodes so a closed middle node does
// not hide an open descendant.
func (g *Graph) countUnblocks() {
	for _, n := range g.Order {
		if n.Quarantined() {
			n.UnblockCount = -1
			continue
		}
		n.UnblockCount = len(ReachableUnblocks(n))
	}
}

// ReachableUnblocks returns the unique non-closed, non-quarantined nodes
// reachable from start via Unblocks edges, sorted by ID. The walk passes
// through closed and quarantined nodes so a closed middle node does not hide
// an open descendant; those nodes are walked but never returned.
// UnblockCount is len(ReachableUnblocks(n)) for every non-quarantined node.
func ReachableUnblocks(start *Node) []*Node {
	seen := map[string]bool{start.Item.ID: true}
	var out []*Node
	queue := append([]*Node(nil), start.Unblocks...)
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if seen[n.Item.ID] {
			continue
		}
		seen[n.Item.ID] = true
		if !n.Quarantined() && n.Item.Status != item.StatusClosed {
			out = append(out, n)
		}
		queue = append(queue, n.Unblocks...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Item.ID < out[j].Item.ID })
	return out
}

func (g *Graph) Ready() []*Node {
	var out []*Node
	for _, n := range g.Order {
		if n.Ready {
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UnblockCount != out[j].UnblockCount {
			return out[i].UnblockCount > out[j].UnblockCount
		}
		return out[i].Item.ID < out[j].Item.ID
	})
	return out
}

func (g *Graph) Blocked() []*Node {
	var out []*Node
	for _, n := range g.Order {
		if n.Blocked {
			out = append(out, n)
		}
	}
	return out
}

func (g *Graph) Quarantined() []*Node {
	var out []*Node
	for _, n := range g.Order {
		if n.Quarantined() {
			out = append(out, n)
		}
	}
	return out
}

func (g *Graph) Closed() []*Node {
	var out []*Node
	for _, n := range g.Order {
		if n.Item.Status == item.StatusClosed && !n.Quarantined() {
			out = append(out, n)
		}
	}
	return out
}

// WouldCycle returns the dependency chain that adding "from depends on to"
// would close, or nil. DFS from `to` over Node.Deps looking for `from`.
func (g *Graph) WouldCycle(from, to string) []string {
	if g.Nodes[from] == nil || g.Nodes[to] == nil {
		return nil
	}
	if from == to {
		return []string{from, from}
	}
	seen := make(map[string]bool)
	var dfs func(cur string) []string
	dfs = func(cur string) []string {
		if cur == from {
			return []string{cur}
		}
		if seen[cur] {
			return nil
		}
		seen[cur] = true
		n := g.Nodes[cur]
		if n == nil {
			return nil
		}
		for _, d := range n.Deps {
			if rest := dfs(d.Item.ID); rest != nil {
				return append([]string{cur}, rest...)
			}
		}
		return nil
	}
	rest := dfs(to)
	if rest == nil {
		return nil
	}
	return append([]string{from}, rest...)
}

// FilterLabels keeps nodes matching every group (AND) where a group matches
// if any label in it is present (OR). Empty groups (nil or length 0) returns
// nodes unchanged.
func FilterLabels(nodes []*Node, groups [][]string) []*Node {
	if len(groups) == 0 {
		return nodes
	}
	var out []*Node
next:
	for _, n := range nodes {
		for _, group := range groups {
			matched := false
			for _, label := range group {
				if n.Item.HasLabel(label) {
					matched = true
					break
				}
			}
			if !matched {
				continue next
			}
		}
		out = append(out, n)
	}
	return out
}
