package lazy

import "testing"

func TestIssuesRowsAndQuarantineUnselectable(t *testing.T) {
	m := newModel(t, newFixture())
	rows := m.issues.list.rows
	if len(rows) != 8 {
		t.Fatalf("rows = %d, want 8", len(rows))
	}
	for _, r := range rows {
		if r.id == "AWIT-LAZY0008" && r.selectable {
			t.Fatal("quarantined row selectable")
		}
	}
	for i := 0; i < 10; i++ {
		m, _ = press(m, "j")
	}
	if m.selectedID() != "AWIT-LAZY0007" {
		t.Fatalf("cursor stopped on %s, want L7 (L8 unselectable)", m.selectedID())
	}
	golden(t, "issues_open", m.View())
}

func TestIssuesDetailFollowsCursor(t *testing.T) {
	m := newModel(t, newFixture())
	m, _ = press(m, "j")
	if !contains(m.View(), "detail of AWIT-LAZY0002") {
		t.Fatalf("detail did not follow:\n%s", m.View())
	}
}

func TestIssuesSearchPromptAppliesAndEscDiscards(t *testing.T) {
	m := newModel(t, newFixture())
	m, _ = press(m, "/")
	if m.mode != modeSearch {
		t.Fatal("/ did not open search")
	}
	golden(t, "issues_search_prompt", m.View())
	m, _ = press(m, "-", "-", "r", "e", "a", "d", "y", "enter")
	if m.mode != modeNormal || len(m.issues.list.rows) != 3 {
		t.Fatalf("after --ready: mode=%d rows=%d", m.mode, len(m.issues.list.rows))
	}
	golden(t, "issues_filtered", m.View())
	m, _ = press(m, "/", "-", "x", "esc")
	if len(m.issues.list.rows) != 3 || m.toast != "" {
		t.Fatalf("esc must discard the draft: rows=%d toast=%q", len(m.issues.list.rows), m.toast)
	}
	m, _ = press(m, "/", "-", "x", "enter")
	if m.toast != "error: unknown flag -x" || len(m.issues.list.rows) != 3 {
		t.Fatalf("bad query: toast=%q rows=%d", m.toast, len(m.issues.list.rows))
	}
	m, _ = press(m, "/", "q", "enter")
	if m.mode != modeNormal || m.issues.filter.Search != "q" {
		t.Fatal("q inside the prompt must type, not quit")
	}
}

func TestIssuesArchiveToggle(t *testing.T) {
	f := newFixture()
	m := newModel(t, f)
	if contains(m.View(), "archive: 2") {
		t.Fatal("archive counted before load")
	}
	m, cmd := press(m, "j", "o")
	if cmd == nil {
		t.Fatal("first o must dispatch an async LoadArchive")
	}
	if n := countCalls(f, "LoadArchive"); n != 0 {
		t.Fatalf("LoadArchive called synchronously (%d), want async", n)
	}
	next, _ := m.Update(cmd())
	m = next.(Model)
	if !m.issues.showArchive || len(m.issues.list.rows) != 2 || m.issues.list.cursor != 1 {
		t.Fatalf("archive: show=%v rows=%d cursor=%d", m.issues.showArchive, len(m.issues.list.rows), m.issues.list.cursor)
	}
	if m.selectedID() != "" {
		t.Fatal("archive rows must not be mutation targets")
	}
	if !contains(m.View(), "archive of AWIT-LAZY0102") || !contains(m.View(), "archive: 2") {
		t.Fatalf("archive view:\n%s", m.View())
	}
	golden(t, "issues_archive", m.View())
	m, _ = press(m, "o", "o")
	if n := countCalls(f, "LoadArchive"); n != 1 {
		t.Fatalf("LoadArchive called %d times, want 1 (cached)", n)
	}
	m, cmd = press(m, "R")
	if cmd == nil {
		t.Fatal("R must dispatch an async reload")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if n := countCalls(f, "LoadArchive"); n != 2 {
		t.Fatalf("R must reload the archive once loaded; calls=%d", n)
	}
}
