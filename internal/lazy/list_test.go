package lazy

import (
	"strings"
	"testing"
)

func sampleRows() []row {
	return []row{
		{id: "h1", text: "READY (2)", selectable: false},
		{id: "AWIT-00000001", text: "[AWIT-00000001] open first", selectable: true},
		{id: "AWIT-00000006", text: "[AWIT-00000006] quarantined sixth", selectable: false},
		{id: "AWIT-00000002", text: "[AWIT-00000002] open second", selectable: true},
	}
}

func TestCursorListMoveSkipsUnselectable(t *testing.T) {
	t.Parallel()
	var l cursorList
	l.height = 10
	l.setRows(sampleRows(), "")
	if got, _ := l.selected(); got.id != "AWIT-00000001" {
		t.Fatalf("setRows without keepID = %q, want first selectable", got.id)
	}
	l.move(1)
	if got, _ := l.selected(); got.id != "AWIT-00000002" {
		t.Fatalf("move(1) = %q, want AWIT-00000002 (skip quarantined row)", got.id)
	}
	l.move(1)
	if got, _ := l.selected(); got.id != "AWIT-00000002" {
		t.Fatalf("move(1) at end = %q, want no wrap", got.id)
	}
	l.move(-1)
	if got, _ := l.selected(); got.id != "AWIT-00000001" {
		t.Fatalf("move(-1) = %q, want AWIT-00000001", got.id)
	}
	l.move(-1)
	if got, _ := l.selected(); got.id != "AWIT-00000001" {
		t.Fatalf("move(-1) at start = %q, want clamp", got.id)
	}
}

func TestCursorListSetRowsKeepsSelection(t *testing.T) {
	t.Parallel()
	var l cursorList
	l.height = 10
	l.setRows(sampleRows(), "AWIT-00000002")
	if got, _ := l.selected(); got.id != "AWIT-00000002" {
		t.Fatalf("setRows keepID = %q, want AWIT-00000002", got.id)
	}
	// keepID on an unselectable row falls back to the first selectable row.
	l.setRows(sampleRows(), "AWIT-00000006")
	if got, _ := l.selected(); got.id != "AWIT-00000001" {
		t.Fatalf("setRows unselectable keepID = %q, want first selectable", got.id)
	}
	// keepID that no longer exists falls back to the first selectable row.
	l.setRows(sampleRows(), "AWIT-9ZZZZZZZ")
	if got, _ := l.selected(); got.id != "AWIT-00000001" {
		t.Fatalf("setRows missing keepID = %q, want first selectable", got.id)
	}
	// Nothing selectable: cursor -1 and selected() reports false.
	var empty cursorList
	empty.height = 4
	empty.setRows([]row{{text: "no items match"}}, "")
	if _, ok := empty.selected(); ok {
		t.Fatal("selected() on unselectable rows = true, want false")
	}
	if empty.cursor != -1 {
		t.Fatalf("cursor = %d, want -1", empty.cursor)
	}
}

func TestCursorListViewHeight(t *testing.T) {
	t.Parallel()
	var l cursorList
	l.height = 4
	l.setRows(sampleRows(), "")
	view := l.view(100, true)
	lines := strings.Split(view, "\n")
	if len(lines) != 4 {
		t.Fatalf("view lines = %d, want exactly height (4):\n%q", len(lines), view)
	}
	if !strings.HasPrefix(lines[1], "> ") {
		t.Fatalf("cursor line = %q, want \"> \" prefix", lines[1])
	}
	if !strings.HasPrefix(lines[0], "  ") {
		t.Fatalf("header line = %q, want \"  \" prefix", lines[0])
	}
	// Rune-truncation to width.
	wide := l.view(10, true)
	for _, line := range strings.Split(wide, "\n") {
		if n := len([]rune(line)); n > 10 {
			t.Fatalf("line %q exceeds width 10", line)
		}
	}
}
