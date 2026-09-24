package lazy

import (
	"fmt"
	"strings"

	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

// Filter is the Issues tab selection: awit list state/status/label flags
// plus a case-insensitive substring search over id, title and labels.
type Filter struct {
	Ready, Blocked, Quarantined bool
	Statuses                    []item.Status
	Labels                      [][]string
	Search                      string // lower-cased; "" = none
}

// ParseQuery reads the / prompt grammar (plan §D.9). Unknown -x tokens
// return "unknown flag -x"; a bad status returns the item.ParseStatus error.
func ParseQuery(q string) (Filter, error) {
	var f Filter
	var search []string
	toks := strings.Fields(q)
	for i := 0; i < len(toks); i++ {
		tok := toks[i]
		switch tok {
		case "--ready":
			f.Ready = true
		case "--blocked":
			f.Blocked = true
		case "--quarantined":
			f.Quarantined = true
		case "-s", "--status":
			if i+1 >= len(toks) {
				return Filter{}, fmt.Errorf("flag %s requires a value", tok)
			}
			i++
			for _, part := range strings.Split(toks[i], ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				st, err := item.ParseStatus(part)
				if err != nil {
					return Filter{}, err
				}
				f.Statuses = append(f.Statuses, st)
			}
		case "-l", "--label":
			if i+1 >= len(toks) {
				return Filter{}, fmt.Errorf("flag %s requires a value", tok)
			}
			i++
			if group := splitLabelGroup(toks[i]); len(group) > 0 {
				f.Labels = append(f.Labels, group)
			}
		default:
			if strings.HasPrefix(tok, "-") {
				return Filter{}, fmt.Errorf("unknown flag %s", tok)
			}
			search = append(search, tok)
		}
	}
	if len(search) > 0 {
		f.Search = strings.ToLower(strings.Join(search, " "))
	}
	return f, nil
}

// splitLabelGroup mirrors internal/cli.SplitLabels for one -l value: trim,
// drop empties; an all-empty flag contributes no group.
func splitLabelGroup(flag string) []string {
	var group []string
	for _, part := range strings.Split(flag, ",") {
		if label := strings.TrimSpace(part); label != "" {
			group = append(group, label)
		}
	}
	return group
}

func (f Filter) badges() string {
	var parts []string
	if len(f.Statuses) > 0 {
		names := make([]string, len(f.Statuses))
		for i, st := range f.Statuses {
			names[i] = string(st)
		}
		parts = append(parts, "[-s "+strings.Join(names, ",")+"]")
	}
	for _, group := range f.Labels {
		parts = append(parts, "[-l "+strings.Join(group, "|")+"]")
	}
	if f.Ready {
		parts = append(parts, "[--ready]")
	}
	if f.Blocked {
		parts = append(parts, "[--blocked]")
	}
	if f.Quarantined {
		parts = append(parts, "[--quarantined]")
	}
	if f.Search != "" {
		parts = append(parts, "[/ "+f.Search+"]")
	}
	if len(parts) == 0 {
		return "no filter"
	}
	return strings.Join(parts, " ")
}

func (f Filter) applyOpen(g *graph.Graph) []*graph.Node {
	nodes := g.Filter(graph.Filter{
		Ready:       f.Ready,
		Blocked:     f.Blocked,
		Quarantined: f.Quarantined,
		Statuses:    f.Statuses,
		Labels:      f.Labels,
	})
	if f.Search == "" {
		return nodes
	}
	var out []*graph.Node
	for _, n := range nodes {
		if matchSearch(n.Item, f.Search) {
			out = append(out, n)
		}
	}
	return out
}

func (f Filter) applyArchive(items []*item.Item) []*item.Item {
	var allow map[item.Status]bool
	if len(f.Statuses) > 0 {
		allow = make(map[item.Status]bool, len(f.Statuses))
		for _, st := range f.Statuses {
			allow[st] = true
		}
	}
	var out []*item.Item
	for _, it := range items {
		if allow != nil && !allow[it.Status] {
			continue
		}
		if !graph.MatchLabels(it.Labels, f.Labels) {
			continue
		}
		if f.Search != "" && !matchSearch(it, f.Search) {
			continue
		}
		out = append(out, it)
	}
	return out
}

func matchSearch(it *item.Item, q string) bool {
	if strings.Contains(strings.ToLower(it.ID), q) {
		return true
	}
	if strings.Contains(strings.ToLower(it.Title), q) {
		return true
	}
	for _, label := range it.Labels {
		if strings.Contains(strings.ToLower(label), q) {
			return true
		}
	}
	return false
}
