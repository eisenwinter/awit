package lazy

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type row struct {
	id         string
	text       string
	status     string
	head       bool
	hstatus    string // header slot kind: ready, blocked, critical, or "" (accent)
	selectable bool
}

type cursorList struct {
	rows           []row
	cursor, offset int // cursor -1 = nothing selectable
	height         int
}

// setRows replaces the rows, keeping the cursor on keepID when that row is
// selectable, else on the first selectable row, else -1. The offset is
// clamped so the cursor stays visible.
func (l *cursorList) setRows(rows []row, keepID string) {
	l.rows = rows
	l.cursor = -1
	if keepID != "" {
		for i, r := range rows {
			if r.id == keepID && r.selectable {
				l.cursor = i
				break
			}
		}
	}
	if l.cursor == -1 {
		for i, r := range rows {
			if r.selectable {
				l.cursor = i
				break
			}
		}
	}
	l.clampOffset()
}

func (l *cursorList) clampOffset() {
	if l.height <= 0 || l.cursor < 0 {
		l.offset = 0
		return
	}
	if l.offset > l.cursor {
		l.offset = l.cursor
	}
	if l.offset < l.cursor-l.height+1 {
		l.offset = l.cursor - l.height + 1
	}
	if l.offset < 0 {
		l.offset = 0
	}
}

// move steps the cursor delta selectable rows in the given direction,
// skipping unselectable rows. No wrap; the cursor clamps at the ends and
// stays within [offset, offset+height).
func (l *cursorList) move(delta int) {
	if len(l.rows) == 0 || l.cursor < 0 {
		return
	}
	dir := 1
	if delta < 0 {
		dir = -1
	}
	for n := delta * dir; n > 0; n-- {
		i := l.cursor + dir
		for i >= 0 && i < len(l.rows) && !l.rows[i].selectable {
			i += dir
		}
		if i < 0 || i >= len(l.rows) {
			break
		}
		l.cursor = i
	}
	l.clampOffset()
}

func (l *cursorList) selected() (row, bool) {
	if l.cursor < 0 || l.cursor >= len(l.rows) {
		return row{}, false
	}
	if r := l.rows[l.cursor]; r.selectable {
		return r, true
	}
	return row{}, false
}

// view renders exactly l.height lines. When focused, the cursor row carries
// "> " plus reverse; unfocused (detail has focus) it renders plain like
// every other row, so focus stays unambiguous without color. Text is
// rune-truncated to width, short lists padded with blank lines. Styling runs
// after truncation so width math never sees ANSI.
func (l *cursorList) view(width int, focused bool) string {
	if width < 3 {
		width = 3
	}
	lines := make([]string, 0, l.height)
	for i := range l.height {
		idx := l.offset + i
		if idx < 0 || idx >= len(l.rows) {
			lines = append(lines, "")
			continue
		}
		r := l.rows[idx]
		if r.head {
			lines = append(lines, "  "+graphHeadStyle(r.hstatus).Render(truncateRunes(r.text, width-2)))
			continue
		}
		if focused && idx == l.cursor && r.selectable {
			lines = append(lines, stCursor.Render(stCursorMarker.Render("> ")+styleRow(truncateRunes(r.text, width-2), r.status)))
			continue
		}
		lines = append(lines, "  "+styleRow(truncateRunes(r.text, width-2), r.status))
	}
	return strings.Join(lines, "\n")
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	w := 0
	var out []rune
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > n-1 {
			break
		}
		out = append(out, r)
		w += rw
	}
	return string(out) + "…"
}

// truncateHard cuts s to n cells without an ellipsis (hints line).
func truncateHard(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	w := 0
	var out []rune
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > n {
			break
		}
		out = append(out, r)
		w += rw
	}
	return string(out)
}
