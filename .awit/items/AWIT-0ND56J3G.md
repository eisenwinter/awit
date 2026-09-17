---
id: AWIT-0ND56J3G
title: 'awit list (-s, -l, --ready, --blocked, --quarantined, --format)'
brief: >-
  Add `awit list` with status, label, and ready/blocked/quarantined filters,
  plus shared `loadGraph` and `toEntry` on the root CLI package. Default
  listing is g.Order including closed; --ready alone uses g.Ready().
status: closed
deps: [AWIT-0ND56G3G, AWIT-0ND56F3G, AWIT-0ND56Q3G]
labels: [phase2, p1]
refs:
  - ../../plan/implementation-guide.md
  - ../../plan/awit-implementation-plan.md
---

## Summary
After this ticket `internal/cli/list.go` registers `awit list`. Flags are `-s/--status`, `-l/--label` (AND across flags, OR within a flag via `SplitLabels`/`FilterLabels`), and `--ready` / `--blocked` / `--quarantined` which are OR-ed. When none of those three set-filters is set, the listing is every node in `g.Order` including closed. `--ready` alone (no `--blocked`, no `--quarantined`) uses `g.Ready()` so UnblockCount-desc order is visible. Tests pass `--format compact` except `TestListJSON`. `loadGraph` and `toEntry` live in `app.go` for later commands.

## Context (read first)
- Guide §2 decision 1 — AND across `-l` flags, OR within a flag. `SplitLabels(["p0,p1","auth"])` → `[["p0","p1"],["auth"]]`. `graph.FilterLabels` already implements that.
- Guide §4.6 — `Ready()` is UnblockCount desc then ID asc. `Blocked` / `Quarantined` / `Closed` / `Order` are ID asc. `Order` contains every parseable node, including closed and quarantined.
- Guide §4.7 — `format.Write`, `format.Line`, `format.Entry`. Compact line is `[ID] Title | labels | Unblocks: N` with `-` when labels are empty and ` | QUARANTINED` when `State == "quarantined"`. JSON is an indented array. This ticket converts `*graph.Node` → `format.Entry` in `toEntry`; `pkg/format` never imports `pkg/graph`.
- Guide §4.11 — `loadGraph = store.LoadAll + graph.Build`. `toEntry` is defined here. If AWIT-0ND56S3G already defined `loadGraph`, do not redeclare it — the body must be identical.
- Guide §8 `clean` fixture (AWIT-0ND56N3G):
  - `0001` open, labels `auth,p1`, Ready, UnblockCount 2, title `Implement OAuth2 bearer token extraction`
  - `0002` open, labels `db`, Ready, UnblockCount 0, title `Update database migration scripts`
  - `0003` open, no labels, Blocked, UnblockCount 1, title `Add E2E auth tests`
  - `0004` open, labels `p0`, Blocked, UnblockCount 0, title `Rotate API tokens`
  - `0005` closed, no labels, title `Write auth middleware spec`
  - `0006` in_progress, no labels, Ready, UnblockCount 0, title `Implement refresh-token rotation`
  - Ready order: `0001`, `0002`, `0006`. `-l p0 -l auth` matches nobody. `-l auth,db` matches `0001` and `0002`.
- Guide §8 `cyclic` fixture (AWIT-0ND56P3G): `0001`/`0002`/`0003` triangle quarantined, `0004` self-loop quarantined, `0005` clean. `--quarantined` lists four nodes, each compact line ending ` | QUARANTINED`. `0005` is absent.
- Guide §5 — `run`, `copyFixture`, `golden` already exist. Always pass `--repo`. Tests that are not JSON pass `--format compact` even though a `bytes.Buffer` already selects compact.
- `detectFormat` is defined by AWIT-0ND56H3G. This ticket's deps do not include H3G; define `detectFormat` if absent (exact code in Step 3).
- `-s` takes `open`, `in_progress`, or `closed` (`item.ParseStatus`). Repeatable / comma-separated values are OR. Unknown status is the `ParseStatus` error (exit 1).
- Set-filters OR: `--ready --blocked` is the union, walked in `g.Order` (ID asc), not Ready-order concatenated with Blocked-order.

## Files
- Create: `internal/cli/list.go`
- Create: `internal/cli/list_test.go`
- Create: `testdata/golden/list_clean_compact.golden`
- Modify: `internal/cli/app.go` — add `loadGraph` (if absent) and `toEntry`; append `listCmd` to `Commands`. Keep every command already in the slice.

## Interfaces
- Consumes:
  ```go
  func openStore(cmd *cli.Command) (*item.Store, error)
  func SplitLabels(flags []string) [][]string
  func FilterLabels(nodes []*graph.Node, groups [][]string) []*graph.Node
  func Detect(flag string, stdout *os.File) (format.Format, error)
  func Write(w io.Writer, f format.Format, entries []format.Entry) error
  func ParseStatus(s string) (item.Status, error)
  ```
- Produces (guide §4.11, this ticket owns them):
  ```go
  func loadGraph(s *item.Store) (*graph.Graph, error)
  func toEntry(n *graph.Node) format.Entry
  ```
- Produces (package-private):
  ```go
  var listCmd *cli.Command
  func listAction(_ context.Context, cmd *cli.Command) error
  func detectFormat(cmd *cli.Command) (format.Format, error) // define if absent
  ```

## Steps

- [ ] **Step 1: Write the golden and failing tests.**

  `testdata/golden/list_clean_compact.golden` (trailing newline, ID order, closed included):

  ```text
  [AWIT-TEST0001] Implement OAuth2 bearer token extraction | auth,p1 | Unblocks: 2
  [AWIT-TEST0002] Update database migration scripts | db | Unblocks: 0
  [AWIT-TEST0003] Add E2E auth tests | - | Unblocks: 1
  [AWIT-TEST0004] Rotate API tokens | p0 | Unblocks: 0
  [AWIT-TEST0005] Write auth middleware spec | - | Unblocks: 0
  [AWIT-TEST0006] Implement refresh-token rotation | - | Unblocks: 0
  ```

  Create `internal/cli/list_test.go`:

  ```go
  package cli

  import (
  	"encoding/json"
  	"strings"
  	"testing"
  )

  func TestListAllCompact(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "list")
  	if code != 0 || stderr != "" {
  		t.Fatalf("exit %d stderr %q stdout %q", code, stderr, stdout)
  	}
  	golden(t, "list_clean_compact.golden", []byte(stdout))
  }

  func TestListReadyOrder(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "list", "--ready")
  	if code != 0 || stderr != "" {
  		t.Fatalf("exit %d stderr %q", code, stderr)
  	}
  	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
  	if len(lines) != 3 {
  		t.Fatalf("ready count = %d, want 3\n%s", len(lines), stdout)
  	}
  	if !strings.HasPrefix(lines[0], "[AWIT-TEST0001]") {
  		t.Fatalf("first ready = %q, want AWIT-TEST0001", lines[0])
  	}
  	if !strings.HasPrefix(lines[1], "[AWIT-TEST0002]") {
  		t.Fatalf("second ready = %q, want AWIT-TEST0002", lines[1])
  	}
  	if !strings.HasPrefix(lines[2], "[AWIT-TEST0006]") {
  		t.Fatalf("third ready = %q, want AWIT-TEST0006", lines[2])
  	}
  }

  func TestListStatusFilter(t *testing.T) {
  	dir := copyFixture(t, "clean")

  	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "list", "-s", "closed")
  	if code != 0 || stderr != "" {
  		t.Fatalf("closed: exit %d stderr %q", code, stderr)
  	}
  	if stdout != "[AWIT-TEST0005] Write auth middleware spec | - | Unblocks: 0\n" {
  		t.Fatalf("closed stdout = %q", stdout)
  	}

  	code, stdout, stderr = run(t, "--repo", dir, "--format", "compact", "list", "-s", "open")
  	if code != 0 || stderr != "" {
  		t.Fatalf("open: exit %d stderr %q", code, stderr)
  	}
  	if strings.Contains(stdout, "AWIT-TEST0005") || strings.Contains(stdout, "AWIT-TEST0006") {
  		t.Fatalf("open filter leaked closed/in_progress:\n%s", stdout)
  	}
  	for _, id := range []string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0003", "AWIT-TEST0004"} {
  		if !strings.Contains(stdout, id) {
  			t.Fatalf("open filter missing %s\n%s", id, stdout)
  		}
  	}

  	code, stdout, stderr = run(t, "--repo", dir, "--format", "compact", "list", "-s", "in_progress")
  	if code != 0 || stderr != "" {
  		t.Fatalf("in_progress: exit %d stderr %q", code, stderr)
  	}
  	if !strings.HasPrefix(stdout, "[AWIT-TEST0006]") || strings.Count(stdout, "\n") != 1 {
  		t.Fatalf("in_progress stdout = %q", stdout)
  	}

  	code, _, stderr = run(t, "--repo", dir, "--format", "compact", "list", "-s", "banana")
  	if code != 1 || !strings.Contains(stderr, "Error:") {
  		t.Fatalf("bad status exit %d stderr %q", code, stderr)
  	}
  }

  func TestListLabelAnd(t *testing.T) {
  	dir := copyFixture(t, "clean")

  	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "list", "-l", "p0", "-l", "auth")
  	if code != 0 || stderr != "" {
  		t.Fatalf("AND: exit %d stderr %q", code, stderr)
  	}
  	if stdout != "" {
  		t.Fatalf("AND p0 AND auth = %q, want empty", stdout)
  	}

  	code, stdout, stderr = run(t, "--repo", dir, "--format", "compact", "list", "-l", "auth,db")
  	if code != 0 || stderr != "" {
  		t.Fatalf("OR: exit %d stderr %q", code, stderr)
  	}
  	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
  	if len(lines) != 2 {
  		t.Fatalf("OR count = %d, want 2\n%s", len(lines), stdout)
  	}
  	if !strings.HasPrefix(lines[0], "[AWIT-TEST0001]") {
  		t.Fatalf("OR first = %q, want 0001", lines[0])
  	}
  	if !strings.HasPrefix(lines[1], "[AWIT-TEST0002]") {
  		t.Fatalf("OR second = %q, want 0002", lines[1])
  	}
  }

  func TestListJSON(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "list")
  	if code != 0 || stderr != "" {
  		t.Fatalf("exit %d stderr %q", code, stderr)
  	}
  	var rows []struct {
  		ID     string `json:"id"`
  		Status string `json:"status"`
  		State  string `json:"state"`
  	}
  	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
  		t.Fatalf("json: %v\n%s", err, stdout)
  	}
  	if len(rows) != 6 {
  		t.Fatalf("len = %d, want 6", len(rows))
  	}
  	var found bool
  	for _, r := range rows {
  		if r.ID == "AWIT-TEST0005" {
  			found = true
  			if r.Status != "closed" || r.State != "closed" {
  				t.Fatalf("0005 status=%q state=%q, want closed/closed", r.Status, r.State)
  			}
  		}
  	}
  	if !found {
  		t.Fatal("0005 missing from json list")
  	}
  }

  func TestListQuarantined(t *testing.T) {
  	dir := copyFixture(t, "cyclic")
  	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "list", "--quarantined")
  	if code != 0 || stderr != "" {
  		t.Fatalf("exit %d stderr %q", code, stderr)
  	}
  	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
  	if len(lines) != 4 {
  		t.Fatalf("quarantined count = %d, want 4 (triangle + self-loop)\n%s", len(lines), stdout)
  	}
  	want := []string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0003", "AWIT-TEST0004"}
  	for i, id := range want {
  		if !strings.HasPrefix(lines[i], "["+id+"]") {
  			t.Fatalf("line %d = %q, want %s", i, lines[i], id)
  		}
  		if !strings.HasSuffix(lines[i], " | QUARANTINED") {
  			t.Fatalf("line %d missing | QUARANTINED: %q", i, lines[i])
  		}
  	}
  	if strings.Contains(stdout, "AWIT-TEST0005") {
  		t.Fatalf("0005 is not quarantined:\n%s", stdout)
  	}
  }
  ```

- [ ] **Step 2: Run it, see it fail.**

  ```bash
  go test ./internal/cli -run 'TestList' -v
  ```

  Expected: `list` is an unknown command (exit 2) on every test that reaches `run`, or a compile error `undefined: toEntry` if a helper test referenced it. Either form is a valid red:

  ```text
  --- FAIL: TestListAllCompact
      list_test.go: exit 2 stderr "unknown command \"list\" (run \"awit --help\")"
  ```

- [ ] **Step 3: Implement `loadGraph`, `toEntry`, `list.go`, and register the command.**

  If `detectFormat` is not already in the package, add it (one definition only):

  ```go
  func detectFormat(cmd *cli.Command) (format.Format, error) {
  	var stdout *os.File
  	if f, ok := cmd.Root().Writer.(*os.File); ok {
  		stdout = f
  	}
  	return format.Detect(cmd.Root().String("format"), stdout)
  }
  ```

  If `loadGraph` is not already in the package, add it to `internal/cli/app.go`:

  ```go
  func loadGraph(s *item.Store) (*graph.Graph, error) {
  	items, broken, err := s.LoadAll()
  	if err != nil {
  		return nil, err
  	}
  	return graph.Build(items, broken), nil
  }
  ```

  Add `toEntry` to `internal/cli/app.go` (this ticket owns it; do not put it in `list.go` if next/show will import it from the package — package-level in `app.go` is the home):

  ```go
  func toEntry(n *graph.Node) format.Entry {
  	e := format.Entry{
  		ID:       n.Item.ID,
  		Title:    n.Item.Title,
  		Brief:    n.Item.Brief,
  		Status:   string(n.Item.Status),
  		Labels:   n.Item.Labels,
  		Deps:     n.Item.Deps,
  		Assignee: n.Item.Assignee,
  		Unblocks: n.UnblockCount,
  	}
  	if e.Labels == nil {
  		e.Labels = []string{}
  	}
  	if e.Deps == nil {
  		e.Deps = []string{}
  	}
  	switch {
  	case n.Quarantined():
  		e.State = "quarantined"
  		for _, f := range n.Faults {
  			e.Faults = append(e.Faults, "["+string(f.Reason)+"] "+f.Detail)
  		}
  	case n.Item.Status == item.StatusClosed:
  		e.State = "closed"
  	case n.Ready:
  		e.State = "ready"
  	default:
  		e.State = "blocked"
  	}
  	return e
  }
  ```

  `app.go` imports to add if missing: `"github.com/eisenwinter/awit/pkg/format"`, `"github.com/eisenwinter/awit/pkg/graph"`, `"github.com/eisenwinter/awit/pkg/item"`.

  Create `internal/cli/list.go`:

  ```go
  package cli

  import (
  	"context"
  	"fmt"

  	"github.com/eisenwinter/awit/pkg/format"
  	"github.com/eisenwinter/awit/pkg/graph"
  	"github.com/eisenwinter/awit/pkg/item"
  	"github.com/urfave/cli/v3"
  )

  var listCmd = &cli.Command{
  	Name:  "list",
  	Usage: "List items",
  	Flags: []cli.Flag{
  		&cli.StringSliceFlag{Name: "status", Aliases: []string{"s"}, Usage: "filter by status (open, in_progress, closed); repeatable, OR"},
  		&cli.StringSliceFlag{Name: "label", Aliases: []string{"l"}, Usage: "AND across flags, OR within a flag"},
  		&cli.BoolFlag{Name: "ready", Usage: "include ready items"},
  		&cli.BoolFlag{Name: "blocked", Usage: "include blocked items"},
  		&cli.BoolFlag{Name: "quarantined", Usage: "include quarantined items"},
  	},
  	Action: listAction,
  }

  func listAction(_ context.Context, cmd *cli.Command) error {
  	s, err := openStore(cmd)
  	if err != nil {
  		return err
  	}
  	g, err := loadGraph(s)
  	if err != nil {
  		return err
  	}

  	ready := cmd.Bool("ready")
  	blocked := cmd.Bool("blocked")
  	quarantined := cmd.Bool("quarantined")

  	var nodes []*graph.Node
  	switch {
  	case ready && !blocked && !quarantined:
  		nodes = g.Ready()
  	case ready || blocked || quarantined:
  		for _, n := range g.Order {
  			if (ready && n.Ready) || (blocked && n.Blocked) || (quarantined && n.Quarantined()) {
  				nodes = append(nodes, n)
  			}
  		}
  	default:
  		nodes = g.Order
  	}

  	if statuses := cmd.StringSlice("status"); len(statuses) > 0 {
  		allow := map[item.Status]bool{}
  		for _, raw := range statuses {
  			st, err := item.ParseStatus(raw)
  			if err != nil {
  				return err
  			}
  			allow[st] = true
  		}
  		var filtered []*graph.Node
  		for _, n := range nodes {
  			if allow[n.Item.Status] {
  				filtered = append(filtered, n)
  			}
  		}
  		nodes = filtered
  	}

  	nodes = graph.FilterLabels(nodes, SplitLabels(cmd.StringSlice("label")))

  	f, err := detectFormat(cmd)
  	if err != nil {
  		return err
  	}
  	entries := make([]format.Entry, 0, len(nodes))
  	for _, n := range nodes {
  		entries = append(entries, toEntry(n))
  	}
  	if err := format.Write(cmd.Writer, f, entries); err != nil {
  		return fmt.Errorf("format: %w", err)
  	}
  	return nil
  }
  ```

  In `newRoot`, append `listCmd` to `Commands`. Keep every existing command.

- [ ] **Step 4: Run it, see it pass, commit.**

  ```bash
  go test ./internal/cli -run 'TestList' -v
  ```

  Expected: `TestListAllCompact`, `TestListReadyOrder`, `TestListStatusFilter`, `TestListLabelAnd`, `TestListJSON`, `TestListQuarantined` all PASS.

  ```bash
  go test ./internal/cli -count=1
  gofmt -w internal/cli/list.go internal/cli/list_test.go internal/cli/app.go
  git add internal/cli/list.go internal/cli/list_test.go internal/cli/app.go testdata/golden/list_clean_compact.golden
  git commit -m "cli/list: status, label and ready/blocked/quarantined filters"
  ```

- [ ] **Step 5: Full check and close.**

  ```bash
  go build ./... && go vet ./... && go test ./internal/cli -count=1
  ```

  Set `status: closed` on this file, write `.awit/comments/AWIT-0ND56J3G/<YYYYMMDDTHHMMSSZ>-<author>.md` with the acceptance output, append that ref, commit `tickets: close AWIT-0ND56J3G`.

## Acceptance Criteria
- `go test ./internal/cli -run 'TestList' -v` — all six tests PASS.
- `list --format compact` on clean matches `list_clean_compact.golden` (6 lines, ID order, 0005 present).
- `list --ready --format compact` on clean: first line is `AWIT-TEST0001`, then `0002`, then `0006`.
- `list -s closed` is only 0005; `list -s open` is 0001–0004; `list -s in_progress` is only 0006.
- `list -l p0 -l auth` prints nothing; `list -l auth,db` prints 0001 then 0002.
- `list --format json` has 6 objects; 0005 has `status` and `state` equal to `closed`.
- `list --quarantined` on cyclic: 4 lines (0001–0004), each ending ` | QUARANTINED`; 0005 absent.
- `loadGraph` and `toEntry` exist once in package `cli`. `gofmt -l internal/cli/list.go` prints nothing.

## Out of scope
- `awit label` (AWIT-0ND5763G). `awit show`. Changing `pkg/format` or `FilterLabels`. Defaulting format to table inside this command — `detectFormat` already handles TTY.
