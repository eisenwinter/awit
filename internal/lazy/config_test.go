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

func TestConfigEditPrefillsAndEscDiscards(t *testing.T) {
	t.Parallel()
	f := newFixture()
	m, _ := press(newModel(t, f), "4", "e")
	if m.mode != modeInput || m.inputKind != inputConfig || m.inputTarget != "prefix" || m.input.Value() != "AWIT" {
		t.Fatalf("mode=%v kind=%v target=%q value=%q", m.mode, m.inputKind, m.inputTarget, m.input.Value())
	}
	if !contains(m.View(), "prefix: > AWIT") {
		t.Fatalf("prompt:\n%s", m.View())
	}
	golden(t, "prompt_config", m.View())
	m, _ = press(m, "X", "esc")
	if m.mode != modeNormal || countCalls(f, "SaveConfig") != 0 || configRowTexts(m)[0] != "prefix         AWIT" {
		t.Fatalf("esc: mode=%v calls=%v rows=%v", m.mode, f.calls, configRowTexts(m))
	}
	m, _ = press(m, "j", "j", "j", "enter")
	if m.mode != modeInput || m.inputTarget != "stale_claim" || m.input.Value() != "2h" {
		t.Fatalf("enter edits: target=%q value=%q", m.inputTarget, m.input.Value())
	}
}

func TestConfigSaveValid(t *testing.T) {
	t.Parallel()
	f := newFixture()
	m, _ := press(newModel(t, f), "4", "j", "j", "j", "e", "backspace", "backspace", "9", "0", "m", "enter")
	if countCalls(f, "SaveConfig") != 1 || m.toast != "saved stale_claim" || m.mode != modeNormal {
		t.Fatalf("calls=%v toast=%q mode=%v", f.calls, m.toast, m.mode)
	}
	if got := time.Duration(f.saved[0].StaleClaim); got != 90*time.Minute {
		t.Fatalf("saved stale_claim = %s", got)
	}
	if f.saved[0].Prefix != "AWIT" || strings.Join(f.saved[0].Labels, ",") != "auth,db" {
		t.Fatalf("other fields changed: %+v", f.saved[0])
	}
	if configRowTexts(m)[3] != "stale_claim    90m" {
		t.Fatalf("rows = %v", configRowTexts(m))
	}
	if r, _ := m.config.list.selected(); r.id != "stale_claim" {
		t.Fatalf("selection = %q", r.id)
	}
}

func TestConfigRefusesInvalid(t *testing.T) {
	t.Parallel()
	cases := []struct {
		row    int
		key    string
		typed  string
		toast  string
		direct bool // feed saveConfigField, skipping the prompt
	}{
		{0, "prefix", "awit", "prefix must be 2-8 uppercase alphanumerics starting with a letter", false},
		{0, "prefix", "", "prefix must be 2-8 uppercase alphanumerics starting with a letter", false},
		{3, "stale_claim", "banana", `time: invalid duration "banana"`, false},
		{7, "template", "../x.md", "config: template escapes repository root", false},
		{7, "template", "/etc/x.md", "config: template must be a repo-root-relative path", false},
		// The textinput sanitizer replaces tabs/newlines with spaces and
		// drops other control characters, so a control char can never
		// reach setConfigField through the prompt; exercise the rule at
		// the layer where it is reachable.
		{2, "labels", "a\tb", "config: labels entry must not contain control characters", true},
		{5, "commit", "maybe", "commit must be true, false, or empty", false},
		{6, "external_push", "yes", "external_push must be true, false, or empty", false},
	}
	for _, c := range cases {
		t.Run(c.key+"="+c.typed, func(t *testing.T) {
			f := newFixture()
			m, _ := press(newModel(t, f), "4")
			for range c.row {
				m, _ = press(m, "j")
			}
			if c.direct {
				m.saveConfigField(c.key, c.typed)
			} else {
				m, _ = press(m, "e")
				m.input.SetValue(c.typed)
				m, _ = press(m, "enter")
			}
			if m.toast != c.toast {
				t.Fatalf("toast = %q, want %q", m.toast, c.toast)
			}
			if countCalls(f, "SaveConfig") != 0 {
				t.Fatalf("wrote: %v", f.calls)
			}
			if r, _ := m.config.list.selected(); r.id != c.key {
				t.Fatalf("selection = %q, want %q", r.id, c.key)
			}
			if m.config.cfg.Prefix != "AWIT" || m.config.cfg.Template != ".awit/templates/workitem.md" || strings.Join(m.config.cfg.Labels, ",") != "auth,db" {
				t.Fatalf("in-memory config changed: %+v", m.config.cfg)
			}
		})
	}
}

func TestConfigSaveErrorToast(t *testing.T) {
	t.Parallel()
	f := newFixture()
	f.fail = errTest
	m, _ := press(newModel(t, f), "4", "j", "j", "j", "j", "e", "x", "enter")
	if m.toast != "error: boom" || countCalls(f, "SaveConfig") != 1 || len(f.saved) != 0 {
		t.Fatalf("toast=%q calls=%v saved=%d", m.toast, f.calls, len(f.saved))
	}
	if r, _ := m.config.list.selected(); r.id != "agent_id" {
		t.Fatalf("selection = %q", r.id)
	}
	if configRowTexts(m)[4] != "agent_id       (unset)" {
		t.Fatalf("rows changed on failed save: %v", configRowTexts(m))
	}
}

func TestConfigListsBoolsAndEmpties(t *testing.T) {
	t.Parallel()
	edit := func(t *testing.T, row int, typed string) config.Config {
		t.Helper()
		f := newFixture()
		m, _ := press(newModel(t, f), "4")
		for range row {
			m, _ = press(m, "j")
		}
		m, _ = press(m, "e")
		m.input.SetValue(typed)
		m, _ = press(m, "enter")
		if len(f.saved) != 1 {
			t.Fatalf("saved %d times, toast %q", len(f.saved), m.toast)
		}
		return f.saved[0]
	}
	if got := edit(t, 2, "a, b, ,a"); strings.Join(got.Labels, ",") != "a,b" {
		t.Fatalf("labels = %v", got.Labels)
	}
	if got := edit(t, 1, "p1, p2"); strings.Join(got.DefaultLabels, ",") != "p1,p2" {
		t.Fatalf("default_labels = %v", got.DefaultLabels)
	}
	if got := edit(t, 1, ""); got.DefaultLabels != nil {
		t.Fatalf("default_labels empty = %v", got.DefaultLabels)
	}
	if got := edit(t, 2, ""); got.Labels != nil {
		t.Fatalf("labels empty = %v", got.Labels)
	}
	if got := edit(t, 5, "false"); got.Commit == nil || *got.Commit {
		t.Fatalf("commit false = %v", got.Commit)
	}
	if got := edit(t, 6, "true"); got.ExternalPush == nil || !*got.ExternalPush {
		t.Fatalf("external_push true = %v", got.ExternalPush)
	}
	if got := edit(t, 5, ""); got.Commit != nil {
		t.Fatalf("commit unset = %v", got.Commit)
	}
	if got := edit(t, 3, ""); time.Duration(got.StaleClaim) != 2*time.Hour {
		t.Fatalf("stale_claim empty = %s", time.Duration(got.StaleClaim))
	}
	if got := edit(t, 7, ""); got.Template != "" {
		t.Fatalf("template empty = %q", got.Template)
	}
	if got := edit(t, 4, "agent/claude"); got.AgentID != "agent/claude" {
		t.Fatalf("agent_id = %q", got.AgentID)
	}
}

func TestEditKeyIgnoredElsewhere(t *testing.T) {
	t.Parallel()
	f := newFixture()
	m, _ := press(newModel(t, f), "e")
	if m.mode != modeNormal || len(f.calls) != 0 {
		t.Fatalf("e on issues tab: mode=%v calls=%v", m.mode, f.calls)
	}
}
