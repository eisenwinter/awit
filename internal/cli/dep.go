package cli

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var depCmd = &cli.Command{
	Name:  "dep",
	Usage: "Add or remove item dependencies",
	Commands: []*cli.Command{
		{
			Name:      "add",
			Usage:     "Add a dependency with cycle pre-check",
			ArgsUsage: "<id> <dep>",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if cmd.Args().Len() != 2 {
					return cli.Exit("dep add needs <id> <dep>", 2)
				}
				return depAdd(cmd, cmd.Args().Get(0), cmd.Args().Get(1))
			},
		},
		{
			Name:      "rm",
			Usage:     "Remove a dependency",
			ArgsUsage: "<id> <dep>",
			Action: func(ctx context.Context, cmd *cli.Command) error {
				if cmd.Args().Len() != 2 {
					return cli.Exit("dep rm needs <id> <dep>", 2)
				}
				return depRm(cmd, cmd.Args().Get(0), cmd.Args().Get(1))
			},
		},
	},
}

func depAdd(cmd *cli.Command, id, dep string) error {
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
	g, err := loadGraph(s)
	if err != nil {
		return err
	}
	// Dependency CLI inputs accept ids, aliases and external keys; the
	// serialized edge is always the canonical AWIT ID.
	id, err = resolveItemID(graphItems(g), id)
	if err != nil {
		return err
	}
	dep, err = resolveItemID(graphItems(g), dep)
	if err != nil {
		return err
	}
	it := g.Nodes[id].Item
	if slices.Contains(it.Deps, dep) {
		fmt.Fprintln(cmd.Root().Writer, "dependency already present")
		return nil
	}
	if cyc := g.WouldCycle(id, dep); cyc != nil {
		fmt.Fprintf(cmd.Root().ErrWriter, "Error: cannot add dependency %s to %s.\n", dep, id)
		fmt.Fprintf(cmd.Root().ErrWriter, "Cycle: %s\n", strings.Join(cyc, " -> "))
		return cli.Exit("", 1)
	}
	next := append(slices.Clone(it.Deps), dep)
	it.SetDeps(next)
	if err := s.Save(it); err != nil {
		return err
	}
	return printCompact(cmd, s, id)
}

func depRm(cmd *cli.Command, id, dep string) error {
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
	g, err := loadGraph(s)
	if err != nil {
		return err
	}
	// Dependency CLI inputs accept ids, aliases and external keys; the
	// serialized edge is always the canonical AWIT ID.
	id, err = resolveItemID(graphItems(g), id)
	if err != nil {
		return err
	}
	dep, err = resolveItemID(graphItems(g), dep)
	if err != nil {
		return err
	}
	it := g.Nodes[id].Item
	if !slices.Contains(it.Deps, dep) {
		return fmt.Errorf("%s does not depend on %s", id, dep)
	}
	next := make([]string, 0, len(it.Deps)-1)
	for _, d := range it.Deps {
		if d != dep {
			next = append(next, d)
		}
	}
	it.SetDeps(next)
	if err := s.Save(it); err != nil {
		return err
	}
	return printCompact(cmd, s, id)
}

// printCompact reloads the graph so the printed unblock count reflects the
// edit, then prints one compact line for id.
func printCompact(cmd *cli.Command, s *item.Store, id string) error {
	g, err := loadGraph(s)
	if err != nil {
		return err
	}
	n, ok := g.Nodes[id]
	if !ok {
		return fmt.Errorf("unknown item %s", id)
	}
	fmt.Fprintln(cmd.Root().Writer, format.Line(toEntry(n)))
	return nil
}
