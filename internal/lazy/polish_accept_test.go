package lazy

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// Hints must fit narrow terminals (<=78 cells per the polish ticket).
func TestPolishHintsFitWidth(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())
	for _, tab := range []tab{tabIssues, tabGraph, tabQueue, tabConfig} {
		m.tab = tab
		if w := lipgloss.Width(m.hintsView()); w > 78 {
			t.Fatalf("tab %d hints width = %d, want <= 78: %q", tab, w, m.hintsView())
		}
	}
}

// Every emitted line fits m.width at 60x20 and 100x30, with zero SGR.
func TestPolishFramesFitWidth(t *testing.T) {
	t.Parallel()
	for _, size := range [][2]int{{60, 20}, {100, 30}} {
		for _, tab := range []string{"1", "2", "3", "4"} {
			m := resize(newModel(t, newFixture()), size[0], size[1])
			m, _ = press(m, tab)
			for _, help := range []bool{false, true} {
				mm := m
				if help {
					mm, _ = press(m, "?")
				}
				got := mm.View()
				if strings.ContainsRune(got, '\x1b') {
					t.Fatalf("%dx%d tab %s help=%v: output contains escape byte", size[0], size[1], tab, help)
				}
				for _, ln := range strings.Split(got, "\n") {
					if w := lipgloss.Width(ln); w > size[0] {
						t.Fatalf("%dx%d tab %s help=%v: line width %d > %d: %q", size[0], size[1], tab, help, w, size[0], ln)
					}
				}
				if help {
					break
				}
			}
		}
	}
}

// Active tab carries an ascii ">" marker even with zero SGR.
func TestPolishActiveTabMarker(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		press string
		want  string
	}{
		{"1", ">[1] Issues"},
		{"2", ">[2] Graph"},
		{"3", ">[3] Queue"},
		{"4", ">[4] Config"},
	} {
		m := newModel(t, newFixture())
		m, _ = press(m, tc.press)
		got := m.View()
		if strings.ContainsRune(got, '\x1b') {
			t.Fatalf("tab %s: output contains escape byte", tc.press)
		}
		if !strings.Contains(got, tc.want) {
			t.Fatalf("tab %s header missing %q:\n%s", tc.press, tc.want, got)
		}
	}
}

// Empty graph root shows the no-root header, not a blank "Focused on ".
func TestPolishGraphEmptyRootHeader(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())
	m.tab = tabGraph
	m.graphTab.focused = true
	m.graphTab.rootID = ""
	if got := m.tabHeaderView(); got != "Focused (no root — select an item on Issues)" {
		t.Fatalf("header = %q, want no-root hint", got)
	}
}

// Help never exceeds the body height (30-row and 20-row windows).
func TestPolishHelpFitsBody(t *testing.T) {
	t.Parallel()
	for _, h := range []int{30, 20} {
		m := resize(newModel(t, newFixture()), 100, h)
		lines := strings.Split(m.helpView(), "\n")
		if len(lines) > m.bodyHeight() {
			t.Fatalf("height %d: help lines %d > body %d", h, len(lines), m.bodyHeight())
		}
	}
}
