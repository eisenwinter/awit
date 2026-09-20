package cli

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/eisenwinter/awit/internal/gitx"
	"github.com/eisenwinter/awit/pkg/config"
	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var nextCmd = &cli.Command{
	Name:      "next",
	Usage:     "Print the top unblocked item, or [id], optionally claiming it",
	ArgsUsage: "[id]",
	Description: `Ranked selection skips manually blocked items. An explicit [id] without
--claim is a lookup, so it can print a blocked item. To claim one, resolve
the recorded condition and run awit unblock <id> first.`,
	// urfave splits slice-flag values on "," by default, which would turn
	// "-l auth,db" into two ANDed groups. Disable it so SplitLabels sees
	// each -l occurrence intact (OR within a flag, AND across flags).
	DisableSliceFlagSeparator: true,
	Flags: []cli.Flag{
		&cli.StringSliceFlag{Name: "label", Aliases: []string{"l"}, Usage: "AND across flags, OR within a flag"},
		&cli.BoolFlag{Name: "claim", Usage: "claim [id] or the pick: sets in_progress, commits (needs --agent or AWIT_AGENT)"},
		&cli.StringFlag{Name: "commit", Usage: "with --claim, `true|false` overrides the commit policy (config.yaml commit:, default true)"},
		&cli.BoolFlag{Name: "no-commit", Usage: "deprecated: with --claim, skip the git commit; prefer --commit=false or commit: false in config.yaml"},
		&cli.Int64Flag{Name: "seed", Usage: "tie-break RNG seed; 0 (default) uses time.Now().UnixNano()"},
		&cli.BoolFlag{Name: "why", Usage: "explain the pick on stderr: unblocks, critical-path membership, selection and tie-break"},
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

// commitPolicy resolves whether a claim is committed. Precedence: an
// explicit --commit or a true --no-commit beats config commit, which beats
// the documented default true. --no-commit=false is neutral and overrides
// nothing.
//
// The command tree is reused across Main calls, so "was the flag passed?"
// is decided by value alone — an empty --commit value means unset — never
// by cmd.IsSet, whose hasBeenSet sticks to reused flags (guide §5).
func commitPolicy(cmd *cli.Command, cfg config.Config) (bool, error) {
	raw := cmd.String("commit")
	var want bool
	if raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return false, cli.Exit(fmt.Sprintf("invalid --commit value %q: use --commit=true or --commit=false", raw), 2)
		}
		want = v
	}
	noCommit := cmd.Bool("no-commit")
	switch {
	case raw != "" && noCommit:
		return false, cli.Exit("--commit and --no-commit cannot be combined", 2)
	case noCommit:
		return false, nil
	case raw != "":
		return want, nil
	default:
		return cfg.ShouldCommit(), nil
	}
}

func nextAction(_ context.Context, cmd *cli.Command) error {
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	if cmd.Args().Len() > 1 {
		return cli.Exit("next takes at most one item id", 2)
	}
	// Commit policy is validated before any mutation: a bad or conflicting
	// flag refuses the run before the claim lock or a file write, with or
	// without --claim.
	commit, err := commitPolicy(cmd, s.Config)
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
	warnQuarantined(cmd, g)
	var n *graph.Node
	selection := "max-unblocks"
	tieBreak := "none"
	if id := cmd.Args().First(); id != "" {
		n, err = nextNode(g, id)
		if err != nil {
			return err
		}
		// An exact lookup is never a ranking win, even when the item
		// happens to be ready: report it as an explicit selection.
		selection = "explicit"
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
		// K is the equal-maximum group after label filtering: the
		// leading run of cands sharing the top UnblockCount. Counted
		// here from the already-ranked slice, never via a second
		// pickNext call. K == 1 means no tie-break ran.
		k := 1
		for k < len(cands) && cands[k].UnblockCount == cands[0].UnblockCount {
			k++
		}
		if k > 1 {
			tieBreak = fmt.Sprintf("pcg(seed=%d,candidates=%d)", seed, k)
		}
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
		if commit {
			if err := gitx.Commit(s.Root, []string{it.Path}, "awit: claim "+it.ID); err != nil {
				return fmt.Errorf("claimed %s but git commit failed: %w", it.ID, err)
			}
		}
	}
	f, err := detectFormat(cmd)
	if err != nil {
		return err
	}
	if err := format.WriteOne(cmd.Root().Writer, f, toEntry(n)); err != nil {
		return err
	}
	// The explanation goes to stderr only after the selection or claim
	// and the entry write all succeeded. CriticalPath is whole-graph
	// context, not a ranking input: it is computed here, only for --why.
	if cmd.Bool("why") {
		onPath := false
		for _, cp := range g.CriticalPath() {
			if cp.Item.ID == n.Item.ID {
				onPath = true
				break
			}
		}
		cp := "no"
		if onPath {
			cp = "yes"
		}
		fmt.Fprintf(cmd.Root().ErrWriter, "why: %s; unblocks=%d; critical-path=%s; selection=%s; tie-break=%s\n",
			n.Item.ID, n.UnblockCount, cp, selection, tieBreak)
	}
	return nil
}

// nextNode resolves the positional form of next — canonical id, alias, or
// external key — through the same helper every command uses. Exact [key]
// remains a lookup, never a rerank. A broken file that could not become an
// item refuses like a quarantined node; anything else unknown keeps the
// existing "unknown item" string.
func nextNode(g *graph.Graph, key string) (*graph.Node, error) {
	id, err := resolveItemID(graphItems(g), key)
	if err == nil {
		return g.Nodes[id], nil
	}
	if errors.Is(err, errUnknownItem) {
		var reasons []string
		seen := map[string]bool{}
		for _, br := range g.Broken {
			if br.ID == key && !seen[string(br.Reason)] {
				seen[string(br.Reason)] = true
				reasons = append(reasons, "["+string(br.Reason)+"]")
			}
		}
		if len(reasons) > 0 {
			return nil, cli.Exit(fmt.Sprintf("%s is quarantined %s; run awit validate", key, strings.Join(reasons, ", ")), 1)
		}
	}
	return nil, err
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
	if n.Item.BlockedReason != "" {
		return cli.Exit(fmt.Sprintf("%s is manually blocked (%s); awit unblock %s once resolved", id, n.Item.BlockedReason, id), 1)
	}
	if n.Blocked {
		return cli.Exit(fmt.Sprintf("%s is blocked by %s", id, strings.Join(n.OpenDepIDs(), ", ")), 1)
	}
	if n.Item.Status == item.StatusInProgress && n.Item.Assignee != "" {
		return cli.Exit(fmt.Sprintf("%s is claimed by %s; awit release %s", id, n.Item.Assignee, id), 1)
	}
	return nil
}
