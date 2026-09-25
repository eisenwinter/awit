package lazy

import (
	"testing"

	"github.com/eisenwinter/awit/pkg/item"
)

func TestRedTruncateEllipsis(t *testing.T) {
	t.Parallel()
	if got := truncateRunes("hello", 4); got != "hel…" {
		t.Fatalf("truncateRunes(hello,4) = %q, want hel…", got)
	}
}

func TestRedIssuesEmptyPlaceholder(t *testing.T) {
	t.Parallel()
	f := newFixture()
	f.items = map[string]*item.Item{}
	m := newModel(t, f)
	rows := m.issuesRows()
	if len(rows) == 0 {
		t.Fatal("issuesRows empty, want unselectable placeholder")
	}
	if rows[0].selectable {
		t.Fatal("placeholder selectable, want unselectable")
	}
	if rows[0].text != "No open items" {
		t.Fatalf("placeholder = %q, want No open items", rows[0].text)
	}
}

func TestRedIssuesArchiveEmptyPlaceholder(t *testing.T) {
	t.Parallel()
	f := newFixture()
	f.archive = nil
	m := newModel(t, f)
	m.issues.showArchive = true
	m.issues.archiveN = 0
	rows := m.issuesRows()
	if len(rows) == 0 {
		t.Fatal("archive issuesRows empty, want unselectable placeholder")
	}
	if rows[0].selectable {
		t.Fatal("archive placeholder selectable, want unselectable")
	}
	if rows[0].text != "Archive is empty" {
		t.Fatalf("archive placeholder = %q, want Archive is empty", rows[0].text)
	}
}
