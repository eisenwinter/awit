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
	},
	Action: closeAction,
}

func closeAction(_ context.Context, cmd *cli.Command) error {
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
		_, err = s.AddComment(it, author, time.Now().UTC(), reason)
		return err
	}
	return s.Save(it)
}
