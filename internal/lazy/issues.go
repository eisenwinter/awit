package lazy

import "fmt"

type issuesState struct {
	filter      Filter
	showArchive bool
	archiveN    int // len(archive) after LoadArchive; header shows "(not loaded)" until archiveLoaded
	list        cursorList
}

func (m *Model) issuesRows() []row {
	if m.issues.showArchive {
		var rows []row
		for _, it := range m.issues.filter.applyArchive(m.archive) {
			rows = append(rows, row{id: it.ID, text: sanitize(m.ops.ArchiveLine(it), false), status: "closed", selectable: true})
		}
		if len(rows) == 0 {
			return []row{{text: "Archive is empty"}}
		}
		return rows
	}
	if m.g == nil {
		return []row{{text: "No open items"}}
	}
	var rows []row
	for _, n := range m.issues.filter.applyOpen(m.g) {
		rows = append(rows, row{id: n.Item.ID, text: sanitize(m.ops.Line(n), false), status: string(n.Item.Status), selectable: !n.Quarantined()})
	}
	if len(rows) == 0 {
		return []row{{text: "No open items"}}
	}
	return rows
}

func (m *Model) issuesHeader() string {
	n := 0
	if m.g != nil {
		n = len(m.g.Order)
	}
	arch := "(not loaded)"
	if m.archiveLoaded {
		arch = fmt.Sprintf("%d", m.issues.archiveN)
	}
	src := "open"
	if m.issues.showArchive {
		src = "archive"
	}
	return fmt.Sprintf("open: %d  archive: %s   filters: %s   source: %s", n, arch, m.issues.filter.badges(), src)
}

func (m *Model) issuesDetail() string {
	r, ok := m.issues.list.selected()
	if !ok {
		if len(m.issues.list.rows) == 0 {
			return "no items match"
		}
		return "no selectable item; press V for the validate report"
	}
	if m.issues.showArchive {
		text, err := m.ops.ArchiveDetail(r.id)
		if err != nil {
			return err.Error()
		}
		return text
	}
	if m.g == nil {
		return "no items match"
	}
	return m.ops.Detail(m.g, r.id)
}

func (m *Model) toggleArchive() {
	idx := m.issues.list.cursor
	m.issues.showArchive = !m.issues.showArchive
	if m.issues.showArchive && !m.archiveLoaded {
		items, err := m.ops.LoadArchive()
		if err != nil {
			m.toast = "error: " + err.Error()
			m.issues.showArchive = false
			return
		}
		m.archive = items
		m.archiveLoaded = true
		m.issues.archiveN = len(items)
	}
	m.issues.list.setRows(m.issuesRows(), "")
	n := len(m.issues.list.rows)
	if n == 0 {
		m.issues.list.cursor = -1
	} else {
		if idx < 0 {
			idx = 0
		}
		if idx >= n {
			idx = n - 1
		}
		m.issues.list.cursor = idx
		m.issues.list.clampOffset()
	}
	m.refreshDetail()
}

func (m *Model) submitSearch() {
	f, err := ParseQuery(m.input.Value())
	if err != nil {
		m.toast = "error: " + err.Error()
		return
	}
	keep := ""
	if r, ok := m.issues.list.selected(); ok {
		keep = r.id
	}
	m.issues.filter = f
	m.issues.list.setRows(m.issuesRows(), keep)
	m.refreshDetail()
}
