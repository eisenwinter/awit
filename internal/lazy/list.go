package lazy

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var cursorStyle = lipgloss.NewStyle().Reverse(true)

type row struct {
	id         string
	text       string
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

// view renders exactly l.height lines: "> "+text for the cursor row,
// "  "+text otherwise, rune-truncated to width, padded with blank lines.
func (l *cursorList) view(width int) string {
	if width < 3 {
		width = 3
	}
	lines := make([]string, 0, l.height)
	for i := 0; i < l.height; i++ {
		idx := l.offset + i
		if idx < 0 || idx >= len(l.rows) {
			lines = append(lines, "")
			continue
		}
		r := l.rows[idx]
		if idx == l.cursor && r.selectable {
			lines = append(lines, cursorStyle.Render("> "+truncateRunes(r.text, width-2)))
			continue
		}
		lines = append(lines, "  "+truncateRunes(r.text, width-2))
	}
	return strings.Join(lines, "\n")
}

func truncateRunes(s string, n int) string {
	if n < 0 {
		n = 0
	}
	if r := []rune(s); len(r) > n {
		return string(r[:n])
	}
	return s
}
