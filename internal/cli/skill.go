package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/eisenwinter/awit/internal/skill"
	"github.com/eisenwinter/awit/pkg/config"
	"github.com/urfave/cli/v3"
)

var skillCmd = &cli.Command{
	Name:  "skill",
	Usage: "Refresh detected agent skills",
	Commands: []*cli.Command{
		skillSyncCmd,
	},
}

var skillSyncCmd = &cli.Command{
	Name:   "sync",
	Usage:  "Refresh every detected skill target, overwriting hand edits",
	Action: skillSyncAction,
}

func skillSyncAction(_ context.Context, cmd *cli.Command) error {
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	noteWalkedUp(cmd, s)
	for _, t := range skill.Detect(s.Root) {
		dest := filepath.Join(s.Root, t.Dir, t.Path)
		shown := filepath.ToSlash(filepath.Join(t.Dir, t.Path))
		rendered := skill.Render(t)
		existing, err := os.ReadFile(dest)
		switch {
		case err == nil && bytes.Equal(existing, rendered):
			fmt.Fprintf(cmd.Root().Writer, "current %s\n", shown)
		case err == nil:
			if err := config.WriteAtomic(dest, rendered); err != nil {
				return fmt.Errorf("%s: %w", shown, err)
			}
			fmt.Fprintf(cmd.Root().Writer, "updated %s\n", shown)
		case errors.Is(err, os.ErrNotExist):
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return fmt.Errorf("%s: %w", shown, err)
			}
			if err := config.WriteAtomic(dest, rendered); err != nil {
				return fmt.Errorf("%s: %w", shown, err)
			}
			fmt.Fprintf(cmd.Root().Writer, "created %s\n", shown)
		default:
			return fmt.Errorf("%s: %w", shown, err)
		}
	}
	return nil
}
