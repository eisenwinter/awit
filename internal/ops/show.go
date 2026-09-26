package ops

import (
	"fmt"
	"strings"

	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/eisenwinter/awit/pkg/resolver"
)

// ShowFull is the `show <id> --full` text: the default view plus every
// ref resolved against the item's refs base (items dir, or repo root for
// refs_base: repo).
func ShowFull(s *item.Store, g *graph.Graph, n *graph.Node) string {
	return fullView(g, n, RefsBaseDir(s, n), s.ItemsDir())
}

// RefsBaseDir is the directory n's refs resolve against.
func RefsBaseDir(s *item.Store, n *graph.Node) string {
	if n.Item.RefsBase == "repo" {
		return s.Root
	}
	return s.ItemsDir()
}

// DefaultView renders the core item: header, status, faults, deps,
// assignee, brief, ref count, blank line, verbatim body.
func DefaultView(n *graph.Node) string {
	e := ToEntry(n)
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] %s\n", e.ID, e.Title)
	state := e.State
	if n.Quarantined() {
		state = "QUARANTINED"
	}
	labels := strings.Join(e.Labels, ",")
	if labels == "" {
		labels = "-"
	}
	fmt.Fprintf(&b, "status: %s (%s) | labels: %s | unblocks: %d\n", e.Status, state, labels, e.Unblocks)
	for _, f := range n.Faults {
		fmt.Fprintf(&b, "fault: [%s] %s\n", string(f.Reason), f.Detail)
	}
	deps := "-"
	if len(e.Deps) > 0 {
		deps = strings.Join(e.Deps, ", ")
	}
	fmt.Fprintf(&b, "deps: %s\n", deps)
	assignee := e.Assignee
	if assignee == "" {
		assignee = "-"
	}
	fmt.Fprintf(&b, "assignee: %s\n", assignee)
	if e.BlockedReason != "" {
		fmt.Fprintf(&b, "blocked_reason: %s\n", e.BlockedReason)
	}
	fmt.Fprintf(&b, "brief: %s\n", e.Brief)
	if n.Item.External != nil {
		x := n.Item.External
		fmt.Fprintf(&b, "external: %s %s#%d %s\n", x.Tracker, x.Repo, x.ID, x.URL)
	}
	if n.Item.Alias != "" {
		fmt.Fprintf(&b, "alias: %s\n", n.Item.Alias)
	}
	fmt.Fprintf(&b, "refs: %d (use --full)\n", len(n.Item.Refs))
	body := n.Item.Body()
	// Bodies start with the blank line after the closing fence, so add the
	// separator only when the body does not already begin with a blank
	// line. The body itself is always written byte-for-byte.
	if len(body) == 0 || (body[0] != '\n' && body[0] != '\r') {
		b.WriteString("\n")
	}
	b.Write(body)
	if len(body) > 0 && body[len(body)-1] != '\n' {
		b.WriteString("\n")
	}
	return b.String()
}

// BrokenView renders a file that could not become an item. Exit stays 0:
// the user asked what is there, and something is there.
func BrokenView(id string, broken []item.Broken) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] (unparseable)\n", id)
	for _, br := range broken {
		fmt.Fprintf(&b, "fault: [%s] %s\n", string(br.Reason), br.Detail)
	}
	return b.String()
}

// fullView is the default view plus one delimited block per ref. Item
// refs render the target's default view; their refs are not followed.
func fullView(g *graph.Graph, n *graph.Node, baseDir, itemsDir string) string {
	var b strings.Builder
	b.WriteString(DefaultView(n))
	resolved := resolver.Resolve(baseDir, n.Item.Refs)
	for i, r := range resolved {
		fmt.Fprintf(&b, "===== REF %d/%d: %s =====\n", i+1, len(resolved), r.Ref)
		b.WriteString(refBody(g, itemsDir, r))
		fmt.Fprintf(&b, "===== END REF %d/%d =====\n", i+1, len(resolved))
	}
	return b.String()
}

// refBody renders one ref's content, always ending in "\n".
func refBody(g *graph.Graph, itemsDir string, r resolver.Resolved) string {
	if r.Err != nil {
		return "[missing]\n"
	}
	if id, ok := resolver.IsItemRef(itemsDir, r.Path); ok {
		if target, ok := g.Nodes[id]; ok {
			return DefaultView(target)
		}
		// Item file exists on disk but did not parse: show the same
		// fault block `show <id>` would, without failing.
		var matches []item.Broken
		for _, br := range g.Broken {
			if br.ID == id {
				matches = append(matches, br)
			}
		}
		if len(matches) > 0 {
			return BrokenView(id, matches)
		}
	}
	if len(r.Content) == 0 {
		return "\n"
	}
	s := string(r.Content)
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	return s
}
