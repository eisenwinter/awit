package lazy

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/eisenwinter/awit/pkg/config"
)

// configState is the Config tab: the last config read through Ops and one
// selectable row per key.
type configState struct {
	cfg  config.Config
	list cursorList
}

// configKeys is the row order (the docs/schema.md table order).
var configKeys = []string{"prefix", "default_labels", "labels", "stale_claim", "agent_id", "commit", "external_push", "template"}

// configValue is key's editable text: the value as the user types it.
// Lists join with ", ", stale_claim uses Duration.String, bools are
// "true"/"false", an absent bool or empty string is "".
func configValue(c config.Config, key string) string {
	switch key {
	case "prefix":
		return c.Prefix
	case "default_labels":
		return strings.Join(c.DefaultLabels, ", ")
	case "labels":
		return strings.Join(c.Labels, ", ")
	case "stale_claim":
		return c.StaleClaim.String()
	case "agent_id":
		return c.AgentID
	case "commit":
		if c.Commit == nil {
			return ""
		}
		if *c.Commit {
			return "true"
		}
		return "false"
	case "external_push":
		if c.ExternalPush == nil {
			return ""
		}
		if *c.ExternalPush {
			return "true"
		}
		return "false"
	case "template":
		return c.Template
	default:
		panic("unknown config key: " + key)
	}
}

// configRows renders one selectable row per key (row.id = key): the key
// padded to 14 columns, a space, then configValue or "(unset)".
func configRows(c config.Config) []row {
	rows := make([]row, 0, len(configKeys))
	for _, k := range configKeys {
		shown := configValue(c, k)
		if shown == "" {
			shown = "(unset)"
		}
		rows = append(rows, row{id: k, text: sanitize(fmt.Sprintf("%-14s %s", k, shown), false), selectable: true})
	}
	return rows
}

// configDetail is the detail-pane text for key: the rule line, "default: …"
// where one exists, "current: <value|(unset)>", a blank line, then the two
// note lines. Lines joined with "\n", trailing "\n".
func configDetail(c config.Config, key string) string {
	rule, def := configRule(key)
	lines := []string{rule}
	if def != "" {
		lines = append(lines, def)
	}
	cur := configValue(c, key)
	if cur == "" {
		cur = "(unset)"
	}
	lines = append(lines, "current: "+cur, "",
		"e/enter edits, esc discards, R re-reads the file.",
		"saving rewrites .awit/config.yaml; comments and unknown keys are dropped.")
	return strings.Join(lines, "\n") + "\n"
}

// configRule returns the rule sentence for key and its "default: …" line
// ("" when the key has no default to state).
func configRule(key string) (rule, def string) {
	switch key {
	case "prefix":
		return "required. Id prefix for new items: 2-8 uppercase alphanumerics starting with a letter ([A-Z][A-Z0-9]{1,7}). Existing item ids keep their prefix.", ""
	case "default_labels":
		return "optional. Comma-separated labels merged (first wins) into every awit create -l.", ""
	case "labels":
		return "optional advisory vocabulary; empty disables warnings. create/update warn on unknown names but still store them. Entries: nonempty, no surrounding whitespace or control characters; duplicates are dropped.", ""
	case "stale_claim":
		return "Go duration (2h, 90m) used by awit validate --stale-claims.", "default: 2h (also when 0)"
	case "agent_id":
		return "identity for claims and comment authors. AWIT_AGENT overrides it; --agent / --author override both.", ""
	case "commit":
		return "true or false: whether awit next --claim git-commits. Only next --claim reads it.", "default: true (unset)"
	case "external_push":
		return "true or false: whether close, release and update --status push to the linked issue.", "default: true (unset)"
	case "template":
		return "repo-root-relative forward-slash path to a body-only Markdown file for create. Absolute paths, backslashes and ../ escapes are refused.", "default: built-in skeleton (unset)"
	default:
		panic("unknown config key: " + key)
	}
}

// setConfigField parses raw into key on a copy of c. prefix must satisfy
// config.ValidPrefix; stale_claim goes through time.ParseDuration ("" is 0,
// which Normalize turns into 2h); commit/external_push accept "true",
// "false" or "" (nil); default_labels/labels split on commas, trim spaces
// and drop empty pieces (the awit create -l rule); agent_id/template are
// taken verbatim. Slices and *bool are always fresh, so a refused edit
// leaves c untouched. Normalize is the caller's job. An unknown key panics.
func setConfigField(c config.Config, key, raw string) (config.Config, error) {
	switch key {
	case "prefix":
		if !config.ValidPrefix(raw) {
			return c, errors.New("prefix must be 2-8 uppercase alphanumerics starting with a letter")
		}
		c.Prefix = raw
	case "default_labels":
		c.DefaultLabels = splitList(raw)
	case "labels":
		c.Labels = splitList(raw)
	case "stale_claim":
		if raw == "" {
			c.StaleClaim = 0
			break
		}
		d, err := time.ParseDuration(raw)
		if err != nil {
			return c, err
		}
		c.StaleClaim = config.Duration(d)
	case "commit", "external_push":
		var v *bool
		switch raw {
		case "":
		case "true", "false":
			b := raw == "true"
			v = &b
		default:
			return c, fmt.Errorf("%s must be true, false, or empty", key)
		}
		if key == "commit" {
			c.Commit = v
		} else {
			c.ExternalPush = v
		}
	case "agent_id":
		c.AgentID = raw
	case "template":
		c.Template = raw
	default:
		panic("unknown config key " + key)
	}
	return c, nil
}

// splitList splits a comma list, trims each piece and drops empties; nil
// when nothing remains.
func splitList(s string) []string {
	var out []string
	for p := range strings.SplitSeq(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// beginConfigEdit opens the prompt on the selected key (inputConfig),
// prefilled with configValue and the cursor at the end. Focus is the
// caller's Cmd.
func (m *Model) beginConfigEdit() {
	r, _ := m.config.list.selected()
	m.mode = modeInput
	m.inputKind = inputConfig
	m.inputTarget = r.id
	m.input.SetValue(configValue(m.config.cfg, r.id))
	m.input.CursorEnd()
}

// saveConfigField parses and normalizes raw for key; a failure becomes a
// toast with nothing written. A success saves the whole config through
// Ops.SaveConfig via act ("saved <key>"), which reloads rows from the
// re-read file with the cursor still on key.
func (m *Model) saveConfigField(key, raw string) {
	next, err := setConfigField(m.config.cfg, key, raw)
	if err == nil {
		next, err = next.Normalize()
	}
	if err != nil {
		m.toast = "error: " + err.Error()
		return
	}
	m.act(func() error { return m.ops.SaveConfig(next) }, "saved "+key)
}
