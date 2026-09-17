---
id: AWIT-0ND5763G
title: 'awit label (vocabulary with counts, --state)'
brief: >-
  Add internal/cli/label.go: the label command validates --state
  open|closed|all (default open, open = status != closed), counts labels over
  LoadAll's parseable items only (graph-quarantined items count, broken files
  never do), sorts rows count descending then label ascending, and renders
  through format.WriteLabels with the global --format flag.
status: closed
deps: [AWIT-0ND56G3G, AWIT-0ND56F3G]
labels: [phase2, p2]
refs:
  - ../../plan/implementation-guide.md
  - ../../plan/awit-implementation-plan.md
---

## Summary
After this ticket `awit label` prints the label vocabulary with usage counts. `--state open` (the default) counts every item whose status is **not** `closed` (so `open` and `in_progress`), `--state closed` counts only closed items, `--state all` counts every parseable item. The command never builds a graph: it reads `store.LoadAll()` and counts `Item.Labels`, which by construction includes items the graph would quarantine (dangling dep, cycle member) and excludes unparseable files (they come back in the `Broken` slice, which is discarded on purpose). Rows are sorted by count descending then label ascending and rendered by `format.WriteLabels`, so `--format compact|table|json` and the non-TTY compact default all work; an empty vocabulary prints nothing in compact, `[]` in JSON, and exits 0. An invalid `--state` value is a plain action error: `Error: --state must be open, closed or all`, exit 1.

## Context (read first)
- **guide §2, row "awit label semantics"** (verbatim decision): "`--state open` (default) counts items whose status is **not** `closed` (so `open` + `in_progress`); `--state closed` counts only closed; `--state all` counts every parseable item. Quarantined items are counted (they still carry labels); unparseable files are not. Rows sorted by count desc, then label asc. Labels never declared anywhere — the vocabulary is whatever items use."
- **spec `plan/awit-implementation-plan.md` §CLI command matrix** — row `awit label`: flags `--state open|closed|all`, `--format`; purpose "Label vocabulary with usage counts; answers 'what labels exist and how busy are they'". "Priority" in the plan: no dedicated field, `p0`/`p1` are just labels — this command is how an agent discovers them.
- **guide §4.7 `pkg/format`** — this ticket consumes, never reimplements:
  - `format.LabelCount{Label string; Count int}` with json tags `label`, `count`.
  - `WriteLabels(w, f, counts)`: compact is `<label> <count>\n` per line (nothing when empty); table is a tabwriter block with the uppercase header `LABEL  COUNT`; json is an indented two-space array with a trailing newline, `[]\n` when empty.
  - `Detect(flag, stdout *os.File)`: non-empty flag wins and is validated (`format: unknown format "yaml" (compact|table|json)`), else terminal → table, non-terminal → compact. `IsTerminal(nil)` is false, so a type-asserted nil `*os.File` means compact — that is exactly what happens under `go test`, where the writer is a `bytes.Buffer`.
- **guide §4.11 `internal/cli`** — `openStore(cmd)` honours `--repo` (AWIT-0ND56G3G). `--format` and `--repo` are persistent root flags read with `cmd.Root().String("format")`; a command-local flag like `--state` is read with `cmd.String("state")`. Precedent: G3G's `initAction` validates its local flag and returns `fmt.Errorf`, which `Main`'s `report` prints as `Error: <msg>` with exit 1 (`TestInitBadPrefix`). This ticket's `--state` validation follows that precedent exactly.
- **guide §1** — deterministic output (the sort makes it so; never range over a map into output), no colour, errors as `Error: ` on stderr, `filepath` never a hardcoded `/`.
- **AWIT-0ND56G3G test harness** (`internal/cli/helpers_test.go`) — consume, do not redeclare: `run(t, args...) (code, stdout, stderr)`, `copyFixture(t, name) string` (returns the temp repo root containing the `.awit` copy), `golden(t, name, got []byte)` with the `-update` flag, `readItem`. Tests always pass `--repo <copy>` and put global flags before the subcommand, locals after (G3G style: `run(t, "--repo", dir, "init", "--prefix", "PROJ")`).
- **Fixture facts (AWIT-0ND56N3G, guide §8)** — the `clean` fixture, all six items:

  | item | status | labels |
  | --- | --- | --- |
  | `AWIT-TEST0001` | open | `auth, p1` |
  | `AWIT-TEST0002` | open | `db` |
  | `AWIT-TEST0003` | open | — |
  | `AWIT-TEST0004` | open | `p0` |
  | `AWIT-TEST0005` | closed | — |
  | `AWIT-TEST0006` | in_progress | — |

  `0005` and `0006` carry `labels: []`. So the pristine vocabulary is: **open** (default) → `auth 1, db 1, p0 1, p1 1` (all counts 1, order is the label-ascending tie-break); **closed** → empty (`0005` has no labels); **all** → same rows as open (`0005` adds nothing). The `dangling` fixture: `0001` is open with `labels: [auth, p1]` and `deps: [AWIT-TEST9999]` — parseable, so its labels count even though the graph quarantines it; `0002` is open with `labels: []`. The parse-error shape `title: [unclosed` makes a file unparseable.
- **Quarantine is a graph concept; label never imports `pkg/graph`.** `LoadAll() ([]*Item, []Broken, error)` already splits exactly the way this command needs: parseable items (including future dangling-dep/cycle quarantine victims) on the left, broken files on the right. Discard the `Broken` slice with a comment saying why. Do not "fix" a counting bug by building a graph — building one here is the bug.
- **Duplicate label inside one item** counts once per occurrence (`labels: [auth, auth]` → `auth` +2). No dedupe; fixtures never repeat a label, and the vocabulary semantics do not care.
- **`--state ""`** (explicitly passed empty) is invalid: urfave only applies the flag's `Value: "open"` default when the flag is absent, so an explicit empty string reaches the action and must produce the same error as `bogus`.
- The fixtures on disk come from AWIT-0ND56N3G (phase2 p0; in dependency practice it lands before this p2 ticket). If `copyFixture(t, "clean")` fails with a missing directory, N3G has not landed yet — stop and pick another ticket, do not create fixtures here.
- `awit label` takes no positional arguments and writes nothing to disk: it is a read-only view, so no locking, no commit, no temp-then-rename.

## Files
- Create: `internal/cli/label.go` — `labelCmd`, `labelAction`, `labelCounts`.
- Create: `internal/cli/label_test.go`.
- Create (via `-update`): `testdata/golden/label_clean_table.golden`.
- Modify: `internal/cli/app.go` — append `labelCmd` to the `Commands` slice in `newRoot`.

## Interfaces
- Consumes (already implemented by closed deps, do not reimplement):

  ```go
  // internal/cli (AWIT-0ND56G3G / AWIT-0ND5683G)
  func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int
  func openStore(cmd *cli.Command) (*item.Store, error)
  // test harness (helpers_test.go)
  func run(t *testing.T, args ...string) (code int, stdout, stderr string)
  func copyFixture(t *testing.T, name string) string
  func golden(t *testing.T, name string, got []byte)

  // pkg/item (AWIT-0ND56E3G)
  func (s *Store) LoadAll() ([]*item.Item, []item.Broken, error)
  const StatusClosed Status = "closed"

  // pkg/format (AWIT-0ND56F3G)
  func Detect(flag string, stdout *os.File) (Format, error)
  type LabelCount struct {
      Label string `json:"label"`
      Count int    `json:"count"`
  }
  func WriteLabels(w io.Writer, f Format, counts []LabelCount) error
  ```

- Produces (package-private, this ticket):

  ```go
  var labelCmd *cli.Command
  func labelAction(_ context.Context, cmd *cli.Command) error
  // labelCounts builds the vocabulary rows over the parseable items; state is
  // "open" (status != closed), "closed" (status == closed) or "all", already
  // validated. Rows sorted count desc, then label asc; empty vocabulary →
  // empty (possibly nil) slice.
  func labelCounts(items []*item.Item, state string) []format.LabelCount
  ```

- Not produced: `loadGraph`, `toEntry` (those belong to `awit list`, AWIT-0ND56J3G — do not define them here even as stubs); anything in `pkg/format` or `pkg/item`.

## Steps

- [ ] **Step 1: Write the failing tests.**

  Create `internal/cli/label_test.go`:

  ```go
  package cli

  import (
  	"os"
  	"path/filepath"
  	"testing"

  	"github.com/eisenwinter/awit/pkg/item"
  )

  // writeLabelItem drops a raw item file into a copied fixture. text is the
  // complete file: frontmatter fences included.
  func writeLabelItem(t *testing.T, repo, name, text string) {
  	t.Helper()
  	path := filepath.Join(repo, item.DirName, "items", name)
  	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
  		t.Fatalf("write %s: %v", path, err)
  	}
  }

  // enrichLabelFixture returns a copy of the clean fixture plus
  // AWIT-TEST0007 (closed, labels [auth]) and AWIT-TEST0008 (in_progress,
  // labels [db]), so the three --state values have vocabularies to differ in.
  func enrichLabelFixture(t *testing.T) string {
  	t.Helper()
  	repo := copyFixture(t, "clean")
  	writeLabelItem(t, repo, "AWIT-TEST0007.md", `---
  id: AWIT-TEST0007
  title: Retire legacy session cookies
  brief: Remove the cookie fallback once bearer auth ships.
  status: closed
  deps: []
  labels: [auth]
  refs: []
  ---

  ## Summary

  Closed work that used to carry the auth label.

  ## Acceptance Criteria

  - Cookie fallback is gone
  `)
  	writeLabelItem(t, repo, "AWIT-TEST0008.md", `---
  id: AWIT-TEST0008
  title: Harden refresh-token storage
  brief: Encrypt refresh tokens at rest before rotation ships.
  status: in_progress
  deps: []
  labels: [db]
  refs: []
  ---

  ## Summary

  Refresh tokens are stored in plain text today.

  ## Acceptance Criteria

  - Refresh tokens are encrypted at rest
  `)
  	return repo
  }

  // The pristine clean fixture: open vocabulary is auth 1, db 1, p0 1, p1 1
  // (0005 closed and 0006 in_progress carry no labels; 0003 has none either).
  // All counts tie at 1, so the order is the label-ascending tie-break.
  func TestLabelDefaultOpen(t *testing.T) {
  	repo := copyFixture(t, "clean")
  	code, stdout, stderr := run(t, "--repo", repo, "label")
  	if code != 0 {
  		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
  	}
  	want := "auth 1\ndb 1\np0 1\np1 1\n"
  	if stdout != want {
  		t.Fatalf("stdout = %q, want %q", stdout, want)
  	}
  	if stderr != "" {
  		t.Fatalf("stderr = %q, want empty", stderr)
  	}
  }

  // The only closed item in clean (0005) carries no labels: the closed
  // vocabulary is empty. Compact prints nothing, json prints "[]", exit 0.
  func TestLabelClosed(t *testing.T) {
  	repo := copyFixture(t, "clean")
  	code, stdout, stderr := run(t, "--repo", repo, "label", "--state", "closed")
  	if code != 0 {
  		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
  	}
  	if stdout != "" {
  		t.Fatalf("stdout = %q, want empty", stdout)
  	}
  	code, stdout, stderr = run(t, "--repo", repo, "--format", "json", "label", "--state", "closed")
  	if code != 0 {
  		t.Fatalf("json exit %d, want 0 (stderr %q)", code, stderr)
  	}
  	if stdout != "[]\n" {
  		t.Fatalf("json stdout = %q, want %q", stdout, "[]\n")
  	}
  }

  // all counts every parseable item: auth on 0001+0007, db on 0002+0008,
  // p0 on 0004, p1 on 0001. auth and db tie at 2 and order label-ascending.
  func TestLabelAll(t *testing.T) {
  	repo := enrichLabelFixture(t)
  	code, stdout, stderr := run(t, "--repo", repo, "label", "--state", "all")
  	if code != 0 {
  		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
  	}
  	want := "auth 2\ndb 2\np0 1\np1 1\n"
  	if stdout != want {
  		t.Fatalf("stdout = %q, want %q", stdout, want)
  	}
  }

  // open means "status != closed": the closed 0007 (auth) drops out, the
  // in_progress 0008 (db) stays in, and the count-desc sort puts db (2) first.
  func TestLabelOpenExcludesClosedIncludesInProgress(t *testing.T) {
  	repo := enrichLabelFixture(t)
  	code, stdout, stderr := run(t, "--repo", repo, "label")
  	if code != 0 {
  		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
  	}
  	want := "db 2\nauth 1\np0 1\np1 1\n"
  	if stdout != want {
  		t.Fatalf("stdout = %q, want %q", stdout, want)
  	}
  	// On the same copy, closed sees only 0007's label.
  	code, stdout, stderr = run(t, "--repo", repo, "label", "--state", "closed")
  	if code != 0 {
  		t.Fatalf("closed exit %d, want 0 (stderr %q)", code, stderr)
  	}
  	if stdout != "auth 1\n" {
  		t.Fatalf("closed stdout = %q, want %q", stdout, "auth 1\n")
  	}
  }

  func TestLabelBadState(t *testing.T) {
  	repo := copyFixture(t, "clean")
  	const msg = "Error: --state must be open, closed or all\n"
  	for _, state := range []string{"bogus", "OPEN", ""} {
  		t.Run(state, func(t *testing.T) {
  			code, stdout, stderr := run(t, "--repo", repo, "label", "--state", state)
  			if code != 1 {
  				t.Fatalf("exit %d, want 1 (stderr %q)", code, stderr)
  			}
  			if stderr != msg {
  				t.Fatalf("stderr = %q, want %q", stderr, msg)
  			}
  			if stdout != "" {
  				t.Fatalf("stdout = %q, want empty", stdout)
  			}
  		})
  	}
  }

  func TestLabelJSON(t *testing.T) {
  	repo := copyFixture(t, "clean")
  	code, stdout, stderr := run(t, "--repo", repo, "--format", "json", "label")
  	if code != 0 {
  		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
  	}
  	want := `[
    {
      "label": "auth",
      "count": 1
    },
    {
      "label": "db",
      "count": 1
    },
    {
      "label": "p0",
      "count": 1
    },
    {
      "label": "p1",
      "count": 1
    }
  ]
  `
  	if stdout != want {
  		t.Fatalf("stdout =\n%s\nwant\n%s", stdout, want)
  	}
  }

  func TestLabelTableGolden(t *testing.T) {
  	repo := copyFixture(t, "clean")
  	code, stdout, stderr := run(t, "--repo", repo, "--format", "table", "label")
  	if code != 0 {
  		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
  	}
  	golden(t, "label_clean_table.golden", []byte(stdout))
  }

  // 0001 in the dangling fixture has deps: [AWIT-TEST9999], so the graph
  // quarantines it — but it parses, so its auth and p1 must count. The extra
  // AWIT-TEST0003 is invalid YAML, so its ghost label must not count at all.
  func TestLabelCountsQuarantinedSkipsBroken(t *testing.T) {
  	repo := copyFixture(t, "dangling")
  	writeLabelItem(t, repo, "AWIT-TEST0003.md", `---
  id: AWIT-TEST0003
  title: [unclosed
  status: open
  deps: []
  labels: [ghost]
  refs: []
  ---

  Invalid YAML on purpose; this label must not count.
  `)
  	code, stdout, stderr := run(t, "--repo", repo, "label")
  	if code != 0 {
  		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
  	}
  	want := "auth 1\np1 1\n"
  	if stdout != want {
  		t.Fatalf("stdout = %q, want %q (ghost must be absent, quarantined 0001 counted)", stdout, want)
  	}
  }

  // A bad --format value must surface Detect's error, not be swallowed.
  func TestLabelBadFormat(t *testing.T) {
  	repo := copyFixture(t, "clean")
  	code, stdout, stderr := run(t, "--repo", repo, "--format", "yaml", "label")
  	if code != 1 {
  		t.Fatalf("exit %d, want 1 (stderr %q)", code, stderr)
  	}
  	want := "Error: format: unknown format \"yaml\" (compact|table|json)\n"
  	if stderr != want {
  		t.Fatalf("stderr = %q, want %q", stderr, want)
  	}
  	if stdout != "" {
  		t.Fatalf("stdout = %q, want empty", stdout)
  	}
  }
  ```

- [ ] **Step 2: Run it, see it fail.**

  ```bash
  go test ./internal/cli -run 'TestLabel' -count=1 -v
  ```

  Expected: `label` is not a registered command, so every test fails with exit 2 (line numbers will differ):

  ```text
  === RUN   TestLabelDefaultOpen
      label_test.go:64: exit 2, want 0 (stderr "unknown command \"label\" (run \"awit --help\")\n")
  --- FAIL: TestLabelDefaultOpen (0.00s)
  === RUN   TestLabelBadState
      label_test.go:132: exit 2, want 1 (stderr "unknown command \"label\" (run \"awit --help\")\n")
  --- FAIL: TestLabelBadState (0.00s)
  ...
  FAIL
  FAIL	github.com/eisenwinter/awit/internal/cli
  ```

  Do not skip this red. If the failure is a compile error instead (undefined helper), fix the test file to match the helper names in `helpers_test.go` and re-run.

- [ ] **Step 3: Implement `internal/cli/label.go` and register the command.**

  Create `internal/cli/label.go`:

  ```go
  package cli

  import (
  	"cmp"
  	"context"
  	"fmt"
  	"os"
  	"slices"

  	"github.com/eisenwinter/awit/pkg/format"
  	"github.com/eisenwinter/awit/pkg/item"
  	"github.com/urfave/cli/v3"
  )

  var labelCmd = &cli.Command{
  	Name:  "label",
  	Usage: "Show the label vocabulary with usage counts",
  	Flags: []cli.Flag{
  		&cli.StringFlag{
  			Name:  "state",
  			Value: "open",
  			Usage: "count items whose status is open, closed or all (default open)",
  		},
  	},
  	Action: labelAction,
  }

  func labelAction(_ context.Context, cmd *cli.Command) error {
  	state := cmd.String("state")
  	switch state {
  	case "open", "closed", "all":
  	default:
  		return fmt.Errorf("--state must be open, closed or all")
  	}
  	store, err := openStore(cmd)
  	if err != nil {
  		return err
  	}
  	items, _, err := store.LoadAll() // broken files are deliberately not counted
  	if err != nil {
  		return err
  	}
  	// cmd.Writer is the root's stream, inherited by subcommands. In tests it
  	// is a bytes.Buffer, which is not an *os.File, so Detect falls back to
  	// compact — exactly the non-TTY default we want.
  	out, _ := cmd.Writer.(*os.File)
  	fm, err := format.Detect(cmd.Root().String("format"), out)
  	if err != nil {
  		return err
  	}
  	return format.WriteLabels(cmd.Writer, fm, labelCounts(items, state))
  }

  // labelCounts builds the label vocabulary rows over the parseable items.
  // Labels are never declared anywhere (guide §2): the vocabulary is whatever
  // the items use. state is one of open|closed|all and was validated by the
  // caller: "open" counts every status except closed, "closed" only closed,
  // "all" everything. Graph quarantine is irrelevant here — a parseable item
  // with a dangling dep still carries its labels — and unparseable files are
  // not in items at all, so they never count. Rows sort by count descending,
  // then label ascending, so the output is deterministic.
  func labelCounts(items []*item.Item, state string) []format.LabelCount {
  	counts := make(map[string]int)
  	for _, it := range items {
  		switch state {
  		case "open":
  			if it.Status == item.StatusClosed {
  				continue
  			}
  		case "closed":
  			if it.Status != item.StatusClosed {
  				continue
  			}
  		}
  		for _, label := range it.Labels {
  			counts[label]++
  		}
  	}
  	rows := make([]format.LabelCount, 0, len(counts))
  	for label, n := range counts {
  		rows = append(rows, format.LabelCount{Label: label, Count: n})
  	}
  	slices.SortFunc(rows, func(a, b format.LabelCount) int {
  		return cmp.Or(
  			cmp.Compare(b.Count, a.Count), // count descending
  			cmp.Compare(a.Label, b.Label), // label ascending
  		)
  	})
  	return rows
  }
  ```

  In `internal/cli/app.go`, append `labelCmd` to the `Commands` slice in `newRoot`. Keep every command other tickets already registered (`create`, `update`, …) — this ticket only adds one entry:

  ```go
  		Commands: []*cli.Command{
  			initCmd,
  			labelCmd, // awit label — AWIT-0ND5763G
  		},
  ```

- [ ] **Step 4: Create the golden, run everything green, commit.**

  First run without `-update` to see the golden miss:

  ```bash
  go test ./internal/cli -run 'TestLabel' -count=1 -v
  ```

  Expected: eight tests PASS; `TestLabelTableGolden` FAILs with
  `read golden ...label_clean_table.golden: ... (run: go test ./... -update)`.
  Then write the golden and re-run:

  ```bash
  go test ./internal/cli -run TestLabelTableGolden -update
  go test ./internal/cli -run 'TestLabel' -count=1 -v
  ```

  Expected: `--- PASS` for `TestLabelDefaultOpen`, `TestLabelClosed`,
  `TestLabelAll`, `TestLabelOpenExcludesClosedIncludesInProgress`,
  `TestLabelBadState` (three subtests), `TestLabelJSON`,
  `TestLabelTableGolden`, `TestLabelCountsQuarantinedSkipsBroken`,
  `TestLabelBadFormat`, then `ok github.com/eisenwinter/awit/internal/cli`.

  Verify the golden bytes (tabwriter: `LABEL` is the widest cell, plus the
  two-space gutter, so the first column is seven characters):

  ```bash
  cat testdata/golden/label_clean_table.golden
  ```

  Expected exactly (file ends with one newline after the `p1` row):

  ```text
  LABEL  COUNT
  auth   1
  db     1
  p0     1
  p1     1
  ```

  ```bash
  gofmt -w internal/cli/label.go internal/cli/label_test.go
  git add internal/cli/label.go internal/cli/label_test.go internal/cli/app.go testdata/golden/label_clean_table.golden
  git commit -m "cli/label: add label vocabulary with counts and --state"
  ```

- [ ] **Step 5: Full check for the packages touched.**

  ```bash
  go build ./...
  go vet ./...
  go test ./internal/cli -count=1
  ```

  Expected: no output from `build` and `vet`; the whole `internal/cli` package
  passes (skeleton, init and any other landed command tests included).

- [ ] **Step 6: Close the ticket.**

  1. Create `.awit/comments/AWIT-0ND5763G/<YYYYMMDDTHHMMSSZ>-<author>.md`
     (UTC stamp) with the real acceptance output pasted into a fenced block.
  2. In `.awit/items/AWIT-0ND5763G.md` set `status: closed` and append
     `- ../comments/AWIT-0ND5763G/<file>.md` to `refs`.
  3. Commit:

     ```bash
     git add .awit/items/AWIT-0ND5763G.md .awit/comments/AWIT-0ND5763G
     git commit -m "tickets: close AWIT-0ND5763G"
     ```

## Acceptance Criteria

- `go test ./internal/cli -run 'TestLabel' -count=1 -v` — all nine tests PASS,
  including every `TestLabelBadState` subtest (`bogus`, `OPEN`, empty).
- `go test ./internal/cli -count=1` — exit 0; earlier tickets' tests still pass.
- `go build ./... && go vet ./...` — no output, exit 0.
- `cat testdata/golden/label_clean_table.golden` — exactly the five lines shown
  in Step 4, ending `p1     1` plus one newline.
- `go build -o /tmp/awit ./cmd/awit` from the repo root, then:
  - `/tmp/awit --repo testdata/fixtures/clean --format compact label` prints
    `auth 1`, `db 1`, `p0 1`, `p1 1` (one per line), exit 0.
  - `/tmp/awit --repo testdata/fixtures/clean --format compact label --state closed`
    prints nothing, exit 0.
  - `/tmp/awit --repo testdata/fixtures/clean --format json label --state closed`
    prints `[]`, exit 0.
  - `/tmp/awit --repo testdata/fixtures/clean label --state bogus; echo exit=$?`
    prints `Error: --state must be open, closed or all` to stderr and
    `exit=1`.
  - `/tmp/awit --repo testdata/fixtures/dangling --format compact label`
    prints `auth 1` and `p1 1` (the graph-quarantined 0001 still counts).
- `gofmt -l internal/cli` prints nothing.

## Out of scope
- Any rendering change in `pkg/format` — `WriteLabels`, `Detect`, table/json
  shapes are AWIT-0ND56F3G's contract; a mismatch is a bug there, not here.
- `loadGraph` and `toEntry` (AWIT-0ND56J3G). This command never imports
  `pkg/graph`; if a counting rule seems to need the graph, reread guide §2.
- Label filtering (`-l`), which belongs to `list` / `next` / `prime`; label
  renaming or normalisation; a declared vocabulary file; case-folding.
- New fixture trees — tests only copy `clean` and `dangling` and add extra
  item files inside the copy. The only new testdata is the one golden file.
- Any `app.go` change beyond appending `labelCmd` to `Commands`.
- Writing anything to disk, locking, or committing from the command.
