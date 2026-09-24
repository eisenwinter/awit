package lazy

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestViewFatalGolden(t *testing.T) {
	t.Parallel()
	m := New(newFixture(), context.Background(), Options{Fatal: "no .awit directory found"})
	um, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	got := um.(Model).View()
	if !strings.Contains(got, "run awit init") {
		t.Fatalf("fatal view missing init hint:\n%s", got)
	}
	if !strings.Contains(got, "press q to quit") {
		t.Fatalf("fatal view missing quit hint:\n%s", got)
	}
	golden(t, "fatal", got)
}

func TestViewFrameGolden(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())
	golden(t, "frame-issues", m.View())

	m, _ = press(m, "2")
	golden(t, "frame-graph", m.View())

	m, _ = press(m, "3")
	golden(t, "frame-queue", m.View())
}

func TestViewHelpGolden(t *testing.T) {
	t.Parallel()
	m, _ := press(newModel(t, newFixture()), "?")
	if m.mode != modeHelp {
		t.Fatalf("mode = %v, want modeHelp", m.mode)
	}
	golden(t, "frame-help", m.View())
}

func TestViewQuarantineFooter(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())
	if m.quarantined == 0 {
		t.Fatal("quarantined = 0, want footer data from the dangling-dep item")
	}
	if !strings.Contains(m.View(), "warning: 1 items quarantined, run awit validate") {
		t.Fatalf("frame missing quarantine footer:\n%s", m.View())
	}
}

func TestReloadOnErrorKeepsGraph(t *testing.T) {
	t.Parallel()
	fx := newFixture()
	m := New(fx, context.Background(), Options{})
	um, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = um.(Model)
	before := m.selectedID()
	fx.loadErr = errors.New("store locked")
	m.reload()
	if m.g == nil || len(m.g.Order) == 0 {
		t.Fatal("reload on error dropped the graph")
	}
	if m.toast != "store locked" {
		t.Fatalf("toast = %q, want the load error", m.toast)
	}
	if got := m.selectedID(); got != before {
		t.Fatalf("selectedID after failed reload = %q, want %q", got, before)
	}
}

func TestActToastAndReload(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())
	id := m.selectedID()
	if id == "" {
		t.Fatal("no selection to act on")
	}
	m.act(func() error { return errors.New("boom") }, "ok")
	if m.toast != "boom" {
		t.Fatalf("toast on error = %q, want boom", m.toast)
	}
	m.act(func() error { return nil }, "closed "+id)
	if m.toast != "closed "+id {
		t.Fatalf("toast on ok = %q", m.toast)
	}
}
