package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var prefixRE = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,7}$`)

var initCmd = &cli.Command{
	Name:  "init",
	Usage: "Create a .awit directory",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "prefix",
			Value: "AWIT",
			Usage: "item id prefix (2-8 uppercase alphanumerics starting with a letter)",
		},
	},
	Action: initAction,
}

func initAction(_ context.Context, cmd *cli.Command) error {
	prefix := cmd.String("prefix")
	if !prefixRE.MatchString(prefix) {
		return fmt.Errorf("prefix must be 2-8 uppercase alphanumerics starting with a letter")
	}
	root := cmd.Root().String("repo")
	var err error
	if root == "" {
		root, err = os.Getwd()
	} else {
		root, err = filepath.Abs(root)
	}
	if err != nil {
		return err
	}
	if _, err := item.Init(root, prefix); err != nil {
		return err
	}
	fmt.Fprintf(cmd.Root().Writer, "Initialized .awit in %s (prefix %s)\n", root, prefix)
	return nil
}
