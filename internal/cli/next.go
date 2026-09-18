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
	Name:  "next",
	Usage: "Print the top unblocked item, optionally claiming it",
	// urfave splits slice-flag values on "," by default, which would turn
	// "-l p0,p1" into two ANDed groups. Disable it so SplitLabels sees
	// each -l occurrence intact (OR within a flag, AND across flags).
	DisableSliceFlagSeparator: true,
	Flags: []cli.Flag{
		&cli.StringSliceFlag{Name: "label", Aliases: []string{"l"}, Usage: "AND across flags, OR within a flag"},
		&cli.BoolFlag{Name: "claim", Usage: "set in_progress, assignee, claimed_at, and commit"},
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
	labelFlags := cmd.StringSlice("label")
	cands := graph.FilterLabels(g.Ready(), SplitLabels(labelFlags))
	if len(cands) == 0 {
		return cli.Exit(noReadyMessage(labelFlags), 1)
	}
	seed := cmd.Int64("seed")
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	n := pickNext(cands, seed)

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
