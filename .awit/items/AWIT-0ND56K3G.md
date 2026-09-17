---
id: AWIT-0ND56K3G
title: 'awit show (default view, quarantine flag)'
brief: >-
  Implement `awit show <id>` with the default view: header line, status
  with readiness, quarantine faults, deps, assignee, brief, ref count, and
  the verbatim body. Broken files render an unparseable view with exit 0;
  unknown IDs exit 1. JSON renders the entry plus body.
status: closed
deps: [AWIT-0ND56G3G, AWIT-0ND56F3G, AWIT-0ND56Q3G]
labels: [phase2, p1]
refs:
  - ../../plan/implementation-guide.md
  - ../../plan/awit-implementation-plan.md
---

## Summary

After this ticket `internal/cli/show.go` registers `awit show <id>` with
`ArgsUsage: <id>` and no local flags. The default text view prints the
header, the `status:` line, one `fault:` line per quarantine fault before
`deps:`, then assignee, brief, ref count, a blank line, and the body
verbatim. Quarantined items show `(QUARANTINED)` in the status line.
Unparseable files (parse errors, conflict markers, ID mismatches,
duplicate IDs) render `[ID] (unparseable)` plus their faults and exit 0.
Unknown IDs print `Error: unknown item <id>` and exit 1. With the global
`--format json` the command prints the entry plus `body`. Five command
tests run green, one against a committed golden file.

## Context (read first)

- Guide §4.7 — `format.Entry` fields (`ID`, `Title`, `Brief`, `Status`,
  `State`, `Labels`, `Deps`, `Assignee`, `Unblocks`, `Faults`) and
  `format.Line`. No new format code in this ticket.
- Guide §4.11 — `Main`, `openStore`, error contract. `detectFormat(cmd)`
  lives in `create.go` (AWIT-0ND56H3G, closed); consume it, do not
  redefine it.
- **AWIT-0ND56J3G** owns `loadGraph` + `toEntry` in `internal/cli`. If
  that ticket has not landed, define both in `show.go` with exactly the
  code quoted in AWIT-0ND56R3G's Context and note it in the commit
  message. `AWIT-0ND5703G` later extends this file with `--full` and
  `--refs-only`; keep rendering in small functions it can reuse
  (`defaultView(n *graph.Node) string`, `brokenView(...)`,
  `showJSON` struct).
- Helpers already present (do not redeclare): `openStore` (AWIT-0ND56G3G),
  `run`, `copyFixture`, `golden`, `readItem` (helpers_test.go),
  `item.Broken` (`ID`, `Path`, `Reason`, `Detail`).
- Fixture titles come from AWIT-0ND56N3G — copy them verbatim into the
  golden file, do not paraphrase:
  - `AWIT-TEST0001`: `Implement OAuth2 bearer token extraction`,
    brief `Fix header parsing so URL-safe bearer tokens authenticate.`,
    labels `[auth, p1]`, open, no deps, unblocks 2.
  - `AWIT-TEST0001` body (exact bytes, trailing newline):

    ```markdown
    ## Summary

    The auth middleware splits on spaces and assumes the token is strict base64.
    URL-safe tokens with `-` and `_` are dropped before signature verification.

    ## Acceptance Criteria

    - Tokens using the URL-safe base64 alphabet authenticate successfully
    - Invalid signatures still return a structured 401
    ```

  - `cyclic` fixture `AWIT-TEST0001` (deps `[AWIT-TEST0002]`) is
    quarantined with a `[CYCLE]` fault whose `Detail` carries the example
    chain (AWIT-0ND56P3G). The test asserts the status line and the
    `fault: [CYCLE]` prefix, not the full chain, so it is robust to the
    exact chain wording.
  - `parse-error` fixture `AWIT-TEST0001.md` (`title: [unclosed`) loads
    as `Broken` with `ReasonParse`.

## Files

- Create: `internal/cli/show.go`
- Create: `internal/cli/show_test.go`
- Create: `testdata/golden/show-clean.golden`
- Modify: `internal/cli/app.go` — register `showCmd` in the root
  command's `Commands` list (one line).

## Interfaces

- Consumes (do not reimplement):

  ```go
  func openStore(cmd *cli.Command) (*item.Store, error)
  func loadGraph(s *item.Store) (*graph.Graph, error) // AWIT-0ND56J3G; define-if-absent
  func toEntry(n *graph.Node) format.Entry             // AWIT-0ND56J3G; define-if-absent
  func detectFormat(cmd *cli.Command) (format.Format, error)
  ```

- Produces (this ticket, `internal/cli/show.go`):

  ```go
  var showCmd *cli.Command
  func defaultView(n *graph.Node) string
  func brokenView(id string, broken []item.Broken) string
  type showJSON struct {
      format.Entry
      Body string `json:"body"`
  }
  ```

## Steps

- [ ] **Step 1: Write the failing tests plus the golden file.**

  Create `testdata/golden/show-clean.golden` with exactly these bytes
  (trailing newline, no CRLF):

  ```text
  [AWIT-TEST0001] Implement OAuth2 bearer token extraction
  status: open (ready) | labels: auth,p1 | unblocks: 2
  deps: -
  assignee: -
  brief: Fix header parsing so URL-safe bearer tokens authenticate.
  refs: 0 (use --full)

  ## Summary

  The auth middleware splits on spaces and assumes the token is strict base64.
  URL-safe tokens with `-` and `_` are dropped before signature verification.

  ## Acceptance Criteria

  - Tokens using the URL-safe base64 alphabet authenticate successfully
  - Invalid signatures still return a structured 401
  ```

  Create `internal/cli/show_test.go`:

  ```go
  package cli

  import (
      "encoding/json"
      "strings"
      "testing"

      "github.com/eisenwinter/awit/pkg/item"
  )

  func TestShowClean(t *testing.T) {
      dir := copyFixture(t, "clean")
      code, stdout, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0001")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
      golden(t, "show-clean.golden", []byte(stdout))
  }

  func TestShowQuarantined(t *testing.T) {
      dir := copyFixture(t, "cyclic")
      code, stdout, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0001")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
      lines := strings.Split(stdout, "\n")
      if lines[0] != "[AWIT-TEST0001] Implement OAuth2 bearer token extraction" {
          t.Fatalf("header = %q", lines[0])
      }
      if lines[1] != "status: open (QUARANTINED) | labels: - | unblocks: -1" {
          t.Fatalf("status line = %q", lines[1])
      }
      if !strings.HasPrefix(lines[2], "fault: [CYCLE] ") {
          t.Fatalf("fault line = %q, want fault: [CYCLE] ...", lines[2])
      }
      if lines[3] != "deps: AWIT-TEST0002" {
          t.Fatalf("deps line = %q", lines[3])
      }
  }

  func TestShowBrokenFile(t *testing.T) {
      dir := copyFixture(t, "parse-error")
      code, stdout, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0001")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q, want exit 0", code, stderr)
      }
      lines := strings.Split(stdout, "\n")
      if lines[0] != "[AWIT-TEST0001] (unparseable)" {
          t.Fatalf("header = %q", lines[0])
      }
      if len(lines) < 2 || !strings.HasPrefix(lines[1], "fault: ["+string(item.ReasonParse)+"] ") {
          t.Fatalf("stdout = %q, want fault: [PARSE ERROR] ... on line 2", stdout)
      }
  }

  func TestShowUnknown(t *testing.T) {
      dir := copyFixture(t, "clean")
      code, stdout, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0099")
      if code != 1 {
          t.Fatalf("exit %d, want 1", code)
      }
      if stdout != "" {
          t.Fatalf("stdout = %q, want empty", stdout)
      }
      if stderr != "Error: unknown item AWIT-TEST0099\n" {
          t.Fatalf("stderr = %q", stderr)
      }
  }

  func TestShowJSON(t *testing.T) {
      dir := copyFixture(t, "clean")
      code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "show", "AWIT-TEST0001")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
      var got struct {
          ID       string   `json:"id"`
          Title    string   `json:"title"`
          Status   string   `json:"status"`
          State    string   `json:"state"`
          Labels   []string `json:"labels"`
          Deps     []string `json:"deps"`
          Unblocks int      `json:"unblocks"`
          Body     string   `json:"body"`
      }
      if err := json.Unmarshal([]byte(stdout), &got); err != nil {
          t.Fatalf("unmarshal: %v\nstdout = %q", err, stdout)
      }
      if got.ID != "AWIT-TEST0001" || got.State != "ready" || got.Unblocks != 2 {
          t.Fatalf("entry = %+v", got)
      }
      if len(got.Labels) != 2 || got.Labels[0] != "auth" || got.Labels[1] != "p1" {
          t.Fatalf("labels = %v", got.Labels)
      }
      if !strings.Contains(got.Body, "URL-safe tokens with `-` and `_` are dropped") {
          t.Fatalf("body = %q", got.Body)
      }
      if !strings.HasSuffix(stdout, "\n") {
          t.Fatal("json output must end with a single newline")
      }
  }
  ```

  Note: `TestShowQuarantined` pins `unblocks: -1` (guide §2: quarantined
  nodes have `UnblockCount == -1`) and `labels: -` (the cyclic 0001 has
  no labels). The fault line asserts the prefix only.

- [ ] **Step 2: Run them, see them fail.**

  ```bash
  go test ./internal/cli -run 'TestShow' -v
  ```

  Expected failure:

  ```text
  # github.com/eisenwinter/awit/internal/cli [github.com/eisenwinter/awit/internal/cli.test]
  internal/cli/show_test.go: undefined: copyFixture
  FAIL	github.com/eisenwinter/awit/internal/cli [build failed]
  ```

  (or `undefined: showCmd` once helpers exist — either red counts; do
  not skip it.)

- [ ] **Step 3: Implement `show.go`.**

  Create `internal/cli/show.go`:

  ```go
  package cli

  import (
      "context"
      "encoding/json"
      "fmt"
      "strings"

      "github.com/eisenwinter/awit/pkg/format"
      "github.com/eisenwinter/awit/pkg/graph"
      "github.com/eisenwinter/awit/pkg/item"
      "github.com/urfave/cli/v3"
  )

  var showCmd = &cli.Command{
      Name:      "show",
      Usage:     "Show one item (default view)",
      ArgsUsage: "<id>",
      Action: func(ctx context.Context, cmd *cli.Command) error {
          if cmd.Args().Len() != 1 {
              return cli.Exit("show needs an item id", 2)
          }
          return showOne(cmd, cmd.Args().Get(0))
      },
  }

  // showJSON is the --format json shape: the list entry plus the raw body.
  // AWIT-0ND5703G adds a Refs field; keep the embedding so that still works.
  type showJSON struct {
      format.Entry
      Body string `json:"body"`
  }

  func showOne(cmd *cli.Command, id string) error {
      s, err := openStore(cmd)
      if err != nil {
          return err
      }
      g, err := loadGraph(s)
      if err != nil {
          return err
      }
      f, err := detectFormat(cmd)
      if err != nil {
          return err
      }
      if n, ok := g.Nodes[id]; ok {
          if f == format.JSON {
              out, err := json.MarshalIndent(showJSON{Entry: toEntry(n), Body: string(n.Item.Body())}, "", "  ")
              if err != nil {
                  return err
              }
              fmt.Fprintf(cmd.Writer, "%s\n", out)
              return nil
          }
          fmt.Fprint(cmd.Writer, defaultView(n))
          return nil
      }
      var matches []item.Broken
      for _, br := range g.Broken {
          if br.ID == id {
              matches = append(matches, br)
          }
      }
      if len(matches) > 0 {
          // Broken files always render the text view, even as json.
          fmt.Fprint(cmd.Writer, brokenView(id, matches))
          return nil
      }
      return fmt.Errorf("unknown item %s", id)
  }

  // defaultView renders the core ticket: header, status, faults, deps,
  // assignee, brief, ref count, blank line, verbatim body.
  func defaultView(n *graph.Node) string {
      e := toEntry(n)
      var b strings.Builder
      fmt.Fprintf(&b, "[%s] %s\n", e.ID, e.Title)
      state := e.State
      if n.Quarantined() {
          state = "QUARANTINED"
      }
      labels := strings.Join(e.Labels, ",")
      if labels == "" {
          labels = "-"
      }
      fmt.Fprintf(&b, "status: %s (%s) | labels: %s | unblocks: %d\n", e.Status, state, labels, e.Unblocks)
      for _, f := range n.Faults {
          fmt.Fprintf(&b, "fault: [%s] %s\n", string(f.Reason), f.Detail)
      }
      deps := "-"
      if len(e.Deps) > 0 {
          deps = strings.Join(e.Deps, ", ")
      }
      fmt.Fprintf(&b, "deps: %s\n", deps)
      assignee := e.Assignee
      if assignee == "" {
          assignee = "-"
      }
      fmt.Fprintf(&b, "assignee: %s\n", assignee)
      fmt.Fprintf(&b, "brief: %s\n", e.Brief)
      fmt.Fprintf(&b, "refs: %d (use --full)\n", len(n.Item.Refs))
      b.WriteString("\n")
      body := n.Item.Body()
      b.Write(body)
      if len(body) > 0 && body[len(body)-1] != '\n' {
          b.WriteString("\n")
      }
      return b.String()
  }

  // brokenView renders a file that could not become an item. Exit stays 0:
  // the user asked what is there, and something is there.
  func brokenView(id string, broken []item.Broken) string {
      var b strings.Builder
      fmt.Fprintf(&b, "[%s] (unparseable)\n", id)
      for _, br := range broken {
          fmt.Fprintf(&b, "fault: [%s] %s\n", string(br.Reason), br.Detail)
      }
      return b.String()
  }
  ```

  If `loadGraph`/`toEntry` do not exist yet, add the define-if-absent
  block from AWIT-0ND56R3G's Context to this same file. In
  `internal/cli/app.go`, add `showCmd` to the root `Commands` slice.

  Details that matter:

  - `status: %s (%s)`: status word is the raw status (`open`,
    `in_progress`, `closed`); the parenthesised word is the graph state
    (`ready`, `blocked`, `closed`), except quarantined nodes print
    `(QUARANTINED)` in capitals. A closed item prints
    `status: closed (closed)`.
  - `fault:` lines come after the status line and before `deps:`, one
    per `Node.Faults` entry, already in build order.
  - `deps:` joins with `", "`; `labels:` joins with `","` (no space,
    matching `format.Line`).
  - `refs: %d (use --full)` always prints the count, even 0.
  - The body is written byte-for-byte; the blank line separating it from
    the header block is exactly one `\n` on its own line.
  - JSON embeds `format.Entry`, so `id`, `title`, `brief`, `status`,
    `state`, `labels`, `deps`, `unblocks` (and `faults` when
    quarantined) appear alongside `body`. Indent is two spaces plus one
    trailing newline, matching `format.Write`.

- [ ] **Step 4: Run the tests, see them pass.**

  ```bash
  go test ./internal/cli -run 'TestShow' -v
  ```

  Expected:

  ```text
  === RUN   TestShowClean
  --- PASS: TestShowClean
  === RUN   TestShowQuarantined
  --- PASS: TestShowQuarantined
  === RUN   TestShowBrokenFile
  --- PASS: TestShowBrokenFile
  === RUN   TestShowUnknown
  --- PASS: TestShowUnknown
  === RUN   TestShowJSON
  --- PASS: TestShowJSON
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
  git add internal/cli/show.go internal/cli/show_test.go internal/cli/app.go testdata/golden/show-clean.golden
  git commit -m "cli/show: default view with quarantine flag"
  ```

- [ ] **Step 6: Close this ticket.**

  Set `status: closed` in this file's frontmatter, then:

  ```bash
  git add .awit/items/AWIT-0ND56K3G.md
  git commit -m "tickets: close AWIT-0ND56K3G"
  ```

## Acceptance Criteria

- `go test ./internal/cli -run 'TestShow' -count=1 -v` — all five tests
  PASS; `go test ./internal/cli -count=1` stays green.
- On a copy of the `clean` fixture, `awit show AWIT-TEST0001` prints
  `testdata/golden/show-clean.golden` byte-for-byte.
- On the `cyclic` fixture, `awit show AWIT-TEST0001` prints
  `status: open (QUARANTINED) | labels: - | unblocks: -1` and a
  `fault: [CYCLE] ...` line before `deps: AWIT-TEST0002`.
- On the `parse-error` fixture, `awit show AWIT-TEST0001` prints
  `[AWIT-TEST0001] (unparseable)` plus `fault: [PARSE ERROR] ...` and
  exits 0.
- `awit show AWIT-TEST0099` exits 1 with
  `Error: unknown item AWIT-TEST0099`.
- `awit --format json show AWIT-TEST0001` prints one indented JSON object
  with the entry fields plus `body`, trailing newline.
- `gofmt -l internal/cli` prints nothing.

## Out of scope

- `--full` and `--refs-only` (AWIT-0ND5703G extends `show.go`; do not
  add the flags here).
- Following refs, resolving item refs, missing-ref reporting.
- `awit list`, `awit dep`, `awit validate`, `awit prime`.
- New `pkg/format` code; changing `toEntry`, `detectFormat`, or any
  fixture.
