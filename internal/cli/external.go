package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/eisenwinter/awit/internal/glabx"
	"github.com/eisenwinter/awit/internal/ops"
	"github.com/eisenwinter/awit/internal/teax"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var externalCmd = &cli.Command{
	Name:  "external",
	Usage: "Check and repair external body drift for linked items",
	Commands: []*cli.Command{
		externalCheckCmd,
		externalPushBodyCmd,
	},
}

var externalCheckCmd = &cli.Command{
	Name:      "check",
	Usage:     "Compare local bodies against linked external issues (read-only)",
	ArgsUsage: "[key]",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "tea-login", Usage: "tea login name for a Gitea issue; ignored for GitLab"},
	},
	Action: externalCheckAction,
}

var externalPushBodyCmd = &cli.Command{
	Name:      "push-body",
	Usage:     "Push the local body bytes to the linked external issue (explicit repair)",
	ArgsUsage: "<key>",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "tea-login", Usage: "tea login name for a Gitea issue; ignored for GitLab"},
	},
	Action: externalPushBodyAction,
}

func externalCheckAction(ctx context.Context, cmd *cli.Command) error {
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	items, _, err := s.LoadAll()
	if err != nil {
		return err
	}
	login := cmd.String("tea-login")
	var targets []*item.Item
	if key := cmd.Args().First(); key != "" {
		id, err := ops.ResolveItemID(items, key)
		if err != nil {
			return err
		}
		var found *item.Item
		for _, it := range items {
			if it.ID == id {
				found = it
				break
			}
		}
		if found == nil {
			return fmt.Errorf("unknown item %s", key)
		}
		if found.External == nil {
			return fmt.Errorf("%s has no external link; link it with awit update --external-* first", found.ID)
		}
		targets = []*item.Item{found}
	} else {
		for _, it := range items {
			if it.External != nil || it.ExternalProblem != "" {
				targets = append(targets, it)
			}
		}
		sort.Slice(targets, func(i, j int) bool { return targets[i].ID < targets[j].ID })
	}
	rows := make([]ops.ExternalCheckRow, 0, len(targets))
	for _, it := range targets {
		rows = append(rows, ops.CheckOne(ctx, it, login))
	}
	if cmd.Root().String("format") == "json" {
		enc := json.NewEncoder(cmd.Root().Writer)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			return err
		}
		if failedRows(rows) {
			return cli.Exit("", 1)
		}
		return nil
	}
	var match, drift, faulty int
	for _, r := range rows {
		switch r.Result {
		case "match":
			match++
			fmt.Fprintf(cmd.Root().Writer, "MATCH %s %s\n", r.ID, r.URL)
		case "drift":
			drift++
			if r.Detail != "" {
				fmt.Fprintf(cmd.Root().Writer, "DRIFT %s %s: %s\n", r.ID, r.URL, r.Detail)
			} else {
				fmt.Fprintf(cmd.Root().Writer, "DRIFT %s %s\n", r.ID, r.URL)
			}
		default:
			faulty++
			if r.URL != "" {
				fmt.Fprintf(cmd.Root().Writer, "ERROR %s %s: %s\n", r.ID, r.URL, r.Detail)
			} else {
				fmt.Fprintf(cmd.Root().Writer, "ERROR %s: %s\n", r.ID, r.Detail)
			}
		}
	}
	fmt.Fprintf(cmd.Root().Writer, "Checked %d items: %d match, %d drift, %d error\n", len(rows), match, drift, faulty)
	if drift > 0 || faulty > 0 {
		return cli.Exit("", 1)
	}
	return nil
}

func failedRows(rows []ops.ExternalCheckRow) bool {
	for _, r := range rows {
		if r.Result != "match" {
			return true
		}
	}
	return false
}

func externalPushBodyAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return cli.Exit("push-body needs exactly one item key", 2)
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
	items, _, err := s.LoadAll()
	if err != nil {
		return err
	}
	id, err := ops.ResolveItemID(items, cmd.Args().First())
	if err != nil {
		return err
	}
	var target *item.Item
	for _, it := range items {
		if it.ID == id {
			target = it
			break
		}
	}
	if target == nil {
		return fmt.Errorf("unknown item %s", cmd.Args().First())
	}
	if target.External == nil {
		return fmt.Errorf("%s has no external link; link it with awit update --external-* first", target.ID)
	}
	ext := *target.External
	if dups := duplicateExternalLinks(items, ext); len(dups) > 1 {
		return fmt.Errorf("ambiguous external link %s#%d matches %s; refusing to push", ext.Repo, ext.ID, joinIDs(dups))
	}
	body := append([]byte(nil), target.Body()...)
	if err := setExternalBody(ctx, ext, cmd.String("tea-login"), body); err != nil {
		return err
	}
	fmt.Fprintf(cmd.Root().Writer, "pushed body for %s to %s (%d bytes)\n", target.ID, ext.URL, len(body))
	return nil
}

// setExternalBody pushes the local body bytes to the linked issue,
// dispatching on tracker: teax for Gitea (with the tea login), glabx for
// GitLab (which ignores the tea login). It changes no title, labels, or
// state; subgroup encoding and quick-action refusal stay in glabx.
// Unsupported trackers are refused before any mutation.
func setExternalBody(ctx context.Context, ext item.External, teaLogin string, body []byte) error {
	switch ext.Tracker {
	case "gitea":
		client, err := teax.Open(ctx, ext, teaLogin)
		if err != nil {
			return err
		}
		return client.SetBody(ctx, ext.ID, body)
	case "gitlab":
		client, err := glabx.Open(ctx, ext)
		if err != nil {
			return err
		}
		return client.SetBody(ctx, ext.ID, body)
	default:
		return fmt.Errorf("unsupported tracker %q", ext.Tracker)
	}
}

// duplicateExternalLinks returns the canonical IDs of every parseable item
// carrying the same tracker, installation base, repo, and issue number, sorted.
func duplicateExternalLinks(items []*item.Item, want item.External) []string {
	base, err := ops.ExternalBase(want)
	if err != nil {
		return []string{}
	}
	var out []string
	for _, it := range items {
		if sameImportIdentity(base, want, it.External) {
			out = append(out, it.ID)
		}
	}
	sort.Strings(out)
	return out
}

func joinIDs(ids []string) string {
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ", "
		}
		out += id
	}
	return out
}
