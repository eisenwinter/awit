---
id: AWIT-0ND56D3G
title: 'pkg/item: frontmatter split, parse, setters, byte-identical round-trip'
brief: >-
  Implement pkg/item frontmatter splitting, yaml.v3 Node parse, field setters
  and Bytes so a parsed file round-trips byte-identically when no setter ran.
status: closed
deps: []
labels: [phase1, p0]
refs:
  - ../../plan/implementation-guide.md
  - ../../plan/awit-implementation-plan.md
---

## Summary
After this ticket `pkg/item` can split a Markdown file into YAML frontmatter and a raw body, parse the YAML into an `Item` that keeps the original `yaml.Node` mapping (unknown keys included), mutate individual fields through setters that edit that node, and render `Bytes()` so that parse-then-write with no setter is `bytes.Equal` to the input. Store, comments, minting and CLI are not part of this ticket.

## Context (read first)
- **guide §4.3 `pkg/item`** — exact exported signatures. Copy them; do not rename. Store (`§4.4`) is the next ticket; do not add it here.
- **guide §1** — module `github.com/eisenwinter/awit`, only `gopkg.in/yaml.v3` plus stdlib in this package, Linux+Windows, tests are stdlib `testing` only. `refs` in frontmatter stay forward-slash. `Bytes()` always writes `\n`; `Split` must also accept `\r\n`.
- **guide §2 additional decisions** — new-item key order `id, title, brief, status, deps, labels, refs` (omit empty `assignee`/`claimed_at`); `deps`/`labels` flow style; `refs` block style; existing node `Style` preserved on edit; `brief` is `yaml.FoldedStyle` (`>-`) when it contains a newline or `len > 80`; `claimed_at` is `time.RFC3339` UTC seconds; body template `\n## Summary\n\n## Acceptance Criteria\n\n`.
- **guide §5** — `Split` accepts CRLF; `Bytes()` writes LF. Round-trip fixtures in this ticket are LF only.
- **spec Data model** (`plan/awit-implementation-plan.md`) — required keys `id`, `title`, `status`; unknown keys preserved; `status` is `open|in_progress|closed`.
- **yaml.v3 facts** (do not re-derive): `yaml.Unmarshal(front, &root)` yields `root.Kind == yaml.DocumentNode` and the mapping is `root.Content[0]`. Mapping `.Content` is `[k1, v1, k2, v2, ...]`. `Encoder.SetIndent(2)`; do **not** call `SetDocStartExplicit` / `SetDocEndExplicit`. Encode the **mapping node**, not the DocumentNode (encoding the document would emit a second `---`). `Encode` writes a trailing `\n`. Flow sequences render `[a, b]` (space after comma). Empty sequences render `[]`. Preserve `Style` on existing value nodes; `Node.SetString` may change quoting — never call it on a node you are only updating.

## Files
- Create: `pkg/item/frontmatter.go`
- Create: `pkg/item/frontmatter_test.go`
- Create: `pkg/item/reason.go`
- Create: `pkg/item/item.go`
- Create: `pkg/item/item_test.go`
- Modify: `go.mod` / `go.sum` via `go get gopkg.in/yaml.v3@v3.0.1` only.

## Interfaces
- Consumes: `gopkg.in/yaml.v3` and stdlib. No awit package.
- Produces (verbatim from guide §4.3):

```go
package item

type Status string
const (
    StatusOpen       Status = "open"
    StatusInProgress Status = "in_progress"
    StatusClosed     Status = "closed"
)
func ParseStatus(s string) (Status, error)

type Reason string
const (
    ReasonParse      Reason = "PARSE ERROR"
    ReasonConflict   Reason = "CONFLICT MARKERS"
    ReasonIDMismatch Reason = "ID MISMATCH"
    ReasonDuplicate  Reason = "DUPLICATE ID"
    ReasonDangling   Reason = "DANGLING DEP"
    ReasonCycle      Reason = "CYCLE"
)

type Broken struct {
    ID     string
    Path   string
    Reason Reason
    Detail string
}

type Item struct {
    ID        string
    Title     string
    Brief     string
    Status    Status
    Deps      []string
    Labels    []string
    Assignee  string
    ClaimedAt *time.Time
    Refs      []string
    Path      string
    // unexported: doc *yaml.Node (mapping), body []byte
}

func Split(data []byte) (front, body []byte, err error)
func HasConflictMarkers(data []byte) bool
func Parse(path string, data []byte) (*Item, error)
func New(id, title, brief string, deps, labels []string) *Item
func (it *Item) SetTitle(s string)
func (it *Item) SetBrief(s string)
func (it *Item) SetStatus(s Status)
func (it *Item) SetAssignee(s string)
func (it *Item) SetClaimedAt(t *time.Time)
func (it *Item) SetDeps(v []string)
func (it *Item) SetLabels(v []string)
func (it *Item) SetRefs(v []string)
func (it *Item) HasLabel(l string) bool
func (it *Item) Bytes() ([]byte, error)
func (it *Item) Body() []byte
```

- Produces (package-private, this ticket):

```go
var (
    ErrNoFrontmatter           = errors.New("item: no frontmatter")
    ErrUnterminatedFrontmatter = errors.New("item: unterminated frontmatter")
)
func (it *Item) findKey(key string) (k, v *yaml.Node, idx int)
func (it *Item) setScalar(key, value string)
func (it *Item) deleteKey(key string)
func (it *Item) setSeq(key string, values []string, defaultStyle yaml.Style)
```

## Steps

- [ ] **Step 1: Failing tests for `Split`, `HasConflictMarkers`, `FuzzSplit`.**
  Create `pkg/item/frontmatter_test.go`:

```go
package item

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestSplit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		in        []byte
		front     []byte
		body      []byte
		wantErr   error
	}{
		{
			name:  "LF",
			in:    []byte("---\nid: x\n---\nBODY"),
			front: []byte("id: x\n"),
			body:  []byte("BODY"),
		},
		{
			name:  "CRLF",
			in:    []byte("---\r\nid: x\r\n---\r\nBODY"),
			front: []byte("id: x\r\n"),
			body:  []byte("BODY"),
		},
		{
			name:    "no fence",
			in:      []byte("id: x\n---\nBODY"),
			wantErr: ErrNoFrontmatter,
		},
		{
			name:    "unterminated",
			in:      []byte("---\nid: x\n"),
			wantErr: ErrUnterminatedFrontmatter,
		},
		{
			name:    "opening fence without newline",
			in:      []byte("---"),
			wantErr: ErrNoFrontmatter,
		},
		{
			name:  "closing fence trimmed",
			in:    []byte("---\nid: x\n---  \nBODY"),
			front: []byte("id: x\n"),
			body:  []byte("BODY"),
		},
		{
			name:  "empty body",
			in:    []byte("---\nid: x\n---\n"),
			front: []byte("id: x\n"),
			body:  []byte(""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			front, body, err := Split(tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Split() err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Split() err = %v", err)
			}
			if !bytes.Equal(front, tt.front) {
				t.Fatalf("front = %q, want %q", front, tt.front)
			}
			if !bytes.Equal(body, tt.body) {
				t.Fatalf("body = %q, want %q", body, tt.body)
			}
		})
	}
}

func TestHasConflictMarkers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"head", "<<<<<<< HEAD\n", true},
		{"equals7", "=======\n", true},
		{"equals8", "========\n", true},
		{"equals6", "======\n", false},
		{"tail", ">>>>>>> branch\n", true},
		{"show delimiter", "===== REF 1/1 =====\n", false},
		{"trimmed head", "  <<<<<<< HEAD  \n", true},
		{"no space after chevrons", "<<<<<<<HEAD\n", false},
		{"clean", "---\nid: x\n---\nbody\n", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasConflictMarkers([]byte(tt.in)); got != tt.want {
				t.Fatalf("HasConflictMarkers(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func FuzzSplit(f *testing.F) {
	f.Add([]byte("---\nid: x\n---\nbody"))
	f.Add([]byte("---\r\nid: x\r\n---\r\nbody"))
	f.Add([]byte("nope"))
	f.Add([]byte("---\n"))
	f.Add([]byte(""))
	f.Add([]byte("---\n---\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		front, body, err := Split(data)
		if err != nil {
			return
		}
		if len(front)+len(body) > len(data) {
			t.Fatalf("len(front)+len(body)=%d > len(data)=%d", len(front)+len(body), len(data))
		}
	})
}

func TestSplitBodyMayContainFence(t *testing.T) {
	front, body, err := Split([]byte("---\nid: x\n---\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(front, []byte("id: x\n")) {
		t.Fatalf("front = %q", front)
	}
	if !bytes.Equal(body, []byte("---\n")) {
		t.Fatalf("body = %q", body)
	}
	_ = strings.Count
}
```

- [ ] **Step 2: Run it, see it fail to compile.**

```sh
go test ./pkg/item -run 'TestSplit|TestHasConflictMarkers|FuzzSplit' -v
```

  Expected: `undefined: Split`, `undefined: HasConflictMarkers`, `undefined: ErrNoFrontmatter`, `undefined: ErrUnterminatedFrontmatter`, ending `FAIL	github.com/eisenwinter/awit/pkg/item [build failed]`.

- [ ] **Step 3: Implement `frontmatter.go`.**
  Create `pkg/item/frontmatter.go`:

```go
package item

import (
	"bytes"
	"errors"
	"strings"
)

var (
	ErrNoFrontmatter           = errors.New("item: no frontmatter")
	ErrUnterminatedFrontmatter = errors.New("item: unterminated frontmatter")
)

// Split separates YAML frontmatter from the markdown body.
// data must start with "---\n" or "---\r\n". The closing fence is a line
// whose trimmed text is exactly "---". front is the bytes between the two
// fence lines; body is every byte after the closing fence line (including
// its terminating newline being consumed, not copied into body).
func Split(data []byte) (front, body []byte, err error) {
	var rest []byte
	switch {
	case bytes.HasPrefix(data, []byte("---\r\n")):
		rest = data[len("---\r\n"):]
	case bytes.HasPrefix(data, []byte("---\n")):
		rest = data[len("---\n"):]
	default:
		return nil, nil, ErrNoFrontmatter
	}
	lineStart := 0
	for lineStart <= len(rest) {
		lineEnd := lineStart
		for lineEnd < len(rest) && rest[lineEnd] != '\n' {
			lineEnd++
		}
		line := rest[lineStart:lineEnd]
		if bytes.Equal(bytes.TrimSpace(line), []byte("---")) {
			front = rest[:lineStart]
			after := lineEnd
			if after < len(rest) && rest[after] == '\n' {
				after++
			}
			return front, rest[after:], nil
		}
		if lineEnd == len(rest) {
			break
		}
		lineStart = lineEnd + 1
	}
	return nil, nil, ErrUnterminatedFrontmatter
}

// HasConflictMarkers reports git conflict markers. A line matches when its
// TrimSpace text has prefix "<<<<<<< " or ">>>>>>> ", or is entirely '='
// runes of length >= 7. A show delimiter "===== REF 1/1 =====" must not match
// (it is not all-equals, and it is not 7 leading equals as the whole line).
func HasConflictMarkers(data []byte) bool {
	lineStart := 0
	for lineStart <= len(data) {
		lineEnd := lineStart
		for lineEnd < len(data) && data[lineEnd] != '\n' {
			lineEnd++
		}
		trimmed := strings.TrimSpace(string(data[lineStart:lineEnd]))
		switch {
		case strings.HasPrefix(trimmed, "<<<<<<< "):
			return true
		case strings.HasPrefix(trimmed, ">>>>>>> "):
			return true
		case len(trimmed) >= 7 && strings.Trim(trimmed, "=") == "":
			return true
		}
		if lineEnd == len(data) {
			break
		}
		lineStart = lineEnd + 1
	}
	return false
}
```

- [ ] **Step 4: Run it, see it pass, commit.**

```sh
go test ./pkg/item -run 'TestSplit|TestHasConflictMarkers|FuzzSplit' -v
```

  Expected: `--- PASS: TestSplit` with subtests `LF`, `CRLF`, `no fence`, `unterminated`; `--- PASS: TestHasConflictMarkers`; `--- PASS: FuzzSplit` (seed corpus only). `ok  	github.com/eisenwinter/awit/pkg/item`.

```sh
gofmt -w pkg/item/frontmatter.go pkg/item/frontmatter_test.go
git add pkg/item/frontmatter.go pkg/item/frontmatter_test.go
git commit -m "item: split frontmatter and detect conflict markers"
```

- [ ] **Step 5: Failing tests for `ParseStatus`, `Parse`, `HasLabel`, `BodyRaw`.**
  Create `pkg/item/item_test.go` with the block below (setters and round-trip are Step 9).

```go
package item

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestParseStatus(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"open", "in_progress", "closed"} {
		got, err := ParseStatus(s)
		if err != nil {
			t.Fatalf("ParseStatus(%q) err = %v", s, err)
		}
		if string(got) != s {
			t.Fatalf("ParseStatus(%q) = %q", s, got)
		}
	}
	_, err := ParseStatus("Open")
	if err == nil {
		t.Fatal("ParseStatus(\"Open\") want error")
	}
	msg := err.Error()
	for _, v := range []string{"open", "in_progress", "closed"} {
		if !strings.Contains(msg, v) {
			t.Fatalf("error %q must list %q", msg, v)
		}
	}
}

const fullDoc = `---
id: AWIT-TEST0006
title: Implement OAuth2 bearer token extraction
brief: >-
  The API gateway rejects valid bearer tokens.
status: in_progress
deps: [AWIT-TEST0005]
labels: [auth, api, p1]
assignee: agent/claude
claimed_at: 2026-09-17T14:32:05Z
refs:
  - ../comments/AWIT-TEST0006/20260917T143205Z-claude.md
  - ../../docs/architecture/auth-middleware-spec.md
---

## Summary

Fix header parsing.
`

func TestParseFull(t *testing.T) {
	it, err := Parse("/abs/AWIT-TEST0006.md", []byte(fullDoc))
	if err != nil {
		t.Fatal(err)
	}
	if it.ID != "AWIT-TEST0006" || it.Title != "Implement OAuth2 bearer token extraction" {
		t.Fatalf("id/title = %q %q", it.ID, it.Title)
	}
	if it.Brief != "The API gateway rejects valid bearer tokens." {
		t.Fatalf("brief = %q", it.Brief)
	}
	if it.Status != StatusInProgress {
		t.Fatalf("status = %q", it.Status)
	}
	if len(it.Deps) != 1 || it.Deps[0] != "AWIT-TEST0005" {
		t.Fatalf("deps = %#v", it.Deps)
	}
	if len(it.Labels) != 3 || it.Labels[0] != "auth" || it.Labels[2] != "p1" {
		t.Fatalf("labels = %#v", it.Labels)
	}
	if it.Assignee != "agent/claude" {
		t.Fatalf("assignee = %q", it.Assignee)
	}
	if it.ClaimedAt == nil || !it.ClaimedAt.Equal(time.Date(2026, 9, 17, 14, 32, 5, 0, time.UTC)) {
		t.Fatalf("claimed_at = %v", it.ClaimedAt)
	}
	if len(it.Refs) != 2 || it.Refs[0] != "../comments/AWIT-TEST0006/20260917T143205Z-claude.md" {
		t.Fatalf("refs = %#v", it.Refs)
	}
	if it.Path != "/abs/AWIT-TEST0006.md" {
		t.Fatalf("path = %q", it.Path)
	}
}

func TestParseMissingTitle(t *testing.T) {
	_, err := Parse("x.md", []byte("---\nid: AWIT-TEST0001\nstatus: open\n---\n"))
	if err == nil || !strings.Contains(err.Error(), "title") {
		t.Fatalf("err = %v, want it to name title", err)
	}
}

func TestParseBadClaimedAt(t *testing.T) {
	_, err := Parse("x.md", []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nclaimed_at: not-a-time\n---\n"))
	if err == nil || !strings.Contains(err.Error(), "claimed_at") {
		t.Fatalf("err = %v, want claimed_at", err)
	}
}

func TestParseUnknownKeyKept(t *testing.T) {
	raw := []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nexternal: gitlab#42\n---\n")
	it, err := Parse("x.md", raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("external: gitlab#42")) {
		t.Fatalf("unknown key dropped:\n%s", got)
	}
}

func TestHasLabel(t *testing.T) {
	it, err := Parse("x.md", []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nlabels: [auth, p1]\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !it.HasLabel("auth") || !it.HasLabel("p1") || it.HasLabel("AUTH") || it.HasLabel("p0") {
		t.Fatalf("HasLabel mismatch labels=%v", it.Labels)
	}
}

func TestBodyRaw(t *testing.T) {
	it, err := Parse("x.md", []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\n---\n\n## Summary\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(it.Body(), []byte("\n## Summary\n")) {
		t.Fatalf("Body() = %q", it.Body())
	}
}
```

- [ ] **Step 6: Run it, see it fail.**

```sh
go get gopkg.in/yaml.v3@v3.0.1
go test ./pkg/item -run 'TestParseStatus|TestParseFull|TestParseMissingTitle|TestParseBadClaimedAt|TestParseUnknownKeyKept|TestHasLabel|TestBodyRaw' -v
```

  Expected: `undefined: ParseStatus`, `undefined: Parse`, `undefined: StatusInProgress`, ending `[build failed]`.

- [ ] **Step 7: Implement `reason.go` and `item.go` (types, Parse, Body, HasLabel). Leave New/setters/Bytes as stubs that compile if you must — or implement Bytes now as encoding `it.doc` so `TestParseUnknownKeyKept` can call it. Implement Bytes in this step; New/setters still wait.**
  Create `pkg/item/reason.go`:

```go
package item

import "fmt"

type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusClosed     Status = "closed"
)

func ParseStatus(s string) (Status, error) {
	switch Status(s) {
	case StatusOpen, StatusInProgress, StatusClosed:
		return Status(s), nil
	default:
		return "", fmt.Errorf("item: unknown status %q (open|in_progress|closed)", s)
	}
}

type Reason string

const (
	ReasonParse      Reason = "PARSE ERROR"
	ReasonConflict   Reason = "CONFLICT MARKERS"
	ReasonIDMismatch Reason = "ID MISMATCH"
	ReasonDuplicate  Reason = "DUPLICATE ID"
	ReasonDangling   Reason = "DANGLING DEP"
	ReasonCycle      Reason = "CYCLE"
)

type Broken struct {
	ID     string
	Path   string
	Reason Reason
	Detail string
}
```

  Create `pkg/item/item.go` with `Item`, `Parse`, `Bytes`, `Body`, `HasLabel`. `Parse` **must**:

  1. `Split(data)` then `yaml.Unmarshal(front, &root)` into a `yaml.Node`.
  2. Mapping = `root.Content[0]` (DocumentNode). Error if missing or not a mapping.
  3. Walk `i += 2` key/value pairs. Switch on `key.Value`. Unknown keys are left in `doc`.
  4. Missing `id`, `title`, or `status` → `fmt.Errorf("item: missing required key %q", key)` (the error string must contain the key name).
  5. `status` goes through `ParseStatus`. `claimed_at` through `time.Parse(time.RFC3339, …)` wrapped as `item: claimed_at: %w`. Sequences collected from child scalar `.Value`.
  6. Store `path` on `Item.Path`, mapping on unexported `doc`, body on unexported `body`.

  `Bytes`:

```go
func (it *Item) Bytes() ([]byte, error) {
	var yb bytes.Buffer
	enc := yaml.NewEncoder(&yb)
	enc.SetIndent(2)
	if err := enc.Encode(it.doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(yb.Bytes())
	buf.WriteString("---\n")
	buf.Write(it.body)
	return buf.Bytes(), nil
}
```

  `HasLabel` is exact string match over `it.Labels`. `Body` returns `it.body`.

- [ ] **Step 8: Run parse tests, see them pass, commit.**

```sh
go test ./pkg/item -run 'TestParseStatus|TestParseFull|TestParseMissingTitle|TestParseBadClaimedAt|TestParseUnknownKeyKept|TestHasLabel|TestBodyRaw' -v
```

  Expected: every listed test `PASS`. `ok  	github.com/eisenwinter/awit/pkg/item`.

```sh
gofmt -w pkg/item/reason.go pkg/item/item.go pkg/item/item_test.go
git add pkg/item go.mod go.sum
git commit -m "item: parse yaml.Node mapping and render Bytes"
```

- [ ] **Step 9: Failing round-trip and setter tests.**
  Append to `pkg/item/item_test.go`:

```go
func TestRoundTripByteIdentical(t *testing.T) {
	docs := []string{
		"---\nid: AWIT-TEST0001\ntitle: Title\nstatus: open\n---\nbody\n",
		fullDoc,
		"---\nid: AWIT-TEST0001\ntitle: Title\nstatus: open\nexternal: gitlab#42 # comment\n---\n",
		"---\nid: AWIT-TEST0001\ntitle: Title\nstatus: open\ndeps: []\n---\n",
	}
	for i, raw := range docs {
		it, err := Parse("x.md", []byte(raw))
		if err != nil {
			t.Fatalf("doc %d parse: %v", i, err)
		}
		got, err := it.Bytes()
		if err != nil {
			t.Fatalf("doc %d bytes: %v", i, err)
		}
		if !bytes.Equal(got, []byte(raw)) {
			t.Fatalf("doc %d not byte-identical\ngot:\n%s\nwant:\n%s", i, got, raw)
		}
	}
}

func TestNewBytesGolden(t *testing.T) {
	it := New("AWIT-TEST0001", "Title", "Brief.", []string{"AWIT-TEST0002"}, []string{"auth", "p1"})
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("---\nid: AWIT-TEST0001\ntitle: Title\nbrief: Brief.\nstatus: open\ndeps: [AWIT-TEST0002]\nlabels: [auth, p1]\nrefs: []\n---\n\n## Summary\n\n## Acceptance Criteria\n\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("New Bytes mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestSetStatusOneLineDiff(t *testing.T) {
	raw := "---\nid: AWIT-TEST0001\ntitle: Title\nbrief: Brief.\nstatus: open\ndeps: []\nlabels: [auth]\nrefs: []\n---\n\n## Summary\n"
	it, err := Parse("x.md", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	it.SetStatus(StatusClosed)
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	wantLines := strings.Split(raw, "\n")
	gotLines := strings.Split(string(got), "\n")
	if len(gotLines) != len(wantLines) {
		t.Fatalf("line count got %d want %d\ngot:\n%s", len(gotLines), len(wantLines), got)
	}
	changed := 0
	var line string
	for i := range wantLines {
		if gotLines[i] != wantLines[i] {
			changed++
			line = gotLines[i]
		}
	}
	if changed != 1 {
		t.Fatalf("changed %d lines, want 1\ngot:\n%s", changed, got)
	}
	if line != "status: closed" {
		t.Fatalf("changed line = %q, want %q", line, "status: closed")
	}
}

func TestSetAssigneeEmptyDeletesKey(t *testing.T) {
	raw := "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nassignee: agent/claude\n---\n"
	it, err := Parse("x.md", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	it.SetAssignee("")
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("assignee:")) {
		t.Fatalf("assignee key still present:\n%s", got)
	}
}

func TestSetClaimedAtNilDeletesKey(t *testing.T) {
	raw := "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nclaimed_at: 2026-09-17T14:32:05Z\n---\n"
	it, err := Parse("x.md", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	it.SetClaimedAt(nil)
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("claimed_at:")) {
		t.Fatalf("claimed_at key still present:\n%s", got)
	}
}

func TestSetLabelsPreservesFlowStyle(t *testing.T) {
	raw := "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nlabels: [auth, p1]\n---\n"
	it, err := Parse("x.md", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	it.SetLabels([]string{"x"})
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("labels: [x]")) {
		t.Fatalf("flow style lost:\n%s", got)
	}
}

func TestSetRefsBlockStyle(t *testing.T) {
	it := New("AWIT-TEST0001", "T", "B", nil, nil)
	it.SetRefs([]string{"../comments/AWIT-TEST0001/a.md"})
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("refs: [")) {
		t.Fatalf("want block refs, got flow:\n%s", got)
	}
	if !bytes.Contains(got, []byte("refs:\n")) {
		t.Fatalf("missing block refs key:\n%s", got)
	}
	if !bytes.Contains(got, []byte("- ../comments/AWIT-TEST0001/a.md")) {
		t.Fatalf("missing ref entry:\n%s", got)
	}
}

func TestSetBriefFolded(t *testing.T) {
	brief := strings.Repeat("abcdefghij", 12) // 120
	it := New("AWIT-TEST0001", "T", "short", nil, nil)
	it.SetBrief(brief)
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("brief: >-")) {
		t.Fatalf("want folded brief:\n%s", got)
	}
}
```

  Do **not** loosen `bytes.Equal` in `TestRoundTripByteIdentical`. If a document fails, the encoder is wrapping the DocumentNode, changing indent, or rebuilding YAML from struct fields — fix `Bytes`/`Parse`, not the assertion.

- [ ] **Step 10: Run it, see New/setters fail.**

```sh
go test ./pkg/item -run 'TestRoundTripByteIdentical|TestNewBytesGolden|TestSetStatusOneLineDiff|TestSetAssigneeEmptyDeletesKey|TestSetClaimedAtNilDeletesKey|TestSetLabelsPreservesFlowStyle|TestSetRefsBlockStyle|TestSetBriefFolded' -v
```

  Expected: `undefined: New` (and/or `undefined: SetStatus` …) `[build failed]`, **or** if you declared empty stubs, `TestNewBytesGolden` / `TestSetStatusOneLineDiff` FAIL with a bytes mismatch.

- [ ] **Step 11: Implement `New` and setters on `item.go`.**
  Canonical `New` key order: `id, title, brief, status, deps, labels, refs`. Omit `assignee` and `claimed_at`. `deps`/`labels` use `yaml.FlowStyle`. `refs` uses default (block) style, empty Content. `Status` is `StatusOpen`. Copy `deps`/`labels` so the caller cannot mutate the item. `body` is exactly `\n## Summary\n\n## Acceptance Criteria\n\n`. Brief node: `yaml.FoldedStyle` when `strings.Contains(s, "\n") || len(s) > 80`, otherwise plain.

  Helpers (keep unexported):

```go
func (it *Item) findKey(key string) (k, v *yaml.Node, idx int) {
	if it.doc == nil {
		return nil, nil, -1
	}
	for i := 0; i+1 < len(it.doc.Content); i += 2 {
		if it.doc.Content[i].Value == key {
			return it.doc.Content[i], it.doc.Content[i+1], i
		}
	}
	return nil, nil, -1
}

func (it *Item) setScalar(key, value string) {
	_, val, idx := it.findKey(key)
	if idx >= 0 {
		val.Kind = yaml.ScalarNode
		val.Tag = "!!str"
		val.Value = value
		return
	}
	it.doc.Content = append(it.doc.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
	)
}

func (it *Item) deleteKey(key string) {
	_, _, idx := it.findKey(key)
	if idx < 0 {
		return
	}
	it.doc.Content = append(it.doc.Content[:idx], it.doc.Content[idx+2:]...)
}

func (it *Item) setSeq(key string, values []string, defaultStyle yaml.Style) {
	nodes := make([]*yaml.Node, 0, len(values))
	for _, v := range values {
		nodes = append(nodes, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v})
	}
	_, val, idx := it.findKey(key)
	if idx >= 0 {
		val.Kind = yaml.SequenceNode
		val.Tag = "!!seq"
		val.Value = ""
		val.Content = nodes
		return
	}
	it.doc.Content = append(it.doc.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: defaultStyle, Content: nodes},
	)
}
```

  `setSeq` **must not** overwrite `val.Style` when the key already exists. `SetStatus` updates `it.Status` and `setScalar("status", string(s))` only — no other keys. `SetAssignee("")` and `SetClaimedAt(nil)` call `deleteKey`. Non-nil `SetClaimedAt` writes `t.UTC().Truncate(time.Second).Format(time.RFC3339)`. `SetBrief` sets `FoldedStyle` when newline or `len > 80`, else `Style = 0`. `SetDeps` defaultStyle `yaml.FlowStyle`; `SetLabels` same; `SetRefs` defaultStyle `0`.

- [ ] **Step 12: Run the whole package, see it pass, commit.**

```sh
go test ./pkg/item -count=1
go test ./pkg/item -fuzz=FuzzSplit -fuzztime=3s
```

  Expected: `PASS` / `ok  	github.com/eisenwinter/awit/pkg/item` and the fuzz run prints `PASS` with no panic. `gofmt -l pkg/item` prints nothing.

```sh
gofmt -w pkg/item/*.go
git add pkg/item
git commit -m "item: New, setters, byte-identical round-trip"
```

## Acceptance Criteria
- `go test ./pkg/item -count=1` passes.
- `go test ./pkg/item -run TestRoundTripByteIdentical` uses `bytes.Equal` on four documents (minimal; full with folded brief + flow deps + block refs + assignee + claimed_at; `external: gitlab#42` plus `# comment`; `deps: []`).
- `go test ./pkg/item -run TestSetStatusOneLineDiff` — exactly one line changes, that line is `status: closed`.
- `go test ./pkg/item -run TestNewBytesGolden` — exact bytes for `New("AWIT-TEST0001", "Title", "Brief.", []string{"AWIT-TEST0002"}, []string{"auth", "p1"})`.
- `go test ./pkg/item -run TestHasConflictMarkers` — `===== REF 1/1 =====` is false; `=======` is true.
- `gofmt -l pkg/item` is empty. No `pkg/item/store.go`.

## Out of scope
- `Store`, `Find`, `Init`, `LoadAll`, `Save`, `Mint`, comments, `BrokenError`.
- CLI, graph, fixtures under `testdata/`.
- Changing `Bytes()` to rebuild YAML from struct fields, or loosening `bytes.Equal`.
