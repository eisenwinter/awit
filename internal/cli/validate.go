package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var validateCmd = &cli.Command{
	Name:   "validate",
	Usage:  "Report graph faults and brief warnings",
	Action: validateAction,
}

type validateFaultJSON struct {
	Reason string   `json:"reason"`
	IDs    []string `json:"ids"`
	Detail string   `json:"detail"`
	Fix    string   `json:"fix"`
}

func loadGraph(s *item.Store) (*graph.Graph, error) {
	items, broken, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	return graph.Build(items, broken), nil
}

// sentenceCount counts sentences in s. A sentence ends at '.', '!' or '?'
// that is at end-of-string or followed by whitespace. A non-empty brief
// with no terminator is one sentence. Empty / whitespace-only is zero.
func sentenceCount(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	runes := []rune(s)
	n := 0
	for i, r := range runes {
		if r != '.' && r != '!' && r != '?' {
			continue
		}
		if i+1 == len(runes) || unicode.IsSpace(runes[i+1]) {
			n++
		}
	}
	if n == 0 {
		return 1
	}
	return n
}

func validateAction(_ context.Context, cmd *cli.Command) error {
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	g, err := loadGraph(s)
	if err != nil {
		return err
	}
	nItems := len(g.Order) + len(g.Broken)
	nQuar := len(g.Quarantined()) + len(g.Broken)
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
		if len(g.Faults) > 0 {
			return cli.Exit("", 1)
		}
		return nil
	}

	status := "PASS"
	if len(g.Faults) > 0 {
		status = "FAIL"
	}
	fmt.Fprintf(w, "%s  %d items, %d quarantined\n", status, nItems, nQuar)
	for _, f := range g.Faults {
		fmt.Fprintf(w, "[%s] %s\n  fix: %s\n", f.Reason, f.Detail, f.Fix)
	}
	for _, n := range g.Order {
		brief := strings.TrimSpace(n.Item.Brief)
		if brief == "" {
			fmt.Fprintf(w, "WARN  %s: missing brief\n", n.Item.ID)
			continue
		}
		if sentenceCount(brief) > 3 {
			fmt.Fprintf(w, "WARN  %s: brief is longer than 3 sentences\n", n.Item.ID)
		}
	}
	if len(g.Faults) > 0 {
		return cli.Exit("", 1)
	}
	return nil
}
