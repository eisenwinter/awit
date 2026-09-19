package cli

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
)

var refCmd = &cli.Command{
	Name:  "ref",
	Usage: "Add or remove file references on an item",
	Commands: []*cli.Command{
		{
			Name:      "add",
			Usage:     "Add a repo-root-relative file reference",
			ArgsUsage: "<id> <path>",
			Action: func(_ context.Context, cmd *cli.Command) error {
				if cmd.Args().Len() != 2 {
					return cli.Exit("ref add needs <id> <path>", 2)
				}
				return refAdd(cmd, cmd.Args().Get(0), cmd.Args().Get(1))
			},
		},
		{
			Name:      "rm",
			Usage:     "Remove a file reference",
			ArgsUsage: "<id> <path>",
			Action: func(_ context.Context, cmd *cli.Command) error {
				if cmd.Args().Len() != 2 {
					return cli.Exit("ref rm needs <id> <path>", 2)
				}
				return refRm(cmd, cmd.Args().Get(0), cmd.Args().Get(1))
			},
		},
	},
}

func normalizeRefArg(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	slash := filepath.ToSlash(p)
	slash = strings.ReplaceAll(slash, "\\", "/")
	if filepath.IsAbs(p) || path.IsAbs(slash) {
		return "", fmt.Errorf("absolute path")
	}
	cleaned := path.Clean(slash)
	if cleaned == "." || cleaned == "" {
		return "", fmt.Errorf("empty path")
	}
	return cleaned, nil
}

func refAdd(cmd *cli.Command, id, raw string) error {
	p, err := normalizeRefArg(raw)
	if err != nil {
		return err
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
	if err := s.NormalizeRefs(it); err != nil {
		return err
	}
	if slices.Contains(it.Refs, p) {
		fmt.Fprintf(cmd.Root().Writer, "ref already present %s: %s\n", it.ID, p)
		return nil
	}
	it.SetRefs(append(slices.Clone(it.Refs), p))
	if err := s.Save(it); err != nil {
		return err
	}
	fmt.Fprintf(cmd.Root().Writer, "ref added %s: %s\n", it.ID, p)
	return nil
}

func refRm(cmd *cli.Command, id, raw string) error {
	p, err := normalizeRefArg(raw)
	if err != nil {
		return err
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
	if err := s.NormalizeRefs(it); err != nil {
		return err
	}
	if !slices.Contains(it.Refs, p) {
		return fmt.Errorf("%s has no ref %s", id, p)
	}
	next := make([]string, 0, len(it.Refs)-1)
	for _, r := range it.Refs {
		if r != p {
			next = append(next, r)
		}
	}
	it.SetRefs(next)
	if err := s.Save(it); err != nil {
		return err
	}
	fmt.Fprintf(cmd.Root().Writer, "ref removed %s: %s\n", it.ID, p)
	return nil
}
