package ops

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

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

// ValidateText is the text report of validate without --stale-claims
// lines: the PASS/FAIL status line, each fault with its fix, brief and
// external WARN lines in graph order, then alias warnings.
func ValidateText(g *graph.Graph) string {
	var b strings.Builder
	status := "PASS"
	if len(g.Faults) > 0 {
		status = "FAIL"
	}
	nItems := len(g.Order) + len(g.Broken)
	nQuar := len(g.Quarantined()) + len(g.Broken)
	fmt.Fprintf(&b, "%s  %d items, %d quarantined\n", status, nItems, nQuar)
	for _, f := range g.Faults {
		fmt.Fprintf(&b, "[%s] %s\n  fix: %s\n", f.Reason, f.Detail, f.Fix)
	}
	for _, n := range g.Order {
		brief := strings.TrimSpace(n.Item.Brief)
		if brief == "" {
			fmt.Fprintf(&b, "WARN  %s: missing brief\n", n.Item.ID)
		} else if sentenceCount(brief) > 3 {
			fmt.Fprintf(&b, "WARN  %s: brief is longer than 3 sentences\n", n.Item.ID)
		}
		if n.Item.ExternalProblem != "" {
			fmt.Fprintf(&b, "WARN  %s: %s\n", n.Item.ID, n.Item.ExternalProblem)
		}
	}
	for _, line := range AliasWarnLines(g) {
		fmt.Fprintln(&b, line)
	}
	return b.String()
}

// AliasWarnLines reports invalid optional aliases and case-insensitive
// duplicates across active parseable items. Neither is a graph fault;
// duplicates make alias lookup refuse instead of choosing arbitrarily.
func AliasWarnLines(g *graph.Graph) []string {
	var out []string
	byFold := map[string][]string{}
	for _, n := range g.Order {
		a := n.Item.Alias
		if a == "" {
			continue
		}
		if err := item.ValidateAlias(a); err != nil {
			out = append(out, fmt.Sprintf("WARN  %s: %s", n.Item.ID, err))
		}
		byFold[strings.ToUpper(a)] = append(byFold[strings.ToUpper(a)], n.Item.ID)
	}
	for _, n := range g.Order {
		a := n.Item.Alias
		if a == "" {
			continue
		}
		if ids := byFold[strings.ToUpper(a)]; len(ids) > 1 {
			out = append(out, fmt.Sprintf("WARN  %s: duplicate alias %q shared with %s", n.Item.ID, a, strings.Join(ids, ", ")))
		}
	}
	return out
}
