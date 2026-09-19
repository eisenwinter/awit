package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var updateCmd = &cli.Command{
	Name: "update", Usage: "Change fields on an existing item", ArgsUsage: "<id>",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "status", Usage: "set status: `open`, in_progress or closed"},
		&cli.StringFlag{Name: "brief"},
		&cli.StringFlag{Name: "assign"},
		&cli.StringFlag{Name: "title"},
		&cli.StringFlag{Name: "alias", Usage: "short human alias (e.g. `DTRM-F21`)"},
		&cli.BoolFlag{Name: "clear-alias", Usage: "remove the alias"},
		&cli.StringSliceFlag{Name: "label", Aliases: []string{"l"}},
		&cli.StringSliceFlag{Name: "unlabel"},
		&cli.StringFlag{Name: "external-tracker", Usage: "external tracker (`gitea` or `gitlab`)"},
		&cli.StringFlag{Name: "external-repo", Usage: "external repository (`owner/repo`; GitLab may include subgroups)"},
		&cli.StringFlag{Name: "external-id", Usage: "Gitea issue number or GitLab iid"},
		&cli.StringFlag{Name: "external-url", Usage: "external issue URL"},
		&cli.BoolFlag{Name: "clear-external", Usage: "remove external metadata"},
		&cli.BoolFlag{Name: "no-push", Usage: "skip pushing a status change to the linked external issue"},
		&cli.StringFlag{Name: "tea-login", Usage: "tea login name for a Gitea issue; ignored for GitLab"},
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

func updateAction(ctx context.Context, cmd *cli.Command) error {
	id := cmd.Args().First()
	if id == "" {
		return fmt.Errorf("update needs an item id")
	}
	// NOTE: package-level updateCmd is reused across Main calls in-process,
	// and urfave's hasBeenSet persists while values reset (see createAction),
	// so IsSet misreports flags from earlier runs. Detect via values instead.
	status, brief, assign, title := cmd.String("status"), cmd.String("brief"), cmd.String("assign"), cmd.String("title")
	alias, clearAlias := cmd.String("alias"), cmd.Bool("clear-alias")
	add, remove := splitFlagCSV(cmd.StringSlice("label")), splitFlagCSV(cmd.StringSlice("unlabel"))
	clearExt := cmd.Bool("clear-external")
	ext, err := parseExternalMapping(cmd)
	if err != nil {
		return err
	}
	if clearExt && ext != nil {
		return cli.Exit(`Incorrect usage: --clear-external cannot be combined with --external-tracker, --external-repo, --external-id, or --external-url`, 2)
	}
	if alias != "" && clearAlias {
		return cli.Exit(`Incorrect usage: --alias and --clear-alias cannot be combined`, 2)
	}
	if status == "" && brief == "" && assign == "" && title == "" && alias == "" && !clearAlias && len(add) == 0 && len(remove) == 0 && !clearExt && ext == nil {
		return fmt.Errorf("nothing to update")
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
	prev := make(map[string]bool, len(it.Labels))
	for _, l := range it.Labels {
		prev[l] = true
	}
	// Echo order is fixed: status, title, brief, assignee, labels, alias, external.
	var changed []string
	if status != "" {
		st, err := item.ParseStatus(status)
		if err != nil {
			return err
		}
		it.SetStatus(st)
		if st == item.StatusClosed {
			it.SetClaimedAt(nil)
		}
		changed = append(changed, "status="+string(st))
	}
	if title != "" {
		it.SetTitle(title)
		changed = append(changed, "title="+title)
	}
	if brief != "" {
		it.SetBrief(brief)
		changed = append(changed, "brief="+brief)
	}
	if assign != "" {
		it.SetAssignee(assign)
		changed = append(changed, "assignee="+assign)
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
		changed = append(changed, "labels="+strings.Join(kept, ","))
	}
	if clearAlias {
		if err := it.SetAlias(""); err != nil {
			return err
		}
		changed = append(changed, "alias=-")
	}
	if alias != "" {
		if err := it.SetAlias(alias); err != nil {
			return cli.Exit(err.Error(), 2)
		}
		changed = append(changed, "alias="+alias)
	}
	if clearExt {
		if err := it.SetExternal(nil); err != nil {
			return err
		}
		changed = append(changed, "external=-")
	}
	if ext != nil {
		same := it.External != nil && *it.External == *ext
		if err := it.SetExternal(ext); err != nil {
			return cli.Exit(err.Error(), 2)
		}
		if !same {
			changed = append(changed, fmt.Sprintf("external=%s %s#%d", ext.Tracker, ext.Repo, ext.ID))
		}
	}
	if err := s.Save(it); err != nil {
		return err
	}
	var introduced []string
	for _, l := range it.Labels {
		if !prev[l] {
			introduced = append(introduced, l)
		}
	}
	warnUnknownLabels(cmd, s.Config.Labels, introduced)
	w := cmd.Root().Writer
	for _, c := range changed {
		fmt.Fprintf(w, "updated %s: %s\n", it.ID, c)
	}
	if status != "" {
		remote := "open"
		if it.Status == item.StatusClosed {
			remote = "closed"
		}
		if items, _, err := s.LoadAll(); err == nil {
			maybePushExternalState(ctx, cmd, items, it, remote)
		} else {
			maybePushExternalState(ctx, cmd, []*item.Item{it}, it, remote)
		}
	}
	return nil
}
