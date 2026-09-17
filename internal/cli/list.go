package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var listCmd = &cli.Command{
	Name:  "list",
	Usage: "List items",
	// urfave splits slice-flag values on "," by default, which would turn
	// "-l auth,db" into two ANDed groups. Disable it so SplitLabels sees
	// each -l occurrence intact (OR within a flag, AND across flags).
	DisableSliceFlagSeparator: true,
	Flags: []cli.Flag{
		&cli.StringSliceFlag{Name: "status", Aliases: []string{"s"}, Usage: "filter by status (open, in_progress, closed); repeatable, OR"},
		&cli.StringSliceFlag{Name: "label", Aliases: []string{"l"}, Usage: "AND across flags, OR within a flag"},
		&cli.BoolFlag{Name: "ready", Usage: "include ready items"},
		&cli.BoolFlag{Name: "blocked", Usage: "include blocked items"},
		&cli.BoolFlag{Name: "quarantined", Usage: "include quarantined items"},
	},
	Action: listAction,
}

func listAction(_ context.Context, cmd *cli.Command) error {
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	g, err := loadGraph(s)
	if err != nil {
		return err
	}

	ready := cmd.Bool("ready")
	blocked := cmd.Bool("blocked")
	quarantined := cmd.Bool("quarantined")

	var nodes []*graph.Node
	switch {
	case ready && !blocked && !quarantined:
		nodes = g.Ready()
	case ready || blocked || quarantined:
		for _, n := range g.Order {
			if (ready && n.Ready) || (blocked && n.Blocked) || (quarantined && n.Quarantined()) {
				nodes = append(nodes, n)
			}
		}
	default:
		nodes = g.Order
	}

	if statuses := cmd.StringSlice("status"); len(statuses) > 0 {
		allow := map[item.Status]bool{}
		for _, raw := range statuses {
			for _, part := range strings.Split(raw, ",") {
				part = strings.TrimSpace(part)
				if part == "" {
					continue
				}
				st, err := item.ParseStatus(part)
				if err != nil {
					return err
				}
				allow[st] = true
			}
		}
		var filtered []*graph.Node
		for _, n := range nodes {
			if allow[n.Item.Status] {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
	}

	nodes = graph.FilterLabels(nodes, SplitLabels(cmd.StringSlice("label")))

	f, err := detectFormat(cmd)
	if err != nil {
		return err
	}
	entries := make([]format.Entry, 0, len(nodes))
	for _, n := range nodes {
		entries = append(entries, toEntry(n))
	}
	if err := format.Write(cmd.Root().Writer, f, entries); err != nil {
		return fmt.Errorf("format: %w", err)
	}
	return nil
}
