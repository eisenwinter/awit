package cli

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/eisenwinter/awit/internal/gitx"
	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var nextCmd = &cli.Command{
	Name:      "next",
	Usage:     "Print the top unblocked item, or [id], optionally claiming it",
	ArgsUsage: "[id]",
	// urfave splits slice-flag values on "," by default, which would turn
	// "-l auth,db" into two ANDed groups. Disable it so SplitLabels sees
	// each -l occurrence intact (OR within a flag, AND across flags).
	DisableSliceFlagSeparator: true,
	Flags: []cli.Flag{
		&cli.StringSliceFlag{Name: "label", Aliases: []string{"l"}, Usage: "AND across flags, OR within a flag"},
		&cli.BoolFlag{Name: "claim", Usage: "claim [id] or the pick: sets in_progress, commits (needs --agent or AWIT_AGENT)"},
		&cli.BoolFlag{Name: "no-commit", Usage: "with --claim, skip the git commit"},
		&cli.Int64Flag{Name: "seed", Usage: "tie-break RNG seed; 0 (default) uses time.Now().UnixNano()"},
		&cli.StringFlag{
			Name:    "agent",
			Usage:   "agent identity for --claim",
			Sources: cli.EnvVars("AWIT_AGENT"),
		},
	},
	Action: nextAction,
}

func noReadyMessage(labelFlags []string) string {
	msg := "No ready items"
	groups := SplitLabels(labelFlags)
	if len(groups) == 0 {
		return msg
	}
	var parts []string
	for _, g := range groups {
		parts = append(parts, strings.Join(g, ","))
	}
	return msg + " (labels: " + strings.Join(parts, ", ") + ")"
}

// pickNext returns the winner. cands must already be Ready()-ordered
// (UnblockCount desc, ID asc). The leading equal-UnblockCount group is
// shuffled with PCG(seed, seed); the rest of the slice is ignored.
func pickNext(cands []*graph.Node, seed int64) *graph.Node {
	if len(cands) == 0 {
		return nil
	}
	top := cands[0].UnblockCount
	end := 1
	for end < len(cands) && cands[end].UnblockCount == top {
		end++
	}
	group := append([]*graph.Node(nil), cands[:end]...)
	r := rand.New(rand.NewPCG(uint64(seed), uint64(seed)))
	r.Shuffle(len(group), func(i, j int) {
		group[i], group[j] = group[j], group[i]
	})
	return group[0]
}

func nextAction(_ context.Context, cmd *cli.Command) error {
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	if cmd.Args().Len() > 1 {
		return cli.Exit("next takes at most one item id", 2)
	}
	if cmd.Bool("claim") {
		noteWalkedUp(cmd, s)
		release, err := s.Lock(5 * time.Second)
		if err != nil {
			return err
		}
		defer release()
	}
	g, err := loadGraph(s)
	if err != nil {
		return err
	}
	var n *graph.Node
	if id := cmd.Args().First(); id != "" {
		n, err = nextNode(g, id)
		if err != nil {
			return err
		}
		// Without --claim the exact item prints as-is, whatever its
		// state; with --claim it must be ready and unclaimed.
		if cmd.Bool("claim") {
			if err := refuseClaim(n); err != nil {
				return err
			}
		}
	} else {
		labelFlags := cmd.StringSlice("label")
		cands := graph.FilterLabels(g.Ready(), SplitLabels(labelFlags))
		if len(cands) == 0 {
			return cli.Exit(noReadyMessage(labelFlags), 1)
		}
		seed := cmd.Int64("seed")
		if seed == 0 {
			seed = time.Now().UnixNano()
		}
		n = pickNext(cands, seed)
	}

	if cmd.Bool("claim") {
		agent := s.Config.Agent(cmd.String("agent"))
		if agent == "" {
			return fmt.Errorf("no agent identity; pass --agent or set AWIT_AGENT")
		}
		it, err := s.Load(n.Item.ID)
		if err != nil {
			return err
		}
		now := time.Now().UTC().Truncate(time.Second)
		it.SetStatus(item.StatusInProgress)
		it.SetAssignee(withAgentPrefix(agent))
		it.SetClaimedAt(&now)
		if err := s.Save(it); err != nil {
			return err
		}
		n.Item = it
		if !cmd.Bool("no-commit") {
			if err := gitx.Commit(s.Root, []string{it.Path}, "awit: claim "+it.ID); err != nil {
				return fmt.Errorf("claimed %s but git commit failed: %w", it.ID, err)
			}
		}
	}

	f, err := detectFormat(cmd)
	if err != nil {
		return err
	}
	return format.WriteOne(cmd.Root().Writer, f, toEntry(n))
}

// nextNode resolves the positional ID form of next. A broken file that
// could not become an item refuses like a quarantined node; anything else
// unknown keeps the existing "unknown item" string.
func nextNode(g *graph.Graph, id string) (*graph.Node, error) {
	if n, ok := g.Nodes[id]; ok {
		return n, nil
	}
	var reasons []string
	seen := map[string]bool{}
	for _, br := range g.Broken {
		if br.ID == id && !seen[string(br.Reason)] {
			seen[string(br.Reason)] = true
			reasons = append(reasons, "["+string(br.Reason)+"]")
		}
	}
	if len(reasons) > 0 {
		return nil, cli.Exit(fmt.Sprintf("%s is quarantined %s; run awit validate", id, strings.Join(reasons, ", ")), 1)
	}
	return nil, fmt.Errorf("unknown item %s", id)
}

// refuseClaim errors when the exact item cannot be claimed: quarantined,
// closed, blocked, or already claimed by someone. Messages carry no
// "Error: " prefix; Main prints the cli.Exit body as-is with exit 1.
func refuseClaim(n *graph.Node) error {
	id := n.Item.ID
	if n.Quarantined() {
		var reasons []string
		seen := map[string]bool{}
		for _, f := range n.Faults {
			if !seen[string(f.Reason)] {
				seen[string(f.Reason)] = true
				reasons = append(reasons, "["+string(f.Reason)+"]")
			}
		}
		return cli.Exit(fmt.Sprintf("%s is quarantined %s; run awit validate", id, strings.Join(reasons, ", ")), 1)
	}
	if n.Item.Status == item.StatusClosed {
		return cli.Exit(fmt.Sprintf("%s is closed; awit release %s to reopen it", id, id), 1)
	}
	if n.Blocked {
		return cli.Exit(fmt.Sprintf("%s is blocked by %s", id, strings.Join(n.OpenDepIDs(), ", ")), 1)
	}
	if n.Item.Status == item.StatusInProgress && n.Item.Assignee != "" {
		return cli.Exit(fmt.Sprintf("%s is claimed by %s; awit release %s", id, n.Item.Assignee, id), 1)
	}
	return nil
}
