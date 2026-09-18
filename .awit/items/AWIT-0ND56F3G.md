---
id: AWIT-0ND56F3G
title: "pkg/format: compact, table, json with golden files"
brief: >-
  Implement the output layer every list-shaped command renders through: format
  detection, the one-line compact form, the tabwriter table, and indented JSON,
  all pinned by golden files with an -update flag.
status: closed
deps: []
labels: [phase1, p1]
refs:
  - ../../plan/implementation-guide.md
  - ../../plan/awit-implementation-plan.md
---

## Summary
After this ticket `pkg/format` exists and renders `format.Entry` rows and `format.LabelCount` rows in three formats (`compact`, `table`, `json`), plus `Detect` for choosing a format from a flag or a TTY check. Five golden files under `pkg/format/testdata/golden/` pin the byte-exact output, and `pkg/format/golden_test.go` provides the `-update` helper used by the rest of the repo. This package has **no** dependency on `pkg/item`, `pkg/graph` or `internal/cli`: it only knows the flat `Entry` struct. Conversion from graph nodes to `Entry` does **not** exist after this ticket (it lands in `internal/cli` with `awit list`), and no command wires this package up yet.

## Context (read first)
- **guide §4.7 `pkg/format`** — the exact signatures to implement. Copy them; do not rename anything.
- **guide §1 global constraints** — stdlib only (`encoding/json`, `text/tabwriter`, `os`, `strings`, `fmt`, `io`); deterministic output (no map iteration in rendering); no colour anywhere; tests use `testing` only, table-driven; golden files live in `pkg/format/testdata/golden/` with `var update = flag.Bool("update", false, "rewrite golden files")`.
- **guide §5 testing conventions** — golden comparison is `bytes.Equal`; on mismatch print got and want and hint `go test ./... -update`.
- **guide §2 decisions that matter here** (one line each):
  - Unblock count is `-1` for quarantined nodes, so `Unblocks: -1` is a legal rendered value.
  - No colour in v1 output; `--no-color` is a no-op elsewhere, so this package never emits escape sequences.
  - Output compared in tests is deterministic: rendering order is the caller's slice order, never sorted or shuffled here.
- **spec `plan/awit-implementation-plan.md` §CLI command matrix** — "every list-shaped output honours `--format compact|table|json` (compact when stdout is not a TTY)".
- **spec §Agent surface → `awit next`** — the compact line shape this package produces.

## Files
- Create: `pkg/format/format.go`
- Create: `pkg/format/format_test.go`
- Create: `pkg/format/golden_test.go`
- Create (via `-update`): `pkg/format/testdata/golden/format_compact.golden`
- Create (via `-update`): `pkg/format/testdata/golden/format_table.golden`
- Create (via `-update`): `pkg/format/testdata/golden/format_json.golden`
- Create (via `-update`): `pkg/format/testdata/golden/format_one_table.golden`
- Create (via `-update`): `pkg/format/testdata/golden/format_labels_table.golden`

## Interfaces
- Consumes: nothing. `pkg/format` imports only the standard library.
- Produces (verbatim from guide §4.7):

```go
package format

type Format string

const (
	Compact Format = "compact"
	Table   Format = "table"
	JSON    Format = "json"
)

func Detect(flag string, stdout *os.File) (Format, error)
func IsTerminal(f *os.File) bool

type Entry struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Brief    string   `json:"brief,omitempty"`
	Status   string   `json:"status"`
	State    string   `json:"state"`
	Labels   []string `json:"labels"`
	Deps     []string `json:"deps"`
	Assignee string   `json:"assignee,omitempty"`
	Unblocks int      `json:"unblocks"`
	Faults   []string `json:"faults,omitempty"`
}

func Line(e Entry) string
func Write(w io.Writer, f Format, entries []Entry) error
func WriteOne(w io.Writer, f Format, e Entry) error

type LabelCount struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

func WriteLabels(w io.Writer, f Format, counts []LabelCount) error
```

- Produces (package-private helpers introduced by this ticket):

```go
func labelsOrDash(labels []string) string // strings.Join(labels, ",") or "-" when empty
func normalise(e Entry) Entry             // copy with nil Labels/Deps replaced by []string{}
func unknownFormat(f Format) error        // fmt.Errorf("format: unknown format %q (compact|table|json)", string(f))
```

- Produces (test-only, `pkg/format/golden_test.go`) — ticket `AWIT-0ND56G3G` copies this helper into `internal/cli/helpers_test.go` with a different relative root (`repoRoot()` via `runtime.Caller`), so keep the behaviour identical and the path logic local:

```go
var update = flag.Bool("update", false, "rewrite golden files")
func golden(t *testing.T, name string, got []byte)
```

## Steps

- [ ] **Step 1: Failing test for `Detect` and `IsTerminal`**

  Create `pkg/format/format_test.go`:

```go
package format

import (
	"os"
	"path/filepath"
	"testing"
)

// nonTTY returns an open handle to a regular file inside t.TempDir().
//
// Why not os.DevNull: on Linux and macOS /dev/null IS a character device, so
// os.ModeCharDevice is set and IsTerminal would report true.
// Why not os.Stdout: under `go test` it is a pipe, but when the compiled test
// binary is run directly from a shell it is a terminal, so the expected value
// would depend on how the test was started.
func nonTTY(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Create(filepath.Join(t.TempDir(), "out.txt"))
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		want    Format
		wantErr string
	}{
		{name: "flag compact", flag: "compact", want: Compact},
		{name: "flag table", flag: "table", want: Table},
		{name: "flag json", flag: "json", want: JSON},
		{name: "unknown flag", flag: "yaml", wantErr: `format: unknown format "yaml" (compact|table|json)`},
		{name: "empty flag, no tty", flag: "", want: Compact},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Detect(tt.flag, nonTTY(t))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Detect(%q) = %q, want error", tt.flag, got)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
				}
				if got != "" {
					t.Fatalf("format on error = %q, want empty", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Detect(%q): unexpected error %v", tt.flag, err)
			}
			if got != tt.want {
				t.Fatalf("Detect(%q) = %q, want %q", tt.flag, got, tt.want)
			}
		})
	}
}

func TestIsTerminalFalseForFile(t *testing.T) {
	if IsTerminal(nonTTY(t)) {
		t.Fatal("IsTerminal(regular file) = true, want false")
	}
	if IsTerminal(nil) {
		t.Fatal("IsTerminal(nil) = true, want false")
	}
	closed, err := os.Create(filepath.Join(t.TempDir(), "closed.txt"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := closed.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if IsTerminal(closed) {
		t.Fatal("IsTerminal(closed file) = true, want false (Stat fails)")
	}
}
```

- [ ] **Step 2: Run it, see it fail**

```sh
go test ./pkg/format -run 'TestDetect|TestIsTerminal' -v
```

  Expected: the package does not compile —
  `pkg/format/format_test.go:29:15: undefined: Detect` and `undefined: Compact`, `undefined: Format`, `undefined: IsTerminal`, ending in `FAIL github.com/eisenwinter/awit/pkg/format [build failed]`.

- [ ] **Step 3: Implement `Format`, `Detect`, `IsTerminal`**

  Create `pkg/format/format.go`:

```go
// Package format renders the flat rows every awit command prints, in the three
// output formats compact, table and json.
package format

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

// Format is an output rendering mode.
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
```

- [ ] **Step 4: Run it, see it pass, commit**

```sh
go test ./pkg/format -run 'TestDetect|TestIsTerminal' -v
```

  Expected: `--- PASS: TestDetect` with five subtests, `--- PASS: TestIsTerminalFalseForFile`, `ok  	github.com/eisenwinter/awit/pkg/format`.

```sh
gofmt -w pkg/format/format.go pkg/format/format_test.go
git add pkg/format/format.go pkg/format/format_test.go
git commit -m "format: add Format, Detect and IsTerminal"
```

- [ ] **Step 5: Failing test for `Entry` and `Line`**

  Append to `pkg/format/format_test.go`:

```go
// sampleEntries is the fixed input behind every golden file in this package.
// Entry B deliberately leaves Labels nil to exercise the JSON normalisation.
func sampleEntries() []Entry {
	return []Entry{
		{
			ID:       "AWIT-TEST0001",
			Title:    "Implement OAuth2 token extraction",
			Brief:    "Fix header parsing.",
			Status:   "open",
			State:    "ready",
			Labels:   []string{"auth", "p1"},
			Deps:     []string{},
			Unblocks: 2,
		},
		{
			ID:       "AWIT-TEST0003",
			Title:    "Add E2E auth tests",
			Status:   "open",
			State:    "blocked",
			Deps:     []string{"AWIT-TEST0001"},
			Unblocks: 0,
		},
		{
			ID:       "AWIT-TEST0009",
			Title:    "Rotate tokens",
			Status:   "open",
			State:    "quarantined",
			Labels:   []string{"p0"},
			Deps:     []string{"AWIT-TEST0009"},
			Unblocks: -1,
			Faults:   []string{"[CYCLE] AWIT-TEST0009 -> AWIT-TEST0009"},
		},
	}
}

func TestLine(t *testing.T) {
	entries := sampleEntries()
	tests := []struct {
		name string
		in   Entry
		want string
	}{
		{
			name: "labels and unblocks",
			in:   entries[0],
			want: "[AWIT-TEST0001] Implement OAuth2 token extraction | auth,p1 | Unblocks: 2",
		},
		{
			name: "no labels renders dash",
			in:   entries[1],
			want: "[AWIT-TEST0003] Add E2E auth tests | - | Unblocks: 0",
		},
		{
			name: "quarantined suffix and negative unblocks",
			in:   entries[2],
			want: "[AWIT-TEST0009] Rotate tokens | p0 | Unblocks: -1 | QUARANTINED",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Line(tt.in); got != tt.want {
				t.Fatalf("Line() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 6: Run it, see it fail**

```sh
go test ./pkg/format -run TestLine -v
```

  Expected: build failure `undefined: Entry` and `undefined: Line`.

- [ ] **Step 7: Implement `Entry` and `Line`, run, see it pass, commit**

  Append to `pkg/format/format.go`:

```go
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
// "[ID] Title | label1,label2 | Unblocks: N", with "-" for no labels and a
// " | QUARANTINED" suffix for quarantined entries.
func Line(e Entry) string {
	s := fmt.Sprintf("[%s] %s | %s | Unblocks: %d", e.ID, e.Title, labelsOrDash(e.Labels), e.Unblocks)
	if e.State == "quarantined" {
		s += " | QUARANTINED"
	}
	return s
}
```

```sh
go test ./pkg/format -run TestLine -v
```

  Expected: `--- PASS: TestLine` with its three subtests.

```sh
gofmt -w pkg/format/format.go pkg/format/format_test.go
git add pkg/format
git commit -m "format: add Entry and compact Line rendering"
```

- [ ] **Step 8: Golden helper**

  Create `pkg/format/golden_test.go`:

```go
package format

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

// goldenPath resolves testdata/golden next to this package.
func goldenPath(name string) string {
	return filepath.Join("testdata", "golden", name)
}

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := goldenPath(name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run: go test ./... -update)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("golden %s mismatch\ngot:\n%s\nwant:\n%s\nrun: go test ./... -update", name, got, want)
	}
}
```

- [ ] **Step 9: Failing tests for `Write` in all three formats**

  Append to `pkg/format/format_test.go`:

```go
func TestWriteCompactGolden(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, Compact, sampleEntries()); err != nil {
		t.Fatalf("Write compact: %v", err)
	}
	golden(t, "format_compact.golden", buf.Bytes())
}

func TestWriteTableGolden(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, Table, sampleEntries()); err != nil {
		t.Fatalf("Write table: %v", err)
	}
	golden(t, "format_table.golden", buf.Bytes())
}

func TestWriteJSONGolden(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, JSON, sampleEntries()); err != nil {
		t.Fatalf("Write json: %v", err)
	}
	golden(t, "format_json.golden", buf.Bytes())

	var back []Entry
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("json round-trip: %v", err)
	}
	if len(back) != 3 {
		t.Fatalf("decoded %d entries, want 3", len(back))
	}
	if back[1].Labels == nil || len(back[1].Labels) != 0 {
		t.Fatalf("entry B labels = %#v, want empty non-nil slice", back[1].Labels)
	}
	if len(back[0].Deps) != 0 {
		t.Fatalf("entry A deps = %#v, want empty", back[0].Deps)
	}
	if back[2].Unblocks != -1 {
		t.Fatalf("entry C unblocks = %d, want -1", back[2].Unblocks)
	}
}

func TestWriteJSONEmpty(t *testing.T) {
	for _, in := range [][]Entry{nil, {}} {
		var buf bytes.Buffer
		if err := Write(&buf, JSON, in); err != nil {
			t.Fatalf("Write json: %v", err)
		}
		if got := buf.String(); got != "[]\n" {
			t.Fatalf("Write(json, %#v) = %q, want %q", in, got, "[]\n")
		}
	}
}

func TestWriteCompactEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, Compact, nil); err != nil {
		t.Fatalf("Write compact: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("Write(compact, nil) = %q, want empty", buf.String())
	}
}

func TestWriteJSONDoesNotMutateInput(t *testing.T) {
	entries := sampleEntries()
	var buf bytes.Buffer
	if err := Write(&buf, JSON, entries); err != nil {
		t.Fatalf("Write json: %v", err)
	}
	if entries[1].Labels != nil {
		t.Fatalf("input entry mutated: labels = %#v, want nil", entries[1].Labels)
	}
}
```

  Add `"bytes"` and `"encoding/json"` to the imports of `format_test.go`.

- [ ] **Step 10: Run them, see them fail**

```sh
go test ./pkg/format -run TestWrite -v
```

  Expected: build failure `undefined: Write`.

- [ ] **Step 11: Implement `Write`**

  Append to `pkg/format/format.go`:

```go
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
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if _, err := w.Write(b); err != nil {
		return err
	}
	_, err = w.Write([]byte("\n"))
	return err
}

// Write renders entries in the given format. JSON is always an array, indented
// two spaces, with a trailing newline; an empty input renders "[]\n".
// Table columns are ID, STATE, TITLE, LABELS, UNBLOCKS, left aligned with a
// two-space gutter and an uppercase header row.
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
		if _, err := fmt.Fprintln(tw, "ID\tSTATE\tTITLE\tLABELS\tUNBLOCKS"); err != nil {
			return err
		}
		for _, e := range entries {
			if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\n",
				e.ID, e.State, e.Title, labelsOrDash(e.Labels), e.Unblocks); err != nil {
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
```

- [ ] **Step 12: Create the golden files and verify their exact bytes**

```sh
go test ./pkg/format -run TestWrite -update
go test ./pkg/format -run TestWrite -v
```

  Expected: first run writes the files, second run passes without `-update`. Now check each file byte for byte against the content below; if a file differs, the implementation is wrong — fix the code, not the golden.

  `pkg/format/testdata/golden/format_compact.golden`:

```text
[AWIT-TEST0001] Implement OAuth2 token extraction | auth,p1 | Unblocks: 2
[AWIT-TEST0003] Add E2E auth tests | - | Unblocks: 0
[AWIT-TEST0009] Rotate tokens | p0 | Unblocks: -1 | QUARANTINED
```

  `pkg/format/testdata/golden/format_table.golden` (column widths: ID 15, STATE 13, TITLE 35, LABELS 9, UNBLOCKS unpadded because it is the last cell on the line):

```text
ID             STATE        TITLE                              LABELS   UNBLOCKS
AWIT-TEST0001  ready        Implement OAuth2 token extraction  auth,p1  2
AWIT-TEST0003  blocked      Add E2E auth tests                 -        0
AWIT-TEST0009  quarantined  Rotate tokens                      p0       -1
```

  `pkg/format/testdata/golden/format_json.golden`:

```json
[
  {
    "id": "AWIT-TEST0001",
    "title": "Implement OAuth2 token extraction",
    "brief": "Fix header parsing.",
    "status": "open",
    "state": "ready",
    "labels": [
      "auth",
      "p1"
    ],
    "deps": [],
    "unblocks": 2
  },
  {
    "id": "AWIT-TEST0003",
    "title": "Add E2E auth tests",
    "status": "open",
    "state": "blocked",
    "labels": [],
    "deps": [
      "AWIT-TEST0001"
    ],
    "unblocks": 0
  },
  {
    "id": "AWIT-TEST0009",
    "title": "Rotate tokens",
    "status": "open",
    "state": "quarantined",
    "labels": [
      "p0"
    ],
    "deps": [
      "AWIT-TEST0009"
    ],
    "unblocks": -1,
    "faults": [
      "[CYCLE] AWIT-TEST0009 -> AWIT-TEST0009"
    ]
  }
]
```

  (The file ends with a newline after the closing `]`.)

```sh
gofmt -w pkg/format
git add pkg/format
git commit -m "format: render entries as compact, table and json"
```

- [ ] **Step 13: Failing test for `WriteOne`**

  Append to `pkg/format/format_test.go`:

```go
func TestWriteOneTableGolden(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOne(&buf, Table, sampleEntries()[0]); err != nil {
		t.Fatalf("WriteOne table: %v", err)
	}
	golden(t, "format_one_table.golden", buf.Bytes())
}

func TestWriteOneTableSkipsEmptyAndKeepsUnblocks(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOne(&buf, Table, sampleEntries()[1]); err != nil {
		t.Fatalf("WriteOne table: %v", err)
	}
	got := buf.String()
	want := "ID: AWIT-TEST0003\nTitle: Add E2E auth tests\nStatus: open\nState: blocked\nDeps: AWIT-TEST0001\nUnblocks: 0\n"
	if got != want {
		t.Fatalf("WriteOne table =\n%q\nwant\n%q", got, want)
	}
}

func TestWriteOneTableFaults(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOne(&buf, Table, sampleEntries()[2]); err != nil {
		t.Fatalf("WriteOne table: %v", err)
	}
	got := buf.String()
	want := "ID: AWIT-TEST0009\nTitle: Rotate tokens\nStatus: open\nState: quarantined\n" +
		"Labels: p0\nDeps: AWIT-TEST0009\nUnblocks: -1\nFaults: [CYCLE] AWIT-TEST0009 -> AWIT-TEST0009\n"
	if got != want {
		t.Fatalf("WriteOne table =\n%q\nwant\n%q", got, want)
	}
}

func TestWriteOneCompact(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOne(&buf, Compact, sampleEntries()[0]); err != nil {
		t.Fatalf("WriteOne compact: %v", err)
	}
	want := "[AWIT-TEST0001] Implement OAuth2 token extraction | auth,p1 | Unblocks: 2\n"
	if got := buf.String(); got != want {
		t.Fatalf("WriteOne compact = %q, want %q", got, want)
	}
}

func TestWriteOneJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOne(&buf, JSON, sampleEntries()[1]); err != nil {
		t.Fatalf("WriteOne json: %v", err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("{\n")) {
		t.Fatalf("WriteOne json = %q, want a single object", buf.String())
	}
	if !bytes.HasSuffix(buf.Bytes(), []byte("}\n")) {
		t.Fatalf("WriteOne json = %q, want trailing newline after object", buf.String())
	}
	var back Entry
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.ID != "AWIT-TEST0003" || back.Labels == nil || len(back.Labels) != 0 {
		t.Fatalf("decoded = %#v, want id AWIT-TEST0003 and empty non-nil labels", back)
	}
}
```

- [ ] **Step 14: Run, see fail, implement `WriteOne`, run, see pass, commit**

```sh
go test ./pkg/format -run TestWriteOne -v
```

  Expected: build failure `undefined: WriteOne`. Then append to `pkg/format/format.go`:

```go
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
```

```sh
go test ./pkg/format -run TestWriteOne -update
go test ./pkg/format -run TestWriteOne -v
```

  `pkg/format/testdata/golden/format_one_table.golden` must be exactly:

```text
ID: AWIT-TEST0001
Title: Implement OAuth2 token extraction
Status: open
State: ready
Labels: auth,p1
Unblocks: 2
Brief: Fix header parsing.
```

```sh
gofmt -w pkg/format
git add pkg/format
git commit -m "format: add WriteOne single-entry rendering"
```

- [ ] **Step 15: Failing tests for `WriteLabels`**

  Append to `pkg/format/format_test.go`:

```go
func sampleLabels() []LabelCount {
	return []LabelCount{{Label: "auth", Count: 3}, {Label: "p1", Count: 3}, {Label: "db", Count: 1}}
}

func TestWriteLabelsGolden(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteLabels(&buf, Table, sampleLabels()); err != nil {
		t.Fatalf("WriteLabels table: %v", err)
	}
	golden(t, "format_labels_table.golden", buf.Bytes())
}

func TestWriteLabelsCompact(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteLabels(&buf, Compact, sampleLabels()); err != nil {
		t.Fatalf("WriteLabels compact: %v", err)
	}
	want := "auth 3\np1 3\ndb 1\n"
	if got := buf.String(); got != want {
		t.Fatalf("WriteLabels compact = %q, want %q", got, want)
	}
}

func TestWriteLabelsJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteLabels(&buf, JSON, sampleLabels()); err != nil {
		t.Fatalf("WriteLabels json: %v", err)
	}
	want := "[\n  {\n    \"label\": \"auth\",\n    \"count\": 3\n  },\n  {\n    \"label\": \"p1\",\n    \"count\": 3\n  },\n  {\n    \"label\": \"db\",\n    \"count\": 1\n  }\n]\n"
	if got := buf.String(); got != want {
		t.Fatalf("WriteLabels json =\n%q\nwant\n%q", got, want)
	}
}

func TestWriteLabelsEmptyJSON(t *testing.T) {
	for _, in := range [][]LabelCount{nil, {}} {
		var buf bytes.Buffer
		if err := WriteLabels(&buf, JSON, in); err != nil {
			t.Fatalf("WriteLabels json: %v", err)
		}
		if got := buf.String(); got != "[]\n" {
			t.Fatalf("WriteLabels(json, %#v) = %q, want %q", in, got, "[]\n")
		}
	}
}

func TestWriteUnknownFormat(t *testing.T) {
	const want = `format: unknown format "yaml" (compact|table|json)`
	var buf bytes.Buffer
	if err := Write(&buf, Format("yaml"), sampleEntries()); err == nil || err.Error() != want {
		t.Fatalf("Write error = %v, want %q", err, want)
	}
	if err := WriteOne(&buf, Format("yaml"), sampleEntries()[0]); err == nil || err.Error() != want {
		t.Fatalf("WriteOne error = %v, want %q", err, want)
	}
	if err := WriteLabels(&buf, Format("yaml"), sampleLabels()); err == nil || err.Error() != want {
		t.Fatalf("WriteLabels error = %v, want %q", err, want)
	}
}
```

- [ ] **Step 16: Run, see fail, implement `WriteLabels`, run, see pass, commit**

```sh
go test ./pkg/format -run 'TestWriteLabels|TestWriteUnknownFormat' -v
```

  Expected: build failure `undefined: LabelCount` and `undefined: WriteLabels`. Then append to `pkg/format/format.go`:

```go
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
```

```sh
go test ./pkg/format -run 'TestWriteLabels|TestWriteUnknownFormat' -update
go test ./pkg/format -v
```

  `pkg/format/testdata/golden/format_labels_table.golden` must be exactly:

```text
LABEL  COUNT
auth   3
p1     3
db     1
```

```sh
gofmt -w pkg/format
git add pkg/format
git commit -m "format: add label vocabulary rendering"
```

- [ ] **Step 17: Full package verification**

```sh
go build ./... && go vet ./... && go test ./pkg/format -v
git status --porcelain pkg/format/testdata/golden
```

  Expected: build and vet silent, every test `PASS`, and `git status --porcelain pkg/format/testdata/golden` prints nothing (a plain test run must not rewrite goldens).

- [ ] **Step 18: Close ticket**

  1. Set `status: closed` in the frontmatter of `.awit/items/AWIT-0ND56F3G.md`.
  2. Write `.awit/comments/AWIT-0ND56F3G/<YYYYMMDDTHHMMSSZ>-<author>.md` (UTC stamp, e.g. `20260917T181500Z-claude.md`) containing:

```markdown
---
author: agent/claude
created: 2026-09-17T18:15:00Z
---

go build ./... && go vet ./... && go test ./pkg/format -v
ok  	github.com/eisenwinter/awit/pkg/format

Golden files created: format_compact.golden, format_table.golden,
format_json.golden, format_one_table.golden, format_labels_table.golden
```

  (Replace the pasted output with the real output of the acceptance commands.)
  3. Append the ref `../comments/AWIT-0ND56F3G/<file>.md` to this ticket's `refs` list.
  4. Commit:

```sh
git add .awit/items/AWIT-0ND56F3G.md .awit/comments/AWIT-0ND56F3G
git commit -m "tickets: close AWIT-0ND56F3G"
```

## Acceptance Criteria
- `go build ./...` exits 0 with no output.
- `go vet ./...` exits 0 with no output.
- `go test ./pkg/format -v` exits 0; output contains `--- PASS: TestDetect`, `--- PASS: TestIsTerminalFalseForFile`, `--- PASS: TestLine`, `--- PASS: TestWriteCompactGolden`, `--- PASS: TestWriteTableGolden`, `--- PASS: TestWriteJSONGolden`, `--- PASS: TestWriteJSONEmpty`, `--- PASS: TestWriteOneTableGolden`, `--- PASS: TestWriteLabelsGolden`, `--- PASS: TestWriteLabelsEmptyJSON`, `--- PASS: TestWriteUnknownFormat`.
- `go test ./pkg/format` run twice in a row both exit 0, and `git status --porcelain pkg/format/testdata/golden` prints nothing.
- `cat pkg/format/testdata/golden/format_table.golden` prints exactly the four lines shown in Step 12 (header plus three rows), the last line ending `-1`.
- `cat pkg/format/testdata/golden/format_compact.golden` prints exactly the three lines shown in Step 12, the third ending ` | QUARANTINED`.
- `cat pkg/format/testdata/golden/format_labels_table.golden` prints exactly `LABEL  COUNT` / `auth   3` / `p1     3` / `db     1`.
- `gofmt -l pkg/format` prints nothing.

## Out of scope
- Converting `*graph.Node` into `format.Entry` (`toEntry`) — that is `AWIT-0ND56J3G` (`awit list`), which also owns state naming from the graph.
- Wiring `--format` into any command or calling `Detect` from `internal/cli` — `AWIT-0ND5683G` owns the global flag, individual commands call `Detect` themselves.
- The `internal/cli` copy of the golden helper — `AWIT-0ND56G3G` creates `internal/cli/helpers_test.go`; do not add a shared testing package.
- Counting labels out of a store or graph — `AWIT-0ND5763G` (`awit label`) computes the `LabelCount` rows; this ticket only renders them.
- Terminal colour, width detection, or paging: no colour in v1 (guide §1), no `--no-color` handling here.
- Creating `testdata/fixtures/*` — fixtures belong to `AWIT-0ND56N3G`; this package needs no fixtures, its input is the inline `sampleEntries()`.
