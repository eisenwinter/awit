package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/eisenwinter/awit/internal/ops"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/urfave/cli/v3"
)

var archiveCmd = &cli.Command{
	Name:  "archive",
	Usage: "Move closed items nothing depends on to .awit/archive, one collapsed file each",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "dry-run", Usage: "print what would move; write nothing"},
	},
	Action: archiveAction,
}

func archiveAction(_ context.Context, cmd *cli.Command) error {
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	noteWalkedUp(cmd, s)
	release, err := s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer release()
	g, err := ops.LoadGraph(s)
	if err != nil {
		return err
	}
	warnQuarantined(cmd, g)
	w := cmd.Root().Writer
	set := g.Archivable()
	if cmd.Bool("dry-run") {
		in := make(map[*graph.Node]bool, len(set))
		for _, n := range set {
			in[n] = true
			fmt.Fprintf(w, "would archive %s\n", n.Item.ID)
		}
		for _, n := range g.Closed() {
			if in[n] || n.Quarantined() {
				continue
			}
			fmt.Fprintf(w, "skip %s: dependant %s not archivable\n", n.Item.ID, pinnedBy(n, in))
		}
		fmt.Fprintf(w, "Would archive %d items\n", len(set))
		return nil
	}
	for _, n := range set {
		if err := s.Archive(n.Item); err != nil {
			return fmt.Errorf("archive %s: %w", n.Item.ID, err)
		}
		fmt.Fprintf(w, "archived %s\n", n.Item.ID)
	}
	fmt.Fprintf(w, "Archived %d items\n", len(set))
	return nil
}

// pinnedBy returns the ID of the smallest Unblocks neighbour outside
// the archive set (Unblocks is sorted by ID in Build).
func pinnedBy(n *graph.Node, in map[*graph.Node]bool) string {
	for _, u := range n.Unblocks {
		if !in[u] {
			return u.Item.ID
		}
	}
	return "?" // unreachable: a closed non-quarantined node outside the set has such a neighbour
}
