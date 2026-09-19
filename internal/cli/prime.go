package cli

import (
	"context"

	"github.com/eisenwinter/awit/pkg/prime"
	"github.com/urfave/cli/v3"
)

var primeCmd = &cli.Command{
	Name:  "prime",
	Usage: "Print deterministic state snapshot for prompt injection",
	// urfave splits slice-flag values on "," by default, which would turn
	// "-l p0,p1" into two ANDed groups. Disable it so SplitLabels sees
	// each -l occurrence intact (OR within a flag, AND across flags).
	DisableSliceFlagSeparator: true,
	Flags: []cli.Flag{
		&cli.IntFlag{Name: "max-tokens", Usage: "token budget, 0 = unlimited"},
		&cli.StringSliceFlag{Name: "label", Aliases: []string{"l"}, Usage: "filter ready+blocked by label (repeatable)"},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		if max := cmd.Int("max-tokens"); max < 0 {
			return cli.Exit(`Incorrect usage: --max-tokens must be >= 0 (run "awit --help")`, 2)
		}
		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		g, err := loadGraph(s)
		if err != nil {
			return err
		}
		// NOTE: the global --format flag is intentionally ignored.
		return prime.Render(cmd.Root().Writer, g, prime.Options{
			MaxTokens: cmd.Int("max-tokens"),
			Labels:    SplitLabels(cmd.StringSlice("label")),
		})
	},
}
