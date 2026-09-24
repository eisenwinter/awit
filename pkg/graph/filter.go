package graph

import (
	"slices"

	"github.com/eisenwinter/awit/pkg/item"
)

// Filter is the awit list selection: state flags choose the source set
// (Ready alone keeps ranked order; any other combination scans Order in ID
// order; none means Order), then Statuses (OR) and Labels (AND across
// groups, OR within) narrow it.
type Filter struct {
	Ready, Blocked, Quarantined bool
	Statuses                    []item.Status
	Labels                      [][]string
}

// Filter selects the nodes a list view shows for f, in list order.
func (g *Graph) Filter(f Filter) []*Node {
	var nodes []*Node
	switch {
	case f.Ready && !f.Blocked && !f.Quarantined:
		nodes = g.Ready()
	case f.Ready || f.Blocked || f.Quarantined:
		for _, n := range g.Order {
			if (f.Ready && n.Ready) || (f.Blocked && n.Blocked) || (f.Quarantined && n.Quarantined()) {
				nodes = append(nodes, n)
			}
		}
	default:
		nodes = g.Order
	}
	return Narrow(nodes, f.Statuses, f.Labels)
}

// Narrow keeps nodes whose status is in statuses (empty = any) and that
// match every label group; order is preserved.
func Narrow(nodes []*Node, statuses []item.Status, labels [][]string) []*Node {
	if len(statuses) > 0 {
		allow := make(map[item.Status]bool, len(statuses))
		for _, st := range statuses {
			allow[st] = true
		}
		var filtered []*Node
		for _, n := range nodes {
			if allow[n.Item.Status] {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
	}
	return FilterLabels(nodes, labels)
}

// MatchLabels reports whether labels satisfy every group (AND), a group
// matching when any of its labels is present (OR). No groups = true.
func MatchLabels(labels []string, groups [][]string) bool {
	for _, group := range groups {
		matched := false
		for _, label := range group {
			if slices.Contains(labels, label) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	return true
}
