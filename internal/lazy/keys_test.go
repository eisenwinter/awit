package lazy

import (
	"fmt"
	"testing"

	"github.com/charmbracelet/bubbles/key"
)

func TestKeymapBindings(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		binding key.Binding
		press   []string
		miss    string
	}{
		{"Tab1", keys.Tab1, []string{"1"}, "2"},
		{"Tab2", keys.Tab2, []string{"2"}, "1"},
		{"Tab3", keys.Tab3, []string{"3"}, "1"},
		{"Tab4", keys.Tab4, []string{"4"}, "3"},
		{"Edit", keys.Edit, []string{"e"}, "enter"},
		{"Up", keys.Up, []string{"k", "up"}, "j"},
		{"Down", keys.Down, []string{"j", "down"}, "k"},
		{"Left", keys.Left, []string{"h", "left"}, "l"},
		{"Right", keys.Right, []string{"l", "right"}, "h"},
		{"Tab", keys.Tab, []string{"tab"}, "enter"},
		{"Enter", keys.Enter, []string{"enter"}, "tab"},
		{"Esc", keys.Esc, []string{"esc"}, "q"},
		{"Search", keys.Search, []string{"/"}, "o"},
		{"Source", keys.Source, []string{"o"}, "/"},
		{"Close", keys.Close, []string{"c"}, "b"},
		{"Block", keys.Block, []string{"b"}, "c"},
		{"Unblock", keys.Unblock, []string{"u"}, "m"},
		{"Comment", keys.Comment, []string{"m"}, "u"},
		{"Claim", keys.Claim, []string{" "}, "r"},
		{"Release", keys.Release, []string{"r"}, " "},
		{"Reload", keys.Reload, []string{"R"}, "r"},
		{"External", keys.External, []string{"P"}, "V"},
		{"Validate", keys.Validate, []string{"V"}, "P"},
		{"Help", keys.Help, []string{"?"}, "q"},
		{"Quit", keys.Quit, []string{"q", "ctrl+c"}, "x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			for _, k := range tc.press {
				msg, ok := keyMsg(k).(fmt.Stringer)
				if !ok {
					t.Fatalf("keyMsg(%q) is not a Stringer", k)
				}
				if !key.Matches(msg, tc.binding) {
					t.Fatalf("%s does not match %q", tc.name, k)
				}
			}
			miss, ok := keyMsg(tc.miss).(fmt.Stringer)
			if !ok {
				t.Fatalf("keyMsg(%q) is not a Stringer", tc.miss)
			}
			if key.Matches(miss, tc.binding) {
				t.Fatalf("%s unexpectedly matches %q", tc.name, tc.miss)
			}
		})
	}
}

func TestKeymapTabs(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())
	if m.tab != tabIssues {
		t.Fatalf("initial tab = %v, want tabIssues", m.tab)
	}
	m, _ = press(m, "2")
	if m.tab != tabGraph {
		t.Fatalf("after 2 tab = %v, want tabGraph", m.tab)
	}
	m, _ = press(m, "3")
	if m.tab != tabQueue {
		t.Fatalf("after 3 tab = %v, want tabQueue", m.tab)
	}
	m, _ = press(m, "4")
	if m.tab != tabConfig {
		t.Fatalf("after 4 tab = %v, want tabConfig", m.tab)
	}
	m, _ = press(m, "1")
	if m.tab != tabIssues {
		t.Fatalf("after 1 tab = %v, want tabIssues", m.tab)
	}
}

func TestKeymapMoveAndFocus(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())
	first := m.selectedID()
	m, _ = press(m, "j")
	if got := m.selectedID(); got == first || got == "" {
		t.Fatalf("after j selectedID = %q (was %q), want next row", got, first)
	}
	m, _ = press(m, "k")
	if got := m.selectedID(); got != first {
		t.Fatalf("after j,k selectedID = %q, want %q", got, first)
	}
	// The dangling-dep (quarantined) row is visible but never selected.
	m, _ = press(m, "j", "j", "j", "j", "j")
	if got := m.selectedID(); got != "AWIT-LAZY0006" {
		t.Fatalf("after j*5 selectedID = %q, want AWIT-LAZY0006", got)
	}
	m, _ = press(m, "l")
	if m.focus != focusDetail {
		t.Fatalf("after l focus = %v, want focusDetail", m.focus)
	}
	m, _ = press(m, "h")
	if m.focus != focusList {
		t.Fatalf("after h focus = %v, want focusList", m.focus)
	}
	m, _ = press(m, "tab")
	if m.focus != focusDetail {
		t.Fatalf("after tab focus = %v, want focusDetail", m.focus)
	}
	m, _ = press(m, "esc")
	if m.focus != focusList {
		t.Fatalf("after esc focus = %v, want focusList", m.focus)
	}
}

func TestKeymapHelp(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())
	m, _ = press(m, "?")
	if m.mode != modeHelp {
		t.Fatalf("after ? mode = %v, want modeHelp", m.mode)
	}
	m, _ = press(m, "esc")
	if m.mode != modeNormal {
		t.Fatalf("after esc mode = %v, want modeNormal", m.mode)
	}
}
