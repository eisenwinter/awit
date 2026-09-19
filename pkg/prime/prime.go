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
// (or is empty when there is nothing to show) and never contains "\r".
//
// MaxTokens is a soft budget in EstimateTokens units (0 or negative
// means unlimited). When the full snapshot does not fit, Render sheds
// material richest-first — blocked rows from the end, then ready rows
// from the end (never the first), then the critical-path section, then
// scaffolding — and emits the richest form that fits. Warning detail
// lines and the top ready row are never shed: when even they exceed the
// budget, Render emits them anyway, so over-budget output is always
// exactly that documented floor, never forgotten accounting.
func Render(w io.Writer, g *graph.Graph, opts Options) error {
	ready := g.Ready()
	blocked := g.Blocked()
	if len(opts.Labels) > 0 {
		ready = graph.FilterLabels(ready, opts.Labels)
		blocked = graph.FilterLabels(blocked, opts.Labels)
	}

	var warnHead string
	var warnLines []string
	if len(g.Faults) > 0 {
		warnHead = "=== GRAPH WARNINGS ===\n"
		for _, f := range g.Faults {
			line := "[" + string(f.Reason) + "] " + f.Detail
			if f.Reason == item.ReasonCycle || f.Reason == item.ReasonDangling {
				line += " (excluded from next)"
			}
			warnLines = append(warnLines, line)
		}
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

	_, err := io.WriteString(w, assemble(warnHead, warnLines, readyLines, blockedLines, tail.String(), len(ready), len(blocked), opts.MaxTokens))
	return err
}

// assemble joins the sections and applies the token budget. Every
// candidate length is computed arithmetically from precomputed piece
// lengths (rows as prefix sums, separators, headers, the omission
// notice); the retained form is chosen before anything is written, so
// no candidate strings are built and discarded while budgeting.
//
// Shedding order, richest candidate first:
//
//  1. Blocked rows from the end, then ready rows from the end, never
//     the first ready row. Retained rows are always prefixes: a long
//     higher-ranked row is never skipped to admit a shorter
//     lower-ranked one. The critical-path section stays.
//  2. The critical-path section as a whole: contextual payload, less
//     important than the work the agent can claim.
//  3. Empty sections (header and all), then the omission notice.
//  4. READY/BLOCKED headers, keeping warnings plus kept ready rows.
//  5. The warnings heading, keeping warning detail lines plus kept
//     ready rows in full.
//
// The first candidate that fits is written once. When nothing fits,
// the stage-5 floor (all warning details plus the top ready row, if
// any) is emitted anyway: --max-tokens is a soft budget with this
// minimum, not a promise that one full row fits into one token.
// Header counts are post-filter, pre-truncation; the omission notice
// counts shed ready+blocked rows, never removed headers or
// critical-path nodes.
func assemble(warnHead string, warnLines, readyLines, blockedLines []string, tail string, nReady, nBlocked, max int) string {
	readyHead := fmt.Sprintf("=== READY (%d) ===\n", nReady)
	blockedHead := fmt.Sprintf("=== BLOCKED (%d) ===\n", nBlocked)
	rows := func(lines []string, n int) string {
		if n == 0 {
			return ""
		}
		return strings.Join(lines[:n], "\n") + "\n"
	}
	suffix := func(dropped int) string {
		if dropped <= 0 {
			return ""
		}
		return fmt.Sprintf("(+%d more)\n", dropped)
	}

	// Precomputed byte costs, rows as prefix sums including "\n".
	readySum := make([]int, len(readyLines)+1)
	for i, l := range readyLines {
		readySum[i+1] = readySum[i] + len(l) + 1
	}
	blockedSum := make([]int, len(blockedLines)+1)
	for i, l := range blockedLines {
		blockedSum[i+1] = blockedSum[i] + len(l) + 1
	}
	warnLen := len(warnHead)
	blankAfterWarn := 0
	if len(warnLines) > 0 {
		for _, l := range warnLines {
			warnLen += len(l) + 1
		}
		blankAfterWarn = 1 // blank line between warnings and READY
	}
	headLen := warnLen + blankAfterWarn
	minReady := 0
	if len(readyLines) > 0 {
		minReady = 1
	}
	// Full scaffolding with kR/kB kept rows and optional crit: the
	// exact bytes renderFull writes, so the length never drifts from
	// the output.
	fullLen := func(kR, kB int, crit bool) int {
		n := headLen + len(readyHead) + readySum[kR] + 1 + len(blockedHead) + blockedSum[kB]
		if crit && tail != "" {
			n += 1 + len(tail)
		}
		return n + len(suffix(len(readyLines)-kR+len(blockedLines)-kB))
	}
	renderFull := func(kR, kB int, crit bool) string {
		var out strings.Builder
		out.WriteString(warnHead)
		out.WriteString(warnBody(warnLines))
		if len(warnLines) > 0 {
			out.WriteString("\n")
		}
		out.WriteString(readyHead)
		out.WriteString(rows(readyLines, kR))
		out.WriteString("\n")
		out.WriteString(blockedHead)
		out.WriteString(rows(blockedLines, kB))
		if crit && tail != "" {
			out.WriteString("\n")
			out.WriteString(tail)
		}
		out.WriteString(suffix(len(readyLines) - kR + len(blockedLines) - kB))
		return out.String()
	}

	if max <= 0 {
		return renderFull(len(readyLines), len(blockedLines), true)
	}
	fits := func(n int) bool { return n/4 <= max }

	// Stage 1: shed blocked rows, then ready rows (never the first).
	for kB := len(blockedLines); kB >= 0; kB-- {
		if fits(fullLen(len(readyLines), kB, true)) {
			return renderFull(len(readyLines), kB, true)
		}
	}
	for kR := len(readyLines) - 1; kR >= minReady; kR-- {
		if fits(fullLen(kR, 0, true)) {
			return renderFull(kR, 0, true)
		}
	}
	// Stage 2: drop the critical-path section as a whole.
	if fits(fullLen(minReady, 0, false)) {
		return renderFull(minReady, 0, false)
	}
	dropped := len(readyLines) - minReady + len(blockedLines)
	readySec := ""
	if minReady > 0 {
		readySec = readyHead + rows(readyLines, minReady)
	}
	// warnBlock renders the warnings block, appending the blank
	// separator only when content follows it; without that guard a
	// warnings-only snapshot ends with "\n\n".
	warnBlock := func(tail string) string {
		s := warnHead + warnBody(warnLines)
		if s != "" && tail != "" {
			s += "\n"
		}
		return s
	}
	warnBlockLen := func(tailLen int) int {
		n := warnLen + tailLen
		if len(warnLines) > 0 && tailLen > 0 {
			n++
		}
		return n
	}
	// Stage 3: drop empty sections, keep the omission notice.
	tail3 := readySec + suffix(dropped)
	if fits(warnBlockLen(len(tail3))) {
		return warnBlock(tail3) + tail3
	}
	// Stage 4: drop the omission notice and READY/BLOCKED headers.
	tail4 := rows(readyLines, minReady)
	if fits(warnBlockLen(len(tail4))) {
		return warnBlock(tail4) + tail4
	}
	// Stage 5 (floor): warning details plus the top ready row, kept
	// whole and emitted even over budget.
	return warnBody(warnLines) + rows(readyLines, minReady)
}

// warnBody renders warning detail lines in full, each newline
// terminated, without the section heading.
func warnBody(warnLines []string) string {
	var b strings.Builder
	for _, l := range warnLines {
		b.WriteString(l + "\n")
	}
	return b.String()
}
