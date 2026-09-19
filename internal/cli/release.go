package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var releaseCmd = &cli.Command{
	Name: "release", Usage: "Return an in-progress or closed item to open and clear its claim", ArgsUsage: "<id>",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "no-push", Usage: "skip pushing the reopened state to the linked Gitea issue"},
		&cli.StringFlag{Name: "tea-login", Usage: "tea login name for the issue's instance"},
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
	release, err := s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer release()
	it, err := loadItem(s, id)
	if err != nil {
		return err
	}
	it.SetStatus(item.StatusOpen)
	it.SetAssignee("")
	it.SetClaimedAt(nil)
	if err := s.Save(it); err != nil {
		return err
	}
	fmt.Fprintf(cmd.Root().Writer, "reopened %s\n", it.ID)
	if items, _, err := s.LoadAll(); err == nil {
		maybePushExternalState(ctx, cmd, items, it, "open")
	} else {
		maybePushExternalState(ctx, cmd, []*item.Item{it}, it, "open")
	}
	return nil
}
