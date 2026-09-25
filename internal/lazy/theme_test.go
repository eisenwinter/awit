package lazy

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// (slots, styles, sanitize, styleToast/styleRow/styleDetail, bordered panes,
// sticky error toasts) which does not exist yet — this file must FAIL to
// compile/run before the implementation lands.

// Hostile control sequences must not survive the lazy trust boundary.
func TestThemeSanitizeStripsANSI(t *testing.T) {
	t.Parallel()
	got := sanitize("\x1b[31mhi\x1b[0m", false)
	if got != "hi" {
		t.Fatalf("sanitize CSI = %q, want %q", got, "hi")
	}
	got = sanitize("a\x1b]0;owned\x07b", false)
	if got != "ab" {
		t.Fatalf("sanitize OSC = %q, want %q", got, "ab")
	}
	got = sanitize("one\ntwo", false)
	if strings.Contains(got, "\n") {
		t.Fatalf("single-line sanitize kept newline: %q", got)
	}
	got = sanitize("one\ntwo", true)
	if got != "one\ntwo" {
		t.Fatalf("multiline sanitize = %q, want newline kept", got)
	}
}

// Under the NO_COLOR test profile every style helper is plain-text identity:
// no SGR bytes, output equal to input.
func TestThemeStylesPlainFallback(t *testing.T) {
	t.Parallel()
	for _, s := range []string{
		styleToast("hello"),
		styleToast("error: boom"),
		styleRow("[AWIT-1] open Title here", "open"),
		styleDetail("[AWIT-1] Title\nstatus: open\n\n## Summary\n"),
		styleHints("j/k move  enter detail"),
	} {
		if strings.ContainsRune(s, '\x1b') {
			t.Fatalf("styled output contains escape byte: %q", s)
		}
	}
	if got := styleToast("hello"); got != "ok: hello" {
		t.Fatalf("styleToast(ok) = %q, want ok: prefix", got)
	}
	if got := styleToast("error: boom"); got != "error: boom" {
		t.Fatalf("styleToast(err) = %q, want unchanged", got)
	}
}

// Bordered panes: rounded-light chrome, joined no-gap, every line fits.
func TestThemeBorderGeometry(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())
	got := m.View()
	if strings.ContainsRune(got, '\x1b') {
		t.Fatal("bordered view contains escape byte")
	}
	for _, glyph := range []string{"╭", "╮", "╰", "╯"} {
		if !strings.Contains(got, glyph) {
			t.Fatalf("bordered view missing %q:\n%s", glyph, got)
		}
	}
	for _, ln := range strings.Split(got, "\n") {
		if w := lipgloss.Width(ln); w > 100 {
			t.Fatalf("line width %d > 100: %q", w, ln)
		}
	}
}

// F2: error toasts stick until esc; success toasts clear on any key.
func TestThemeStickyErrorToast(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())
	m.toast = "error: boom"
	m, _ = press(m, "j")
	if m.toast != "error: boom" {
		t.Fatalf("error toast after j = %q, want sticky", m.toast)
	}
	m, _ = press(m, "esc")
	if m.toast != "" {
		t.Fatalf("error toast after esc = %q, want cleared", m.toast)
	}
	m.toast = "error: boom"
	m, _ = press(m, "2")
	if m.toast != "" {
		t.Fatalf("error toast after tab switch = %q, want cleared", m.toast)
	}
	m.toast = "claimed AWIT-LAZY0001"
	m, _ = press(m, "j")
	if m.toast != "" {
		t.Fatalf("ok toast after j = %q, want cleared", m.toast)
	}
}

// Wide-cell rows keep the joined borders aligned: every emitted line still
// fits the window (cell math, not runes).
func TestThemeCJKBorderAligned(t *testing.T) {
	t.Parallel()
	f := newFixture()
	f.items["AWIT-LAZY0001"].Title = "日本語タイトル日本語タイトル"
	m := resize(newModel(t, f), 100, 30)
	got := m.View()
	if strings.ContainsRune(got, '\x1b') {
		t.Fatal("CJK view contains escape byte")
	}
	widths := map[int]int{}
	for _, ln := range strings.Split(got, "\n") {
		w := lipgloss.Width(ln)
		if w > 100 {
			t.Fatalf("line width %d > 100: %q", w, ln)
		}
		if strings.Contains(ln, "╭") || strings.Contains(ln, "│") || strings.Contains(ln, "╰") {
			widths[w]++
		}
	}
	if len(widths) != 1 {
		t.Fatalf("border line widths = %v, want one aligned column", widths)
	}
}

// Graph section headers pair bold with their slot color: READY ok,
// BLOCKED warn, CRITICAL PATH (and other structural headers) accent.
func TestThemeGraphHeadSlots(t *testing.T) {
	t.Parallel()
	for kind, want := range map[string]lipgloss.TerminalColor{
		"ready":    cOK,
		"blocked":  cWarn,
		"critical": cAcc,
		"":         cAcc,
	} {
		s := graphHeadStyle(kind)
		if !s.GetBold() {
			t.Fatalf("graphHeadStyle(%q) not bold", kind)
		}
		if got := s.GetForeground(); got != want {
			t.Fatalf("graphHeadStyle(%q) fg = %v, want %v", kind, got, want)
		}
	}
}

// Overview section headers carry their slot kind at construction.
func TestThemeOverviewHeadKinds(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())
	rows := overviewRows(m.g)
	kinds := map[string]string{}
	for _, r := range rows {
		if r.head {
			kinds[r.text] = r.hstatus
		}
	}
	if len(kinds) == 0 {
		t.Fatal("no head rows in overview")
	}
	for text, kind := range kinds {
		var want string
		switch {
		case strings.Contains(text, "READY"):
			want = "ready"
		case strings.Contains(text, "BLOCKED"):
			want = "blocked"
		case strings.Contains(text, "CRITICAL"):
			want = "critical"
		default:
			want = "blocked" // warnings share the warn slot
		}
		if kind != want {
			t.Fatalf("header %q kind = %q, want %q", text, kind, want)
		}
	}
}

// Hostile titles/bodies are inert at row and detail construction.
func TestThemeHostileRowsAndDetail(t *testing.T) {
	t.Parallel()
	f := newFixture()
	f.items["AWIT-LAZY0001"].Title = "x\x1b[2Jtitle\x1b]0;owned\x07y\nnewline"
	m := newModel(t, f)
	for _, r := range m.issuesRows() {
		if strings.ContainsRune(r.text, '\x1b') || strings.ContainsRune(r.text, '\x07') || strings.ContainsRune(r.text, '\n') {
			t.Fatalf("row text unsanitized: %q", r.text)
		}
	}
	f.detail = "ok\x1b[2Jdetail\x1b]0;owned\x07line\nsecond"
	m2 := newModel(t, f)
	if got := m2.detail.View(); strings.ContainsRune(got, '\x1b') || strings.Contains(got, "owned") {
		t.Fatalf("detail unsanitized: %q", got)
	}
}

// A full frame over hostile data carries no escape bytes; the title text
// itself stays readable but inert.
func TestThemeHostileFrame(t *testing.T) {
	t.Parallel()
	f := newFixture()
	f.items["AWIT-LAZY0001"].Title = "x\x1b[2Jtitle\x1b]0;owned\x07y"
	m := resize(newModel(t, f), 100, 30)
	got := m.View()
	if strings.ContainsRune(got, '\x1b') || strings.Contains(got, "owned") {
		t.Fatalf("hostile frame leaked control bytes:\n%s", got)
	}
	if !strings.Contains(got, "xtitley") {
		t.Fatalf("hostile title text missing from frame:\n%s", got)
	}
}

// Toast strings built from err.Error() are sanitized at render time: the
// model keeps its bytes, the frame stays clean.
func TestThemeErrToastSanitized(t *testing.T) {
	t.Parallel()
	m := newModel(t, newFixture())
	m.act(func() error { return errors.New("\x1b[2Jboom\x1b]0;owned\x07") }, "ok")
	if m.toast != "error: \x1b[2Jboom\x1b]0;owned\x07" {
		t.Fatalf("model toast = %q, want raw error bytes kept", m.toast)
	}
	got := m.View()
	if strings.ContainsRune(got, '\x1b') || strings.Contains(got, "owned") {
		t.Fatalf("error toast leaked control bytes:\n%s", got)
	}
	if !strings.Contains(got, "error: boom") {
		t.Fatalf("sanitized error toast missing from frame:\n%s", got)
	}
}
