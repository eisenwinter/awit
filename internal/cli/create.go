package cli

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/id"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var createCmd = &cli.Command{
	Name:      "create",
	Usage:     "Mint an ID and write a new item",
	ArgsUsage: "<title>",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "brief", Usage: "one to three sentences", Required: true},
		&cli.StringSliceFlag{Name: "dep", Aliases: []string{"d"}},
		&cli.StringSliceFlag{Name: "label", Aliases: []string{"l"}},
		&cli.StringFlag{Name: "assign"},
		&cli.StringFlag{Name: "id", Usage: "override minted id (imports)"},
		&cli.StringFlag{Name: "external-tracker", Usage: "external tracker (`gitea`)"},
		&cli.StringFlag{Name: "external-repo", Usage: "external repository (`owner/repo`)"},
		&cli.StringFlag{Name: "external-id", Usage: "external issue number"},
		&cli.StringFlag{Name: "external-url", Usage: "external issue URL"},
	},
	Action: createAction,
}

func parseExternalMapping(cmd *cli.Command) (*item.External, error) {
	tracker := cmd.String("external-tracker")
	repo := cmd.String("external-repo")
	idStr := cmd.String("external-id")
	rawURL := cmd.String("external-url")
	n := 0
	for _, s := range []string{tracker, repo, idStr, rawURL} {
		if s != "" {
			n++
		}
	}
	if n == 0 {
		return nil, nil
	}
	if n != 4 {
		return nil, cli.Exit(`Incorrect usage: --external-tracker, --external-repo, --external-id, and --external-url must be set together`, 2)
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return nil, cli.Exit("invalid external: id must be a positive integer", 2)
	}
	ext := item.External{Tracker: tracker, Repo: repo, ID: id, URL: rawURL}
	if err := item.ValidateExternal(ext); err != nil {
		return nil, cli.Exit(err.Error(), 2)
	}
	return &ext, nil
}

func parseIDList(values []string) []string {
	var out []string
	for _, v := range values {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func mergeLabels(defaults, flags []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, src := range [][]string{defaults, flags} {
		for _, l := range src {
			l = strings.TrimSpace(l)
			if l == "" || seen[l] {
				continue
			}
			seen[l] = true
			out = append(out, l)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

func detectFormat(cmd *cli.Command) (format.Format, error) {
	var stdout *os.File
	if f, ok := cmd.Root().Writer.(*os.File); ok {
		stdout = f
	}
	return format.Detect(cmd.Root().String("format"), stdout)
}

func createAction(_ context.Context, cmd *cli.Command) error {
	// createCmd is package-level, so urfave's Required check only fires on
	// the first Main call per process (flag hasBeenSet persists across runs
	// while values reset); enforce it here too so every call exits 2.
	if cmd.String("brief") == "" {
		return cli.Exit(`Incorrect usage: Required flag "brief" not set (run "awit --help")`, 2)
	}
	title := strings.TrimSpace(strings.Join(cmd.Args().Slice(), " "))
	if title == "" {
		return fmt.Errorf("create needs a title")
	}
	ext, err := parseExternalMapping(cmd)
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
	itemID := cmd.String("id")
	if itemID == "" {
		itemID, err = s.Mint(time.Now())
		if err != nil {
			return err
		}
	} else {
		if !id.Valid(s.Config.Prefix, itemID) {
			return fmt.Errorf("invalid id %s", itemID)
		}
		if s.Exists(itemID) {
			return fmt.Errorf("item %s already exists", itemID)
		}
	}
	deps := parseIDList(cmd.StringSlice("dep"))
	for _, d := range deps {
		if !id.Valid(s.Config.Prefix, d) {
			return fmt.Errorf("invalid id %s", d)
		}
		if !s.Exists(d) {
			return fmt.Errorf("unknown dep %s", d)
		}
	}
	labels := mergeLabels(s.Config.DefaultLabels, parseIDList(cmd.StringSlice("label")))
	it := item.New(itemID, title, cmd.String("brief"), deps, labels)
	if a := cmd.String("assign"); a != "" {
		it.SetAssignee(a)
	}
	if ext != nil {
		if err := it.SetExternal(ext); err != nil {
			return cli.Exit(err.Error(), 2)
		}
	}
	if err := s.Save(it); err != nil {
		return err
	}
	f, err := detectFormat(cmd)
	if err != nil {
		return err
	}
	// State is reported ready without building the graph — deliberate v1;
	// true state is `awit list`.
	return format.WriteOne(cmd.Root().Writer, f, format.Entry{
		ID:       it.ID,
		Title:    it.Title,
		Brief:    it.Brief,
		Status:   string(it.Status),
		State:    "ready",
		Labels:   it.Labels,
		Deps:     it.Deps,
		Assignee: it.Assignee,
		Unblocks: 0,
		External: it.External,
	})
}
