package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/eisenwinter/awit/internal/ops"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var closeCmd = &cli.Command{
	Name: "close", Usage: "Mark an item closed, clearing the claim timestamp and any manual block", ArgsUsage: "<id>",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "reason"},
		&cli.StringFlag{Name: "author"},
		&cli.StringFlag{Name: "push", Usage: "`true|false` overrides the external-push policy (config.yaml external_push:, default true)"},
		&cli.BoolFlag{Name: "no-push", Usage: "skip pushing the closed state to the linked external issue"},
		&cli.StringFlag{Name: "tea-login", Usage: "tea login name for a Gitea issue; ignored for GitLab"},
	},
	Action: closeAction,
}

func closeAction(ctx context.Context, cmd *cli.Command) error {
	id := cmd.Args().First()
	if id == "" {
		return fmt.Errorf("close needs an item id")
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
	reason := cmd.String("reason")
	author := ""
	if reason != "" {
		if author, err = ops.ResolveAuthor(cmd.String("author"), s.Root, s.Config); err != nil {
			return err
		}
	}
	if err := ops.CloseItem(s, it, reason, author, time.Now().UTC()); err != nil {
		return err
	}
	fmt.Fprintf(cmd.Root().Writer, "closed %s\n", it.ID)
	if items, _, err := s.LoadAll(); err == nil {
		maybePushExternalState(ctx, cmd, items, it, "closed", push)
	} else {
		maybePushExternalState(ctx, cmd, []*item.Item{it}, it, "closed", push)
	}
	return nil
}
