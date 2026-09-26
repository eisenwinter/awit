---
id: AWIT-0ND56S3G
title: awit validate
brief: >-
  Add `awit validate`, which prints PASS or FAIL lines from g.Faults, WARNs on missing or over-long briefs, and exits 1 if and only if any FAIL exists. Do not register --stale-claims.
status: closed
deps: [AWIT-0ND56Q3G, AWIT-0ND56G3G]
labels: [phase2, p0]
refs_base: repo
refs:
  - plan/implementation-guide.md
  - plan/awit-implementation-plan.md
---

## Summary

After this ticket `internal/cli/validate.go` registers `awit validate`. The command loads the graph, prints one `[REASON] detail` line plus an indented `fix:` line for every entry in `g.Faults`, and otherwise prints `PASS  N items, 0 quarantined`. Item count is `len(g.Order)+len(g.Broken)`. Quarantined count is `len(g.Quarantined())+len(g.Broken)`. WARNs for a missing brief or a brief with more than three sentences never change the exit code. Exit 1 if and only if `len(g.Faults) > 0`. `--format json` prints an array of `{reason, ids, detail, fix}`. Do not register `--stale-claims`.

## Context (read first)

- Guide §1 - `Error: ` via `report`, exit 0/1/2, `filepath`, tests call `Main` through `run` with `--repo`.
- Guide §4.6 - `graph.Build`, `Graph.Faults`, `Graph.Broken`, `Graph.Order`, `Graph.Quarantined`, `Fault{Reason, IDs, Detail, Fix}`. Print faults in `g.Faults` order (already sorted by Reason then IDs).
- Guide §4.11 - `openStore`. `loadGraph` belongs to AWIT-0ND56J3G; this ticket may land first, so define it if absent (exact code in Step 3).
- Guide §4.3 - `item.Reason` strings are `PARSE ERROR`, `CONFLICT MARKERS`, `ID MISMATCH`, `DUPLICATE ID`, `DANGLING DEP`, `CYCLE`.
- Guide §8 / AWIT-0ND56N3G fixtures (already on disk). Fault text this command must echo:
  - dangling: Detail `AWIT-TEST0001 depends on unknown AWIT-TEST9999`, Fix `awit dep rm AWIT-TEST0001 AWIT-TEST9999`. 2 items, 1 quarantined.
  - cyclic (AWIT-0ND56P3G): two CYCLE faults, triangle Detail `AWIT-TEST0001 -> AWIT-TEST0002 -> AWIT-TEST0003 -> AWIT-TEST0001` Fix `awit dep rm AWIT-TEST0001 AWIT-TEST0002 (break the cycle)`; self Detail `AWIT-TEST0004 -> AWIT-TEST0004` Fix `awit dep rm AWIT-TEST0004 AWIT-TEST0004 (break the cycle)`. 5 items, 4 quarantined.
  - conflicted: Detail `conflict markers in file`, Fix `resolve the git conflict in ` + absolute `Broken.Path`. 1 healthy item + 1 broken = 2 items, 1 quarantined.
  - id-mismatch: Detail `id "AWIT-TEST0009" != filename stem "AWIT-TEST0001"`, Fix `rename the file or fix the id: key`. 1 broken, 0 nodes = 1 item, 1 quarantined.
  - parse-error: Detail is `Broken.Detail` from `LoadAll` (yaml.v3 error), Fix `edit the frontmatter until ` + backtick-wrapped `awit validate` + ` passes`. 1 item, 1 quarantined.
  - clean: 6 items, 0 quarantined, no faults.
- Guide §2 brief: one to three sentences. `validate` WARNs when brief is missing (empty/whitespace) or `sentenceCount` is greater than 3. WARN never changes exit.
- Spec CLI matrix lists `--stale-claims` on validate; that flag is AWIT-0ND5733G. Do **not** add it here. `awit validate --stale-claims` must be a usage error (exit 2).
- Helpers already in the package: `openStore` (AWIT-0ND56G3G), `run` / `copyFixture` / `golden` / `initRepo` (helpers_test.go). Do not redeclare them. `seedItem` may be absent (it lives in update_test.go); write items with `item.New`+`Save` in this ticket's tests.
- Compact/table output is the same text. Only `--format json` changes the renderer. Tests that are not JSON pass `--format compact`.
- Two spaces after `PASS`/`FAIL`/`WARN`. Always the word `items` (including `1 items`).

## Files

- Create: `internal/cli/validate.go`
- Create: `internal/cli/validate_test.go`
- Create: `testdata/golden/validate_clean.golden`
- Create: `testdata/golden/validate_cyclic.golden`
- Create: `testdata/golden/validate_dangling.golden`
- Create: `testdata/golden/validate_conflicted.golden`
- Create: `testdata/golden/validate_id-mismatch.golden`
- Create: `testdata/golden/validate_parse-error.golden`
- Modify: `internal/cli/app.go` - append `validateCmd` to `newRoot`'s `Commands` slice. Keep every command already there.

## Interfaces

- Consumes:
  ```go
  func openStore(cmd *cli.Command) (*item.Store, error)
  func (s *Store) LoadAll() ([]*Item, []Broken, error)
  func Build(items []*Item, broken []Broken) *Graph
  func (g *Graph) Quarantined() []*Node
  ```
- Produces (package-private, this ticket):
  ```go
  var validateCmd *cli.Command
  func validateAction(_ context.Context, cmd *cli.Command) error
  func sentenceCount(s string) int
  func loadGraph(s *item.Store) (*graph.Graph, error) // define if absent
  type validateFaultJSON struct {
      Reason string   `json:"reason"`
      IDs    []string `json:"ids"`
      Detail string   `json:"detail"`
      Fix    string   `json:"fix"`
  }
  ```
- stdout (compact/table):
  - no faults: `PASS  N items, 0 quarantined\n` then zero or more `WARN  <id>: ...\n`
  - any faults: `FAIL  N items, M quarantined\n` then for each `g.Faults` entry:
    ```text
    [REASON] detail
      fix: <Fix>
    ```
    then WARNs. Exit 1 via `cli.Exit("", 1)` after writing (empty message so `report` prints nothing extra).
- stdout (json): two-space-indented JSON array of `validateFaultJSON`, trailing newline, no PASS/FAIL/WARN lines. Empty graph → `[]\n`. Exit 1 iff the array is non-empty.

## Steps

- [ ] **Step 1: Write goldens and failing tests.**

  `testdata/golden/validate_clean.golden` (trailing newline):

  ```text
  PASS  6 items, 0 quarantined
  ```

  `testdata/golden/validate_cyclic.golden` (trailing newline):

  ```text
  FAIL  5 items, 4 quarantined
  [CYCLE] AWIT-TEST0001 -> AWIT-TEST0002 -> AWIT-TEST0003 -> AWIT-TEST0001
    fix: awit dep rm AWIT-TEST0001 AWIT-TEST0002 (break the cycle)
  [CYCLE] AWIT-TEST0004 -> AWIT-TEST0004
    fix: awit dep rm AWIT-TEST0004 AWIT-TEST0004 (break the cycle)
  ```

  `testdata/golden/validate_dangling.golden` (trailing newline):

  ```text
  FAIL  2 items, 1 quarantined
  [DANGLING DEP] AWIT-TEST0001 depends on unknown AWIT-TEST9999
    fix: awit dep rm AWIT-TEST0001 AWIT-TEST9999
  ```

  `testdata/golden/validate_conflicted.golden` - the Fix path is replaced with the literal `PATH` by the test before comparison (trailing newline):

  ```text
  FAIL  2 items, 1 quarantined
  [CONFLICT MARKERS] conflict markers in file
    fix: resolve the git conflict in PATH
  ```

  `testdata/golden/validate_id-mismatch.golden` (trailing newline):

  ```text
  FAIL  1 items, 1 quarantined
  [ID MISMATCH] id "AWIT-TEST0009" != filename stem "AWIT-TEST0001"
    fix: rename the file or fix the id: key
  ```

  `testdata/golden/validate_parse-error.golden` - the Detail is replaced with the literal `DETAIL` by the test (trailing newline):

  ```text
  FAIL  1 items, 1 quarantined
  [PARSE ERROR] DETAIL
    fix: edit the frontmatter until `awit validate` passes
  ```

  Create `internal/cli/validate_test.go`:

  ```go
  package cli

  import (
  	"encoding/json"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  	"unicode"

  	"github.com/eisenwinter/awit/pkg/item"
  )

  func TestSentenceCount(t *testing.T) {
  	tests := []struct {
  		in   string
  		want int
  	}{
  		{"", 0},
  		{"   ", 0},
  		{"Hello", 1},
  		{"Hello.", 1},
  		{"One. Two. Three.", 3},
  		{"One. Two. Three. Four.", 4},
  		{"What? Yes! OK.", 3},
  		{"Dr. Foo went home.", 2},
  	}
  	for _, tt := range tests {
  		if got := sentenceCount(tt.in); got != tt.want {
  			t.Errorf("sentenceCount(%q) = %d, want %d", tt.in, got, tt.want)
  		}
  	}
  }

  func TestValidateGoldens(t *testing.T) {
  	tests := []struct {
  		fixture string
  		code    int
  		rewrite func(t *testing.T, dir string, stdout string) string
  	}{
  		{fixture: "clean", code: 0},
  		{fixture: "cyclic", code: 1},
  		{fixture: "dangling", code: 1},
  		{
  			fixture: "conflicted",
  			code:    1,
  			rewrite: func(t *testing.T, dir, stdout string) string {
  				t.Helper()
  				p := filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md")
  				abs, err := filepath.Abs(p)
  				if err != nil {
  					t.Fatal(err)
  				}
  				return strings.ReplaceAll(stdout, abs, "PATH")
  			},
  		},
  		{fixture: "id-mismatch", code: 1},
  		{
  			fixture: "parse-error",
  			code:    1,
  			rewrite: func(t *testing.T, dir, stdout string) string {
  				t.Helper()
  				st, err := item.Open(dir)
  				if err != nil {
  					t.Fatal(err)
  				}
  				_, broken, err := st.LoadAll()
  				if err != nil {
  					t.Fatal(err)
  				}
  				if len(broken) != 1 || broken[0].Detail == "" {
  					t.Fatalf("broken = %+v, want one PARSE ERROR with Detail", broken)
  				}
  				return strings.ReplaceAll(stdout, broken[0].Detail, "DETAIL")
  			},
  		},
  	}
  	for _, tt := range tests {
  		t.Run(tt.fixture, func(t *testing.T) {
  			dir := copyFixture(t, tt.fixture)
  			code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "validate")
  			if code != tt.code {
  				t.Fatalf("exit %d, want %d stderr %q stdout %q", code, tt.code, stderr, stdout)
  			}
  			if tt.code == 0 && stderr != "" {
  				t.Fatalf("stderr = %q, want empty", stderr)
  			}
  			got := stdout
  			if tt.rewrite != nil {
  				got = tt.rewrite(t, dir, stdout)
  			}
  			golden(t, "validate_"+tt.fixture+".golden", []byte(got))
  		})
  	}
  }

  func TestValidateJSON(t *testing.T) {
  	dir := copyFixture(t, "dangling")
  	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "validate")
  	if code != 1 {
  		t.Fatalf("exit %d stderr %q stdout %q", code, stderr, stdout)
  	}
  	var rows []struct {
  		Reason string   `json:"reason"`
  		IDs    []string `json:"ids"`
  		Detail string   `json:"detail"`
  		Fix    string   `json:"fix"`
  	}
  	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
  		t.Fatalf("json: %v\n%s", err, stdout)
  	}
  	if len(rows) != 1 {
  		t.Fatalf("len = %d, want 1\n%s", len(rows), stdout)
  	}
  	if rows[0].Reason != "DANGLING DEP" {
  		t.Fatalf("reason = %q", rows[0].Reason)
  	}
  	if len(rows[0].IDs) != 1 || rows[0].IDs[0] != "AWIT-TEST0001" {
  		t.Fatalf("ids = %v", rows[0].IDs)
  	}
  	if rows[0].Detail != "AWIT-TEST0001 depends on unknown AWIT-TEST9999" {
  		t.Fatalf("detail = %q", rows[0].Detail)
  	}
  	if rows[0].Fix != "awit dep rm AWIT-TEST0001 AWIT-TEST9999" {
  		t.Fatalf("fix = %q", rows[0].Fix)
  	}
  	if !strings.HasSuffix(stdout, "\n") {
  		t.Fatal("json stdout must end with a newline")
  	}
  }

  func TestValidateJSONClean(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "validate")
  	if code != 0 || stderr != "" {
  		t.Fatalf("exit %d stderr %q", code, stderr)
  	}
  	if stdout != "[]\n" {
  		t.Fatalf("stdout = %q, want []\\n", stdout)
  	}
  }

  func TestValidateWarnMissingBrief(t *testing.T) {
  	dir := initRepo(t)
  	st, err := item.Open(dir)
  	if err != nil {
  		t.Fatal(err)
  	}
  	if err := st.Save(item.New("AWIT-TEST0001", "No brief", "", nil, nil)); err != nil {
  		t.Fatal(err)
  	}
  	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "validate")
  	if code != 0 || stderr != "" {
  		t.Fatalf("exit %d stderr %q (WARN must not fail)", code, stderr)
  	}
  	want := "PASS  1 items, 0 quarantined\nWARN  AWIT-TEST0001: missing brief\n"
  	if stdout != want {
  		t.Fatalf("stdout = %q, want %q", stdout, want)
  	}
  }

  func TestValidateWarnLongBrief(t *testing.T) {
  	dir := initRepo(t)
  	st, err := item.Open(dir)
  	if err != nil {
  		t.Fatal(err)
  	}
  	brief := "One. Two. Three. Four."
  	if err := st.Save(item.New("AWIT-TEST0001", "Long brief", brief, nil, nil)); err != nil {
  		t.Fatal(err)
  	}
  	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "validate")
  	if code != 0 || stderr != "" {
  		t.Fatalf("exit %d stderr %q", code, stderr)
  	}
  	if !strings.Contains(stdout, "PASS  1 items, 0 quarantined\n") {
  		t.Fatalf("stdout = %q, want PASS", stdout)
  	}
  	if !strings.Contains(stdout, "WARN  AWIT-TEST0001: brief is longer than 3 sentences\n") {
  		t.Fatalf("stdout = %q, want long-brief WARN", stdout)
  	}
  }

  func TestValidateWarnDoesNotChangeFailExit(t *testing.T) {
  	dir := copyFixture(t, "dangling")
  	code, stdout, _ := run(t, "--repo", dir, "--format", "compact", "validate")
  	if code != 1 {
  		t.Fatalf("exit %d, want 1", code)
  	}
  	if strings.Contains(stdout, "WARN") {
  		t.Fatalf("dangling fixture has valid briefs; stdout = %q", stdout)
  	}
  }

  func TestValidateNoStaleClaimsFlag(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	code, _, stderr := run(t, "--repo", dir, "validate", "--stale-claims")
  	if code != 2 {
  		t.Fatalf("exit %d, want 2 (unknown flag) stderr %q", code, stderr)
  	}
  }

  func TestSentenceCountTerminatorsNeedBoundary(t *testing.T) {
  	if unicode.IsSpace(' ') != true {
  		t.Fatal("sanity")
  	}
  	if sentenceCount("Hello.World") != 1 {
  		t.Fatalf("sentenceCount(Hello.World) = %d, want 1 (dot not at a boundary)", sentenceCount("Hello.World"))
  	}
  }
  ```

- [ ] **Step 2: Run it, see it fail.**

  ```bash
  go test ./internal/cli -run 'TestValidate|TestSentenceCount' -v
  ```

  Expected: `undefined: sentenceCount` (and `validate` unknown command if the identifier is unused) -

  ```text
  # github.com/eisenwinter/awit/internal/cli [github.com/eisenwinter/awit/internal/cli.test]
  internal/cli/validate_test.go: undefined: sentenceCount
  FAIL	github.com/eisenwinter/awit/internal/cli [build failed]
  ```

- [ ] **Step 3: Implement `validate.go` and register the command.**

  If `loadGraph` is not already in the package, add it to `internal/cli/app.go` (or to `validate.go` - one definition only):

  ```go
  func loadGraph(s *item.Store) (*graph.Graph, error) {
  	items, broken, err := s.LoadAll()
  	if err != nil {
  		return nil, err
  	}
  	return graph.Build(items, broken), nil
  }
  ```

  Create `internal/cli/validate.go`:

  ```go
  package cli

  import (
  	"context"
  	"encoding/json"
  	"fmt"
  	"strings"
  	"unicode"

  	"github.com/eisenwinter/awit/pkg/graph"
  	"github.com/urfave/cli/v3"
  )

  var validateCmd = &cli.Command{
  	Name:  "validate",
  	Usage: "Report graph faults and brief warnings",
  	Action: validateAction,
  }

  type validateFaultJSON struct {
  	Reason string   `json:"reason"`
  	IDs    []string `json:"ids"`
  	Detail string   `json:"detail"`
  	Fix    string   `json:"fix"`
  }

  // sentenceCount counts sentences in s. A sentence ends at '.', '!' or '?'
  // that is at end-of-string or followed by whitespace. A non-empty brief
  // with no terminator is one sentence. Empty / whitespace-only is zero.
  func sentenceCount(s string) int {
  	s = strings.TrimSpace(s)
  	if s == "" {
  		return 0
  	}
  	runes := []rune(s)
  	n := 0
  	for i, r := range runes {
  		if r != '.' && r != '!' && r != '?' {
  			continue
  		}
  		if i+1 == len(runes) || unicode.IsSpace(runes[i+1]) {
  			n++
  		}
  	}
  	if n == 0 {
  		return 1
  	}
  	return n
  }

  func validateAction(_ context.Context, cmd *cli.Command) error {
  	s, err := openStore(cmd)
  	if err != nil {
  		return err
  	}
  	g, err := loadGraph(s)
  	if err != nil {
  		return err
  	}
  	nItems := len(g.Order) + len(g.Broken)
  	nQuar := len(g.Quarantined()) + len(g.Broken)

  	if cmd.Root().String("format") == "json" {
  		rows := make([]validateFaultJSON, 0, len(g.Faults))
  		for _, f := range g.Faults {
  			ids := f.IDs
  			if ids == nil {
  				ids = []string{}
  			}
  			rows = append(rows, validateFaultJSON{
  				Reason: string(f.Reason),
  				IDs:    ids,
  				Detail: f.Detail,
  				Fix:    f.Fix,
  			})
  		}
  		enc := json.NewEncoder(cmd.Writer)
  		enc.SetIndent("", "  ")
  		if err := enc.Encode(rows); err != nil {
  			return err
  		}
  		if len(g.Faults) > 0 {
  			return cli.Exit("", 1)
  		}
  		return nil
  	}

  	status := "PASS"
  	if len(g.Faults) > 0 {
  		status = "FAIL"
  	}
  	fmt.Fprintf(cmd.Writer, "%s  %d items, %d quarantined\n", status, nItems, nQuar)
  	for _, f := range g.Faults {
  		fmt.Fprintf(cmd.Writer, "[%s] %s\n  fix: %s\n", f.Reason, f.Detail, f.Fix)
  	}
  	for _, n := range g.Order {
  		brief := strings.TrimSpace(n.Item.Brief)
  		if brief == "" {
  			fmt.Fprintf(cmd.Writer, "WARN  %s: missing brief\n", n.Item.ID)
  			continue
  		}
  		if sentenceCount(brief) > 3 {
  			fmt.Fprintf(cmd.Writer, "WARN  %s: brief is longer than 3 sentences\n", n.Item.ID)
  		}
  	}
  	if len(g.Faults) > 0 {
  		return cli.Exit("", 1)
  	}
  	return nil
  }
  ```

  In `newRoot`, append `validateCmd` to `Commands`. Keep every existing command. Do not add a `--stale-claims` flag. Do not redeclare `loadGraph` if AWIT-0ND56J3G already defined it - the function body above must match.

- [ ] **Step 4: Run it, see it pass, commit.**

  ```bash
  go test ./internal/cli -run 'TestValidate|TestSentenceCount' -v
  ```

  Expected: every `TestValidateGoldens/<fixture>` PASS, `TestValidateJSON` PASS, `TestValidateJSONClean` PASS, `TestValidateWarnMissingBrief` PASS, `TestValidateWarnLongBrief` PASS, `TestValidateWarnDoesNotChangeFailExit` PASS, `TestValidateNoStaleClaimsFlag` PASS, both `TestSentenceCount*` PASS.

  If `validate_parse-error.golden` fails only on the Detail line, replace `DETAIL` is already done by the test - a remaining mismatch means the FAIL header or Fix line is wrong; fix the command, not the golden.

  ```bash
  go test ./internal/cli -count=1
  gofmt -w internal/cli/validate.go internal/cli/validate_test.go internal/cli/app.go
  git add internal/cli/validate.go internal/cli/validate_test.go internal/cli/app.go testdata/golden/validate_*.golden
  git commit -m "cli/validate: PASS/FAIL report from graph faults"
  ```

- [ ] **Step 5: Full check and close.**

  ```bash
  go build ./... && go vet ./... && go test ./internal/cli -count=1
  ```

  Set `status: closed` on this file, write `.awit/comments/AWIT-0ND56S3G/<YYYYMMDDTHHMMSSZ>-<author>.md` with the acceptance output, append that ref, commit `tickets: close AWIT-0ND56S3G`.

## Acceptance Criteria

- `go test ./internal/cli -run 'TestValidate|TestSentenceCount' -v` - all PASS.
- Clean fixture: stdout is exactly `PASS  6 items, 0 quarantined\n`, exit 0.
- Dangling fixture: exit 1, compact matches `validate_dangling.golden`; json is a one-element array with `reason` `DANGLING DEP`, `ids` `[AWIT-TEST0001]`, the N3G detail/fix.
- Cyclic fixture: exit 1, two `[CYCLE]` blocks in the P3G order, `FAIL  5 items, 4 quarantined`.
- Conflicted / id-mismatch / parse-error goldens match after PATH/DETAIL rewrite.
- Missing brief and four-sentence brief emit `WARN` and still exit 0.
- `awit validate --stale-claims` exits 2.
- `validateCmd.Flags` does not contain `stale-claims`.
- `gofmt -l internal/cli/validate.go internal/cli/validate_test.go` prints nothing.

## Out of scope

- `--stale-claims` (AWIT-0ND5733G). Locking. `prime` / `next` / `list`. Changing `pkg/graph` fault text. Registering any flag on `validateCmd`.
