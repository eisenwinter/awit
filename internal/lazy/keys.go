package lazy

import (
	"github.com/charmbracelet/bubbles/key"
)

type keymap struct {
	Tab1, Tab2, Tab3, Tab4, Up, Down, Left, Right, Tab, Enter, Edit, Esc, Search, Source,
	Close, Block, Unblock, Comment, Claim, Release, Reload, External, Validate, Help, Quit key.Binding
}

// keys is the lazygit-style keymap. Tabs switch panes, j/k and arrows move,
// h/l and tab shift focus, 4 opens config, e edits a config value, c/b/u/m
// mutate through Ops, space/r drive the queue, R reloads, P/V inspect,
// ? helps, q quits.
var keys = keymap{
	Tab1:     key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "work items tab")),
	Tab2:     key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "graph tab")),
	Tab3:     key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "queue tab")),
	Tab4:     key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "config tab")),
	Up:       key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k", "up")),
	Down:     key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j", "down")),
	Left:     key.NewBinding(key.WithKeys("h", "left"), key.WithHelp("h", "focus list")),
	Right:    key.NewBinding(key.WithKeys("l", "right"), key.WithHelp("l", "focus detail")),
	Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "toggle focus")),
	Enter:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "detail")),
	Edit:     key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
	Esc:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Search:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Source:   key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open/archive")),
	Close:    key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "close")),
	Block:    key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "block")),
	Unblock:  key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "unblock")),
	Comment:  key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "comment")),
	Claim:    key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "claim")),
	Release:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "release")),
	Reload:   key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "reload")),
	External: key.NewBinding(key.WithKeys("P"), key.WithHelp("P", "external check")),
	Validate: key.NewBinding(key.WithKeys("V"), key.WithHelp("V", "validate")),
	Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}
