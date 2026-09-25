package lazy

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Tab-active styling lives in theme.go (stTabActive).

// View implements tea.Model: two header lines, the pane area (or help),
// quarantine footer, queue why line, toast slot, and the key-hint line. The
// toast slot renders every frame (blank when empty) so the body height never
// moves when a toast appears. Terminals below 50x12 get only the gate line.
func (m Model) View() string {
	if m.width < 50 || m.height < 12 {
		return fmt.Sprintf("lazyawit needs >= 50x12 (have %dx%d)", m.width, m.height)
	}
	if m.fatal != "" {
		return truncateLines(m.fatalView(), m.width)
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
		b.WriteString("\n" + stQuarantine.Render(fmt.Sprintf("warning: %d item(s) quarantined, run awit validate", m.quarantined)))
	}
	if m.tab == tabQueue {
		b.WriteString("\n" + stWhy.Render(m.whyView()))
	}
	b.WriteString("\n" + styleToast(m.toast))
	hints := false
	if m.mode == modeInput || m.mode == modeSearch {
		b.WriteString("\n" + m.promptView())
	} else {
		b.WriteString("\n" + m.hintsView())
		hints = true
	}
	return truncateViewLines(b.String(), m.width, hints)
}

// truncateViewLines fits every emitted line to width cells: content lines
// get an ellipsis, the hints line is hard-cut.
func truncateViewLines(s string, width int, hints bool) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if hints && i == len(lines)-1 {
			lines[i] = truncateHard(ln, width)
			continue
		}
		lines[i] = truncateRunes(ln, width)
	}
	return strings.Join(lines, "\n")
}

func truncateLines(s string, width int) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = truncateRunes(ln, width)
	}
	return strings.Join(lines, "\n")
}

// topBarGap is the single source for spacing between top-bar tab options.
const topBarGap = "  "

func (m Model) headerView() string {
	tabs := []struct {
		name string
		t    tab
	}{
		{"Work items", tabIssues},
		{"Graph", tabGraph},
		{"Queue", tabQueue},
		{"Config", tabConfig},
	}
	var b strings.Builder
	b.WriteString(stBrand.Render("lazyawit"))
	for i := range tabs {
		b.WriteString(topBarGap)
		label := fmt.Sprintf("[%d] %s", i+1, tabs[i].name)
		if m.tab == tabs[i].t {
			b.WriteString(stTabActive.Render(">" + label))
		} else {
			b.WriteString(stTabInactive.Render(label))
		}
	}
	focusName := "list"
	if m.focus == focusDetail {
		focusName = fmt.Sprintf("detail %d%%", int(m.detail.ScrollPercent()*100+0.5))
	}
	tail := "  focus: " + focusName
	out := b.String() + stMuted.Render(tail)
	if pad := m.width - lipgloss.Width(out); pad > 0 {
		out += strings.Repeat(" ", pad)
	}
	return out
}

// tabHeaderView is the per-tab line under the tab bar: issues badges, the
// graph overview/focused label, the queue counts. Metadata, dimmed.
func (m Model) tabHeaderView() string {
	var s string
	switch m.tab {
	case tabGraph:
		if m.graphTab.focused {
			if m.graphTab.rootID == "" {
				s = "Focused (no root — select an item on Work items)"
				break
			}
			s = "Focused on " + m.graphTab.rootID
			break
		}
		s = "Overview"
	case tabConfig:
		s = ".awit/config.yaml"
	case tabQueue:
		n := 0
		if m.g != nil {
			n = len(m.g.Ready())
		}
		s = fmt.Sprintf("ready: %d", n)
	default:
		s = m.issuesHeader()
	}
	return stMuted.Render(s)
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

// panesView draws the list beside the detail viewport inside rounded-light
// borders, joined with no gap. The list gets the focused flag so its cursor
// goes plain when the detail has focus. Narrow terminals show only the
// focused pane in a single full-width border; the Graph tab (no detail
// pane) always renders one full-width border.
func (m Model) panesView() string {
	body := m.bodyHeight()
	innerH := body - 2
	if innerH < 1 {
		innerH = 1
	}
	listFocused := m.focus == focusList
	if m.tab == tabGraph {
		// No detail pane: the overview or tree fills one bordered body.
		return borderStyle(listFocused).Width(m.width - 2).Height(innerH).Render(
			m.activeList().view(m.width-2, listFocused))
	}
	if m.width < 80 {
		if m.focus == focusDetail {
			return borderStyle(true).Width(m.width - 2).Height(innerH).Render(m.detail.View())
		}
		return borderStyle(listFocused).Width(m.width - 2).Height(innerH).Render(
			m.activeList().view(m.width-2, listFocused))
	}
	leftW, detailW := m.columns()
	left := borderStyle(listFocused).Width(leftW - 2).Height(innerH).Render(
		m.activeList().view(leftW-2, listFocused))
	right := borderStyle(m.focus == focusDetail).Width(detailW - 2).Height(innerH).Render(m.detail.View())
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) fatalView() string {
	return m.fatal + "\nrun awit init\npress q to quit"
}

func (m Model) helpView() string {
	lines := []string{
		"HELP",
		"",
		"1/2/3/4      switch tab (work items/graph/queue/config)",
		"j/k, up/down move selection (list) / scroll (detail)",
		"h/left       focus list",
		"l/right      focus detail",
		"tab          toggle focus list/detail",
		"enter        focus detail",
		"e/enter      edit value (config)",
		"esc          back to list",
		"/            search (work items)",
		"o            toggle open/archive (work items)",
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
	}
	if n := m.bodyHeight(); len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}

func (m Model) hintsView() string {
	var s string
	switch m.tab {
	case tabGraph:
		s = "j/k move  tab mode  enter work items  c/b/u/m mutate  ? help"
	case tabQueue:
		s = "j/k move  space claim  r release  c/b/u/m mutate  P check  ? help"
	case tabConfig:
		s = "j/k move  e/enter edit  esc cancel  R reload  ? help"
	default:
		s = "j/k move  enter detail  / filter  o archive  c/b/u/m mutate  P check  ? help"
	}
	return styleHints(s)
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
