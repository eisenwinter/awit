// Package prime renders the deterministic agent snapshot: warnings,
// ready, blocked, and the critical path. Output is stable for identical
// state (no timestamps, no map order) so prompts cache well.
package prime

import (
	"fmt"
	"io"
	"strings"

	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

// Options tunes Render. MaxTokens 0 means unlimited.
type Options struct {
	MaxTokens int
	Labels    [][]string
}

// EstimateTokens approximates LLM tokens as len(b)/4, integer division.
// It is approximate by design (guide §2 decision 5); callers use it for
// budgeting, never for billing.
func EstimateTokens(b []byte) int {
	return len(b) / 4
}

// entryOf mirrors internal/cli.toEntry without importing it (importing
// internal/cli from here would be a cycle). Keep the two in sync.
func entryOf(n *graph.Node) format.Entry {
	state := "blocked"
	switch {
	case n.Quarantined():
		state = "quarantined"
	case n.Item.Status == item.StatusClosed:
		state = "closed"
	case n.Ready:
		state = "ready"
	}
	var faults []string
	for _, f := range n.Faults {
		faults = append(faults, "["+string(f.Reason)+"] "+f.Detail)
	}
	return format.Entry{
		ID:       n.Item.ID,
		Title:    n.Item.Title,
		Brief:    n.Item.Brief,
		Status:   string(n.Item.Status),
		State:    state,
		Labels:   append([]string(nil), n.Item.Labels...),
		Deps:     n.DepIDs(),
		Assignee: n.Item.Assignee,
		Unblocks: n.UnblockCount,
		Faults:   faults,
	}
}

func blockedLine(n *graph.Node) string {
	deps := n.OpenDepIDs()
	if len(deps) == 0 {
		return fmt.Sprintf("[%s] %s", n.Item.ID, n.Item.Title)
	}
	return fmt.Sprintf("[%s] %s <- %s", n.Item.ID, n.Item.Title, strings.Join(deps, ", "))
}

// Render writes the snapshot: GRAPH WARNINGS (omitted when empty),
// READY, BLOCKED, CRITICAL PATH (omitted when empty). Sections are
// separated by one blank line; the output ends with exactly one "\n"
// and never contains "\r".
func Render(w io.Writer, g *graph.Graph, opts Options) error {
	ready := g.Ready()
	blocked := g.Blocked()
	if len(opts.Labels) > 0 {
		ready = graph.FilterLabels(ready, opts.Labels)
		blocked = graph.FilterLabels(blocked, opts.Labels)
	}

	var head strings.Builder
	if len(g.Faults) > 0 {
		head.WriteString("=== GRAPH WARNINGS ===\n")
		for _, f := range g.Faults {
			line := "[" + string(f.Reason) + "] " + f.Detail
			if f.Reason == item.ReasonCycle || f.Reason == item.ReasonDangling {
				line += " (excluded from next)"
			}
			head.WriteString(line + "\n")
		}
		head.WriteString("\n")
	}

	crit := g.CriticalPath()
	var tail strings.Builder
	if len(crit) > 0 {
		ids := make([]string, 0, len(crit))
		for _, n := range crit {
			ids = append(ids, n.Item.ID)
		}
		fmt.Fprintf(&tail, "=== CRITICAL PATH (%d) ===\n%s\n", len(crit), strings.Join(ids, " -> "))
	}

	readyLines := make([]string, 0, len(ready))
	for _, n := range ready {
		readyLines = append(readyLines, format.Line(entryOf(n)))
	}
	blockedLines := make([]string, 0, len(blocked))
	for _, n := range blocked {
		blockedLines = append(blockedLines, blockedLine(n))
	}

	_, err := io.WriteString(w, assemble(head.String(), readyLines, blockedLines, tail.String(), len(ready), len(blocked), opts.MaxTokens))
	return err
}

// assemble joins the sections and applies the token budget. head
// (warnings) and tail (critical path) always fit; ready lines are
// admitted first, then blocked lines. Each probe builds the exact final
// bytes that would result if nothing else were admitted afterwards, so
// admission is monotone and the finished output always fits (when the
// fixed head+tail alone allow it). Header counts are post-filter,
// pre-truncation; dropped lines become one trailing "(+N more)" line.
func assemble(head string, readyLines, blockedLines []string, tail string, nReady, nBlocked, max int) string {
	readyHead := fmt.Sprintf("=== READY (%d) ===\n", nReady)
	blockedHead := fmt.Sprintf("=== BLOCKED (%d) ===\n", nBlocked)
	sep := ""
	if tail != "" {
		sep = "\n"
	}
	join := func(lines []string) string {
		if len(lines) == 0 {
			return ""
		}
		return strings.Join(lines, "\n") + "\n"
	}
	var keptReady, keptBlocked []string
	dropped := 0
	for _, line := range readyLines {
		if max > 0 {
			probe := head + readyHead + join(append(append([]string{}, keptReady...), line)) + "\n" + blockedHead + sep + tail
			if EstimateTokens([]byte(probe)) > max {
				dropped++
				continue
			}
		}
		keptReady = append(keptReady, line)
	}
	for _, line := range blockedLines {
		if max > 0 {
			probe := head + readyHead + join(keptReady) + "\n" + blockedHead + join(append(append([]string{}, keptBlocked...), line)) + sep + tail
			if EstimateTokens([]byte(probe)) > max {
				dropped++
				continue
			}
		}
		keptBlocked = append(keptBlocked, line)
	}
	var out strings.Builder
	out.WriteString(head)
	out.WriteString(readyHead)
	out.WriteString(join(keptReady))
	out.WriteString("\n")
	out.WriteString(blockedHead)
	out.WriteString(join(keptBlocked))
	out.WriteString(sep)
	out.WriteString(tail)
	if dropped > 0 {
		fmt.Fprintf(&out, "(+%d more)\n", dropped)
	}
	return out.String()
}
