package lazy

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/eisenwinter/awit/pkg/config"
)

func configRowTexts(m Model) []string {
	var out []string
	for _, r := range m.config.list.rows {
		if !r.selectable || r.id == "" {
			panic("config row must be selectable with a key id")
		}
		out = append(out, r.text)
	}
	return out
}

func TestConfigTabRows(t *testing.T) {
	t.Parallel()
	m, _ := press(newModel(t, newFixture()), "4")
	if m.tab != tabConfig || m.focus != focusList {
		t.Fatalf("tab=%v focus=%v", m.tab, m.focus)
	}
	want := []string{
		"prefix         AWIT",
		"default_labels (unset)",
		"labels         auth, db",
		"stale_claim    2h",
		"agent_id       (unset)",
		"commit         (unset)",
		"external_push  (unset)",
		"template       .awit/templates/workitem.md",
	}
	if got := configRowTexts(m); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("rows =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if !contains(m.View(), ".awit/config.yaml") || !contains(m.View(), "[4] Config") {
		t.Fatalf("header:\n%s", m.View())
	}
	golden(t, "frame-config", m.View())
}

func TestConfigDetail(t *testing.T) {
	t.Parallel()
	m, _ := press(newModel(t, newFixture()), "4")
	v := m.View()
	for _, s := range []string{"required. Id prefix for new items", "current: AWIT", "saving rewrites .awit/config.yaml"} {
		if !contains(v, s) {
			t.Fatalf("prefix detail missing %q:\n%s", s, v)
		}
	}
	m, _ = press(m, "j", "j", "j", "j", "j")
	v = m.View()
	for _, s := range []string{"whether awit next --claim git-commits", "default: true (unset)", "current: (unset)"} {
		if !contains(v, s) {
			t.Fatalf("commit detail missing %q:\n%s", s, v)
		}
	}
	if m.selectedID() != "" {
		t.Fatalf("selectedID on config tab = %q, want empty", m.selectedID())
	}
}

func TestConfigMutationKeysInert(t *testing.T) {
	t.Parallel()
	f := newFixture()
	m, _ := press(newModel(t, f), "4")
	before := m.View()
	m, _ = press(m, "c", "b", "u", "m", " ", "r", "P", "V", "/", "o")
	if len(f.calls) != 0 || m.mode != modeNormal || m.View() != before {
		t.Fatalf("config tab reacted: calls=%v mode=%v\n%s", f.calls, m.mode, m.View())
	}
}

func TestConfigReload(t *testing.T) {
	t.Parallel()
	f := newFixture()
	m, _ := press(newModel(t, f), "4", "j", "j", "j")
	f.cfg.AgentID = "claude"
	f.cfg.StaleClaim = config.Duration(90 * time.Minute)
	m, _ = press(m, "R")
	rows := configRowTexts(m)
	if rows[3] != "stale_claim    90m" || rows[4] != "agent_id       claude" {
		t.Fatalf("rows after R = %v", rows)
	}
	if r, _ := m.config.list.selected(); r.id != "stale_claim" {
		t.Fatalf("selection after R = %q", r.id)
	}
	f.cfgErr = errors.New("config: prefix is required")
	m, _ = press(m, "R")
	if m.toast != "config: prefix is required" || configRowTexts(m)[4] != "agent_id       claude" {
		t.Fatalf("failed reload: toast=%q rows=%v", m.toast, configRowTexts(m))
	}
}

func TestNewFatalOnConfigError(t *testing.T) {
	t.Parallel()
	f := newFixture()
	f.cfgErr = errors.New("config: prefix is required")
	m := New(f, context.Background(), Options{})
	um, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if got := um.(Model).View(); !strings.Contains(got, "config: prefix is required") || !strings.Contains(got, "press q to quit") {
		t.Fatalf("fatal view:\n%s", got)
	}
}
