package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var releaseCmd = &cli.Command{
	Name: "release", Usage: "Return an item to open and clear its claim", ArgsUsage: "<id>",
	Action: releaseAction,
}

func releaseAction(_ context.Context, cmd *cli.Command) error {
	id := cmd.Args().First()
	if id == "" {
		return fmt.Errorf("release needs an item id")
	}
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
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
	return s.Save(it)
}
