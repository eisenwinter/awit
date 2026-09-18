// Package format renders the flat rows every awit command prints, in the three
// output formats compact, table and json.
package format

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

type Format string

// The three supported formats.
const (
	Compact Format = "compact"
	Table   Format = "table"
	JSON    Format = "json"
)

func unknownFormat(f Format) error {
	return fmt.Errorf("format: unknown format %q (compact|table|json)", string(f))
}

// Detect resolves the effective format. A non-empty flag value wins and is
// validated; with an empty flag, stdout decides: a terminal gets the table,
// anything else (pipe, file, redirect) gets the compact form.
func Detect(flag string, stdout *os.File) (Format, error) {
	if flag != "" {
		switch Format(flag) {
		case Compact, Table, JSON:
			return Format(flag), nil
		}
		return "", unknownFormat(Format(flag))
	}
	if IsTerminal(stdout) {
		return Table, nil
	}
	return Compact, nil
}

// IsTerminal reports whether f is a character device. A nil file, or a file
// whose Stat fails, is not a terminal.
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}

// Entry is the format-neutral row. The graph → Entry conversion lives in
// internal/cli; this package never imports pkg/graph.
type Entry struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Brief    string   `json:"brief,omitempty"`
	Status   string   `json:"status"`
	State    string   `json:"state"` // ready | blocked | closed | quarantined
	Labels   []string `json:"labels"`
	Deps     []string `json:"deps"`
	Assignee string   `json:"assignee,omitempty"`
	Unblocks int      `json:"unblocks"` // -1 when quarantined
	Faults   []string `json:"faults,omitempty"`
}

func labelsOrDash(labels []string) string {
	s := strings.Join(labels, ",")
	if s == "" {
		return "-"
	}
	return s
}

// Line renders the one-line compact form used by list, next and prime:
// "[ID] status Title | label1,label2 | Unblocks: N", with "-" for no labels
// and a " | QUARANTINED" suffix for quarantined entries.
func Line(e Entry) string {
	s := fmt.Sprintf("[%s] %s %s | %s | Unblocks: %d", e.ID, e.Status, e.Title, labelsOrDash(e.Labels), e.Unblocks)
	if e.State == "quarantined" {
		s += " | QUARANTINED"
	}
	return s
}

func normalise(e Entry) Entry {
	if e.Labels == nil {
		e.Labels = []string{}
	}
	if e.Deps == nil {
		e.Deps = []string{}
	}
	return e
}

func writeJSON(w io.Writer, v any) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

// Write renders entries in the given format. JSON is always an array, indented
// two spaces, with a trailing newline; an empty input renders "[]\n".
// Table columns are ID, STATUS, STATE, TITLE, LABELS, UNBLOCKS, left aligned
// with a two-space gutter and an uppercase header row.
func Write(w io.Writer, f Format, entries []Entry) error {
	switch f {
	case Compact:
		for _, e := range entries {
			if _, err := fmt.Fprintln(w, Line(e)); err != nil {
				return err
			}
		}
		return nil
	case Table:
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(tw, "ID\tSTATUS\tSTATE\tTITLE\tLABELS\tUNBLOCKS"); err != nil {
			return err
		}
		for _, e := range entries {
			if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\n",
				e.ID, e.Status, e.State, e.Title, labelsOrDash(e.Labels), e.Unblocks); err != nil {
				return err
			}
		}
		return tw.Flush()
	case JSON:
		rows := make([]Entry, 0, len(entries))
		for _, e := range entries {
			rows = append(rows, normalise(e))
		}
		return writeJSON(w, rows)
	}
	return unknownFormat(f)
}

// WriteOne renders a single entry: compact → Line; table → a "key: value"
// block in the order ID, Title, Status, State, Labels, Deps, Assignee,
// Unblocks, Brief, Faults, skipping empty values (Unblocks is always printed
// because 0 and -1 are meaningful); json → a single object.
func WriteOne(w io.Writer, f Format, e Entry) error {
	switch f {
	case Compact:
		_, err := fmt.Fprintln(w, Line(e))
		return err
	case Table:
		pairs := []struct{ key, val string }{
			{"ID", e.ID},
			{"Title", e.Title},
			{"Status", e.Status},
			{"State", e.State},
			{"Labels", strings.Join(e.Labels, ",")},
			{"Deps", strings.Join(e.Deps, ",")},
			{"Assignee", e.Assignee},
		}
		for _, p := range pairs {
			if p.val == "" {
				continue
			}
			if _, err := fmt.Fprintf(w, "%s: %s\n", p.key, p.val); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "Unblocks: %d\n", e.Unblocks); err != nil {
			return err
		}
		if e.Brief != "" {
			if _, err := fmt.Fprintf(w, "Brief: %s\n", e.Brief); err != nil {
				return err
			}
		}
		for _, fault := range e.Faults {
			if _, err := fmt.Fprintf(w, "Faults: %s\n", fault); err != nil {
				return err
			}
		}
		return nil
	case JSON:
		return writeJSON(w, normalise(e))
	}
	return unknownFormat(f)
}

// LabelCount is one row of the label vocabulary.
type LabelCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// WriteLabels renders label counts in the caller's order: compact is
// "<label> <count>" per line, table is a LABEL/COUNT tabwriter block, json is
// an array of objects ("[]\n" when there are none).
func WriteLabels(w io.Writer, f Format, counts []LabelCount) error {
	switch f {
	case Compact:
		for _, c := range counts {
			if _, err := fmt.Fprintf(w, "%s %d\n", c.Label, c.Count); err != nil {
				return err
			}
		}
		return nil
	case Table:
		tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		if _, err := fmt.Fprintln(tw, "LABEL\tCOUNT"); err != nil {
			return err
		}
		for _, c := range counts {
			if _, err := fmt.Fprintf(tw, "%s\t%d\n", c.Label, c.Count); err != nil {
				return err
			}
		}
		return tw.Flush()
	case JSON:
		rows := counts
		if rows == nil {
			rows = []LabelCount{}
		}
		return writeJSON(w, rows)
	}
	return unknownFormat(f)
}
