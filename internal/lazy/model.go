package lazy

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/eisenwinter/awit/pkg/config"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

type tab int

const (
	tabIssues tab = iota
	tabGraph
	tabQueue
	tabConfig
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
	inputConfig
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
	// archiveLoading tracks an in-flight LoadArchive; reloadLoading an
	// in-flight reload. Either one renders the shared loading… status line
	// (same toast slot, so the frame height never moves) until its result
	// message arrives. No timers, no overlays: just state-rendered lines.
	archiveLoading bool
	reloadLoading  bool
	// reloadGen is the generation of the latest dispatched reload;
	// reloadMsg carries it so stale (slower-first) results drop instead of
	// overwriting a newer snapshot.
	reloadGen int
	// pendingArchiveIdx is the open-list cursor to restore when an async
	// archive load lands (sync toggleArchive kept idx across the swap).
	pendingArchiveIdx int
	tab               tab
	focus             focus
	mode              mode
	issues            issuesState
	graphTab          graphState
	queue             queueState
	config            configState
	detail            viewport.Model
	detailRaw         string // unwrapped detail source; wrapped to detail.Width on set/resize
	input             textinput.Model
	inputKind         inputKind
	inputTarget       string
	toast             string
}

type Options struct{ Fatal string }

// New builds the root model. opts.Fatal != "" skips Load and renders the
// missing-.awit screen; otherwise Load runs once and a load error becomes
// the fatal screen.
// The initial load stays blocking by design (AWIT-0P21ZYSM): moving it into
// Init would restructure the init flow and every newModel test setup for no
// visible win on tiny repos. Async tea.Cmd loads cover the interactive
// paths (archive toggle, R reload) where input must never block on file I/O.
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
	c, err := ops.Config()
	if err != nil {
		m.fatal = err.Error()
		return m
	}
	m.config.cfg = c
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
	case archiveMsg:
		m.applyArchiveMsg(msg)
		return m, nil
	case reloadMsg:
		m.applyReloadMsg(msg)
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
			var cmd tea.Cmd
			if m.mode == modeSearch {
				m.submitSearch()
			} else {
				cmd = m.submitInput()
			}
			m.mode = modeNormal
			m.input.Blur()
			m.input.Reset()
			return m, cmd
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	// Toasts persist without timers: success clears on the next key, while
	// error: toasts stick until esc, a tab switch, or the next outcome.
	if isErrToast(m.toast) {
		if key.Matches(msg, keys.Esc, keys.Tab1, keys.Tab2, keys.Tab3, keys.Tab4) {
			m.toast = ""
		}
	} else {
		m.toast = ""
	}
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
	case key.Matches(msg, keys.Tab4):
		m.tab = tabConfig
		m.focus = focusList
		m.refreshDetail()
		return m, nil
	case key.Matches(msg, keys.Help):
		m.mode = modeHelp
		return m, nil
	case key.Matches(msg, keys.Reload):
		return m, m.reload()
	case key.Matches(msg, keys.Search):
		if m.tab == tabIssues {
			m.mode = modeSearch
			m.input.Reset()
			return m, m.input.Focus()
		}
		return m, nil
	case key.Matches(msg, keys.Source):
		if m.tab == tabIssues {
			return m, m.toggleArchive()
		}
		return m, nil
	case key.Matches(msg, keys.Claim):
		if m.tab == tabQueue {
			return m, m.claimSelected()
		}
		return m, nil
	case key.Matches(msg, keys.Release):
		if m.tab == tabQueue {
			return m, m.releaseSelected()
		}
		return m, nil
	case m.tab == tabConfig && (key.Matches(msg, keys.Edit) || key.Matches(msg, keys.Enter)):
		m.beginConfigEdit()
		return m, m.input.Focus()
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
	var keepConfig string
	if r, ok := m.config.list.selected(); ok {
		keepConfig = r.id
	}
	m.issues.list.setRows(m.issuesRows(), keepIssues)
	m.graphTab.list.setRows(m.graphRows(), keepGraph)
	m.queue.list.setRows(queueRows(m.g, m.ops.Line), keepQueue)
	m.config.list.setRows(configRows(m.config.cfg), keepConfig)
	m.refreshDetail()
}

func (m *Model) refreshDetail() {
	var content string
	switch m.tab {
	case tabIssues:
		content = m.issuesDetail()
	case tabConfig:
		if r, ok := m.config.list.selected(); ok {
			content = configDetail(m.config.cfg, r.id)
		} else {
			content = "no selectable item"
		}
	default:
		id := m.selectedID()
		if id == "" || m.g == nil {
			content = "no selectable item"
		} else {
			content = m.ops.Detail(m.g, id)
		}
	}
	m.setDetail(content)
	m.detail.GotoTop()
}

// setDetail stores the unwrapped source and wraps it to the current detail
// width; layout re-wraps the stored source on resize. Untrusted content is
// sanitized at this trust boundary, then styled.
func (m *Model) setDetail(content string) {
	m.detailRaw = content
	m.detail.SetContent(styleDetail(wrapDetail(sanitize(content, true), m.detail.Width)))
}

// wrapDetail word-wraps s to width cells without breaking words; long words
// still exceed width and are ellipsis-cut by the pane renderer.
func wrapDetail(s string, width int) string {
	if width < 1 {
		return s
	}
	return ansi.Wordwrap(s, width, " ")
}

// archiveMsg carries an async LoadArchive result. errors keep the open view
// + an error toast (the sync toggleArchive semantics).
type archiveMsg struct {
	items []*item.Item
	err   error
}

// reloadMsg carries an async reload: the fresh graph (or loadErr, which
// keeps the old graph), plus the archive + config refresh the sync reload
// did after a successful Load. archWanted reports whether the archive
// refresh was attempted.
type reloadMsg struct {
	g          *graph.Graph
	loadErr    error
	arch       []*item.Item
	archErr    error
	archWanted bool
	cfg        config.Config
	cfgErr     error
	// gen tags the reload that produced this message; applyReloadMsg drops
	// stale generations so a slower first Load can never overwrite a newer
	// snapshot (double-R / reload-during-pending).
	gen int
}

// isLoading reports an in-flight archive or reload load. The View renders
// the shared loading… status line (same toast slot) while true.
func (m Model) isLoading() bool { return m.archiveLoading || m.reloadLoading }

// statusLine is the toast slot: loading… while a load is in flight,
// otherwise the styled toast (blank when empty). The toast bytes are
// untouched during loading so the outcome (ok/error) reappears after.
func (m Model) statusLine() string {
	if m.isLoading() {
		return "loading…"
	}
	return styleToast(m.toast)
}

// archiveCmd runs LoadArchive off the Update path, mirroring externalCmd.
func (m *Model) archiveCmd() tea.Cmd {
	ops := m.ops
	return func() tea.Msg {
		items, err := ops.LoadArchive()
		return archiveMsg{items: items, err: err}
	}
}

// applyArchiveMsg swaps the loading placeholder for archive rows (or caches
// a background load when the user already toggled back to open).
func (m *Model) applyArchiveMsg(msg archiveMsg) {
	m.archiveLoading = false
	if msg.err != nil {
		m.toast = "error: " + msg.err.Error()
		if !m.issues.showArchive {
			return
		}
		m.issues.showArchive = false
		m.issues.list.setRows(m.issuesRows(), "")
		m.refreshDetail()
		return
	}
	m.archive = msg.items
	m.archiveLoaded = true
	m.issues.archiveN = len(msg.items)
	if !m.issues.showArchive {
		return
	}
	m.issues.list.setRows(m.issuesRows(), "")
	n := len(m.issues.list.rows)
	if n == 0 {
		m.issues.list.cursor = -1
	} else {
		idx := m.pendingArchiveIdx
		if idx < 0 {
			idx = 0
		}
		if idx >= n {
			idx = n - 1
		}
		// setRows lands on the first selectable row; clamp to the saved
		// open cursor so j,o keeps its position like the sync toggle did.
		// Archive rows are all selectable, so idx is directly usable
		// unless the empty placeholder is showing.
		if r := m.issues.list.rows[idx]; r.selectable {
			m.issues.list.cursor = idx
		}
		m.issues.list.clampOffset()
	}
	m.refreshDetail()
}

// reloadCmd runs Load (plus the archive + config refresh the sync reload
// did) off the Update path. A Load error returns early, like sync reload.
func (m *Model) reloadCmd() tea.Cmd {
	ops := m.ops
	wantArchive := m.archiveLoaded || m.archiveLoading
	gen := m.reloadGen
	return func() tea.Msg {
		g, err := ops.Load()
		if err != nil {
			return reloadMsg{loadErr: err, gen: gen}
		}
		var out reloadMsg
		out.g = g
		out.gen = gen
		if wantArchive {
			out.archWanted = true
			out.arch, out.archErr = ops.LoadArchive()
		}
		out.cfg, out.cfgErr = ops.Config()
		return out
	}
}

// applyReloadMsg swaps in the fresh snapshot; on Load error the toast
// reports it and the old graph (and selections) stay.
func (m *Model) applyReloadMsg(msg reloadMsg) {
	if msg.gen != m.reloadGen {
		return
	}
	m.reloadLoading = false
	if msg.loadErr != nil {
		m.toast = "error: " + msg.loadErr.Error()
		return
	}
	if msg.archWanted {
		if msg.archErr != nil {
			m.toast = "error: " + msg.archErr.Error()
		} else {
			m.archive = msg.arch
			m.archiveLoaded = true
			m.issues.archiveN = len(msg.arch)
			m.archiveLoading = false
			// No explicit row swap: setGraph→rebuildRows below re-renders
			// the archive (or open) rows from the new cache while keeping
			// the selection, exactly like the sync reload did.
		}
	}
	if msg.cfgErr != nil {
		m.toast = "error: " + msg.cfgErr.Error()
	} else {
		m.config.cfg = msg.cfg
	}
	m.setGraph(msg.g)
}

// reload starts an async snapshot rebuild with the loading… status; the
// result message swaps the graph (errors keep the old graph + toast).
// Each dispatch bumps reloadGen so a slower first Load drops instead of
// overwriting a newer snapshot.
func (m *Model) reload() tea.Cmd {
	m.reloadGen++
	m.reloadLoading = true
	return m.reloadCmd()
}

// act runs fn; on error the toast shows the message, otherwise the toast
// shows ok and the snapshot reloads async. Failure toasts carry an "error: "
// prefix so they read as failures under NO_COLOR.
func (m *Model) act(fn func() error, ok string) tea.Cmd {
	if err := fn(); err != nil {
		m.toast = "error: " + err.Error()
		return nil
	}
	m.toast = ok
	return m.reload()
}

func (m *Model) selectedID() string {
	if m.tab == tabConfig {
		return ""
	}
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
	case tabConfig:
		return &m.config.list
	default:
		return &m.issues.list
	}
}

// layout sizes the lists and the detail viewport for the current window.
// Panes render inside rounded borders, so content dims are the inner values
// (outer minus the two border cells); columns keeps returning outer widths.
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
	innerH := body - 2
	if innerH < 1 {
		innerH = 1
	}
	for _, l := range []*cursorList{&m.issues.list, &m.graphTab.list, &m.queue.list, &m.config.list} {
		l.height = innerH
	}
	_, detailOuter := m.columns()
	detailW := detailOuter - 2
	if detailW < 1 {
		detailW = 1
	}
	m.detail.Width = detailW
	m.detail.Height = innerH
	m.detail.SetContent(styleDetail(wrapDetail(sanitize(m.detailRaw, true), detailW)))
}

// footerLines reserves fixed footer slots so the body height never moves
// when the toast toggles: the hints/prompt line, the always-rendered toast
// line (blank when empty), the quarantine line when items are quarantined,
// and the why line on the Queue tab (always rendered there).
func (m *Model) footerLines() int {
	n := 2 // hints + toast slot
	if m.quarantined > 0 {
		n++
	}
	if m.tab == tabQueue {
		n++
	}
	return n
}

// columns splits the width into list and detail outer panes at 45/55. The
// borders join with no gap, so detail is the full remainder. Narrow
// terminals show only the focused pane at full width.
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
	return left, m.width - left
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
