package cli

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var labelCmd = &cli.Command{
	Name:  "label",
	Usage: "Show observed label usage counts",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "state",
			Value: "open",
			Usage: "count items whose status is open, closed or all (default open)",
		},
	},
	Action: labelAction,
}

func labelAction(_ context.Context, cmd *cli.Command) error {
	state := cmd.String("state")
	switch state {
	case "open", "closed", "all":
	default:
		return fmt.Errorf("--state must be open, closed or all")
	}
	store, err := openStore(cmd)
	if err != nil {
		return err
	}
	items, _, err := store.LoadAll() // broken files are deliberately not counted
	if err != nil {
		return err
	}
	fm, err := detectFormat(cmd)
	if err != nil {
		return err
	}
	return format.WriteLabels(cmd.Root().Writer, fm, labelCounts(items, state))
}

// labelCounts builds the label vocabulary rows over the parseable items.
// Optional config.yaml labels is an advisory vocabulary only: this command
// still reports actual use (guide §2). Used undeclared labels count; unused
// declared names do not. state is one of open|closed|all and was validated
// by the caller: "open" counts every status except closed, "closed" only
// closed, "all" everything. Graph quarantine is irrelevant here — a
// parseable item with a dangling dep still carries its labels — and
// unparseable files are not in items at all, so they never count. Rows
// sort by count descending, then label ascending, so the output is
// deterministic.
func labelCounts(items []*item.Item, state string) []format.LabelCount {
	counts := make(map[string]int)
	for _, it := range items {
		switch state {
		case "open":
			if it.Status == item.StatusClosed {
				continue
			}
		case "closed":
			if it.Status != item.StatusClosed {
				continue
			}
		}
		for _, label := range it.Labels {
			counts[label]++
		}
	}
	rows := make([]format.LabelCount, 0, len(counts))
	for label, n := range counts {
		rows = append(rows, format.LabelCount{Label: label, Count: n})
	}
	slices.SortFunc(rows, func(a, b format.LabelCount) int {
		return cmp.Or(
			cmp.Compare(b.Count, a.Count), // count descending
			cmp.Compare(a.Label, b.Label), // label ascending
		)
	})
	return rows
}
