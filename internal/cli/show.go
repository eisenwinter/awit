package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/eisenwinter/awit/internal/ops"
	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/eisenwinter/awit/pkg/resolver"
	"github.com/urfave/cli/v3"
)

var showCmd = &cli.Command{
	Name:      "show",
	Usage:     "Show one item (default view)",
	ArgsUsage: "<id>",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "full", Usage: "include resolved ref bodies"},
		&cli.BoolFlag{Name: "refs-only", Usage: "list resolved refs without the item"},
		&cli.BoolFlag{Name: "unblocks", Usage: "list the open items this one transitively unblocks, instead of the item"},
	},
	Action: func(ctx context.Context, cmd *cli.Command) error {
		if cmd.Args().Len() != 1 {
			return cli.Exit("show needs an item id", 2)
		}
		return showOne(cmd, cmd.Args().Get(0))
	},
}

// showJSON is the --format json shape: the list entry plus the raw body.
// AWIT-0ND5703G adds a Refs field; keep the embedding so that still works.
type showJSON struct {
	format.Entry
	Body string        `json:"body"`
	Refs []showRefJSON `json:"refs,omitempty"`
}

func showOne(cmd *cli.Command, key string) error {
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	g, err := ops.LoadGraph(s)
	if err != nil {
		return err
	}
	warnQuarantined(cmd, g)
	f, err := detectFormat(cmd)
	if err != nil {
		return err
	}
	id, err := ops.ResolveItemID(graphItems(g), key)
	if err != nil {
		// Broken files have no resolvable alias or external metadata; the
		// exact canonical stem still shows the quarantine view.
		var matches []item.Broken
		for _, br := range g.Broken {
			if br.ID == key {
				matches = append(matches, br)
			}
		}
		if len(matches) > 0 {
			// Broken files always render the text view, even as json.
			fmt.Fprint(cmd.Root().Writer, ops.BrokenView(key, matches))
			return nil
		}
		return err
	}
	n := g.Nodes[id]
	if cmd.Bool("unblocks") {
		if cmd.Bool("full") || cmd.Bool("refs-only") {
			return cli.Exit("Error: --unblocks cannot be combined with --full or --refs-only", 2)
		}
		var entries []format.Entry
		for _, u := range graph.ReachableUnblocks(n) {
			entries = append(entries, ops.ToEntry(u))
		}
		return format.Write(cmd.Root().Writer, f, entries)
	}
	itemsDir := s.ItemsDir()
	baseDir := ops.RefsBaseDir(s, n)
	refsOnly := cmd.Bool("refs-only")
	full := cmd.Bool("full")
	if refsOnly && full {
		return cli.Exit("Error: pass either --full or --refs-only", 2)
	}
	if f == format.JSON {
		out, err := json.MarshalIndent(fullJSON(g, n, baseDir, itemsDir, full), "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.Root().Writer, "%s\n", out)
		return nil
	}
	if refsOnly {
		fmt.Fprint(cmd.Root().Writer, refsOnlyView(baseDir, n.Item.Refs))
		return nil
	}
	if full {
		fmt.Fprint(cmd.Root().Writer, ops.ShowFull(s, g, n))
		return nil
	}
	fmt.Fprint(cmd.Root().Writer, ops.DefaultView(n))
	return nil
}

// showRefJSON is one resolved ref for --format json --full.
type showRefJSON struct {
	Ref     string `json:"ref"`
	Path    string `json:"path"`
	Bytes   int    `json:"bytes"`
	Missing bool   `json:"missing"`
	Item    string `json:"item,omitempty"`
	Content string `json:"content,omitempty"`
}

// refsOnlyView prints one "<ref> -> <abs> (<N> bytes)" line per ref,
// or "<ref> -> [missing]" when the file cannot be read.
func refsOnlyView(baseDir string, refs []string) string {
	var b strings.Builder
	for _, r := range resolver.Resolve(baseDir, refs) {
		if r.Err != nil {
			fmt.Fprintf(&b, "%s -> [missing]\n", r.Ref)
			continue
		}
		fmt.Fprintf(&b, "%s -> %s (%d bytes)\n", r.Ref, r.Path, len(r.Content))
	}
	return b.String()
}

func fullJSON(g *graph.Graph, n *graph.Node, baseDir, itemsDir string, full bool) showJSON {
	out := showJSON{Entry: ops.ToEntry(n), Body: string(n.Item.Body())}
	if !full {
		return out
	}
	for _, r := range resolver.Resolve(baseDir, n.Item.Refs) {
		jr := showRefJSON{Ref: r.Ref, Path: r.Path}
		if r.Err != nil {
			jr.Missing = true
		} else {
			jr.Bytes = len(r.Content)
			if id, ok := resolver.IsItemRef(itemsDir, r.Path); ok {
				if target, ok := g.Nodes[id]; ok {
					jr.Item = id
					jr.Content = string(target.Item.Body())
				} else {
					jr.Content = string(r.Content)
				}
			} else {
				jr.Content = string(r.Content)
			}
		}
		out.Refs = append(out.Refs, jr)
	}
	return out
}
