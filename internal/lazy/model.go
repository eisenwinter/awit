package lazy

import (
	"context"
	"strings"

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

// queueState is the Queue tab state. graphState (Graph tab) lives in
// graph.go; issuesState in issues.go.
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
	case externalMsg:
		m.toast = msg.line
		return m, nil
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
			if m.mode == modeSearch {
				m.submitSearch()
			} else {
				m.submitInput()
			}
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
	if cmd, ok := m.mutationKey(msg); ok {
		return m, cmd
	}
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Tab1):
		m.tab = tabIssues
		m.refreshDetail()
		return m, nil
	case key.Matches(msg, keys.Tab2):
		m.enterGraphTab()
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
	case key.Matches(msg, keys.Search):
		if m.tab == tabIssues {
			m.mode = modeSearch
			m.input.Reset()
			return m, m.input.Focus()
		}
		return m, nil
	case key.Matches(msg, keys.Source):
		if m.tab == tabIssues {
			m.toggleArchive()
		}
		return m, nil
	case key.Matches(msg, keys.Claim):
		if m.tab == tabQueue {
			m.claimSelected()
		}
		return m, nil
	case key.Matches(msg, keys.Release):
		if m.tab == tabQueue {
			m.releaseSelected()
		}
		return m, nil
	case key.Matches(msg, keys.Esc), key.Matches(msg, keys.Left):
		m.focus = focusList
		return m, nil
	case key.Matches(msg, keys.Tab):
		if m.tab == tabGraph {
			m.toggleGraphMode()
			return m, nil
		}
		m.focus = focusDetail
		return m, nil
	case key.Matches(msg, keys.Enter):
		if m.tab == tabGraph {
			m.jumpToIssues()
			return m, nil
		}
		m.focus = focusDetail
		return m, nil
	case key.Matches(msg, keys.Right):
		if m.tab == tabGraph {
			return m, nil // no detail pane to focus
		}
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
	var keepIssues, keepGraph, keepQueue string
	if r, ok := m.issues.list.selected(); ok {
		keepIssues = r.id
	}
	if r, ok := m.graphTab.list.selected(); ok {
		keepGraph = r.id
	}
	if r, ok := m.queue.list.selected(); ok {
		keepQueue = r.id
	}
	m.issues.list.setRows(m.issuesRows(), keepIssues)
	m.graphTab.list.setRows(m.graphRows(), keepGraph)
	m.queue.list.setRows(queueRows(m.g, m.ops.Line), keepQueue)
	m.refreshDetail()
}

func (m *Model) refreshDetail() {
	var content string
	switch m.tab {
	case tabIssues:
		content = m.issuesDetail()
	default:
		id := m.selectedID()
		if id == "" || m.g == nil {
			content = "no selectable item"
		} else {
			content = m.ops.Detail(m.g, id)
		}
	}
	m.detail.SetContent(content)
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
	if m.archiveLoaded {
		items, err := m.ops.LoadArchive()
		if err != nil {
			m.toast = err.Error()
		} else {
			m.archive = items
			m.issues.archiveN = len(items)
		}
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

func (m *Model) selectedID() string {
	if m.tab == tabIssues && m.issues.showArchive {
		return ""
	}
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

// OverviewText is a test hook: the Graph-tab overview rows (the prime
// snapshot verbatim) joined with newlines plus a trailing newline, or ""
// when there are no rows.
func OverviewText(m Model) string {
	if m.g == nil {
		return ""
	}
	rows := overviewRows(m.g)
	if len(rows) == 0 {
		return ""
	}
	texts := make([]string, 0, len(rows))
	for _, r := range rows {
		texts = append(texts, r.text)
	}
	return strings.Join(texts, "\n") + "\n"
}

// QueueIDs is a test hook: the ids of the selectable Queue-tab rows in
// ready order.
func QueueIDs(m Model) []string {
	if m.g == nil || m.ops == nil {
		return nil
	}
	var ids []string
	for _, r := range queueRows(m.g, m.ops.Line) {
		if r.selectable {
			ids = append(ids, r.id)
		}
	}
	return ids
}
