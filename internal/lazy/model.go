package lazy

import (
	"context"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

type tab int

const (
	tabIssues tab = iota
	tabGraph
	tabQueue
)

type focus int

const (
	focusList focus = iota
	focusDetail
)

type mode int

const (
	modeNormal mode = iota
	modeSearch
	modeInput
	modeHelp
)

type inputKind int

const (
	inputClose inputKind = iota
	inputBlock
	inputComment
)

// issuesState, graphState and queueState are placeholders in the skeleton:
// each tab owns only its cursor list. WI-5..WI-7 grow them into full tab
// state (filters, archive source, overview/focused, why line).
type issuesState struct{ list cursorList }

type graphState struct{ list cursorList }

type queueState struct{ list cursorList }

type Model struct {
	ops           Ops
	ctx           context.Context
	width, height int
	fatal         string
	g             *graph.Graph
	onCritical    map[string]bool
	quarantined   int
	archive       []*item.Item
	archiveLoaded bool
	tab           tab
	focus         focus
	mode          mode
	issues        issuesState
	graphTab      graphState
	queue         queueState
	detail        viewport.Model
	input         textinput.Model
	inputKind     inputKind
	inputTarget   string
	toast         string
}

type Options struct{ Fatal string }

// New builds the root model. opts.Fatal != "" skips Load and renders the
// missing-.awit screen; otherwise Load runs once and a load error becomes
// the fatal screen.
func New(ops Ops, ctx context.Context, opts Options) Model {
	if ctx == nil {
		ctx = context.Background()
	}
	m := Model{
		ops:        ops,
		ctx:        ctx,
		width:      80,
		height:     24,
		onCritical: map[string]bool{},
		detail:     viewport.New(0, 0),
		input:      textinput.New(),
	}
	if opts.Fatal != "" {
		m.fatal = opts.Fatal
		return m
	}
	if ops == nil {
		m.fatal = "no store"
		return m
	}
	g, err := ops.Load()
	if err != nil {
		m.fatal = err.Error()
		return m
	}
	m.setGraph(g)
	m.layout()
	return m
}

// Init implements tea.Model. Loading happens in New; there is nothing to do.
func (m Model) Init() tea.Cmd { return nil }

// Update implements tea.Model: window resizes, global keys (tabs, help,
// reload, quit), list movement, focus toggles, and detail scrolling.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	if m.focus == focusDetail && m.mode == modeNormal {
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.fatal != "" {
		if key.Matches(msg, keys.Quit) {
			return m, tea.Quit
		}
		return m, nil
	}
	switch m.mode {
	case modeHelp:
		// ?/esc/q all close the help screen; q quits only from normal mode.
		if key.Matches(msg, keys.Help) || key.Matches(msg, keys.Esc) || key.Matches(msg, keys.Quit) {
			m.mode = modeNormal
		}
		return m, nil
	case modeSearch, modeInput:
		if key.Matches(msg, keys.Esc) {
			m.mode = modeNormal
			m.input.Blur()
			m.input.Reset()
			return m, nil
		}
		if key.Matches(msg, keys.Enter) {
			// WI-8 dispatches on inputKind; the skeleton discards.
			m.mode = modeNormal
			m.input.Blur()
			m.input.Reset()
			return m, nil
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	// Toasts persist until the next key press; no timers.
	m.toast = ""
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Tab1):
		m.tab = tabIssues
		m.refreshDetail()
		return m, nil
	case key.Matches(msg, keys.Tab2):
		m.tab = tabGraph
		m.refreshDetail()
		return m, nil
	case key.Matches(msg, keys.Tab3):
		m.tab = tabQueue
		m.refreshDetail()
		return m, nil
	case key.Matches(msg, keys.Help):
		m.mode = modeHelp
		return m, nil
	case key.Matches(msg, keys.Reload):
		m.reload()
		return m, nil
	case key.Matches(msg, keys.Esc), key.Matches(msg, keys.Left):
		m.focus = focusList
		return m, nil
	case key.Matches(msg, keys.Tab), key.Matches(msg, keys.Enter), key.Matches(msg, keys.Right):
		// WI-6 retargets Tab on the Graph tab to the Overview/Focused
		// toggle; until then Tab shifts focus like enter.
		m.focus = focusDetail
		return m, nil
	}
	if m.focus == focusList && (key.Matches(msg, keys.Up) || key.Matches(msg, keys.Down)) {
		if key.Matches(msg, keys.Up) {
			m.activeList().move(-1)
		} else {
			m.activeList().move(1)
		}
		m.refreshDetail()
		return m, nil
	}
	if m.focus == focusDetail {
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return m, cmd
	}
	return m, nil
}

// setGraph swaps the snapshot and rebuilds derived state.
func (m *Model) setGraph(g *graph.Graph) {
	m.g = g
	m.onCritical = map[string]bool{}
	for _, n := range g.CriticalPath() {
		m.onCritical[n.Item.ID] = true
	}
	m.quarantined = len(g.Quarantined()) + len(g.Broken)
	m.rebuildRows()
}

// rebuildRows regenerates every tab list, keeping each cursor on its current
// selection when that row still exists and is selectable.
func (m *Model) rebuildRows() {
	lists := []*cursorList{&m.issues.list, &m.graphTab.list, &m.queue.list}
	keep := make([]string, len(lists))
	for i, l := range lists {
		if r, ok := l.selected(); ok {
			keep[i] = r.id
		}
	}
	rows := m.skeletonRows()
	for i, l := range lists {
		l.setRows(rows, keep[i])
	}
	m.refreshDetail()
}

// skeletonRows lists every node in ID order via Ops.Line; quarantined nodes
// stay visible but unselectable. WI-5..WI-7 replace this per tab.
func (m *Model) skeletonRows() []row {
	if m.g == nil {
		return nil
	}
	var rows []row
	for _, n := range m.g.Order {
		rows = append(rows, row{id: n.Item.ID, text: m.ops.Line(n), selectable: !n.Quarantined()})
	}
	return rows
}

func (m *Model) refreshDetail() {
	id := m.selectedID()
	if id == "" || m.g == nil {
		m.detail.SetContent("no selectable item")
		m.detail.GotoTop()
		return
	}
	m.detail.SetContent(m.ops.Detail(m.g, id))
	m.detail.GotoTop()
}

// reload rebuilds the snapshot; on error the toast reports it and the old
// graph (and selections) stay.
func (m *Model) reload() {
	g, err := m.ops.Load()
	if err != nil {
		m.toast = err.Error()
		return
	}
	m.setGraph(g)
}

// act runs fn; on error the toast shows the message, otherwise the toast
// shows ok and the snapshot reloads.
func (m *Model) act(fn func() error, ok string) {
	if err := fn(); err != nil {
		m.toast = err.Error()
		return
	}
	m.toast = ok
	m.reload()
}

// selectedID is the active tab's selectable cursor row id, "" otherwise.
func (m *Model) selectedID() string {
	if r, ok := m.activeList().selected(); ok {
		return r.id
	}
	return ""
}

func (m *Model) activeList() *cursorList {
	switch m.tab {
	case tabGraph:
		return &m.graphTab.list
	case tabQueue:
		return &m.queue.list
	default:
		return &m.issues.list
	}
}

// layout sizes the lists and the detail viewport for the current window.
func (m *Model) layout() {
	if m.width <= 0 {
		m.width = 80
	}
	if m.height <= 0 {
		m.height = 24
	}
	body := m.height - 2 - m.footerLines()
	if body < 1 {
		body = 1
	}
	for _, l := range []*cursorList{&m.issues.list, &m.graphTab.list, &m.queue.list} {
		l.height = body
	}
	_, detailW := m.columns()
	m.detail.Width = detailW
	m.detail.Height = body
}

func (m *Model) footerLines() int {
	n := 1 // hints
	if m.quarantined > 0 {
		n++
	}
	if m.tab == tabQueue {
		n++
	}
	if m.toast != "" {
		n++
	}
	return n
}

// columns splits the width into list and detail panes. Narrow terminals show
// only the focused pane at full width.
func (m *Model) columns() (left, detail int) {
	if m.width < 80 {
		return m.width, m.width
	}
	left = m.width * 45 / 100
	if left < 30 {
		left = 30
	}
	if left > m.width-10 {
		left = m.width - 10
	}
	return left, m.width - left - 3
}
