package lazy

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// RED for AWIT-0P2890SR: the first tab and all user-facing strings say
// "Work items", and the top bar separates tabs with a double-space gap.
func TestRedWorkItemsWordingAndSpacing(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())

	header := m.headerView()
	if !strings.Contains(header, "[1] Work items") {
		t.Fatalf("header missing work-items label: %q", header)
	}
	if strings.Contains(header, "Issues") {
		t.Fatalf("header still says Issues: %q", header)
	}
	if !strings.Contains(header, "Work items  [2]") {
		t.Fatalf("tabs lack double-space gap: %q", header)
	}

	help := m.helpView()
	if !strings.Contains(help, "switch tab (work items/graph/queue/config)") {
		t.Fatalf("help tab row not renamed: %q", help)
	}
	if !strings.Contains(help, "/            search (work items)") {
		t.Fatalf("help search row not renamed: %q", help)
	}
	if !strings.Contains(help, "o            toggle open/archive (work items)") {
		t.Fatalf("help archive row not renamed: %q", help)
	}

	m.tab = tabGraph
	if got := m.hintsView(); !strings.Contains(got, "enter work items") {
		t.Fatalf("graph hint not renamed: %q", got)
	}
	m.graphTab.focused = true
	m.graphTab.rootID = ""
	if got := m.tabHeaderView(); got != "Focused (no root — select an item on Work items)" {
		t.Fatalf("no-root header = %q, want Work items hint", got)
	}
	rows := focusedRows(m.g, "")
	if len(rows) != 1 || !strings.Contains(rows[0].text, "Work items tab") {
		t.Fatalf("focused placeholder not renamed: %+v", rows)
	}

	if help := keys.Tab1.Help(); help.Desc != "work items tab" {
		t.Fatalf("key help = %q, want work items tab", help.Desc)
	}

	for _, tab := range []tab{tabIssues, tabGraph, tabQueue, tabConfig} {
		m.tab = tab
		for _, s := range []string{m.headerView(), m.helpView(), m.hintsView(), m.tabHeaderView()} {
			if strings.Contains(s, "Issues") || strings.Contains(s, "issues") {
				t.Fatalf("tab %d still mentions issues: %q", tab, s)
			}
		}
	}

	wide := resize(newModel(t, newFixture()), 80, 30)
	for _, ln := range strings.Split(wide.View(), "\n") {
		if w := lipgloss.Width(ln); w > 80 {
			t.Fatalf("80-col line width %d > 80: %q", w, ln)
		}
	}
}
