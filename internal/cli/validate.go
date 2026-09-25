package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/eisenwinter/awit/internal/ops"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var validateCmd = &cli.Command{
	Name:  "validate",
	Usage: "Report graph faults and brief warnings",
	Flags: []cli.Flag{
		&cli.BoolFlag{Name: "stale-claims", Usage: "warn on in_progress claims older than stale_claim"},
	},
	Action: validateAction,
}

// now is the clock for stale-claim ages; tests overwrite it.
var now = time.Now()

type validateFaultJSON struct {
	Reason string   `json:"reason"`
	IDs    []string `json:"ids"`
	Detail string   `json:"detail"`
	Fix    string   `json:"fix"`
}

func validateAction(_ context.Context, cmd *cli.Command) error {
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	g, err := ops.LoadGraph(s)
	if err != nil {
		return err
	}
	warnQuarantined(cmd, g)
	w := cmd.Root().Writer

	if cmd.Root().String("format") == "json" {
		rows := make([]validateFaultJSON, 0, len(g.Faults))
		for _, f := range g.Faults {
			ids := f.IDs
			if ids == nil {
				ids = []string{}
			}
			rows = append(rows, validateFaultJSON{
				Reason: string(f.Reason),
				IDs:    ids,
				Detail: f.Detail,
				Fix:    f.Fix,
			})
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			return err
		}
		for _, line := range externalWarnLines(g) {
			fmt.Fprintln(cmd.Root().ErrWriter, line)
		}
		for _, line := range ops.AliasWarnLines(g) {
			fmt.Fprintln(cmd.Root().ErrWriter, line)
		}
		if len(g.Faults) > 0 {
			return cli.Exit("", 1)
		}
		return nil
	}

	fmt.Fprint(w, ops.ValidateText(g))
	if cmd.Bool("stale-claims") {
		for _, line := range staleClaimLines(g, time.Duration(s.Config.StaleClaim), now) {
			fmt.Fprintln(w, line)
		}
	}
	if len(g.Faults) > 0 {
		return cli.Exit("", 1)
	}
	return nil
}

func externalWarnLines(g *graph.Graph) []string {
	var out []string
	for _, n := range g.Order {
		if n.Item.ExternalProblem != "" {
			out = append(out, fmt.Sprintf("WARN  %s: %s", n.Item.ID, n.Item.ExternalProblem))
		}
	}
	return out
}

// staleClaimLines reports in_progress nodes whose claim is older
// than limit, or which carry no claimed_at at all. Quarantined
// nodes are skipped (validate already FAILs them). Output order is
// graph order (ID ascending). Each warning is two lines: the WARN
// line and its fix hint.
func staleClaimLines(g *graph.Graph, limit time.Duration, ref time.Time) []string {
	var out []string
	for _, n := range g.Order {
		if n.Quarantined() {
			continue
		}
		if n.Item.Status != item.StatusInProgress {
			continue
		}
		if n.Item.ClaimedAt == nil {
			out = append(out,
				fmt.Sprintf("WARN  [STALE CLAIM] %s in progress with no claimed_at (limit %s)", n.Item.ID, shortDuration(limit)),
				fmt.Sprintf("  fix: awit release %s", n.Item.ID))
			continue
		}
		if age := ref.Sub(*n.Item.ClaimedAt); age > limit {
			out = append(out,
				fmt.Sprintf("WARN  [STALE CLAIM] %s claimed by %s %s ago (limit %s)", n.Item.ID, n.Item.Assignee, humanDuration(age), shortDuration(limit)),
				fmt.Sprintf("  fix: awit release %s", n.Item.ID))
		}
	}
	return out
}

// humanDuration renders 45m, 3h12m, and 2d3h once beyond 48h.
func humanDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	m := int(d.Minutes())
	h := m / 60
	if h < 1 {
		return fmt.Sprintf("%dm", m)
	}
	if h < 48 {
		return fmt.Sprintf("%dh%dm", h, m%60)
	}
	return fmt.Sprintf("%dd%dh", h/24, h%24)
}

// shortDuration renders whole-unit durations the way config.yaml
// does ("2h", "30m", "90s"). time.Duration.String would emit
// "2h0m0s", which the report must not print.
func shortDuration(d time.Duration) string {
	switch {
	case d == 0:
		return "0s"
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	case d%time.Second == 0:
		return fmt.Sprintf("%ds", d/time.Second)
	default:
		return d.String()
	}
}
