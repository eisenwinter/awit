package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var updateCmd = &cli.Command{
	Name: "update", Usage: "Change fields on an existing item", ArgsUsage: "<id>",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "status"},
		&cli.StringFlag{Name: "brief"},
		&cli.StringFlag{Name: "assign"},
		&cli.StringFlag{Name: "title"},
		&cli.StringSliceFlag{Name: "label", Aliases: []string{"l"}},
		&cli.StringSliceFlag{Name: "unlabel"},
	},
	Action: updateAction,
}

func splitFlagCSV(values []string) []string {
	var out []string
	for _, v := range values {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func updateAction(_ context.Context, cmd *cli.Command) error {
	id := cmd.Args().First()
	if id == "" {
		return fmt.Errorf("update needs an item id")
	}
	// NOTE: package-level updateCmd is reused across Main calls in-process,
	// and urfave's hasBeenSet persists while values reset (see createAction),
	// so IsSet misreports flags from earlier runs. Detect via values instead.
	status, brief, assign, title := cmd.String("status"), cmd.String("brief"), cmd.String("assign"), cmd.String("title")
	add, remove := splitFlagCSV(cmd.StringSlice("label")), splitFlagCSV(cmd.StringSlice("unlabel"))
	if status == "" && brief == "" && assign == "" && title == "" && len(add) == 0 && len(remove) == 0 {
		return fmt.Errorf("nothing to update")
	}
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	it, err := loadItem(s, id)
	if err != nil {
		return err
	}
	if status != "" {
		st, err := item.ParseStatus(status)
		if err != nil {
			return err
		}
		it.SetStatus(st)
		if st == item.StatusClosed {
			it.SetClaimedAt(nil)
		}
	}
	if brief != "" {
		it.SetBrief(brief)
	}
	if assign != "" {
		it.SetAssignee(assign)
	}
	if title != "" {
		it.SetTitle(title)
	}
	if len(add) > 0 || len(remove) > 0 {
		seen := map[string]bool{}
		var labels []string
		for _, l := range it.Labels {
			seen[l] = true
			labels = append(labels, l)
		}
		for _, l := range add {
			if !seen[l] {
				seen[l] = true
				labels = append(labels, l)
			}
		}
		drop := map[string]bool{}
		for _, l := range remove {
			drop[l] = true
		}
		kept := []string{}
		for _, l := range labels {
			if !drop[l] {
				kept = append(kept, l)
			}
		}
		it.SetLabels(kept)
	}
	return s.Save(it)
}
