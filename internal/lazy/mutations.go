package lazy

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

type externalMsg struct{ line string }

func (m *Model) openInput(kind inputKind, id string) {
	m.mode = modeInput
	m.inputKind = kind
	m.inputTarget = id
	m.input.Reset()
	m.input.Focus()
}

func (m *Model) submitInput() {
	v := m.input.Value()
	id := m.inputTarget
	switch m.inputKind {
	case inputClose:
		m.act(func() error { return m.ops.Close(id, v) }, "closed "+id)
	case inputBlock:
		if v == "" {
			m.toast = "block requires a non-empty reason"
			return
		}
		m.act(func() error { return m.ops.Block(id, v) }, "blocked "+id+": "+v)
	case inputComment:
		if v == "" {
			m.toast = "empty comment"
			return
		}
		m.act(func() error { return m.ops.Comment(id, v) }, "commented "+id)
	}
}

func (m *Model) externalCmd(id string) tea.Cmd {
	return func() tea.Msg {
		return externalMsg{m.ops.ExternalCheck(m.ctx, id)}
	}
}

// mutationID is the selected item for a write, or ("", false) after a refusal toast.
func (m *Model) mutationID() (string, bool) {
	if m.tab == tabIssues && m.issues.showArchive {
		m.toast = "archived items are read-only"
		return "", false
	}
	id := m.selectedID()
	if id == "" {
		m.toast = "no item selected"
		return "", false
	}
	return id, true
}

func (m *Model) beginInput(kind inputKind) {
	id, ok := m.mutationID()
	if !ok {
		return
	}
	m.openInput(kind, id)
}

func (m *Model) unblockSelected() {
	id, ok := m.mutationID()
	if !ok {
		return
	}
	m.act(func() error { return m.ops.Unblock(id) }, "unblocked "+id)
}

func (m *Model) startExternalCheck() tea.Cmd {
	id, ok := m.mutationID()
	if !ok {
		return nil
	}
	return m.externalCmd(id)
}

func (m *Model) showValidate() {
	if m.g == nil {
		return
	}
	m.detail.SetContent(m.ops.Validate(m.g))
	m.detail.GotoTop()
	m.focus = focusDetail
}

func (m *Model) mutationKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	if m.tab == tabConfig {
		return nil, false
	}
	switch {
	case key.Matches(msg, keys.Close):
		m.beginInput(inputClose)
		if m.mode == modeInput {
			return m.input.Focus(), true
		}
		return nil, true
	case key.Matches(msg, keys.Block):
		m.beginInput(inputBlock)
		if m.mode == modeInput {
			return m.input.Focus(), true
		}
		return nil, true
	case key.Matches(msg, keys.Comment):
		m.beginInput(inputComment)
		if m.mode == modeInput {
			return m.input.Focus(), true
		}
		return nil, true
	case key.Matches(msg, keys.Unblock):
		m.unblockSelected()
		return nil, true
	case key.Matches(msg, keys.External):
		return m.startExternalCheck(), true
	case key.Matches(msg, keys.Validate):
		m.showValidate()
		return nil, true
	}
	return nil, false
}
