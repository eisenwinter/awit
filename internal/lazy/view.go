package lazy

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var tabActive = lipgloss.NewStyle().Bold(true)

// View implements tea.Model: two header lines, the pane area (or help),
// quarantine footer, queue why line, toast, and the key-hint line.
func (m Model) View() string {
	if m.fatal != "" {
		return m.fatalView()
	}
	m.layout()
	var b strings.Builder
	b.WriteString(m.headerView())
	b.WriteString("\n" + m.tabHeaderView())
	if m.mode == modeHelp {
		b.WriteString("\n" + m.helpView())
	} else {
		b.WriteString("\n" + m.panesView())
	}
	if m.quarantined > 0 {
		fmt.Fprintf(&b, "\nwarning: %d items quarantined, run awit validate", m.quarantined)
	}
	if m.tab == tabQueue {
		b.WriteString("\n" + m.whyView())
	}
	if m.toast != "" {
		b.WriteString("\n" + m.toast)
	}
	if m.mode == modeInput || m.mode == modeSearch {
		b.WriteString("\n" + m.promptView())
	} else {
		b.WriteString("\n" + m.hintsView())
	}
	return b.String()
}

func (m Model) headerView() string {
	tabs := []struct {
		name string
		t    tab
	}{
		{"Issues", tabIssues},
		{"Graph", tabGraph},
		{"Queue", tabQueue},
		{"Config", tabConfig},
	}
	var b strings.Builder
	b.WriteString("awit lazy-human ")
	for i := range tabs {
		label := fmt.Sprintf("[%d] %s", i+1, tabs[i].name)
		if m.tab == tabs[i].t {
			label = tabActive.Render(label)
		}
		b.WriteString(" " + label)
	}
	focusName := "list"
	if m.focus == focusDetail {
		focusName = "detail"
	}
	tail := "  focus: " + focusName
	out := b.String() + tail
	if pad := m.width - len([]rune(out)); pad > 0 {
		out += strings.Repeat(" ", pad)
	}
	return out
}

// tabHeaderView is the per-tab line under the tab bar: issues badges, the
// graph overview/focused label, the queue counts.
func (m Model) tabHeaderView() string {
	switch m.tab {
	case tabGraph:
		if m.graphTab.focused {
			return "Focused on " + m.graphTab.rootID
		}
		return "Overview"
	case tabConfig:
		return ".awit/config.yaml"
	case tabQueue:
		n := 0
		if m.g != nil {
			n = len(m.g.Ready())
		}
		return fmt.Sprintf("ready: %d", n)
	default:
		return m.issuesHeader()
	}
}

func (m Model) bodyHeight() int {
	body := m.height - 2 - m.footerLines()
	if body < 1 {
		return 1
	}
	return body
}

func padLines(lines []string, n int) []string {
	for len(lines) < n {
		lines = append(lines, "")
	}
	return lines[:n]
}

// panesView draws the list beside the detail viewport. Narrow terminals show
// only the focused pane at full width.
func (m Model) panesView() string {
	body := m.bodyHeight()
	if m.tab == tabGraph {
		// No detail pane: the overview or tree fills the body full width.
		return m.activeList().view(m.width)
	}
	if m.width < 80 {
		if m.focus == focusDetail {
			return strings.Join(padLines(strings.Split(m.detail.View(), "\n"), body), "\n")
		}
		return m.activeList().view(m.width)
	}
	leftW, detailW := m.columns()
	left := padLines(strings.Split(m.activeList().view(leftW), "\n"), body)
	right := padLines(strings.Split(m.detail.View(), "\n"), body)
	lines := make([]string, 0, body)
	for i := range body {
		cell := truncateRunes(left[i], leftW)
		cell += leftWLine(leftW, cell)
		lines = append(lines, cell+" │ "+truncateRunes(right[i], detailW))
	}
	return strings.Join(lines, "\n")
}

// leftWLine pads s with trailing spaces to width w (in runes).
func leftWLine(w int, s string) string {
	if pad := w - len([]rune(s)); pad > 0 {
		return strings.Repeat(" ", pad)
	}
	return ""
}

func (m Model) fatalView() string {
	return m.fatal + "\nrun awit init\npress q to quit"
}

func (m Model) helpView() string {
	return strings.Join([]string{
		"HELP",
		"",
		"1/2/3/4      switch tab (issues/graph/queue/config)",
		"j/k, up/down move selection (list) / scroll (detail)",
		"h/left       focus list",
		"l/right      focus detail",
		"tab          toggle focus list/detail",
		"enter        focus detail",
		"e/enter      edit value (config)",
		"esc          back to list",
		"/            search (issues)",
		"o            toggle open/archive (issues)",
		"c            close item",
		"b            block item",
		"u            unblock item",
		"m            comment",
		"space        claim (queue)",
		"r            release (queue)",
		"R            reload graph",
		"P            external check on selection",
		"V            validate report into detail",
		"?            this help",
		"q            quit",
		"",
		"press ?/esc/q to close",
	}, "\n")
}

func (m Model) hintsView() string {
	switch m.tab {
	case tabGraph:
		return "j/k move  tab overview/focused  enter open in issues  c close  b block  u unblock  m comment  ? help"
	case tabQueue:
		return "j/k move  space claim  r release  c close  b block  u unblock  m comment  P check  ? help"
	case tabConfig:
		return "j/k move  e/enter edit  esc cancel  R reload  ? help"
	default:
		return "j/k move  enter pin  / filter  o open/archive  c close  b block  u unblock  m comment  P check  ? help"
	}
}

func (m Model) promptView() string {
	if m.mode == modeInput && m.inputKind == inputConfig {
		return m.inputTarget + ": " + m.input.View()
	}
	label := "filter:"
	if m.mode == modeInput {
		switch m.inputKind {
		case inputBlock:
			label = "block reason:"
		case inputComment:
			label = "comment:"
		default:
			label = "close reason (optional):"
		}
		if m.inputTarget != "" {
			label += " [" + m.inputTarget + "]"
		}
	}
	return label + " " + m.input.View()
}

// whyView explains the queue selection in next --why terms via whyLine.
func (m Model) whyView() string {
	id := m.selectedID()
	if id == "" || m.g == nil {
		return "why: no selection"
	}
	n := m.g.Nodes[id]
	if n == nil {
		return "why: no selection"
	}
	return whyLine(m.g, n, m.onCritical[id])
}
