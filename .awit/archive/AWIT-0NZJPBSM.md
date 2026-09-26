---
id: AWIT-0NZJPBSM
title: 'lazy: skeleton — deps, Ops, cursorList, keymap, root Model, view frame, fatal screen'
brief: >-
  Adds the charm v1 dependencies and the internal/lazy package with the Ops seam, a hand-rolled selectable list, the lazygit keymap, the single root Model with tab/focus/mode state, reload, toast, the plain-text layout and the missing-.awit screen, all covered by TTY-free tests.
status: closed
deps: []
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---
## Summary

Foundation for `awit lazy-human`: `go.mod` gains bubbletea/bubbles/lipgloss (v1 line), and `internal/lazy` gets everything the three tabs will plug into — the `Ops` interface (implemented by `internal/cli` in WI-9), `cursorList`, the keymap, the root `Model` (tabs, focus, modes, reload, toast, `act` helper), the frame renderer, the help screen and the fatal screen. Tab contents are placeholders until WI-5..7. Tests use a fake `Ops` over `graph.Build` of in-memory items and compare `View()` against goldens under `internal/lazy/testdata/`.

## Context (read first)

- `docs/superpowers/specs/2026-09-24-lazy-human-design.md` — Layout, Keymap, Architecture, Error handling, Testing.
- `docs/superpowers/specs/2026-09-24-lazy-human-plan.md` §D.2 (Ops), §D.6 (Model), §D.7 (cursorList), §D.8 (keymap table), §D.13 (view layout), §J (risks).
- `docs/design-spec.md` §7 (package layout; `internal/lazy` is new and may import `pkg/*` and the charm modules only — never `internal/cli`).
- `internal/cli/quarantine_warn.go:25` — footer text `warning: %d items quarantined, run awit validate`.
- Bubble Tea v1: `tea.Model{Init() tea.Cmd; Update(tea.Msg) (tea.Model, tea.Cmd); View() string}`, `tea.KeyMsg` (`.String()` gives "j", "enter", "esc", "tab", " ", "ctrl+c", "up", "down", "pgup", "pgdown"), `tea.WindowSizeMsg`, `tea.Quit`. `bubbles/key`: `key.NewBinding(key.WithKeys(...), key.WithHelp(...))`, `key.Matches(msg, b)`. `bubbles/viewport`: `viewport.New(w, h)`, `SetContent`, `GotoTop`, `Update`, `View`, fields `Width`, `Height`. `bubbles/textinput`: `textinput.New()`, `Focus()`, `Blur()`, `SetValue`, `Value`, `Reset`, `Update`, `View`, `Prompt`.
- lipgloss under `go test` (stdout not a TTY) renders the Ascii profile: `Bold`/`Reverse` styles produce no escape codes, so goldens are plain text. `TestMain` also sets `NO_COLOR=1` for safety and every golden test asserts no `\x1b` byte.

## Files

- `go.mod`, `go.sum` — `go get github.com/charmbracelet/bubbletea@v1.3.10 github.com/charmbracelet/bubbles@v1.0.0 github.com/charmbracelet/lipgloss@v1.1.0`; then `go mod tidy`.
- `internal/lazy/ops.go` — `Ops`.
- `internal/lazy/list.go` — `row`, `cursorList`.
- `internal/lazy/keys.go` — `keymap`, `keys`.
- `internal/lazy/model.go` — `Model`, `Options`, `New`, `Init`, `Update`, `reload`, `act`, `selectedID`.
- `internal/lazy/view.go` — `View`, header/footer/help/fatal/hints renderers.
- `internal/lazy/lazy_test.go` — `TestMain`, `fakeOps`, `newFixture`, `press`, `golden`, `-update` flag.
- `internal/lazy/list_test.go`, `keys_test.go`, `view_test.go`, `internal/lazy/testdata/*.golden`.

## Interfaces

```go
// ops.go
package lazy

// Ops is everything the TUI needs from the store. internal/cli implements it
// with the same helpers the CLI commands use, so every row, detail and byte
// written matches the corresponding command. The TUI never pushes external
// state and never git-commits.
type Ops interface {
	Load() (*graph.Graph, error)
	LoadArchive() ([]*item.Item, error)
	Line(n *graph.Node) string
	ArchiveLine(it *item.Item) string
	Detail(g *graph.Graph, id string) string
	ArchiveDetail(id string) (string, error)
	Close(id, reason string) error
	Block(id, reason string) error
	Unblock(id string) error
	Comment(id, text string) error
	Claim(id string) error
	Release(id string) error
	Validate(g *graph.Graph) string
	ExternalCheck(ctx context.Context, id string) string
}

// list.go
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
func (l *cursorList) setRows(rows []row, keepID string) // cursor on keepID if selectable, else first selectable, else -1; offset clamped
func (l *cursorList) move(delta int)                   // next selectable row in that direction; no wrap; keeps cursor visible
func (l *cursorList) selected() (row, bool)
func (l *cursorList) view(width int) string            // exactly l.height lines: "> "+text for cursor, "  "+text otherwise, rune-truncated to width, padded with blank lines

// keys.go
type keymap struct {
	Tab1, Tab2, Tab3, Up, Down, Left, Right, Tab, Enter, Esc, Search, Source,
	Close, Block, Unblock, Comment, Claim, Release, Reload, External, Validate, Help, Quit key.Binding
}
var keys = keymap{ /* 1,2,3 | k/up | j/down | h/left | l/right | tab | enter | esc | / | o | c | b | u | m | " " | r | R | P | V | ? | q,ctrl+c */ }

// model.go
type tab int   // tabIssues = 0, tabGraph, tabQueue
type focus int // focusList, focusDetail
type mode int  // modeNormal, modeSearch, modeInput, modeHelp
type inputKind int // inputClose, inputBlock, inputComment

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
	issues        issuesState  // WI-5; in WI-4 a struct holding only `list cursorList`
	graphTab      graphState   // WI-6; in WI-4 `list cursorList`
	queue         queueState   // WI-7; in WI-4 `list cursorList`
	detail        viewport.Model
	input         textinput.Model
	inputKind     inputKind
	inputTarget   string
	toast         string
}
type Options struct{ Fatal string }
func New(ops Ops, ctx context.Context, opts Options) Model // opts.Fatal != "" skips Load; else Load, on error fatal = err.Error()
func (m Model) Init() tea.Cmd                             // nil
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd)
func (m Model) View() string
func (m *Model) reload()                                  // Load; error → toast, keep old graph; else rebuild rows keeping selections, recompute onCritical/quarantined, refresh detail
func (m *Model) act(fn func() error, ok string)           // err → toast err.Error(); else toast ok, reload()
func (m *Model) selectedID() string                       // active tab's selectable cursor row id, "" otherwise
func (m *Model) activeList() *cursorList

## Comments

### 2026-09-24T20:23:47Z jan

implemented
