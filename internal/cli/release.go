package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/eisenwinter/awit/internal/ops"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var releaseCmd = &cli.Command{
	Name: "release", Usage: "Return an item to open and clear its claim; any manual block stays", ArgsUsage: "<id>",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "push", Usage: "`true|false` overrides the external-push policy (config.yaml external_push:, default true)"},
		&cli.BoolFlag{Name: "no-push", Usage: "skip pushing the reopened state to the linked external issue"},
		&cli.StringFlag{Name: "tea-login", Usage: "tea login name for a Gitea issue; ignored for GitLab"},
	},
	Action: releaseAction,
}

func releaseAction(ctx context.Context, cmd *cli.Command) error {
	id := cmd.Args().First()
	if id == "" {
		return fmt.Errorf("release needs an item id")
	}
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	noteWalkedUp(cmd, s)
	push, err := externalPushPolicy(cmd, s.Config)
	if err != nil {
		return err
	}
	release, err := s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer release()
	it, err := ops.LoadItem(s, id)
	if err != nil {
		return err
	}
	if err := ops.ReleaseItem(s, it); err != nil {
		return err
	}
	fmt.Fprintf(cmd.Root().Writer, "reopened %s\n", it.ID)
	if items, _, err := s.LoadAll(); err == nil {
		maybePushExternalState(ctx, cmd, items, it, "open", push)
	} else {
		maybePushExternalState(ctx, cmd, []*item.Item{it}, it, "open", push)
	}
	return nil
}
