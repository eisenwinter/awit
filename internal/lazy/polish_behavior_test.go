package lazy

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// Undersized terminals show only the gate message.
func TestSizeGateMessage(t *testing.T) {
	t.Parallel()
	for _, size := range [][2]int{{40, 10}, {100, 11}, {49, 30}, {30, 30}} {
		m := resize(newModel(t, newFixture()), size[0], size[1])
		want := fmt.Sprintf("lazyawit needs >= 50x12 (have %dx%d)", size[0], size[1])
		if got := m.View(); got != want {
			t.Fatalf("%dx%d: View() = %q, want %q", size[0], size[1], got, want)
		}
	}
}

// The minimum size itself is not gated.
func TestSizeGateBoundary(t *testing.T) {
	t.Parallel()
	m := resize(newModel(t, newFixture()), 50, 12)
	if got := m.View(); strings.Contains(got, "needs >=") {
		t.Fatalf("50x12 gated, want full frame:\n%s", got)
	}
}

// Toggling the toast must not change the frame line count at a fixed size.
func TestToastSlotStableLineCount(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())
	base := strings.Count(m.View(), "\n")
	m.toast = "hello"
	m2 := m
	if n := strings.Count(m2.View(), "\n"); n != base {
		t.Fatalf("toast changed frame newlines: %d vs %d", n, base)
	}
	// Blank toast slot: the line before the hints is empty when no toast.
	m3 := newModel(t, newFixture())
	lines := strings.Split(m3.View(), "\n")
	if lines[len(lines)-2] != "" {
		t.Fatalf("toast slot = %q, want blank line before hints", lines[len(lines)-2])
	}
}

// The Queue tab always renders the why line, even with no selection.
func TestQueueWhyAlwaysRendered(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())
	m.tab = tabQueue
	m.queue.list.setRows(nil, "")
	if got := m.View(); !strings.Contains(got, "why: no selection") {
		t.Fatalf("queue without selection missing why line:\n%s", got)
	}
	m2 := newModel(t, newFixture())
	m2, _ = press(m2, "3")
	if got := m2.View(); !strings.Contains(got, "why: ") {
		t.Fatalf("queue frame missing why line:\n%s", got)
	}
}

// An unfocused list renders its cursor row plain: no ">" marker.
func TestCursorListUnfocusedPlain(t *testing.T) {
	t.Parallel()
	var l cursorList
	l.height = 4
	l.setRows(sampleRows(), "")
	focused := strings.Split(l.view(100, true), "\n")
	if !strings.HasPrefix(focused[1], "> ") {
		t.Fatalf("focused cursor line = %q, want \"> \" prefix", focused[1])
	}
	plain := strings.Split(l.view(100, false), "\n")
	for _, ln := range plain {
		if strings.HasPrefix(ln, "> ") {
			t.Fatalf("unfocused line = %q, want no \"> \" marker", ln)
		}
	}
	if !strings.HasPrefix(plain[1], "  ") {
		t.Fatalf("unfocused cursor line = %q, want \"  \" prefix", plain[1])
	}
	if strings.ContainsRune(l.view(100, false), '\x1b') {
		t.Fatal("unfocused view contains escape byte")
	}
}

// The header names the focused pane and shows the detail scroll position.
func TestDetailScrollCue(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())
	if got := m.headerView(); !strings.Contains(got, "focus: list") {
		t.Fatalf("header = %q, want focus: list", got)
	}
	m, _ = press(m, "enter")
	got := m.headerView()
	if !strings.Contains(got, "focus: detail ") || !strings.Contains(got, "%") {
		t.Fatalf("header = %q, want focus: detail <pct>%% cue", got)
	}
}

// Detail content wraps to the detail width; every viewport line fits.
func TestWrapDetailWidth(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("alpha beta ", 30)
	got := wrapDetail(long, 40)
	for _, ln := range strings.Split(got, "\n") {
		if w := lipgloss.Width(ln); w > 40 {
			t.Fatalf("wrapped line width %d > 40: %q", w, ln)
		}
	}
}

func TestDetailWrappedInModel(t *testing.T) {
	t.Parallel()
	f := newFixture()
	f.detail = strings.Repeat("alpha beta ", 30)
	m := newModel(t, f)
	_, detailW := m.columns()
	for _, ln := range strings.Split(m.detail.View(), "\n") {
		if w := lipgloss.Width(ln); w > detailW {
			t.Fatalf("detail line width %d > %d: %q", w, detailW, ln)
		}
	}
}
