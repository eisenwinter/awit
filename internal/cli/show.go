package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var showCmd = &cli.Command{
	Name:      "show",
	Usage:     "Show one item (default view)",
	ArgsUsage: "<id>",
	Action: func(ctx context.Context, cmd *cli.Command) error {
		if cmd.Args().Len() != 1 {
			return cli.Exit("show needs an item id", 2)
		}
		return showOne(cmd, cmd.Args().Get(0))
	},
}

// showJSON is the --format json shape: the list entry plus the raw body.
// AWIT-0ND5703G adds a Refs field; keep the embedding so that still works.
type showJSON struct {
	format.Entry
	Body string `json:"body"`
}

func showOne(cmd *cli.Command, id string) error {
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	g, err := loadGraph(s)
	if err != nil {
		return err
	}
	f, err := detectFormat(cmd)
	if err != nil {
		return err
	}
	if n, ok := g.Nodes[id]; ok {
		if f == format.JSON {
			out, err := json.MarshalIndent(showJSON{Entry: toEntry(n), Body: string(n.Item.Body())}, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.Root().Writer, "%s\n", out)
			return nil
		}
		fmt.Fprint(cmd.Root().Writer, defaultView(n))
		return nil
	}
	var matches []item.Broken
	for _, br := range g.Broken {
		if br.ID == id {
			matches = append(matches, br)
		}
	}
	if len(matches) > 0 {
		// Broken files always render the text view, even as json.
		fmt.Fprint(cmd.Root().Writer, brokenView(id, matches))
		return nil
	}
	return fmt.Errorf("unknown item %s", id)
}

// defaultView renders the core ticket: header, status, faults, deps,
// assignee, brief, ref count, blank line, verbatim body.
func defaultView(n *graph.Node) string {
	e := toEntry(n)
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
	fmt.Fprintf(&b, "brief: %s\n", e.Brief)
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

// brokenView renders a file that could not become an item. Exit stays 0:
// the user asked what is there, and something is there.
func brokenView(id string, broken []item.Broken) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] (unparseable)\n", id)
	for _, br := range broken {
		fmt.Fprintf(&b, "fault: [%s] %s\n", string(br.Reason), br.Detail)
	}
	return b.String()
}
