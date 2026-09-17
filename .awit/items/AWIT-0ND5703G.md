---
id: AWIT-0ND5703G
title: 'awit show --full / --refs-only'
brief: >-
  Extend `awit show` with `--refs-only` (one `<ref> -> <abs> (<N> bytes)`
  line per ref) and `--full` (default view plus delimited ref bodies).
  Item refs render the target's default view without recursion; missing
  files render `[missing]` and never fail. JSON `--full` adds a refs
  array to the entry.
status: open
deps: [AWIT-0ND56Z3G, AWIT-0ND56K3G]
labels: [phase4, p1]
refs:
  - ../../plan/implementation-guide.md
  - ../../plan/awit-implementation-plan.md
---

## Summary

After this ticket `internal/cli/show.go` grows two flags: `--refs-only`
prints one line per ref (`<ref> -> <abs> (<N> bytes)`, or
`<ref> -> [missing]`), and `--full` prints the default view followed by
one `===== REF i/n: <ref> =====` … `===== END REF i/n =====` block per
ref. Refs resolve via `pkg/resolver` against the items directory. A ref
that names another item renders that item's default view inline and its
own refs are NOT followed (no recursion). Missing or unreadable refs
render `[missing]` inside their block and never change the exit code.
With `--format json`, `--full` emits the entry plus `body` plus a
`refs` array (`ref`, `path`, `bytes`, `missing`, plus `item` for item
refs and `content` for hits). Five tests run green on the `loop`
fixture.

## Context (read first)

- **AWIT-0ND56K3G** — `show.go` as built there: `showCmd`,
  `defaultView(n)`, `brokenView`, `showJSON{Entry, Body}`, unknown-ID
  and broken-file handling. Must be `status: closed` before you start.
  Reuse `defaultView` for item refs; extend `showJSON`, do not replace
  it.
- **AWIT-0ND56Z3G** — `pkg/resolver`, closed. Exact API to consume:

  ```go
  type Resolved struct {
      Ref     string // as written in frontmatter
      Path    string // absolute, OS separators
      Content []byte // nil when Err != nil
      Err     error  // os.ErrNotExist etc.
  }
  func Resolve(itemsDir string, refs []string) []Resolved
  func IsItemRef(itemsDir, path string) (id string, ok bool)
  ```

  `Resolve` never returns an error itself; per-ref failures live in
  `Resolved.Err`. `IsItemRef` reports whether the resolved absolute
  `path` names a `.md` file directly inside `itemsDir`, following
  `..` elements (so `AWIT-TEST0002.md` qualifies; `../../docs/spec.md`
  and `../comments/<id>/x.md` do not). Existence is the caller's
  concern — check `Err == nil` first, then `IsItemRef`.
- Guide §1 — refs print with forward slashes exactly as written in
  frontmatter (`Resolved.Ref`); only `Resolved.Path` carries OS
  separators.
- The `loop` fixture (AWIT-0ND56N3G): `AWIT-TEST0001`
  (`Implement OAuth2 bearer token extraction`) has
  `refs: [../../docs/spec.md]` pointing at `<root>/docs/spec.md`, whose
  two paragraphs start with `The Authorization header is Bearer` and
  `invalid_token`. `AWIT-TEST0002` deps `[AWIT-TEST0001]`, labels
  `[auth]`; `AWIT-TEST0003` deps `[AWIT-TEST0002]`, labels `[p0]`.
- Helpers (do not redeclare): `openStore`, `loadGraph`, `toEntry`,
  `detectFormat`, `run`, `copyFixture`, `readItem`. `Store.ItemsDir()`
  gives the items directory for `Resolve`/`IsItemRef` (from
  AWIT-0ND56E3G; if the accessor is named differently there, use the
  real name — the path is `<root>/.awit/items`).
- The copied fixture root IS the repo root (the directory containing
  `.awit/`), which is what `item.Open(repo)` takes. Tests that mutate
  refs open the store with `item.Open(dir)` and `Save` — no new
  production APIs for tests.

## Files

- Modify: `internal/cli/show.go` — add `--full` / `--refs-only` flags,
  ref rendering, JSON refs array.
- Create: `internal/cli/show_full_test.go`
- Modify: `internal/cli/show_test.go` — only if a shared helper needs
  extracting (prefer a private helper in `show.go`); do not rewrite the
  K3G tests.

## Interfaces

- Consumes (do not reimplement):

  ```go
  func Resolve(itemsDir string, refs []string) []Resolved
  func IsItemRef(itemsDir, path string) (id string, ok bool)
  func defaultView(n *graph.Node) string
  func detectFormat(cmd *cli.Command) (format.Format, error)
  ```

- Produces (this ticket, in `internal/cli/show.go`):

  ```go
  type showRefJSON struct {
      Ref     string `json:"ref"`
      Path    string `json:"path"`
      Bytes   int    `json:"bytes"`
      Missing bool   `json:"missing"`
      Item    string `json:"item,omitempty"`
      Content string `json:"content,omitempty"`
  }
  // showJSON gains: Refs []showRefJSON `json:"refs,omitempty"`

  func refsOnlyView(itemsDir string, refs []string) string
  func fullView(g *graph.Graph, n *graph.Node, itemsDir string) string
  func refBody(g *graph.Graph, itemsDir string, r resolver.Resolved) string
  func fullJSON(g *graph.Graph, n *graph.Node, itemsDir string, full bool) showJSON
  ```

## Steps

- [ ] **Step 1: Write the failing tests.**

  Create `internal/cli/show_full_test.go` (imports `encoding/json`,
  `fmt`, `os`, `path/filepath`, `strings`, `testing`, plus
  `github.com/eisenwinter/awit/pkg/item`):

  ```go
  package cli

  // saveItem opens the copied fixture root as a store and writes it back.
  func saveItem(t *testing.T, repo string, it *item.Item) {
      t.Helper()
      st, err := item.Open(repo)
      if err != nil {
          t.Fatal(err)
      }
      if err := st.Save(it); err != nil {
          t.Fatal(err)
      }
  }

  func TestShowRefsOnly(t *testing.T) {
      dir := copyFixture(t, "loop")
      code, stdout, stderr := run(t, "--repo", dir, "show", "--refs-only", "AWIT-TEST0001")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
      st, err := os.Stat(filepath.Join(dir, "docs", "spec.md"))
      if err != nil {
          t.Fatal(err)
      }
      abs, err := filepath.Abs(filepath.Join(dir, "docs", "spec.md"))
      if err != nil {
          t.Fatal(err)
      }
      want := fmt.Sprintf("../../docs/spec.md -> %s (%d bytes)\n", abs, st.Size())
      if stdout != want {
          t.Fatalf("stdout = %q, want %q", stdout, want)
      }
  }

  func TestShowFullIncludesSpec(t *testing.T) {
      dir := copyFixture(t, "loop")
      code, stdout, stderr := run(t, "--repo", dir, "show", "--full", "AWIT-TEST0001")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
      if !strings.HasPrefix(stdout, "[AWIT-TEST0001] Implement OAuth2 bearer token extraction\n") {
          t.Fatalf("must start with the default view:\n%s", stdout)
      }
      if !strings.Contains(stdout, "===== REF 1/1: ../../docs/spec.md =====\n") {
          t.Fatalf("ref header missing:\n%s", stdout)
      }
      if !strings.HasSuffix(stdout, "===== END REF 1/1 =====\n") {
          t.Fatalf("must end with the ref end marker:\n%s", stdout)
      }
      for _, para := range []string{
          "The Authorization header is Bearer followed by a single token.",
          "invalid_token. The gateway never returns 500 for a bad header.",
      } {
          if !strings.Contains(stdout, para) {
              t.Fatalf("spec paragraph missing: %q\n%s", para, stdout)
          }
      }
  }

  func TestShowFullMissingRef(t *testing.T) {
      dir := copyFixture(t, "loop")
      it := readItem(t, dir, "AWIT-TEST0002")
      it.SetRefs([]string{"nope.md"})
      saveItem(t, dir, it)
      code, stdout, stderr := run(t, "--repo", dir, "show", "--full", "AWIT-TEST0002")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q, missing refs must not fail", code, stderr)
      }
      if !strings.Contains(stdout, "===== REF 1/1: nope.md =====\n[missing]\n===== END REF 1/1 =====\n") {
          t.Fatalf("missing block wrong:\n%s", stdout)
      }
  }

  func TestShowFullItemRefNoRecursion(t *testing.T) {
      dir := copyFixture(t, "loop")
      // 0003 refs 0002 (an item); 0002 refs nothing. A recursive
      // renderer would also expand 0001's spec ref — assert it does not.
      it := readItem(t, dir, "AWIT-TEST0003")
      it.SetRefs([]string{"AWIT-TEST0002.md"})
      saveItem(t, dir, it)
      code, stdout, stderr := run(t, "--repo", dir, "show", "--full", "AWIT-TEST0003")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
      if n := strings.Count(stdout, "===== REF "); n != 2 {
          // One header + one END marker for the single top-level ref.
          t.Fatalf("want exactly one ref block, got %d markers:\n%s", n, stdout)
      }
      if !strings.Contains(stdout, "===== REF 1/1: AWIT-TEST0002.md =====\n[AWIT-TEST0002] Add E2E auth tests\n") {
          t.Fatalf("item ref must render the target default view:\n%s", stdout)
      }
      if strings.Contains(stdout, "../../docs/spec.md") {
          t.Fatal("nested refs of the item ref must NOT be followed")
      }
      if !strings.Contains(stdout, "deps: AWIT-TEST0002") {
          t.Fatalf("target view must show its own deps line:\n%s", stdout)
      }
  }

  func TestShowFullJSON(t *testing.T) {
      dir := copyFixture(t, "loop")
      code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "show", "--full", "AWIT-TEST0001")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
      var got struct {
          ID    string `json:"id"`
          State string `json:"state"`
          Body  string `json:"body"`
          Refs  []struct {
              Ref     string `json:"ref"`
              Path    string `json:"path"`
              Bytes   int    `json:"bytes"`
              Missing bool   `json:"missing"`
              Content string `json:"content"`
          } `json:"refs"`
      }
      if err := json.Unmarshal([]byte(stdout), &got); err != nil {
          t.Fatalf("unmarshal: %v\n%s", err, stdout)
      }
      if got.ID != "AWIT-TEST0001" || got.State != "ready" {
          t.Fatalf("entry = %+v", got)
      }
      if !strings.Contains(got.Body, "## Summary") {
          t.Fatalf("body = %q", got.Body)
      }
      if len(got.Refs) != 1 {
          t.Fatalf("refs = %+v, want 1", got.Refs)
      }
      r := got.Refs[0]
      if r.Ref != "../../docs/spec.md" || r.Missing || r.Bytes <= 0 {
          t.Fatalf("ref = %+v", r)
      }
      if !strings.Contains(r.Content, "The Authorization header is Bearer") {
          t.Fatalf("ref content = %q", r.Content)
      }
  }

  func TestShowFullAndRefsOnlyConflict(t *testing.T) {
      dir := copyFixture(t, "loop")
      code, _, stderr := run(t, "--repo", dir, "show", "--full", "--refs-only", "AWIT-TEST0001")
      if code != 2 || stderr != "Error: pass either --full or --refs-only\n" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
  }
  ```

- [ ] **Step 2: Run them, see them fail.**

  ```bash
  go test ./internal/cli -run 'TestShowRefsOnly|TestShowFull' -v
  ```

  Expected failure:

  ```text
  === RUN   TestShowRefsOnly
      show_full_test.go: flag provided but not defined: -refs-only
  --- FAIL: TestShowRefsOnly
  ```

  (urfave reports unknown flags as an error through `Main`; the exact
  wording may differ — any non-zero exit naming the missing flag is the
  red. Do not skip it.)

- [ ] **Step 3: Extend `show.go`.**

  Add the flags to `showCmd`:

  ```go
  Flags: []cli.Flag{
      &cli.BoolFlag{Name: "full", Usage: "include resolved ref bodies"},
      &cli.BoolFlag{Name: "refs-only", Usage: "list resolved refs without the item"},
  },
  ```

  Branch in `showOne` after the node lookup (both flags only apply to
  real nodes; broken/unknown handling is unchanged):

  ```go
  itemsDir := s.ItemsDir()
  refsOnly := cmd.Bool("refs-only")
  full := cmd.Bool("full")
  if refsOnly && full {
      return cli.Exit("pass either --full or --refs-only", 2)
  }
  if f == format.JSON {
      out, err := json.MarshalIndent(fullJSON(g, n, itemsDir, full), "", "  ")
      if err != nil {
          return err
      }
      fmt.Fprintf(cmd.Writer, "%s\n", out)
      return nil
  }
  if refsOnly {
      fmt.Fprint(cmd.Writer, refsOnlyView(itemsDir, n.Item.Refs))
      return nil
  }
  if full {
      fmt.Fprint(cmd.Writer, fullView(g, n, itemsDir))
      return nil
  }
  ```

  Add `Refs []showRefJSON \`json:"refs,omitempty"\`` to `showJSON`.
  (`omitempty` on a nil slice keeps the K3G JSON byte-identical when
  `--full` is off.) New code:

  ```go
  // showRefJSON is one resolved ref for --format json --full.
  type showRefJSON struct {
      Ref     string `json:"ref"`
      Path    string `json:"path"`
      Bytes   int    `json:"bytes"`
      Missing bool   `json:"missing"`
      Item    string `json:"item,omitempty"`
      Content string `json:"content,omitempty"`
  }

  // refsOnlyView prints one "<ref> -> <abs> (<N> bytes)" line per ref,
  // or "<ref> -> [missing]" when the file cannot be read.
  func refsOnlyView(itemsDir string, refs []string) string {
      var b strings.Builder
      for _, r := range resolver.Resolve(itemsDir, refs) {
          if r.Err != nil {
              fmt.Fprintf(&b, "%s -> [missing]\n", r.Ref)
              continue
          }
          fmt.Fprintf(&b, "%s -> %s (%d bytes)\n", r.Ref, r.Path, len(r.Content))
      }
      return b.String()
  }

  // fullView is the default view plus one delimited block per ref. Item
  // refs render the target's default view; their refs are not followed.
  func fullView(g *graph.Graph, n *graph.Node, itemsDir string) string {
      var b strings.Builder
      b.WriteString(defaultView(n))
      resolved := resolver.Resolve(itemsDir, n.Item.Refs)
      for i, r := range resolved {
          fmt.Fprintf(&b, "===== REF %d/%d: %s =====\n", i+1, len(resolved), r.Ref)
          b.WriteString(refBody(g, itemsDir, r))
          fmt.Fprintf(&b, "===== END REF %d/%d =====\n", i+1, len(resolved))
      }
      return b.String()
  }

  // refBody renders one ref's content, always ending in "\n".
  func refBody(g *graph.Graph, itemsDir string, r resolver.Resolved) string {
      if r.Err != nil {
          return "[missing]\n"
      }
      if id, ok := resolver.IsItemRef(itemsDir, r.Path); ok {
          if target, ok := g.Nodes[id]; ok {
              return defaultView(target)
          }
          // Item file exists on disk but did not parse: show the same
          // fault block `show <id>` would, without failing.
          var matches []item.Broken
          for _, br := range g.Broken {
              if br.ID == id {
                  matches = append(matches, br)
              }
          }
          if len(matches) > 0 {
              return brokenView(id, matches)
          }
      }
      if len(r.Content) == 0 {
          return "\n"
      }
      s := string(r.Content)
      if !strings.HasSuffix(s, "\n") {
          s += "\n"
      }
      return s
  }

  func fullJSON(g *graph.Graph, n *graph.Node, itemsDir string, full bool) showJSON {
      out := showJSON{Entry: toEntry(n), Body: string(n.Item.Body())}
      if !full {
          return out
      }
      for _, r := range resolver.Resolve(itemsDir, n.Item.Refs) {
          jr := showRefJSON{Ref: r.Ref, Path: r.Path}
          if r.Err != nil {
              jr.Missing = true
          } else {
              jr.Bytes = len(r.Content)
              if id, ok := resolver.IsItemRef(itemsDir, r.Path); ok {
                  if target, ok := g.Nodes[id]; ok {
                      jr.Item = id
                      jr.Content = string(target.Item.Body())
                  } else {
                      jr.Content = string(r.Content)
                  }
              } else {
                  jr.Content = string(r.Content)
              }
          }
          out.Refs = append(out.Refs, jr)
      }
      return out
  }
  ```

  Details that matter:

  - `defaultView` ends with the body plus exactly one `\n`, so the
    first `===== REF` header starts on a fresh line with no blank line
    between the body and the header.
  - Block content always ends in `\n`, so every `===== END REF`
    marker is on its own line. An empty (0-byte) file contributes one
    blank line inside its block.
  - `[missing]` has no reason suffix — `Resolve` errors are usually
    `ENOENT`; printing the raw `*PathError` would leak OS-specific
    wording into assertions and break Windows CI.
  - Item-ref match is by stem ID against live graph nodes first
    (`g.Nodes[id]`), so an item ref always shows current state, not a
    stale disk copy. `IsItemRef` takes `(itemsDir, r.Path)` — the
    ABSOLUTE resolved path, not the raw ref.
  - `--refs-only` on an item with no refs prints nothing and exits 0.
  - `--full` on an item with no refs prints exactly `defaultView`.
  - `fullJSON`: entry fields and `body` identical to K3G; `refs[i].path`
    is the absolute path; `bytes` is `len(content)`; item refs set
    `item` to the target ID and `content` to the target's raw body;
    file refs set `content` to the file bytes; missing refs set
    `missing: true`, `bytes: 0`, no `content`.

- [ ] **Step 4: Run the tests, see them pass.**

  ```bash
  go test ./internal/cli -run 'TestShowRefsOnly|TestShowFull' -v
  ```

  Expected:

  ```text
  === RUN   TestShowRefsOnly
  --- PASS: TestShowRefsOnly
  === RUN   TestShowFullIncludesSpec
  --- PASS: TestShowFullIncludesSpec
  === RUN   TestShowFullMissingRef
  --- PASS: TestShowFullMissingRef
  === RUN   TestShowFullItemRefNoRecursion
  --- PASS: TestShowFullItemRefNoRecursion
  === RUN   TestShowFullJSON
  --- PASS: TestShowFullJSON
  === RUN   TestShowFullAndRefsOnlyConflict
  --- PASS: TestShowFullAndRefsOnlyConflict
  PASS
  ok  	github.com/eisenwinter/awit/internal/cli
  ```

  Then the whole package stays green (K3G JSON must be byte-identical —
  `refs,omitempty` with nil slice guarantees it):

  ```bash
  go test ./internal/cli -count=1
  ```

- [ ] **Step 5: Commit.**

  ```bash
  gofmt -l internal/cli
  go vet ./internal/cli
  git add internal/cli/show.go internal/cli/show_full_test.go internal/cli/show_test.go
  git commit -m "cli/show: full view and refs-only with item refs"
  ```

- [ ] **Step 6: Close this ticket.**

  Set `status: closed` in this file's frontmatter, then:

  ```bash
  git add .awit/items/AWIT-0ND5703G.md
  git commit -m "tickets: close AWIT-0ND5703G"
  ```

## Acceptance Criteria

- `go test ./internal/cli -run 'TestShowRefsOnly|TestShowFull' -count=1 -v`
  — all six tests PASS; `go test ./internal/cli -count=1` stays green.
- On a copy of the `loop` fixture,
  `awit show --refs-only AWIT-TEST0001` prints exactly
  `../../docs/spec.md -> <abs> (<N> bytes)` with the real size.
- `awit show --full AWIT-TEST0001` starts with the K3G default view and
  ends with `===== END REF 1/1 =====`, embedding both spec paragraphs.
- A missing ref renders `[missing]` inside its block (text) /
  `"missing": true` (JSON) and the exit stays 0.
- An item ref renders the target's default view inline; the target's
  own refs produce no nested `REF` blocks.
- `awit --format json show --full AWIT-TEST0001` emits entry + `body` +
  a one-element `refs` array with `ref`, absolute `path`, `bytes`, and
  spec `content`.
- `awit show --full --refs-only <id>` exits 2 with
  `Error: pass either --full or --refs-only`.
- `gofmt -l internal/cli` prints nothing.

## Out of scope

- Changing the K3G default view bytes (golden stays green untouched).
- Recursive ref expansion (refs of refs are never followed).
- New `pkg/resolver` code; changing `Resolve`/`IsItemRef` signatures.
- `awit prime`, `awit next`, the E2E loop (AWIT-0ND5713G consumes this).
