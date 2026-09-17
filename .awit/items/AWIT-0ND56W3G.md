---
id: AWIT-0ND56W3G
title: 'pkg/prime renderer + awit prime'
brief: >-
  Implement the deterministic `pkg/prime` snapshot renderer (GRAPH
  WARNINGS, READY, BLOCKED, CRITICAL PATH) with label filtering and
  `--max-tokens` truncation, plus the `awit prime` command with
  `--max-tokens` and `-l`. READY lines reuse `format.Line`; prime never
  imports `internal/cli`.
status: closed
deps: [AWIT-0ND56V3G, AWIT-0ND56S3G]
labels: [phase3, p0]
refs:
  - ../../plan/implementation-guide.md
  - ../../plan/awit-implementation-plan.md
---

## Summary

After this ticket `pkg/prime/prime.go` exposes `Options`, `Render`, and
`EstimateTokens` per guide §4.8, and `internal/cli/prime.go` registers
`awit prime` with `--max-tokens` and `-l` (the global `--format` is
accepted and ignored). Sections print in order GRAPH WARNINGS (omitted
when empty), READY, BLOCKED, CRITICAL PATH (omitted when empty) with one
blank line between sections and a single trailing newline. Warnings for
CYCLE and DANGLING carry ` (excluded from next)`. READY renders via
`format.Line`; BLOCKED renders `[ID] Title <- dep1, dep2`; CRITICAL PATH
renders one `A -> B -> C` line. Label groups filter ready and blocked;
header counts are post-filter, pre-truncation. `--max-tokens` always
keeps warnings and the critical path, then admits ready lines, then
blocked lines, while the whole output fits; dropped lines collapse into
one trailing `(+N more)`. Six tests run green against two committed
goldens; output never contains CR bytes.

## Context (read first)

- Guide §4.8 — exact `Options`, `Render`, `EstimateTokens` signatures.
  `EstimateTokens = len(b)/4`, integer division.
- Guide §2 decision 1 — label groups are AND across, OR within; reuse
  `graph.FilterLabels`, do not reimplement it.
- Guide §2 decision 5 — the estimate is documented as approximate; the
  doc comment on `EstimateTokens` must say so.
- Spec §`awit prime` — the four-section example, sort orders (ready by
  unblocks desc then ID asc; blocked by ID), and the truncation rule
  ("warnings and critical path always fit, then ready items drop from
  the bottom, then blocked, ending with `(+N more)`").
- **AWIT-0ND56V3G** — `Graph.CriticalPath()` over non-closed,
  non-quarantined nodes. Must be `status: closed` before you start.
- **AWIT-0ND56S3G** — `awit validate`; sibling surface, only reused for
  the warnings vocabulary (`[REASON] detail`). Do not modify it.
- **AWIT-0ND56Q3G** — `Ready`, `Blocked`, `UnblockCount`, `OpenDepIDs`,
  `FilterLabels`. Consume all five; none change here.
- Fixture facts from AWIT-0ND56N3G (copy titles verbatim, do not
  paraphrase). On `clean`: READY is 0001 (unblocks 2), 0002 (0), 0006
  (0); BLOCKED is 0003 (`Add E2E auth tests`, deps `[AWIT-TEST0001]`),
  0004 (`Rotate API tokens`, deps `[AWIT-TEST0001, AWIT-TEST0003]`,
  labels `[p0]`); critical path is
  `AWIT-TEST0001 -> AWIT-TEST0003 -> AWIT-TEST0004`. Item 0006
  (`Implement refresh-token rotation`) has no labels. No warnings, so no
  WARNINGS section. On `cyclic`: two CYCLE faults (0001→0002→0003→0001
  and the 0004 self-loop); READY is 0005 (`Update the changelog`) alone;
  BLOCKED is empty; critical path is `[AWIT-TEST0005]`.
- `format.Line(e)` renders
  `[ID] Title | label1,label2 | Unblocks: N` (`-` for empty labels).
  The prime package builds its own `format.Entry` from the node — it
  MUST NOT import `internal/cli` (import cycle). Mirror the `toEntry`
  state mapping (quarantined → `quarantined`, closed → `closed`,
  `Ready` → `ready`, else `blocked`); only ready nodes reach `Line`
  here, but keep the mapping total so future callers cannot observe a
  skew.
- Helpers already present (do not redeclare): `openStore`,
  `SplitLabels`, `run`, `copyFixture`, `golden` (helpers_test.go).

## Files

- Create: `pkg/prime/prime.go`
- Create: `pkg/prime/prime_test.go`
- Create: `internal/cli/prime.go`
- Create: `internal/cli/prime_test.go`
- Create: `testdata/golden/prime-clean.golden`
- Create: `testdata/golden/prime-cyclic.golden`

## Interfaces

- Consumes (do not reimplement):

  ```go
  func (g *graph.Graph) Ready() []*Node
  func (g *graph.Graph) Blocked() []*Node
  func (g *graph.Graph) CriticalPath() []*Node
  func FilterLabels(nodes []*Node, groups [][]string) []*Node
  func (n *Node) OpenDepIDs() []string
  func (n *Node) Quarantined() bool
  func Line(e Entry) string
  func SplitLabels(flags []string) [][]string
  func openStore(cmd *cli.Command) (*item.Store, error)
  func loadGraph(s *item.Store) (*graph.Graph, error)
  ```

- Produces (this ticket, `pkg/prime`):

  ```go
  package prime

  type Options struct {
      MaxTokens int        // 0 = unlimited
      Labels    [][]string // FilterLabels groups
  }
  func Render(w io.Writer, g *graph.Graph, opts Options) error
  func EstimateTokens(b []byte) int
  ```

- Produces (this ticket, `internal/cli/prime.go`):

  ```go
  var primeCmd *cli.Command // flags: --max-tokens int, -l/--label StringSlice
  ```

## Steps

- [ ] **Step 1: Write the golden files.**

  Create `testdata/golden/prime-clean.golden` with exactly these bytes
  (single trailing newline, no CRLF, no WARNINGS section):

  ```text
  === READY (3) ===
  [AWIT-TEST0001] Implement OAuth2 bearer token extraction | auth,p1 | Unblocks: 2
  [AWIT-TEST0002] Update database migration scripts | db | Unblocks: 0
  [AWIT-TEST0006] Implement refresh-token rotation | - | Unblocks: 0

  === BLOCKED (2) ===
  [AWIT-TEST0003] Add E2E auth tests <- AWIT-TEST0001
  [AWIT-TEST0004] Rotate API tokens <- AWIT-TEST0001, AWIT-TEST0003

  === CRITICAL PATH (3) ===
  AWIT-TEST0001 -> AWIT-TEST0003 -> AWIT-TEST0004
  ```

  Create `testdata/golden/prime-cyclic.golden` with exactly these bytes;
  if `scc.go` words `Fault.Detail` differently from the chains below,
  generate with `-update` in Step 6 and verify the shape by eye before
  committing — but each warning line MUST still start with `[CYCLE] `
  and end with ` (excluded from next)` (the structural test enforces
  that independently of the golden bytes):

  ```text
  === GRAPH WARNINGS ===
  [CYCLE] AWIT-TEST0001 -> AWIT-TEST0002 -> AWIT-TEST0003 -> AWIT-TEST0001 (excluded from next)
  [CYCLE] AWIT-TEST0004 -> AWIT-TEST0004 (excluded from next)

  === READY (1) ===
  [AWIT-TEST0005] Update the changelog | - | Unblocks: 0

  === BLOCKED (0) ===

  === CRITICAL PATH (1) ===
  AWIT-TEST0005
  ```

- [ ] **Step 2: Write the failing tests.**

  Create `pkg/prime/prime_test.go` (package `prime`; needs imports
  `bytes`, `flag`, `os`, `path/filepath`, `runtime`, `strings`,
  `testing`, plus `graph` and `item`):

  ```go
  var update = flag.Bool("update", false, "rewrite golden files")

  func repoRoot(t *testing.T) string {
      t.Helper()
      _, file, _, ok := runtime.Caller(0)
      if !ok {
          t.Fatal("runtime.Caller")
      }
      return filepath.Dir(filepath.Dir(filepath.Dir(file)))
  }

  func load(t *testing.T, name string) *graph.Graph {
      t.Helper()
      st, err := item.Open(filepath.Join(repoRoot(t), "testdata", "fixtures", name))
      if err != nil {
          t.Fatal(err)
      }
      items, broken, err := st.LoadAll()
      if err != nil {
          t.Fatal(err)
      }
      return graph.Build(items, broken)
  }

  func golden(t *testing.T, name string, got []byte) {
      t.Helper()
      path := filepath.Join(repoRoot(t), "testdata", "golden", name)
      if *update {
          if err := os.WriteFile(path, got, 0o644); err != nil {
              t.Fatal(err)
          }
      }
      want, err := os.ReadFile(path)
      if err != nil {
          t.Fatal(err)
      }
      if !bytes.Equal(got, want) {
          t.Errorf("got:\n%s\nwant:\n%s\nhint: go test ./pkg/prime -update", got, want)
      }
  }

  func render(t *testing.T, g *graph.Graph, opts Options) string {
      t.Helper()
      var b bytes.Buffer
      if err := Render(&b, g, opts); err != nil {
          t.Fatal(err)
      }
      return b.String()
  }

  func TestPrimeDeterministic(t *testing.T) {
      a := render(t, load(t, "clean"), Options{})
      b := render(t, load(t, "clean"), Options{})
      if a != b {
          t.Fatalf("two renders differ:\n%s\n---\n%s", a, b)
      }
  }

  func TestPrimeGoldenClean(t *testing.T) {
      out := render(t, load(t, "clean"), Options{})
      if strings.Contains(out, "=== GRAPH WARNINGS ===") {
          t.Fatal("clean output must omit the warnings section")
      }
      golden(t, "prime-clean.golden", []byte(out))
  }

  func TestPrimeGoldenCyclic(t *testing.T) {
      out := render(t, load(t, "cyclic"), Options{})
      lines := strings.Split(out, "\n")
      if lines[0] != "=== GRAPH WARNINGS ===" {
          t.Fatalf("first line = %q", lines[0])
      }
      var warns []string
      for _, l := range lines[1:] {
          if l == "" {
              break
          }
          warns = append(warns, l)
      }
      if len(warns) != 2 {
          t.Fatalf("warnings = %v, want 2 lines", warns)
      }
      for _, w := range warns {
          if !strings.HasPrefix(w, "[CYCLE] ") || !strings.HasSuffix(w, " (excluded from next)") {
              t.Fatalf("warning = %q", w)
          }
      }
      if !strings.Contains(out, "=== READY (1) ===\n[AWIT-TEST0005] Update the changelog | - | Unblocks: 0\n") {
          t.Fatalf("ready section missing 0005:\n%s", out)
      }
      if !strings.Contains(out, "=== BLOCKED (0) ===\n\n=== CRITICAL PATH (1) ===\nAWIT-TEST0005\n") {
          t.Fatalf("blocked/critical sections wrong:\n%s", out)
      }
      golden(t, "prime-cyclic.golden", []byte(out))
  }

  func TestPrimeMaxTokens(t *testing.T) {
      limited := render(t, load(t, "clean"), Options{MaxTokens: 40})
      if !strings.Contains(limited, "AWIT-TEST0001 -> AWIT-TEST0003 -> AWIT-TEST0004\n") {
          t.Fatalf("critical path must survive truncation:\n%s", limited)
      }
      if !strings.Contains(limited, "=== READY (3) ===") {
          t.Fatalf("header counts stay pre-truncation:\n%s", limited)
      }
      idx := strings.LastIndex(limited, "(+")
      if idx < 0 || !strings.HasSuffix(limited, " more)\n") {
          t.Fatalf("dropped lines must collapse into a trailing (+N more) line:\n%s", limited)
      }
      var dropped int
      if _, err := strings.Sscanf(limited[idx:], "(+%d more)", &dropped); err != nil || dropped < 1 {
          t.Fatalf("bad (+N more) suffix: %q", limited[idx:])
      }
      shown := strings.Count(limited, "[AWIT-TEST")
      if shown+dropped != 5 {
          t.Fatalf("shown %d + dropped %d != 5 ready+blocked lines", shown, dropped)
      }
  }
  ```

  (`5` = 3 ready + 2 blocked on `clean`; the invariant ties truncation
  accounting to the fixture instead of trusting the suffix alone.)

  Create `internal/cli/prime_test.go` (reuses that package's `run`,
  `copyFixture`, `golden` — do not redeclare them):

  ```go
  func TestPrimeCLI(t *testing.T) {
      dir := copyFixture(t, "clean")
      code, stdout, stderr := run(t, "--repo", dir, "prime")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
      golden(t, "prime-clean.golden", []byte(stdout))
      if strings.Contains(stdout, "\r") {
          t.Fatal("output contains CR bytes")
      }
  }

  func TestPrimeLabelFilter(t *testing.T) {
      dir := copyFixture(t, "clean")
      code, stdout, stderr := run(t, "--repo", dir, "prime", "-l", "p0")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
      if !strings.Contains(stdout, "=== READY (0) ===\n") {
          t.Fatalf("ready must be empty after -l p0:\n%s", stdout)
      }
      if !strings.Contains(stdout, "=== BLOCKED (1) ===\n[AWIT-TEST0004] Rotate API tokens <- AWIT-TEST0001, AWIT-TEST0003\n") {
          t.Fatalf("blocked must hold only 0004:\n%s", stdout)
      }
      if !strings.Contains(stdout, "=== CRITICAL PATH (3) ===\nAWIT-TEST0001 -> AWIT-TEST0003 -> AWIT-TEST0004\n") {
          t.Fatalf("critical path is unfiltered:\n%s", stdout)
      }
  }

  func TestPrimeMaxTokensCLI(t *testing.T) {
      dir := copyFixture(t, "clean")
      code, stdout, stderr := run(t, "--repo", dir, "prime", "--max-tokens", "40")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
      if !strings.Contains(stdout, "AWIT-TEST0001 -> AWIT-TEST0003 -> AWIT-TEST0004\n") {
          t.Fatalf("critical path must survive:\n%s", stdout)
      }
      if !strings.Contains(stdout, " more)\n") {
          t.Fatalf("want (+N more) suffix:\n%s", stdout)
      }
      // The global --format flag is ignored by prime, even when bogus.
      code, _, stderr = run(t, "--repo", dir, "--format", "bogus", "prime")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q, want format ignored", code, stderr)
      }
  }
  ```

- [ ] **Step 3: Run them, see them fail.**

  ```bash
  go test ./pkg/prime -run 'TestPrime' -v
  go test ./internal/cli -run 'TestPrime' -v
  ```

  Expected failure:

  ```text
  # github.com/eisenwinter/awit/pkg/prime [github.com/eisenwinter/awit/pkg/prime.test]
  pkg/prime/prime_test.go: undefined: Render
  FAIL	github.com/eisenwinter/awit/pkg/prime [build failed]
  ```

  and `undefined: primeCmd` (or a registration failure) for the CLI
  package. Do not skip the red.

- [ ] **Step 4: Implement `pkg/prime/prime.go`.**

  Write the file exactly as below (no more, no less — in particular no
  probing scaffolding, no unused helpers):

  ```go
  // Package prime renders the deterministic agent snapshot: warnings,
  // ready, blocked, and the critical path. Output is stable for identical
  // state (no timestamps, no map order) so prompts cache well.
  package prime

  import (
      "fmt"
      "io"
      "strings"

      "github.com/eisenwinter/awit/pkg/format"
      "github.com/eisenwinter/awit/pkg/graph"
      "github.com/eisenwinter/awit/pkg/item"
  )

  // Options tunes Render. MaxTokens 0 means unlimited.
  type Options struct {
      MaxTokens int
      Labels    [][]string
  }

  // EstimateTokens approximates LLM tokens as len(b)/4, integer division.
  // It is approximate by design (guide §2 decision 5); callers use it for
  // budgeting, never for billing.
  func EstimateTokens(b []byte) int {
      return len(b) / 4
  }

  // entryOf mirrors internal/cli.toEntry without importing it (importing
  // internal/cli from here would be a cycle). Keep the two in sync.
  func entryOf(n *graph.Node) format.Entry {
      state := "blocked"
      switch {
      case n.Quarantined():
          state = "quarantined"
      case n.Item.Status == item.StatusClosed:
          state = "closed"
      case n.Ready:
          state = "ready"
      }
      var faults []string
      for _, f := range n.Faults {
          faults = append(faults, "["+string(f.Reason)+"] "+f.Detail)
      }
      return format.Entry{
          ID:       n.Item.ID,
          Title:    n.Item.Title,
          Brief:    n.Item.Brief,
          Status:   string(n.Item.Status),
          State:    state,
          Labels:   append([]string(nil), n.Item.Labels...),
          Deps:     n.DepIDs(),
          Assignee: n.Item.Assignee,
          Unblocks: n.UnblockCount,
          Faults:   faults,
      }
  }

  func blockedLine(n *graph.Node) string {
      deps := n.OpenDepIDs()
      if len(deps) == 0 {
          return fmt.Sprintf("[%s] %s", n.Item.ID, n.Item.Title)
      }
      return fmt.Sprintf("[%s] %s <- %s", n.Item.ID, n.Item.Title, strings.Join(deps, ", "))
  }

  // Render writes the snapshot: GRAPH WARNINGS (omitted when empty),
  // READY, BLOCKED, CRITICAL PATH (omitted when empty). Sections are
  // separated by one blank line; the output ends with exactly one "\n"
  // and never contains "\r".
  func Render(w io.Writer, g *graph.Graph, opts Options) error {
      ready := g.Ready()
      blocked := g.Blocked()
      if len(opts.Labels) > 0 {
          ready = graph.FilterLabels(ready, opts.Labels)
          blocked = graph.FilterLabels(blocked, opts.Labels)
      }

      var head strings.Builder
      if len(g.Faults) > 0 {
          head.WriteString("=== GRAPH WARNINGS ===\n")
          for _, f := range g.Faults {
              line := "[" + string(f.Reason) + "] " + f.Detail
              if f.Reason == item.ReasonCycle || f.Reason == item.ReasonDangling {
                  line += " (excluded from next)"
              }
              head.WriteString(line + "\n")
          }
          head.WriteString("\n")
      }

      crit := g.CriticalPath()
      var tail strings.Builder
      if len(crit) > 0 {
          ids := make([]string, 0, len(crit))
          for _, n := range crit {
              ids = append(ids, n.Item.ID)
          }
          fmt.Fprintf(&tail, "=== CRITICAL PATH (%d) ===\n%s\n", len(crit), strings.Join(ids, " -> "))
      }

      readyLines := make([]string, 0, len(ready))
      for _, n := range ready {
          readyLines = append(readyLines, format.Line(entryOf(n)))
      }
      blockedLines := make([]string, 0, len(blocked))
      for _, n := range blocked {
          blockedLines = append(blockedLines, blockedLine(n))
      }

      _, err := io.WriteString(w, assemble(head.String(), readyLines, blockedLines, tail.String(), len(ready), len(blocked), opts.MaxTokens))
      return err
  }

  // assemble joins the sections and applies the token budget. head
  // (warnings) and tail (critical path) always fit; ready lines are
  // admitted first, then blocked lines. Each probe builds the exact final
  // bytes that would result if nothing else were admitted afterwards, so
  // admission is monotone and the finished output always fits (when the
  // fixed head+tail alone allow it). Header counts are post-filter,
  // pre-truncation; dropped lines become one trailing "(+N more)" line.
  func assemble(head string, readyLines, blockedLines []string, tail string, nReady, nBlocked, max int) string {
      readyHead := fmt.Sprintf("=== READY (%d) ===\n", nReady)
      blockedHead := fmt.Sprintf("=== BLOCKED (%d) ===\n", nBlocked)
      sep := ""
      if tail != "" {
          sep = "\n"
      }
      join := func(lines []string) string {
          if len(lines) == 0 {
              return ""
          }
          return strings.Join(lines, "\n") + "\n"
      }
      var keptReady, keptBlocked []string
      dropped := 0
      for _, line := range readyLines {
          if max > 0 {
              probe := head + readyHead + join(append(append([]string{}, keptReady...), line)) + "\n" + blockedHead + sep + tail
              if EstimateTokens([]byte(probe)) > max {
                  dropped++
                  continue
              }
          }
          keptReady = append(keptReady, line)
      }
      for _, line := range blockedLines {
          if max > 0 {
              probe := head + readyHead + join(keptReady) + "\n" + blockedHead + join(append(append([]string{}, keptBlocked...), line)) + sep + tail
              if EstimateTokens([]byte(probe)) > max {
                  dropped++
                  continue
              }
          }
          keptBlocked = append(keptBlocked, line)
      }
      var out strings.Builder
      out.WriteString(head)
      out.WriteString(readyHead)
      out.WriteString(join(keptReady))
      out.WriteString("\n")
      out.WriteString(blockedHead)
      out.WriteString(join(keptBlocked))
      out.WriteString(sep)
      out.WriteString(tail)
      if dropped > 0 {
          fmt.Fprintf(&out, "(+%d more)\n", dropped)
      }
      return out.String()
  }
  ```

  Why the probes are exact: the ready probe contains every byte that
  precedes the blocked section plus the minimal blocked suffix (header
  + separator + tail); later blocked admissions only append bytes after
  passing their own exact probe, so the finished output always fits.
  Order is fixed, so the result is deterministic.

- [ ] **Step 5: Implement `internal/cli/prime.go`.**

  ```go
  package cli

  import (
      "context"

      "github.com/eisenwinter/awit/pkg/prime"
      "github.com/urfave/cli/v3"
  )

  var primeCmd = &cli.Command{
      Name:  "prime",
      Usage: "Print deterministic state snapshot for prompt injection",
      Flags: []cli.Flag{
          &cli.IntFlag{Name: "max-tokens", Usage: "token budget, 0 = unlimited"},
          &cli.StringSliceFlag{Name: "label", Aliases: []string{"l"}, Usage: "filter ready+blocked by label (repeatable)"},
      },
      Action: func(ctx context.Context, cmd *cli.Command) error {
          s, err := openStore(cmd)
          if err != nil {
              return err
          }
          g, err := loadGraph(s)
          if err != nil {
              return err
          }
          // NOTE: the global --format flag is intentionally ignored.
          return prime.Render(cmd.Writer, g, prime.Options{
              MaxTokens: cmd.Int("max-tokens"),
              Labels:    SplitLabels(cmd.StringSlice("label")),
          })
      },
  }
  ```

  Register `primeCmd` in `app.go`.

- [ ] **Step 6: Run everything green.**

  ```bash
  go test ./pkg/prime -run 'TestPrime' -v
  go test ./internal/cli -run 'TestPrime' -v
  go test ./pkg/prime ./internal/cli -count=1
  ```

  Expected: `TestPrimeDeterministic`, `TestPrimeGoldenClean`,
  `TestPrimeGoldenCyclic`, `TestPrimeMaxTokens`, `TestPrimeCLI`,
  `TestPrimeLabelFilter`, `TestPrimeMaxTokensCLI` all PASS. If
  `TestPrimeGoldenCyclic` fails only on the warning-chain wording, run
  `go test ./pkg/prime -run 'TestPrimeGoldenCyclic' -update`, inspect
  the diff (two `[CYCLE]` lines, same sections), and re-run.

- [ ] **Step 7: Commit.**

  ```bash
  gofmt -l pkg/prime internal/cli
  go vet ./pkg/prime ./internal/cli
  git add pkg/prime internal/cli/prime.go internal/cli/prime_test.go internal/cli/app.go testdata/golden/prime-clean.golden testdata/golden/prime-cyclic.golden
  git commit -m "prime: deterministic renderer with token budget and label filter"
  ```

- [ ] **Step 8: Close this ticket.**

  Set `status: closed` in this file's frontmatter, then:

  ```bash
  git add .awit/items/AWIT-0ND56W3G.md
  git commit -m "tickets: close AWIT-0ND56W3G"
  ```

## Acceptance Criteria

- `go test ./pkg/prime ./internal/cli -count=1` green; the seven prime
  tests PASS by name.
- `awit prime` on a copy of `clean` prints
  `testdata/golden/prime-clean.golden` byte-for-byte: READY 3 (0001
  unblocks 2, 0002 0, 0006 0), BLOCKED 2, CRITICAL 3, no WARNINGS.
- `awit prime` on `cyclic` prints 2 warning lines (each ending
  ` (excluded from next)`), READY (1) 0005, BLOCKED (0), CRITICAL (1).
- `awit prime --max-tokens 40` on `clean` still prints the full
  `AWIT-TEST0001 -> AWIT-TEST0003 -> AWIT-TEST0004` line, keeps header
  counts, and ends with a single `(+N more)` where shown + N == 5.
- `awit prime -l p0` on `clean` prints READY (0), BLOCKED (1) 0004, and
  the unfiltered CRITICAL (3).
- No output byte is `\r` on either OS (`TestPrimeCLI` asserts it; this
  renderer only ever writes `\n`).
- `pkg/prime` imports `format`, `graph`, `item` only — never
  `internal/cli`.
- `gofmt -l pkg/prime internal/cli` prints nothing.

## Out of scope

- `awit next` ranking and `--claim` (AWIT-0ND56X3G).
- Changing `CriticalPath`, `Ready`, `Blocked`, `FilterLabels`,
  `format.Line`, or any fixture.
- JSON output for prime (global `--format` is ignored by design).
- A tokenizer dependency: the estimate stays `len/4`.
