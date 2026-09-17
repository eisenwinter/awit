---
id: AWIT-0ND56H3G
title: awit create
brief: >-
  Add `awit create`: mint (or accept `--id`), merge default labels, validate
  deps, write a new item, and print it through format.WriteOne with State
  "ready" and Unblocks 0 without building the graph.
status: closed
deps: [AWIT-0ND56G3G]
labels: [phase1, p0]
refs:
  - ../../plan/implementation-guide.md
  - ../../plan/awit-implementation-plan.md
---

## Summary

After this ticket `awit create <title> --brief ...` mints a snowflake ID (or
uses `--id`), writes one Markdown item via `item.New` + `Store.Save`, and
prints it with `format.WriteOne`. State is always reported `ready` and
Unblocks always `0` — that is a deliberate v1 lie; true ready/blocked comes
from `awit list`. Tests drive `Main` against `initRepo` temp directories; no
fixtures.

## Context (read first)

- **guide §1** — `filepath`, `Error: ` via `report`, exit 0/1/2, `t.TempDir()`,
  urfave/cli v3, yaml.v3 only extra deps.
- **guide §2 body template** — `item.New` already writes
  `\n## Summary\n\n## Acceptance Criteria\n\n`. Do not rebuild the body here.
- **guide §2 key order** — New omits empty `assignee` / `claimed_at`. `--assign`
  sets Assignee only; status stays `open`; do **not** set `claimed_at`.
- **guide §4.1** — `id.Valid(prefix, s)`, `id.Mint` is used inside `Store.Mint`.
- **guide §4.2** — `Config.DefaultLabels`. Merge: config labels first, then
  flag labels, first-wins dedupe (skip a label already present).
- **guide §4.3 / §4.4** — `item.New`, `Store.Mint`, `Exists`, `Save`, `ParseStatus`
  is not used here (status is always open).
- **guide §4.7** — `format.Detect(flag, stdoutFile)`, `format.WriteOne`. Compact
  `Line` is `[ID] Title | labels-or- | Unblocks: N` plus newline.
  Detect takes `*os.File`: type-assert `cmd.Root().Writer`; tests use a
  `bytes.Buffer` so the assert fails and Detect sees a non-TTY (compact).
- **guide §4.11** — `openStore`; create command shape (copy verbatim, then fill
  Action). `--brief` is `Required: true` (missing flag = urfave usage error,
  exit 2). `-d/--dep` and `-l/--label` are `StringSliceFlag`. Also split each
  element on commas yourself so `-d A,B` works even if urfave does not.
- **guide §5** — `run` / `initRepo` / `readItem` already exist in
  `helpers_test.go` (AWIT-0ND56G3G). Do not redeclare them.
- **AWIT-0ND56F3G** provides `pkg/format`. It is not in this ticket's `deps`
  list; it must still be closed before this package compiles.
- Title = `strings.Join(cmd.Args().Slice(), " ")`. Empty (no args, or only
  spaces) → error `create needs a title`.
- Each dep: `id.Valid(s.Config.Prefix, dep)` else `invalid id %s`; then
  `s.Exists(dep)` else `unknown dep %s`. `--id`: Valid else `invalid id %s`;
  `s.Exists` else `item %s already exists`. Mint when `--id` is empty:
  `s.Mint(time.Now())`.
- Document in a comment on the WriteOne call: State is reported ready without
  building the graph — deliberate v1; true state is `awit list`.

## Files

- Create: `internal/cli/create.go`
- Create: `internal/cli/create_test.go`
- Modify: `internal/cli/app.go` — append `createCmd` to `newRoot`'s `Commands`
  (next to `initCmd`). Do not add `createCmd` via `init()`.

## Interfaces

Consumes:

```go
func openStore(cmd *cli.Command) (*item.Store, error)
func (s *Store) Mint(now time.Time) (string, error)
func (s *Store) Exists(id string) bool
func (s *Store) Save(it *Item) error
func New(id, title, brief string, deps, labels []string) *Item
func (it *Item) SetAssignee(s string) // "" deletes; non-empty writes the key
func Valid(prefix, s string) bool     // package id
func Detect(flag string, stdout *os.File) (Format, error)
func WriteOne(w io.Writer, f Format, e Entry) error
```

Produces:

```go
var createCmd *cli.Command
func createAction(ctx context.Context, cmd *cli.Command) error
func detectFormat(cmd *cli.Command) (format.Format, error)
func parseIDList(values []string) []string // comma-split + trim + drop empty
func mergeLabels(defaults, flags []string) []string // defaults first, first-wins
```

`parseIDList` / `mergeLabels` / `detectFormat` are package-private. If a later
file in `package cli` needs the same comma-split, it must call `parseIDList`
— do not redeclare it. `toEntry` is **not** this ticket.

## Steps

- [ ] **Step 1: Failing tests.**

  Create `internal/cli/create_test.go`:

```go
package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/id"
	"github.com/eisenwinter/awit/pkg/item"
)

func TestCreateMintsAndWrites(t *testing.T) {
	dir := initRepo(t)
	code, stdout, stderr := run(t, "--repo", dir, "create", "--brief", "Fix header parsing.", "Implement", "OAuth2", "token", "extraction")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	it := onlyItem(t, dir)
	if it.Title != "Implement OAuth2 token extraction" {
		t.Fatalf("title = %q", it.Title)
	}
	if it.Brief != "Fix header parsing." || it.Status != item.StatusOpen {
		t.Fatalf("brief/status = %q %q", it.Brief, it.Status)
	}
	if it.Assignee != "" || it.ClaimedAt != nil {
		t.Fatalf("assignee/claimed_at set: %q %v", it.Assignee, it.ClaimedAt)
	}
	if string(it.Body()) != "\n## Summary\n\n## Acceptance Criteria\n\n" {
		t.Fatalf("body = %q", it.Body())
	}
	if !strings.Contains(string(it.Body()), "## Acceptance Criteria") {
		t.Fatal("body missing ## Acceptance Criteria")
	}
	if !id.Valid("AWIT", it.ID) {
		t.Fatalf("minted id %q is not valid", it.ID)
	}
	wantLine := "[" + it.ID + "] Implement OAuth2 token extraction | - | Unblocks: 0\n"
	if stdout != wantLine {
		t.Fatalf("stdout = %q, want %q", stdout, wantLine)
	}
}

func TestCreateRequiresBrief(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "a title")
	if code != 2 {
		t.Fatalf("exit %d, want 2 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stderr, "brief") {
		t.Fatalf("stderr = %q, want it to mention brief", stderr)
	}
}

func TestCreateNeedsTitle(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "A brief.")
	if code != 1 {
		t.Fatalf("exit %d, want 1 (stderr %q)", code, stderr)
	}
	if stderr != "Error: create needs a title\n" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestCreateWithDeps(t *testing.T) {
	dir := initRepo(t)
	a := createOne(t, dir, "A", "Brief A.")
	code, stdout, stderr := run(t, "--repo", dir, "create", "--brief", "Brief B.", "-d", a.ID, "B")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	b := readItem(t, dir, itemIDFromCompact(t, stdout))
	if len(b.Deps) != 1 || b.Deps[0] != a.ID {
		t.Fatalf("deps = %v", b.Deps)
	}
}

func TestCreateUnknownDep(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "-d", "AWIT-TEST0001", "X")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if stderr != "Error: unknown dep AWIT-TEST0001\n" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestCreateInvalidDepID(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "-d", "not-an-id", "X")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if stderr != "Error: invalid id not-an-id\n" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestCreateIDOverride(t *testing.T) {
	dir := initRepo(t)
	code, stdout, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", "AWIT-TEST0001", "Override")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	it := readItem(t, dir, "AWIT-TEST0001")
	if it.Title != "Override" || it.ID != "AWIT-TEST0001" {
		t.Fatalf("item = %+v", it)
	}
	if !strings.HasPrefix(stdout, "[AWIT-TEST0001] ") {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestCreateIDOverrideExists(t *testing.T) {
	dir := initRepo(t)
	createOne(t, dir, "First", "B.")
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", onlyItem(t, dir).ID, "Second")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	got := onlyItem(t, dir)
	if !strings.HasPrefix(stderr, "Error: item ") || !strings.HasSuffix(stderr, " already exists\n") {
		t.Fatalf("stderr = %q", stderr)
	}
	if !strings.Contains(stderr, got.ID) {
		t.Fatalf("stderr = %q, want the existing id", stderr)
	}
}

func TestCreateDefaultLabelsMerged(t *testing.T) {
	dir := initRepo(t)
	writeDefaultLabels(t, dir, []byte("prefix: AWIT\ndefault_labels: [phase1, p0]\nstale_claim: 2h\n"))
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "-l", "extra", "Merged")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	it := onlyItem(t, dir)
	if strings.Join(it.Labels, ",") != "phase1,p0,extra" {
		t.Fatalf("labels = %v", it.Labels)
	}
}

func TestCreateLabelDedupe(t *testing.T) {
	dir := initRepo(t)
	writeDefaultLabels(t, dir, []byte("prefix: AWIT\ndefault_labels: [p0]\nstale_claim: 2h\n"))
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "-l", "p0,p1", "Dedupe")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	it := onlyItem(t, dir)
	if strings.Join(it.Labels, ",") != "p0,p1" {
		t.Fatalf("labels = %v", it.Labels)
	}
}

func TestCreateAssign(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--assign", "agent/claude", "Assigned")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	it := onlyItem(t, dir)
	if it.Assignee != "agent/claude" || it.Status != item.StatusOpen || it.ClaimedAt != nil {
		t.Fatalf("assignee=%q status=%q claimed=%v", it.Assignee, it.Status, it.ClaimedAt)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".awit", "items", it.ID+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "claimed_at") {
		t.Fatalf("file contains claimed_at:\n%s", raw)
	}
}

func TestCreateJSON(t *testing.T) {
	dir := initRepo(t)
	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "create", "--brief", "One sentence.", "JSON item")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	var e format.Entry
	if err := json.Unmarshal([]byte(stdout), &e); err != nil {
		t.Fatalf("json: %v (stdout %q)", err, stdout)
	}
	if e.Title != "JSON item" || e.Brief != "One sentence." || e.Status != "open" || e.State != "ready" || e.Unblocks != 0 {
		t.Fatalf("entry = %+v", e)
	}
	if !id.Valid("AWIT", e.ID) {
		t.Fatalf("id %q", e.ID)
	}
}

func createOne(t *testing.T, repo, title, brief string) *item.Item {
	t.Helper()
	code, stdout, stderr := run(t, "--repo", repo, "create", "--brief", brief, title)
	if code != 0 {
		t.Fatalf("create %q: exit %d stderr %q", title, code, stderr)
	}
	return readItem(t, repo, itemIDFromCompact(t, stdout))
}

func onlyItem(t *testing.T, repo string) *item.Item {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(repo, ".awit", "items"))
	if err != nil {
		t.Fatal(err)
	}
	var md []os.DirEntry
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".md") {
			md = append(md, e)
		}
	}
	if len(md) != 1 {
		t.Fatalf("want 1 item, got %d", len(md))
	}
	return readItem(t, repo, strings.TrimSuffix(md[0].Name(), ".md"))
}

func itemIDFromCompact(t *testing.T, stdout string) string {
	t.Helper()
	if !strings.HasPrefix(stdout, "[") {
		t.Fatalf("stdout %q", stdout)
	}
	end := strings.IndexByte(stdout, ']')
	if end < 2 {
		t.Fatalf("stdout %q", stdout)
	}
	return stdout[1:end]
}

func writeDefaultLabels(t *testing.T, repo string, yaml []byte) {
	t.Helper()
	p := filepath.Join(repo, ".awit", "config.yaml")
	if err := os.WriteFile(p, yaml, 0o644); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run it, see it fail.**

  ```bash
  go test ./internal/cli -run TestCreate -v
  ```

  Expected: build failed, `undefined: createCmd` is **not** required (tests
  call `Main`). Runtime: `create` is unknown, exit 2, stderr
  `unknown command "create" (run "awit --help")` on every test that reaches
  `run`. FAIL.

- [ ] **Step 3: Implement `create`.**

  Create `internal/cli/create.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/id"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var createCmd = &cli.Command{
	Name:      "create",
	Usage:     "Mint an ID and write a new item",
	ArgsUsage: "<title>",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "brief", Usage: "one to three sentences", Required: true},
		&cli.StringSliceFlag{Name: "dep", Aliases: []string{"d"}},
		&cli.StringSliceFlag{Name: "label", Aliases: []string{"l"}},
		&cli.StringFlag{Name: "assign"},
		&cli.StringFlag{Name: "id", Usage: "override minted id (imports)"},
	},
	Action: createAction,
}

func parseIDList(values []string) []string {
	var out []string
	for _, v := range values {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func mergeLabels(defaults, flags []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, src := range [][]string{defaults, flags} {
		for _, l := range src {
			l = strings.TrimSpace(l)
			if l == "" || seen[l] {
				continue
			}
			seen[l] = true
			out = append(out, l)
		}
	}
	if out == nil {
		out = []string{}
	}
	return out
}

func detectFormat(cmd *cli.Command) (format.Format, error) {
	var stdout *os.File
	if f, ok := cmd.Root().Writer.(*os.File); ok {
		stdout = f
	}
	return format.Detect(cmd.Root().String("format"), stdout)
}

func createAction(_ context.Context, cmd *cli.Command) error {
	title := strings.TrimSpace(strings.Join(cmd.Args().Slice(), " "))
	if title == "" {
		return fmt.Errorf("create needs a title")
	}
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	itemID := cmd.String("id")
	if itemID == "" {
		itemID, err = s.Mint(time.Now())
		if err != nil {
			return err
		}
	} else {
		if !id.Valid(s.Config.Prefix, itemID) {
			return fmt.Errorf("invalid id %s", itemID)
		}
		if s.Exists(itemID) {
			return fmt.Errorf("item %s already exists", itemID)
		}
	}
	deps := parseIDList(cmd.StringSlice("dep"))
	for _, d := range deps {
		if !id.Valid(s.Config.Prefix, d) {
			return fmt.Errorf("invalid id %s", d)
		}
		if !s.Exists(d) {
			return fmt.Errorf("unknown dep %s", d)
		}
	}
	labels := mergeLabels(s.Config.DefaultLabels, parseIDList(cmd.StringSlice("label")))
	it := item.New(itemID, title, cmd.String("brief"), deps, labels)
	if a := cmd.String("assign"); a != "" {
		it.SetAssignee(a)
	}
	if err := s.Save(it); err != nil {
		return err
	}
	f, err := detectFormat(cmd)
	if err != nil {
		return err
	}
	// State is reported ready without building the graph — deliberate v1;
	// true state is `awit list`.
	return format.WriteOne(cmd.Writer, f, format.Entry{
		ID:       it.ID,
		Title:    it.Title,
		Brief:    it.Brief,
		Status:   string(it.Status),
		State:    "ready",
		Labels:   it.Labels,
		Deps:     it.Deps,
		Assignee: it.Assignee,
		Unblocks: 0,
	})
}
```

  In `newRoot`, Commands becomes:

```go
		Commands: []*cli.Command{
			initCmd,
			createCmd,
		},
```

- [ ] **Step 4: Run it, see it pass, commit.**

  ```bash
  go test ./internal/cli -run TestCreate -v
  ```

  Expected: `--- PASS` for `TestCreateMintsAndWrites`, `TestCreateRequiresBrief`,
  `TestCreateNeedsTitle`, `TestCreateWithDeps`, `TestCreateUnknownDep`,
  `TestCreateInvalidDepID`, `TestCreateIDOverride`, `TestCreateIDOverrideExists`,
  `TestCreateDefaultLabelsMerged`, `TestCreateLabelDedupe`, `TestCreateAssign`,
  `TestCreateJSON`.

  ```bash
  go test ./internal/cli -count=1
  gofmt -w internal/cli
  git add internal/cli/create.go internal/cli/create_test.go internal/cli/app.go
  git commit -m "cli/create: mint items and render via WriteOne"
  ```

- [ ] **Step 5: Full check and close.**

  ```bash
  go build ./... && go vet ./... && go test ./internal/cli -count=1
  ```

  Close this ticket (`status: closed`, comment file with the test output, ref,
  commit `tickets: close AWIT-0ND56H3G`).

## Acceptance Criteria

- `go test ./internal/cli -run TestCreate -v` — all twelve tests named above PASS.
- `go test ./internal/cli -count=1` — init and skeleton tests still PASS.
- Creating with `--brief` missing exits 2; creating with no title args exits 1
  and stderr is exactly `Error: create needs a title`.
- A successful create file body contains `## Acceptance Criteria`; compact
  stdout is `[<id>] <title> | - | Unblocks: 0` (or labels instead of `-`).
- `--format json` stdout unmarshals to an object with `"state":"ready"` and
  `"unblocks":0` even when `-d` listed a dependency.
- `--assign` writes `assignee` and does not write `claimed_at`; status is `open`.
- `gofmt -l internal/cli` prints nothing.

## Out of scope

- Graph / ready-blocked classification (`awit list`). Create always prints
  `state: ready`, `unblocks: 0`.
- `update`, `close`, `release`, comments, git commit, locking.
- Redeclaring `run`, `initRepo`, `readItem`, `openStore`.
- Changing `item.New` or `pkg/format`.
