package ops

import (
	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

// ToEntry converts a graph node into the format-neutral row every
// tabular command prints.
func ToEntry(n *graph.Node) format.Entry {
	e := format.Entry{
		ID:            n.Item.ID,
		Title:         n.Item.Title,
		Brief:         n.Item.Brief,
		Status:        string(n.Item.Status),
		BlockedReason: n.Item.BlockedReason,
		Labels:        n.Item.Labels,
		Deps:          n.Item.Deps,
		Assignee:      n.Item.Assignee,
		Alias:         n.Item.Alias,
		Unblocks:      n.UnblockCount,
		External:      n.Item.External,
	}
	if e.Labels == nil {
		e.Labels = []string{}
	}
	if e.Deps == nil {
		e.Deps = []string{}
	}
	switch {
	case n.Quarantined():
		e.State = "quarantined"
		for _, f := range n.Faults {
			e.Faults = append(e.Faults, "["+string(f.Reason)+"] "+f.Detail)
		}
	case n.Item.Status == item.StatusClosed:
		e.State = "closed"
	case n.Ready:
		e.State = "ready"
	default:
		e.State = "blocked"
	}
	return e
}

// ArchiveEntry converts an archived item into a format-neutral row. Archived
// items are always closed and unblock nothing.
func ArchiveEntry(it *item.Item) format.Entry {
	e := format.Entry{
		ID:            it.ID,
		Title:         it.Title,
		Brief:         it.Brief,
		Status:        string(it.Status),
		State:         "closed",
		BlockedReason: it.BlockedReason,
		Labels:        it.Labels,
		Deps:          it.Deps,
		Assignee:      it.Assignee,
		Alias:         it.Alias,
		Unblocks:      0,
		External:      it.External,
	}
	if e.Labels == nil {
		e.Labels = []string{}
	}
	if e.Deps == nil {
		e.Deps = []string{}
	}
	return e
}
