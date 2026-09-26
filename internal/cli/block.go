package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/eisenwinter/awit/internal/ops"
	"github.com/urfave/cli/v3"
)

var blockCmd = &cli.Command{
	Name:      "block",
	Usage:     "Pause an item with a recorded reason until it is unblocked",
	ArgsUsage: "<id>",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "reason", Usage: "why work stopped and what will unblock it"},
	},
	Description: `Record a local manual block; dependency edges and tracker labels are
unchanged. The reason must be non-empty and single-line. The item's status
becomes open and its claim is cleared. Run awit unblock <id> once the
condition is resolved.`,
	Action: blockAction,
}

var unblockCmd = &cli.Command{
	Name:      "unblock",
	Usage:     "Remove an item's manual block",
	ArgsUsage: "<id>",
	Description: `Remove only the manual block. Open dependencies can still keep the item
blocked. Unblocking does not claim or reopen the item and does not change
tracker labels.`,
	Action: unblockAction,
}

func blockAction(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return cli.Exit("block takes exactly one item id", 2)
	}
	// An absent --reason reads as "" like an explicit empty one; both are
	// usage errors. Deeper validation (whitespace-only, multi-line,
	// control characters) happens in SetBlockedReason below, which never
	// mutates on error.
	if cmd.String("reason") == "" {
		return cli.Exit("block requires --reason with a non-empty, single-line explanation", 2)
	}
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
	it, err := ops.LoadItem(s, cmd.Args().First())
	if err != nil {
		return err
	}
	if err := ops.BlockItem(s, it, cmd.String("reason")); err != nil {
		return err
	}
	fmt.Fprintf(cmd.Root().Writer, "blocked %s: %s\n", it.ID, it.BlockedReason)
	return nil
}

func unblockAction(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return cli.Exit("unblock takes exactly one item id", 2)
	}
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
	it, err := ops.LoadItem(s, cmd.Args().First())
	if err != nil {
		return err
	}
	if err := ops.UnblockItem(s, it); err != nil {
		return err
	}
	fmt.Fprintf(cmd.Root().Writer, "unblocked %s\n", it.ID)
	return nil
}
