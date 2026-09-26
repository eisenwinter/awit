package lazy

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Visual tokens for the lazyawit premium treatment. All color slots are
// ANSI-named (not hex) so the terminal theme translates them on dark AND
// light backgrounds; every colored element keeps a textual marker, so the
// NO_COLOR plain-text fallback (goldens) carries the same meaning.
var (
	cMuted = lipgloss.Color("8")
	cAcc   = lipgloss.AdaptiveColor{Dark: "6", Light: "4"}
	cOK    = lipgloss.Color("2")
	cWarn  = lipgloss.Color("3")
	cErr   = lipgloss.Color("1")
)

var (
	stBrand        = lipgloss.NewStyle().Bold(true)
	stTabActive    = lipgloss.NewStyle().Bold(true).Foreground(cAcc)
	stTabInactive  = lipgloss.NewStyle().Foreground(cMuted)
	stMuted        = lipgloss.NewStyle().Foreground(cMuted)
	stHintKey      = lipgloss.NewStyle().Bold(true)
	stToastOK      = lipgloss.NewStyle().Foreground(cOK)
	stToastErr     = lipgloss.NewStyle().Foreground(cErr)
	stQuarantine   = lipgloss.NewStyle().Foreground(cWarn)
	stWhy          = lipgloss.NewStyle().Foreground(cMuted)
	stDetailTitle  = lipgloss.NewStyle().Bold(true)
	stDetailHead   = lipgloss.NewStyle().Bold(true).Foreground(cAcc)
	stDetailLabel  = lipgloss.NewStyle().Foreground(cMuted)
	stCursorMarker = lipgloss.NewStyle().Foreground(cAcc)
	stCursor       = lipgloss.NewStyle().Reverse(true)
	stGraphReady   = lipgloss.NewStyle().Bold(true).Foreground(cOK)
	stGraphBlocked = lipgloss.NewStyle().Bold(true).Foreground(cWarn)
	stGraphCrit    = lipgloss.NewStyle().Bold(true).Foreground(cAcc)
)

// graphHeadStyle pairs section headers with their slot color: READY ok,
// BLOCKED (and warnings) warn, CRITICAL PATH and other structural headers
// accent. All bold.
func graphHeadStyle(kind string) lipgloss.Style {
	switch kind {
	case "ready":
		return stGraphReady
	case "blocked":
		return stGraphBlocked
	default:
		return stGraphCrit
	}
}

// borderStyle is the single app-wide pane chrome: rounded light, focus
// carried by border color only (never weight - a weight swap would churn
// NO_COLOR goldens on every focus switch).
func borderStyle(focused bool) lipgloss.Style {
	var fg lipgloss.TerminalColor = cMuted
	if focused {
		fg = cAcc
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(fg)
}

// statusStyle maps an item status token to its palette slot. Closed rows
// are muted, not a hue; unknown tokens render unset (plain).
func statusStyle(status string) lipgloss.Style {
	switch status {
	case "open", "ready":
		return lipgloss.NewStyle().Foreground(cOK)
	case "in_progress":
		return lipgloss.NewStyle().Foreground(cAcc)
	case "blocked":
		return lipgloss.NewStyle().Foreground(cWarn)
	case "closed":
		return lipgloss.NewStyle().Foreground(cMuted)
	default:
		return lipgloss.NewStyle()
	}
}

// sanitize strips ANSI/OSC sequences and residual C0/DEL runes from
// untrusted display data (item titles/bodies, archive bytes) at the lazy
// trust boundary. Rows are single-line (newlines become spaces); detail
// keeps newlines.
func sanitize(s string, multiline bool) string {
	s = ansi.Strip(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == '\n':
			if multiline {
				b.WriteRune(r)
			} else {
				b.WriteRune(' ')
			}
		case r == '\t':
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			// drop residual controls
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// styleToast pairs toasts at render time: failures already carry "error: "
// (kept, styled err); everything else gains an "ok: " prefix (styled ok).
// Untrusted bytes (e.g. err.Error() text) are sanitized here so the toast
// slot stays a single clean line; m.toast bytes are untouched - only the
// frame changes.
func styleToast(toast string) string {
	if toast == "" {
		return ""
	}
	clean := sanitize(toast, false)
	if strings.HasPrefix(clean, "error: ") {
		return stToastErr.Render(clean)
	}
	return stToastOK.Render("ok: " + clean)
}

// styleRow colorizes the first occurrence of the status token. Callers
// truncate plain text first so width math never sees ANSI.
func styleRow(text, status string) string {
	if status == "" {
		return text
	}
	i := strings.Index(text, status)
	if i < 0 {
		return text
	}
	return text[:i] + statusStyle(status).Render(status) + text[i+len(status):]
}

// detailLabels are the "label: value" runs dimmed by styleDetail.
var detailLabels = []string{"status:", "deps:", "assignee:", "brief:", "refs:"}

// styleDetail is a pure post-processing pass over wrapped detail text:
// headline bold, "## " section headers bold + accent, label runs muted.
// Plain-text output is byte-identical to the input structure.
func styleDetail(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		switch {
		case i == 0:
			lines[i] = stDetailTitle.Render(ln)
		case strings.HasPrefix(ln, "## "):
			lines[i] = stDetailHead.Render(ln)
		default:
			for _, lb := range detailLabels {
				if strings.HasPrefix(ln, lb) {
					lines[i] = stDetailLabel.Render(lb) + ln[len(lb):]
					break
				}
			}
		}
	}
	return strings.Join(lines, "\n")
}

// styleHints bolds the key token of each double-space-separated hint group
// ("j/k move  enter detail" -> bold "j/k", "enter", ...). No new glyphs, so
// the width guard still holds.
func styleHints(line string) string {
	groups := strings.Split(line, "  ")
	for i, g := range groups {
		if g == "" {
			continue
		}
		j := strings.Index(g, " ")
		if j < 0 {
			groups[i] = stHintKey.Render(g)
			continue
		}
		groups[i] = stHintKey.Render(g[:j]) + g[j:]
	}
	return strings.Join(groups, "  ")
}

// isErrToast reports a sticky failure toast by its "error: " prefix.
func isErrToast(toast string) bool { return strings.HasPrefix(toast, "error: ") }
