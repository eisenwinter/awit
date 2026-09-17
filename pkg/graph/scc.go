package graph

import (
	"fmt"
	"sort"
	"strings"

	"github.com/eisenwinter/awit/pkg/item"
)

// detectCycles runs Tarjan SCC over Node.Deps and quarantines every cyclic
// component. It is the body of the seam Build already calls.
func (g *Graph) detectCycles() {
	index := 0
	indices := make(map[string]int, len(g.Nodes))
	lowlink := make(map[string]int, len(g.Nodes))
	onStack := make(map[string]bool, len(g.Nodes))
	var stack []*Node

	var strongconnect func(n *Node)
	strongconnect = func(n *Node) {
		id := n.Item.ID
		indices[id] = index
		lowlink[id] = index
		index++
		stack = append(stack, n)
		onStack[id] = true

		for _, dep := range n.Deps {
			did := dep.Item.ID
			if _, seen := indices[did]; !seen {
				strongconnect(dep)
				if lowlink[did] < lowlink[id] {
					lowlink[id] = lowlink[did]
				}
			} else if onStack[did] {
				if indices[did] < lowlink[id] {
					lowlink[id] = indices[did]
				}
			}
		}

		if lowlink[id] != indices[id] {
			return
		}
		var scc []*Node
		for {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[w.Item.ID] = false
			scc = append(scc, w)
			if w == n {
				break
			}
		}
		g.quarantineIfCyclic(scc)
	}

	for _, n := range g.Order {
		if _, seen := indices[n.Item.ID]; !seen {
			strongconnect(n)
		}
	}
}

func (g *Graph) quarantineIfCyclic(scc []*Node) {
	if len(scc) > 1 || (len(scc) == 1 && sccHasSelfEdge(scc[0])) {
		ids := sccMemberIDs(scc)
		detail := exampleChain(scc)
		parts := strings.Split(detail, " -> ")
		b := parts[0]
		if len(parts) > 1 {
			b = parts[1]
		}
		f := Fault{
			Reason: item.ReasonCycle,
			IDs:    ids,
			Detail: detail,
			Fix:    fmt.Sprintf("awit dep rm %s %s (break the cycle)", parts[0], b),
		}
		for _, n := range scc {
			n.Faults = append(n.Faults, f)
		}
		g.Faults = append(g.Faults, f)
	}
}

func sccHasSelfEdge(n *Node) bool {
	for _, d := range n.Deps {
		if d.Item.ID == n.Item.ID {
			return true
		}
	}
	return false
}

func sccMemberIDs(scc []*Node) []string {
	ids := make([]string, len(scc))
	for i, n := range scc {
		ids[i] = n.Item.ID
	}
	sort.Strings(ids)
	return ids
}

func exampleChain(scc []*Node) string {
	members := make(map[string]*Node, len(scc))
	ids := make([]string, 0, len(scc))
	for _, n := range scc {
		members[n.Item.ID] = n
		ids = append(ids, n.Item.ID)
	}
	sort.Strings(ids)
	start := ids[0]
	parts := []string{start}
	cur := start
	for {
		n := members[cur]
		next := ""
		for _, d := range n.Deps {
			if _, ok := members[d.Item.ID]; ok {
				next = d.Item.ID
				break
			}
		}
		if next == "" {
			break
		}
		parts = append(parts, next)
		if next == start {
			break
		}
		cur = next
	}
	return strings.Join(parts, " -> ")
}
