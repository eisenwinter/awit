package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var closeCmd = &cli.Command{
	Name: "close", Usage: "Mark an item closed and clear its claim", ArgsUsage: "<id>",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "reason"},
		&cli.StringFlag{Name: "author"},
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
	release, err := s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer release()
	it, err := loadItem(s, id)
	if err != nil {
		return err
	}
	it.SetStatus(item.StatusClosed)
	it.SetClaimedAt(nil)
	if reason := cmd.String("reason"); reason != "" {
		author, err := resolveAuthor(cmd.String("author"), s.Root, s.Config)
		if err != nil {
			return err
		}
		if _, err = s.AddComment(it, author, time.Now().UTC(), reason); err != nil {
			return err
		}
	} else if err := s.Save(it); err != nil {
		return err
	}
	fmt.Fprintf(cmd.Root().Writer, "closed %s\n", it.ID)
	if items, _, err := s.LoadAll(); err == nil {
		maybePushExternalState(ctx, cmd, items, it, "closed")
	} else {
		maybePushExternalState(ctx, cmd, []*item.Item{it}, it, "closed")
	}
	return nil
}
