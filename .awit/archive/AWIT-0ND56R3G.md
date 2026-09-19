---
id: AWIT-0ND56R3G
title: 'awit dep add / dep rm with cycle pre-check'
brief: >-
  Add the `dep` parent command with `add` and `rm` subcommands over `internal/cli/dep.go`. Add refuses cycles via `WouldCycle` before any write, prints the exact two-line error, and leaves the file byte-identical.
status: closed
deps: [AWIT-0ND56Q3G, AWIT-0ND56H3G]
labels: [phase2, p1]
refs_base: repo
refs:
  - plan/implementation-guide.md
  - plan/awit-implementation-plan.md
---

## Summary

After this ticket `awit dep add <id> <dep>` appends `<dep>` to `<id>`'s
`deps` (keeping order) and prints one compact line, unless the edge already
exists (`dependency already present`, exit 0) or would close a cycle
(two-line stderr error, exit 1, no write). `awit dep rm <id> <dep>`
removes the edge and prints one compact line, or exits 1 with
`<id> does not depend on <dep>`. Unknown IDs on either side are
`unknown item <id>`, exit 1. Seven command tests run green on copies of
the `clean` fixture.

## Context (read first)

- Guide §4.6 — `WouldCycle(from, to) []string`: DFS from `to` over `Deps`
  looking for `from`; `[from, from]` for self-edges; nil for unknown IDs.
  The caller validates existence first, so a non-nil result always prints.
- Guide §4.11 — `Main`, `openStore`, error contract (`Error: ` prefix on
  stderr, exit 1 for expected non-success, exit 2 for usage). Errors
  returned from `Action` are printed by `Main` as `Error: <msg>`; use
  `cli.Exit` only to change the code.
- Spec §Graph engine "Cycle pre-check on `dep add A B`" and §Error contract
  for the exact refusal text. Note the argument order in the message:
  `cannot add dependency <dep> to <id>` — the dependency first.
- **AWIT-0ND56Q3G** — `pkg/graph` `Ready`, `WouldCycle`, `DepIDs`.
  Must be `status: closed` before you start.
- **AWIT-0ND56H3G** — `internal/cli/create.go`; sibling command, do not
  modify it. Reuse its `detectFormat` only if you need it (you do not —
  both subcommands always print the compact line).
- **AWIT-0ND56J3G** owns `loadGraph` + `toEntry` in `internal/cli`
  (list ticket). If that ticket has not landed, define both in `dep.go`
  with exactly this code and say so in the commit message:

  ```go
  func loadGraph(s *item.Store) (*graph.Graph, error) {
      items, broken, err := s.LoadAll()
      if err != nil {
          return nil, err
      }
      return graph.Build(items, broken), nil
  }

  func toEntry(n *graph.Node) format.Entry {
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
  ```

- Helpers already present (do not redeclare): `openStore`
  (AWIT-0ND56G3G), `run`, `copyFixture`, `readItem` (helpers_test.go),
  `item.Store.Load` / `Save`, `item.Item.SetDeps`.
- On the `clean` fixture: `AWIT-TEST0004` deps `[AWIT-TEST0001,
  AWIT-TEST0003]`, so `WouldCycle(AWIT-TEST0001, AWIT-TEST0004)` is
  `[AWIT-TEST0001, AWIT-TEST0004, AWIT-TEST0001]` (0004 depends directly
  on 0001). `WouldCycle(AWIT-TEST0002, AWIT-TEST0001)` is nil.

## Files

- Create: `internal/cli/dep.go`
- Create: `internal/cli/dep_test.go`
- Modify: `internal/cli/app.go` — register `depCmd` in the root command's
  `Commands` list (one line).

## Interfaces

- Consumes (do not reimplement):

  ```go
  func openStore(cmd *cli.Command) (*item.Store, error)
  func loadGraph(s *item.Store) (*graph.Graph, error) // AWIT-0ND56J3G; define-if-absent above
  func toEntry(n *graph.Node) format.Entry             // AWIT-0ND56J3G; define-if-absent above
  func (g *graph.Graph) WouldCycle(from, to string) []string
  func (s *item.Store) Save(it *item.Item) error
  func (it *item.Item) SetDeps(v []string)
  func format.Line(e format.Entry) string
  ```

- Produces (this ticket, `internal/cli/dep.go`):

  ```go
  var depCmd *cli.Command     // parent "dep", subcommands add + rm
  func depAdd(cmd *cli.Command, id, dep string) error
  func depRm(cmd *cli.Command, id, dep string) error
  func printCompact(cmd *cli.Command, s *item.Store, id string) error // re-load graph, print format.Line
  ```

## Steps

- [ ] **Step 1: Write the failing tests.**

  Create `internal/cli/dep_test.go`:

  ```go
  package cli

  import (
      "bytes"
      "os"
      "path/filepath"
      "strings"
      "testing"
  )

  func readRaw(t *testing.T, repo, id string) []byte {
      t.Helper()
      data, err := os.ReadFile(filepath.Join(repo, ".awit", "items", id+".md"))
      if err != nil {
          t.Fatal(err)
      }
      return data
  }

  func changedLines(t *testing.T, before, after []byte) int {
      t.Helper()
      bl := strings.Split(string(before), "\n")
      al := strings.Split(string(after), "\n")
      if len(bl) != len(al) {
          // Report the shape so the failure is actionable.
          t.Fatalf("line count changed: %d -> %d", len(bl), len(al))
      }
      n := 0
      for i := range bl {
          if bl[i] != al[i] {
              n++
          }
      }
      return n
  }

  func TestDepAddWritesOneLine(t *testing.T) {
      dir := copyFixture(t, "clean")
      before := readRaw(t, dir, "AWIT-TEST0002")
      code, stdout, stderr := run(t, "--repo", dir, "dep", "add", "AWIT-TEST0002", "AWIT-TEST0001")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
      if !strings.HasPrefix(stdout, "[AWIT-TEST0002] Update database migration scripts | ") {
          t.Fatalf("stdout = %q, want compact line for AWIT-TEST0002", stdout)
      }
      if !strings.HasSuffix(stdout, "\n") || strings.Count(stdout, "\n") != 1 {
          t.Fatalf("stdout = %q, want exactly one line", stdout)
      }
      got := readItem(t, dir, "AWIT-TEST0002")
      if len(got.Deps) != 1 || got.Deps[0] != "AWIT-TEST0001" {
          t.Fatalf("deps = %v, want [AWIT-TEST0001]", got.Deps)
      }
      after := readRaw(t, dir, "AWIT-TEST0002")
      if n := changedLines(t, before, after); n != 1 {
          t.Fatalf("changed lines = %d, want 1", n)
      }
  }

  func TestDepAddRefusesCycle(t *testing.T) {
      dir := copyFixture(t, "clean")
      before := readRaw(t, dir, "AWIT-TEST0001")
      code, stdout, stderr := run(t, "--repo", dir, "dep", "add", "AWIT-TEST0001", "AWIT-TEST0004")
      if code != 1 {
          t.Fatalf("exit %d, want 1", code)
      }
      if stdout != "" {
          t.Fatalf("stdout = %q, want empty", stdout)
      }
      want := "Error: cannot add dependency AWIT-TEST0004 to AWIT-TEST0001.\n" +
          "Cycle: AWIT-TEST0001 -> AWIT-TEST0004 -> AWIT-TEST0001\n"
      if stderr != want {
          t.Fatalf("stderr = %q, want %q", stderr, want)
      }
      if after := readRaw(t, dir, "AWIT-TEST0001"); !bytes.Equal(before, after) {
          t.Fatal("file was written despite refused cycle")
      }
  }

  func TestDepAddSelf(t *testing.T) {
      dir := copyFixture(t, "clean")
      before := readRaw(t, dir, "AWIT-TEST0002")
      code, _, stderr := run(t, "--repo", dir, "dep", "add", "AWIT-TEST0002", "AWIT-TEST0002")
      if code != 1 {
          t.Fatalf("exit %d, want 1", code)
      }
      want := "Error: cannot add dependency AWIT-TEST0002 to AWIT-TEST0002.\n" +
          "Cycle: AWIT-TEST0002 -> AWIT-TEST0002\n"
      if stderr != want {
          t.Fatalf("stderr = %q, want %q", stderr, want)
      }
      if after := readRaw(t, dir, "AWIT-TEST0002"); !bytes.Equal(before, after) {
          t.Fatal("file was written despite self-cycle")
      }
  }

  func TestDepAddUnknown(t *testing.T) {
      dir := copyFixture(t, "clean")
      code, _, stderr := run(t, "--repo", dir, "dep", "add", "AWIT-TEST0099", "AWIT-TEST0001")
      if code != 1 || stderr != "Error: unknown item AWIT-TEST0099\n" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
      code, _, stderr = run(t, "--repo", dir, "dep", "add", "AWIT-TEST0001", "AWIT-TEST0099")
      if code != 1 || stderr != "Error: unknown item AWIT-TEST0099\n" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
  }

  func TestDepAddAlreadyPresent(t *testing.T) {
      dir := copyFixture(t, "clean")
      before := readRaw(t, dir, "AWIT-TEST0003")
      code, stdout, stderr := run(t, "--repo", dir, "dep", "add", "AWIT-TEST0003", "AWIT-TEST0001")
      if code != 0 {
          t.Fatalf("exit %d stderr %q, want 0", code, stderr)
      }
      if stdout != "dependency already present\n" {
          t.Fatalf("stdout = %q, want %q", stdout, "dependency already present\n")
      }
      if after := readRaw(t, dir, "AWIT-TEST0003"); !bytes.Equal(before, after) {
          t.Fatal("file was rewritten for an already-present edge")
      }
  }

  func TestDepRm(t *testing.T) {
      dir := copyFixture(t, "clean")
      code, stdout, stderr := run(t, "--repo", dir, "dep", "rm", "AWIT-TEST0004", "AWIT-TEST0003")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
      if !strings.HasPrefix(stdout, "[AWIT-TEST0004] Rotate API tokens | ") {
          t.Fatalf("stdout = %q, want compact line for AWIT-TEST0004", stdout)
      }
      got := readItem(t, dir, "AWIT-TEST0004")
      if len(got.Deps) != 1 || got.Deps[0] != "AWIT-TEST0001" {
          t.Fatalf("deps = %v, want [AWIT-TEST0001]", got.Deps)
      }
  }

  func TestDepRmAbsent(t *testing.T) {
      dir := copyFixture(t, "clean")
      before := readRaw(t, dir, "AWIT-TEST0002")
      code, _, stderr := run(t, "--repo", dir, "dep", "rm", "AWIT-TEST0002", "AWIT-TEST0001")
      if code != 1 || stderr != "Error: AWIT-TEST0002 does not depend on AWIT-TEST0001\n" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
      if after := readRaw(t, dir, "AWIT-TEST0002"); !bytes.Equal(before, after) {
          t.Fatal("file was written despite absent edge")
      }
      code, _, stderr = run(t, "--repo", dir, "dep", "rm", "AWIT-TEST0099", "AWIT-TEST0001")
      if code != 1 || stderr != "Error: unknown item AWIT-TEST0099\n" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
  }
  ```

- [ ] **Step 2: Run them, see them fail.**

  ```bash
  go test ./internal/cli -run 'TestDepAdd|TestDepRm' -v
  ```

  Expected failure:

  ```text
  # github.com/eisenwinter/awit/internal/cli [github.com/eisenwinter/awit/internal/cli.test]
  internal/cli/dep_test.go: undefined: copyFixture
  FAIL	github.com/eisenwinter/awit/internal/cli [build failed]
  ```

  (or, if helpers already exist, `undefined: depCmd` from the
  registration — either form is a valid red; do not skip it.)

- [ ] **Step 3: Implement `dep.go`.**

  Create `internal/cli/dep.go`:

  ```go
  package cli

  import (
      "context"
      "fmt"
      "slices"
      "strings"

      "github.com/eisenwinter/awit/pkg/format"
      "github.com/eisenwinter/awit/pkg/item"
      "github.com/urfave/cli/v3"
  )

  var depCmd = &cli.Command{
      Name:  "dep",
      Usage: "Add or remove item dependencies",
      Commands: []*cli.Command{
          {
              Name:      "add",
              Usage:     "Add a dependency with cycle pre-check",
              ArgsUsage: "<id> <dep>",
              Action: func(ctx context.Context, cmd *cli.Command) error {
                  if cmd.Args().Len() != 2 {
                      return cli.Exit("dep add needs <id> <dep>", 2)
                  }
                  return depAdd(cmd, cmd.Args().Get(0), cmd.Args().Get(1))
              },
          },
          {
              Name:      "rm",
              Usage:     "Remove a dependency",
              ArgsUsage: "<id> <dep>",
              Action: func(ctx context.Context, cmd *cli.Command) error {
                  if cmd.Args().Len() != 2 {
                      return cli.Exit("dep rm needs <id> <dep>", 2)
                  }
                  return depRm(cmd, cmd.Args().Get(0), cmd.Args().Get(1))
              },
          },
      },
  }

  func depAdd(cmd *cli.Command, id, dep string) error {
      s, err := openStore(cmd)
      if err != nil {
          return err
      }
      g, err := loadGraph(s)
      if err != nil {
          return err
      }
      if _, ok := g.Nodes[id]; !ok {
          return fmt.Errorf("unknown item %s", id)
      }
      if _, ok := g.Nodes[dep]; !ok {
          return fmt.Errorf("unknown item %s", dep)
      }
      it := g.Nodes[id].Item
      if slices.Contains(it.Deps, dep) {
          fmt.Fprintln(cmd.Writer, "dependency already present")
          return nil
      }
      if cyc := g.WouldCycle(id, dep); cyc != nil {
          fmt.Fprintf(cmd.ErrWriter, "Error: cannot add dependency %s to %s.\n", dep, id)
          fmt.Fprintf(cmd.ErrWriter, "Cycle: %s\n", strings.Join(cyc, " -> "))
          return cli.Exit("", 1)
      }
      next := append(slices.Clone(it.Deps), dep)
      it.SetDeps(next)
      if err := s.Save(it); err != nil {
          return err
      }
      return printCompact(cmd, s, id)
  }

  func depRm(cmd *cli.Command, id, dep string) error {
      s, err := openStore(cmd)
      if err != nil {
          return err
      }
      g, err := loadGraph(s)
      if err != nil {
          return err
      }
      if _, ok := g.Nodes[id]; !ok {
          return fmt.Errorf("unknown item %s", id)
      }
      it := g.Nodes[id].Item
      if !slices.Contains(it.Deps, dep) {
          return fmt.Errorf("%s does not depend on %s", id, dep)
      }
      next := make([]string, 0, len(it.Deps)-1)
      for _, d := range it.Deps {
          if d != dep {
              next = append(next, d)
          }
      }
      it.SetDeps(next)
      if err := s.Save(it); err != nil {
          return err
      }
      return printCompact(cmd, s, id)
  }

  // printCompact reloads the graph so the printed unblock count reflects the
  // edit, then prints one compact line for id.
  func printCompact(cmd *cli.Command, s *item.Store, id string) error {
      g, err := loadGraph(s)
      if err != nil {
          return err
      }
      n, ok := g.Nodes[id]
      if !ok {
          return fmt.Errorf("unknown item %s", id)
      }
      fmt.Fprintln(cmd.Writer, format.Line(toEntry(n)))
      return nil
  }
  ```

  If `loadGraph`/`toEntry` do not exist yet, add the define-if-absent
  block from Context above to this same file. In `internal/cli/app.go`,
  add `depCmd` to the root `Commands` slice.

  Details that matter:

  - `cli.Exit("", 1)` after writing both refusal lines to
    `cmd.ErrWriter`: `Main` (AWIT-0ND5683G) maps an `ExitCoder` to its
    code and prints `Message` only when non-empty, so nothing is printed
    twice. Do not return a plain error here — it would add a third line.
  - The already-present check runs before `WouldCycle`: re-adding an
    edge that is part of no cycle must still say `dependency already
    present`, and re-adding must never report a cycle.
  - `append(slices.Clone(it.Deps), dep)` keeps existing order and appends
    at the end; never sort.
  - `rm` keeps the remaining deps in their original order.
  - The `dep` parent takes no action itself; `awit dep` alone shows help
    (urfave default).

- [ ] **Step 4: Run the tests, see them pass.**

  ```bash
  go test ./internal/cli -run 'TestDepAdd|TestDepRm' -v
  ```

  Expected:

  ```text
  === RUN   TestDepAddWritesOneLine
  --- PASS: TestDepAddWritesOneLine
  === RUN   TestDepAddRefusesCycle
  --- PASS: TestDepAddRefusesCycle
  === RUN   TestDepAddSelf
  --- PASS: TestDepAddSelf
  === RUN   TestDepAddUnknown
  --- PASS: TestDepAddUnknown
  === RUN   TestDepAddAlreadyPresent
  --- PASS: TestDepAddAlreadyPresent
  === RUN   TestDepRm
  --- PASS: TestDepRm
  === RUN   TestDepRmAbsent
  --- PASS: TestDepRmAbsent
  PASS
  ok  	github.com/eisenwinter/awit/internal/cli
  ```

  Then the whole package stays green:

  ```bash
  go test ./internal/cli -count=1
  ```

- [ ] **Step 5: Commit.**

  ```bash
  gofmt -l internal/cli
  go vet ./internal/cli
  git add internal/cli/dep.go internal/cli/dep_test.go internal/cli/app.go
  git commit -m "cli/dep: add/remove with cycle pre-check"
  ```

- [ ] **Step 6: Close this ticket.**

  Set `status: closed` in this file's frontmatter, then:

  ```bash
  git add .awit/items/AWIT-0ND56R3G.md
  git commit -m "tickets: close AWIT-0ND56R3G"
  ```

## Acceptance Criteria

- `go test ./internal/cli -run 'TestDepAdd|TestDepRm' -count=1 -v` — all
  seven tests PASS; `go test ./internal/cli -count=1` stays green.
- On a copy of the `clean` fixture,
  `awit dep add AWIT-TEST0002 AWIT-TEST0001` exits 0, prints one compact
  line, and the git diff of `AWIT-TEST0002.md` is exactly one line
  (`deps: []` → `deps: [AWIT-TEST0001]`).
- `awit dep add AWIT-TEST0001 AWIT-TEST0004` exits 1, stderr is exactly
  `Error: cannot add dependency AWIT-TEST0004 to AWIT-TEST0001.\nCycle:
  AWIT-TEST0001 -> AWIT-TEST0004 -> AWIT-TEST0001\n`, and the file is
  byte-identical afterwards.
- `awit dep add AWIT-TEST0003 AWIT-TEST0001` exits 0 with
  `dependency already present` and no write.
- `awit dep rm AWIT-TEST0004 AWIT-TEST0003` exits 0, prints one compact
  line, deps become `[AWIT-TEST0001]`; removing a non-edge exits 1 with
  `Error: AWIT-TEST0004 does not depend on <dep>`-shaped text.
- `gofmt -l internal/cli` prints nothing.

## Out of scope

- `awit list`, `awit show`, `awit validate`, `awit prime`, `awit next`.
- Dangling-dep creation guards beyond the unknown-ID check (a dep on a
  missing file is refused as unknown; quarantine of dangling deps stays
  in `pkg/graph`).
- Quoted or comma-separated multi-dep adds (`-d A,B` belongs to create).
- Redeclaring `openStore`, `loadGraph`, `toEntry`, `run`, `copyFixture`,
  `readItem` if they already exist.
