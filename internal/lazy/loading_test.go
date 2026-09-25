package lazy

import (
	"errors"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/item"
)

// RED-first: archive toggle must dispatch a tea.Cmd and show loading…
// meanwhile instead of blocking on file I/O.
func TestLoadingArchiveToggleAsync(t *testing.T) {
	t.Parallel()
	f := newFixture()
	m := newModel(t, f)
	m, cmd := press(m, "o")
	if cmd == nil {
		t.Fatal("o must return a cmd and not block on LoadArchive")
	}
	if n := countCalls(f, "LoadArchive"); n != 0 {
		t.Fatalf("LoadArchive called synchronously (%d), want async", n)
	}
	if !strings.Contains(m.View(), "loading…") {
		t.Fatalf("archive frame missing loading… status:\n%s", m.View())
	}
	next, _ := m.Update(cmd())
	m = next.(Model)
	if n := countCalls(f, "LoadArchive"); n != 1 {
		t.Fatalf("LoadArchive called %d times, want 1 after cmd", n)
	}
	if !m.issues.showArchive || len(m.issues.list.rows) != 2 {
		t.Fatalf("after archive msg: show=%v rows=%d", m.issues.showArchive, len(m.issues.list.rows))
	}
	if strings.Contains(m.View(), "loading…") {
		t.Fatalf("loading… stuck after archive msg:\n%s", m.View())
	}
}

// RED-first: reload's Load must go through a tea.Cmd; errors keep the old
// graph + toast.
func TestLoadingReloadAsync(t *testing.T) {
	t.Parallel()
	f := newFixture()
	m := newModel(t, f)
	before := m.selectedID()
	delete(f.items, "AWIT-LAZY0008")
	m, cmd := press(m, "R")
	if cmd == nil {
		t.Fatal("R must return a cmd and not block on Load")
	}
	if !strings.Contains(m.View(), "loading…") {
		t.Fatalf("reload frame missing loading… status:\n%s", m.View())
	}
	// Old graph stays while loading.
	if len(m.g.Order) == 0 {
		t.Fatal("reload cleared the graph while loading")
	}
	next, _ := m.Update(cmd())
	m = next.(Model)
	if strings.Contains(m.View(), "loading…") {
		t.Fatalf("loading… stuck after reload msg:\n%s", m.View())
	}
	if contains(m.View(), "quarantined, run awit validate") {
		t.Fatal("reload msg did not swap the graph")
	}
	if m.selectedID() == "" || before == "" {
		t.Fatal("selection lost across async reload")
	}
}

func TestLoadingReloadErrorKeepsGraph(t *testing.T) {
	t.Parallel()
	f := newFixture()
	m := newModel(t, f)
	before := m.selectedID()
	f.loadErr = errors.New("store locked")
	m, cmd := press(m, "R")
	if cmd == nil {
		t.Fatal("R must return a cmd even when Load will fail")
	}
	next, _ := m.Update(cmd())
	m = next.(Model)
	if m.g == nil || len(m.g.Order) == 0 {
		t.Fatal("async reload on error dropped the graph")
	}
	if m.toast != "error: store locked" {
		t.Fatalf("toast = %q, want the load error", m.toast)
	}
	if got := m.selectedID(); got != before {
		t.Fatalf("selectedID after failed reload = %q, want %q", got, before)
	}
}

// Archive Load errors keep the open view + an error toast (sync semantics).
type failArchiveOps struct {
	*fakeOps
	err error
}

func (o failArchiveOps) LoadArchive() ([]*item.Item, error) {
	o.calls = append(o.calls, "LoadArchive")
	return nil, o.err
}

func TestLoadingArchiveErrorKeepsOpen(t *testing.T) {
	t.Parallel()
	f := newFixture()
	m := newModel(t, f)
	openRows := len(m.issues.list.rows)
	m.ops = failArchiveOps{fakeOps: f, err: errors.New("archive locked")}
	m, cmd := press(m, "o")
	if cmd == nil {
		t.Fatal("o must return a cmd even when LoadArchive will fail")
	}
	if !strings.Contains(m.View(), "loading…") {
		t.Fatalf("archive frame missing loading… status:\n%s", m.View())
	}
	next, _ := m.Update(cmd())
	m = next.(Model)
	if m.toast != "error: archive locked" {
		t.Fatalf("toast = %q, want the archive error", m.toast)
	}
	if m.issues.showArchive {
		t.Fatal("showArchive stuck true after archive error")
	}
	if len(m.issues.list.rows) != openRows {
		t.Fatalf("rows = %d, want open rows %d", len(m.issues.list.rows), openRows)
	}
	if strings.Contains(m.View(), "loading…") {
		t.Fatalf("loading… stuck after archive error:\n%s", m.View())
	}
}

// Goldens cover the loading line only so existing frames never churn.
func TestLoadingLineGolden(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())
	m, cmd := press(m, "o")
	if cmd == nil {
		t.Fatal("o must return a cmd (async loading)")
	}
	if got := m.statusLine(); got != "loading…" {
		t.Fatalf("statusLine = %q, want loading…", got)
	}
	// The loading row is a plain state-rendered row (no timers/overlays).
	if len(m.issues.list.rows) != 1 || m.issues.list.rows[0].text != "loading…" {
		t.Fatalf("loading rows = %+v, want single loading… row", m.issues.list.rows)
	}
	golden(t, "loading", m.statusLine()+"\n")
}

// A slower first reload must never overwrite a newer snapshot: double-R
// dispatches two generations; the stale result drops.
func TestLoadingDoubleReloadDropsStale(t *testing.T) {
	t.Parallel()
	f := newFixture()
	m := newModel(t, f)
	m, cmd1 := press(m, "R")
	if cmd1 == nil {
		t.Fatal("first R must dispatch a reload")
	}
	m, cmd2 := press(m, "R")
	if cmd2 == nil {
		t.Fatal("second R during pending reload must dispatch (generation), not be swallowed")
	}
	if !m.isLoading() {
		t.Fatal("loading must stay visible while two reloads are pending")
	}
	msg1 := cmd1()
	delete(f.items, "AWIT-LAZY0008")
	msg2 := cmd2()
	// Newer first: swap to the fresh graph (quarantine gone).
	next, _ := m.Update(msg2)
	m = next.(Model)
	if m.isLoading() {
		t.Fatal("loading stuck after the current reload landed")
	}
	if contains(m.View(), "quarantined, run awit validate") {
		t.Fatal("newer reload did not swap the graph")
	}
	// Stale last: must drop, never revert to the older snapshot.
	next, _ = m.Update(msg1)
	m = next.(Model)
	if m.isLoading() {
		t.Fatal("stale reload toggled the loading state")
	}
	if contains(m.View(), "quarantined, run awit validate") {
		t.Fatal("stale first reload overwrote the newer snapshot")
	}
}

// A late archive error after leaving must keep the open view (no cursor
// reset) + toast: only touch the view if the result is still current.
func TestLoadingLateArchiveErrorKeepsView(t *testing.T) {
	t.Parallel()
	f := newFixture()
	m := newModel(t, f)
	m.ops = failArchiveOps{fakeOps: f, err: errors.New("archive locked")}
	m, cmd := press(m, "o")
	if cmd == nil {
		t.Fatal("o must dispatch an async LoadArchive")
	}
	// Leave before the error lands: instant, load keeps caching.
	m, _ = press(m, "o")
	if m.issues.showArchive {
		t.Fatal("second o must leave the archive while loading")
	}
	// Establish a known open selection after leaving; the late error must
	// not reset it.
	m, _ = press(m, "j")
	wantSel := m.selectedID()
	wantCursor := m.issues.list.cursor
	next, _ := m.Update(cmd())
	m = next.(Model)
	if m.toast != "error: archive locked" {
		t.Fatalf("toast = %q, want the late archive error", m.toast)
	}
	if m.issues.showArchive {
		t.Fatal("late error forced showArchive true")
	}
	if got := m.selectedID(); got != wantSel {
		t.Fatalf("selectedID = %q, want kept open selection %q", got, wantSel)
	}
	if m.issues.list.cursor != wantCursor {
		t.Fatalf("cursor = %d, want kept %d (no reset on stale error)", m.issues.list.cursor, wantCursor)
	}
	if m.isLoading() {
		t.Fatal("loading stuck after the late archive error")
	}
}
