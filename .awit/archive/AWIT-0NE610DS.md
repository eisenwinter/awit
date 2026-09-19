---
id: AWIT-0NE610DS
title: 'awit archive: move eligible closed items to .awit/archive with collapsed comments'
brief: >-
  Add awit archive [--dry-run]: compute the fixed-point set of closed items with no dependant outside the set, collapse each item's comments into one archive/<id>.md, move --file attachments to archive/<id>/, delete items/<id>.md and comments/<id>/. Never breaks validate.
status: closed
deps: [AWIT-0ND56Q3G, AWIT-0ND56Y3G, AWIT-0ND56S3G]
labels: [phase5, p1]
refs_base: repo
refs:
  - plan/implementation-guide.md
  - plan/awit-implementation-plan.md
assignee: agent/orchestrator
---

## Summary

After this ticket `awit archive` moves finished work out of `items/`
without touching the graph engine:

```text
$ awit archive --dry-run
would archive AWIT-TEST0004
would archive AWIT-TEST0005
skip AWIT-TEST0001: dependant AWIT-TEST0002 not archivable
skip AWIT-TEST0002: dependant AWIT-TEST0003 not archivable
Would archive 2 items
$ awit archive
archived AWIT-TEST0004
archived AWIT-TEST0005
Archived 2 items
```

The archive set is the **fixed point** from guide §2 "Archive
eligibility": closed, non-quarantined nodes with no `Unblocks`
neighbour outside the set. Anything else would leave a `DANGLING DEP`
fault behind, so after every `awit archive` the invariant is
`awit validate` still says whatever it said before (PASS stays PASS).

Per item: `archive/<id>.md` = original bytes with comment refs removed
from `refs` and a `## Comments` section appended (one `### <created>
<author>` block per comment, chronological); `--file` attachments move
to `archive/<id>/` and their ref becomes `../archive/<id>/<file>`;
then `items/<id>.md` and `comments/<id>/` are deleted. No commit. No
`unarchive`. `--format` is ignored (like `close`).

## Context (read first)

- Guide §2 rows "Archive eligibility" through "Does `archive` commit?"
  are normative for layout, collapse format, comment-vs-attachment
  rule, write order and output wording. Read them before Step 1; do
  not invent alternatives.
- Guide §4.4 (`Comment`, `ArchiveDir`, `ArchivePath`, `Comments`,
  `Archive`) and §4.6 (`Archivable`) are the signatures. Additive
  only; nothing existing changes.
- `pkg/graph/graph.go` `Build` lines ~118–137: dangling detection has
  **no status check** — a closed item with a dep that left `items/`
  is quarantined too. That is why the fixed point exists. Do not
  "fix" this in `Build`.
- `pkg/item/comment.go`: `AddComment` writes
  `---\nauthor: X\ncreated: <RFC3339>\n---\n\n<text>\n`; `AttachFile`
  copies bytes verbatim, arbitrary extension. Both push
  `../comments/<id>/<file>` onto `Refs`. `Comments` must round-trip
  exactly what these two write.
- `pkg/item/frontmatter.go` `Split` is how you tell a comment from an
  attachment: `Split` ok **and** YAML mapping has `author` and
  `created` → comment; anything else → attachment. No MIME sniffing,
  no extension checks.
- `Item.Bytes()` renders the (possibly edited) frontmatter + body;
  `SetRefs` edits the node. For the archive file: `SetRefs(filtered)`,
  `Bytes()`, then append the Comments section as raw bytes. Never
  parse the body.
- `internal/cli/close.go` is the shape to copy for the command:
  `openStore` → `s.Lock(5*time.Second)` → work → return error. Output
  via `cmd.Root().Writer` (never `cmd.Writer`, see AWIT-0NE5H7DR).
  Register `archiveCmd` in `app.go` `Commands` after `releaseCmd`.
- `config.WriteAtomic(path, data)` is the only allowed way to write a
  file. Attachments move with `os.Rename`; on error (cross-device is
  the realistic case) fall back to read → `WriteAtomic` → `os.Remove`.
- Helpers (do not redeclare): `openStore`, `loadGraph`, `run`,
  `copyFixture`, `golden`, `readItem`.
- Fixture item IDs are `AWIT-TEST000N`; `config.yaml` is
  `prefix: AWIT` + `stale_claim: 2h` (copy from `clean`).

## Files

- Create: `pkg/graph/archivable.go` (or append to `rank.go`) —
  `Graph.Archivable`.
- Create: `pkg/graph/archivable_test.go`
- Modify: `pkg/item/comment.go` — `Comment` type, `Store.Comments`.
- Create: `pkg/item/archive.go` — `ArchiveDir`, `ArchivePath`,
  `Store.Archive`.
- Create: `pkg/item/archive_test.go`
- Create: `internal/cli/archive.go` — `archiveCmd`.
- Create: `internal/cli/archive_test.go`
- Modify: `internal/cli/app.go` — register `archiveCmd`.
- Create: `testdata/fixtures/archive/.awit/…` (see Step 1).
- Create: `testdata/golden/archive-TEST0004.golden`,
  `testdata/golden/archive-dry-run.golden`.

## Interfaces

- Consumes (do not reimplement):

  ```go
  func (s *Store) LoadAll() ([]*Item, []Broken, error)
  func (s *Store) Lock(timeout time.Duration) (func() error, error)
  func (s *Store) CommentsDir(id string) string
  func (s *Store) ItemPath(id string) string
  func Split(data []byte) (front, body []byte, err error)
  func (it *Item) SetRefs(v []string)
  func (it *Item) Bytes() ([]byte, error)
  func config.WriteAtomic(path string, data []byte) error
  func graph.Build(items []*item.Item, broken []item.Broken) *graph.Graph
  ```

- Produces (this ticket):

  ```go
  // pkg/graph
  func (g *Graph) Archivable() []*Node

  // pkg/item
  type Comment struct {
      File       string
      Author     string
      Created    time.Time
      Text       string
      Attachment bool
  }
  func (s *Store) ArchiveDir() string
  func (s *Store) ArchivePath(id string) string
  func (s *Store) Comments(id string) ([]Comment, error)
  func (s *Store) Archive(it *Item) error

  // internal/cli
  var archiveCmd *cli.Command
  ```

  `Archivable` is pure over the built graph. `Archive` does **not**
  check eligibility — the command does, via `Archivable`. Keep that
  split so `Archive` is testable on a single item.

## Steps

- [ ] **Step 1: Build the `archive` fixture.**

  Create `testdata/fixtures/archive/.awit/config.yaml` (copy from
  `clean`) and these items (bodies: `## Summary` + one line,
  `## Acceptance Criteria` + one bullet, like `clean`'s `TEST0005`):

  | ID | status | deps | refs |
  | --- | --- | --- | --- |
  | `AWIT-TEST0001` | closed | `[]` | `[]` |
  | `AWIT-TEST0002` | closed | `[AWIT-TEST0001]` | `[]` |
  | `AWIT-TEST0003` | open | `[AWIT-TEST0002]` | `[]` |
  | `AWIT-TEST0004` | closed | `[]` | three refs below, block style |
  | `AWIT-TEST0005` | closed | `[AWIT-TEST0004]` | `[]` |

  `TEST0004` refs, in this order:

  ```yaml
  refs:
    - ../comments/AWIT-TEST0004/20260916T090000Z-jan.md
    - ../comments/AWIT-TEST0004/20260916T091500Z-jan.log
    - ../comments/AWIT-TEST0004/20260916T093000Z-claude.md
  ```

  Comment files under
  `testdata/fixtures/archive/.awit/comments/AWIT-TEST0004/`:

  `20260916T090000Z-jan.md`:

  ```markdown
  ---
  author: jan
  created: 2026-09-16T09:00:00Z
  ---

  Started on the spec; grammar section drafted.
  ```

  `20260916T091500Z-jan.log` (no frontmatter — an attachment):

  ```text
  2026-09-16T09:15:00Z PASS TestHeaderGrammar
  2026-09-16T09:15:01Z PASS TestUnauthorizedPayload
  ```

  `20260916T093000Z-claude.md`:

  ```markdown
  ---
  author: agent/claude
  created: 2026-09-16T09:30:00Z
  ---

  Reviewed. Two nits fixed inline:

  - header grammar now cites RFC 6750 §2.1
  - 401 body example uses `error_description`
  ```

  Sanity: `bin/awit --repo testdata/fixtures/archive validate` must
  print `PASS` before you continue. Commit `fixtures: add archive`.

- [ ] **Step 2: Failing test for `Graph.Archivable`.**

  Create `pkg/graph/archivable_test.go`. Build items inline with
  `item.New` + `SetStatus` (no fixture needed at this layer):

  ```go
  package graph

  import (
      "testing"

      "github.com/eisenwinter/awit/pkg/item"
  )

  func mk(id string, st item.Status, deps ...string) *item.Item {
      it := item.New(id, "t "+id, "b", deps, nil)
      it.SetStatus(st)
      return it
  }

  func ids(nodes []*Node) []string {
      out := make([]string, 0, len(nodes))
      for _, n := range nodes {
          out = append(out, n.Item.ID)
      }
      return out
  }

  func TestArchivable(t *testing.T) {
      cases := []struct {
          name  string
          items []*item.Item
          want  []string
      }{
          {"empty", nil, nil},
          {"all open", []*item.Item{mk("AWIT-TEST0001", item.StatusOpen)}, nil},
          {"lone closed", []*item.Item{mk("AWIT-TEST0001", item.StatusClosed)}, []string{"AWIT-TEST0001"}},
          {"closed chain fully archivable", []*item.Item{
              mk("AWIT-TEST0001", item.StatusClosed),
              mk("AWIT-TEST0002", item.StatusClosed, "AWIT-TEST0001"),
          }, []string{"AWIT-TEST0001", "AWIT-TEST0002"}},
          {"open dependant pins the whole chain", []*item.Item{
              mk("AWIT-TEST0001", item.StatusClosed),
              mk("AWIT-TEST0002", item.StatusClosed, "AWIT-TEST0001"),
              mk("AWIT-TEST0003", item.StatusOpen, "AWIT-TEST0002"),
              mk("AWIT-TEST0004", item.StatusClosed),
              mk("AWIT-TEST0005", item.StatusClosed, "AWIT-TEST0004"),
          }, []string{"AWIT-TEST0004", "AWIT-TEST0005"}},
          {"in_progress dependant pins", []*item.Item{
              mk("AWIT-TEST0001", item.StatusClosed),
              mk("AWIT-TEST0002", item.StatusInProgress, "AWIT-TEST0001"),
          }, nil},
          {"quarantined closed node is never archivable and pins its dep", []*item.Item{
              mk("AWIT-TEST0001", item.StatusClosed),
              mk("AWIT-TEST0002", item.StatusClosed, "AWIT-TEST0001", "AWIT-TEST9999"),
          }, nil},
          // A broken file has no deps edge, so it cannot pin anything;
          // Archivable ignores Broken entirely.
          {"broken file does not pin", []*item.Item{
              mk("AWIT-TEST0001", item.StatusClosed),
          }, []string{"AWIT-TEST0001"}},
      }
      for _, tc := range cases {
          t.Run(tc.name, func(t *testing.T) {
              var broken []item.Broken
              if tc.name == "broken file does not pin" {
                  broken = []item.Broken{{ID: "AWIT-TEST0002", Reason: item.ReasonParse}}
              }
              g := Build(tc.items, broken)
              got := ids(g.Archivable())
              if len(got) != len(tc.want) {
                  t.Fatalf("got %v, want %v", got, tc.want)
              }
              for i := range got {
                  if got[i] != tc.want[i] {
                      t.Fatalf("got %v, want %v", got, tc.want)
                  }
              }
          })
      }
  }
  ```

  Run `go test ./pkg/graph -run TestArchivable`; expect
  `undefined: (*Graph).Archivable` build failure.

- [ ] **Step 3: Implement `Archivable`.**

  ```go
  // Archivable returns the fixed-point set of closed, non-quarantined
  // nodes with no Unblocks neighbour outside the set. Sorted ID asc.
  func (g *Graph) Archivable() []*Node {
      in := make(map[*Node]bool, len(g.Order))
      for _, n := range g.Order {
          if n.Item.Status == item.StatusClosed && !n.Quarantined() {
              in[n] = true
          }
      }
      for changed := true; changed; {
          changed = false
          for n := range in {
              for _, u := range n.Unblocks {
                  if !in[u] {
                      delete(in, n)
                      changed = true
                      break
                  }
              }
          }
      }
      out := make([]*Node, 0, len(in))
      for _, n := range g.Order { // g.Order is ID asc → deterministic
          if in[n] {
              out = append(out, n)
          }
      }
      return out
  }
  ```

  Deleting from a map during `range` is legal in Go. Run the test,
  see pass. Commit `graph: add Archivable fixed point`.

- [ ] **Step 4: Failing tests for `Store.Comments` and `Store.Archive`.**

  Create `pkg/item/archive_test.go`. Use `t.TempDir()` + `Init`, then
  write the item and the three comment files from Step 1 by hand with
  `os.WriteFile` (keep the bytes identical to the fixture so the golden
  in Step 7 matches):

  ```go
  func TestCommentsSortedAndClassified(t *testing.T) {
      s := archiveStore(t) // helper: Init + write TEST0004 + 3 comment files
      got, err := s.Comments("AWIT-TEST0004")
      if err != nil {
          t.Fatal(err)
      }
      if len(got) != 3 {
          t.Fatalf("len = %d", len(got))
      }
      if got[0].Author != "jan" || got[0].Attachment || got[0].Text != "Started on the spec; grammar section drafted." {
          t.Fatalf("first = %+v", got[0])
      }
      if !got[1].Attachment || got[1].File != "20260916T091500Z-jan.log" || got[1].Author != "" {
          t.Fatalf("second = %+v", got[1])
      }
      if got[2].Author != "agent/claude" || got[2].Created.Format(time.RFC3339) != "2026-09-16T09:30:00Z" {
          t.Fatalf("third = %+v", got[2])
      }
  }

  func TestCommentsMissingDir(t *testing.T) {
      s := archiveStore(t)
      got, err := s.Comments("AWIT-TEST0001")
      if err != nil || len(got) != 0 {
          t.Fatalf("got %v, %v", got, err)
      }
  }

  func TestArchiveMovesEverything(t *testing.T) {
      s := archiveStore(t)
      it, err := s.Load("AWIT-TEST0004")
      if err != nil {
          t.Fatal(err)
      }
      if err := s.Archive(it); err != nil {
          t.Fatal(err)
      }
      if _, err := os.Stat(s.ItemPath("AWIT-TEST0004")); !errors.Is(err, os.ErrNotExist) {
          t.Fatalf("item still present: %v", err)
      }
      if _, err := os.Stat(s.CommentsDir("AWIT-TEST0004")); !errors.Is(err, os.ErrNotExist) {
          t.Fatalf("comments dir still present: %v", err)
      }
      if _, err := os.Stat(filepath.Join(s.ArchiveDir(), "AWIT-TEST0004", "20260916T091500Z-jan.log")); err != nil {
          t.Fatalf("attachment not moved: %v", err)
      }
      data, err := os.ReadFile(s.ArchivePath("AWIT-TEST0004"))
      if err != nil {
          t.Fatal(err)
      }
      arch, err := Parse(s.ArchivePath("AWIT-TEST0004"), data)
      if err != nil {
          t.Fatalf("archive file must still parse: %v", err)
      }
      wantRefs := []string{"../archive/AWIT-TEST0004/20260916T091500Z-jan.log"}
      if !slices.Equal(arch.Refs, wantRefs) {
          t.Fatalf("refs = %v, want %v", arch.Refs, wantRefs)
      }
      if !bytes.Contains(data, []byte("\n## Comments\n\n### 2026-09-16T09:00:00Z jan\n\nStarted on the spec; grammar section drafted.\n\n### 2026-09-16T09:30:00Z agent/claude\n\nReviewed.")) {
          t.Fatalf("comments not collapsed:\n%s", data)
      }
  }

  func TestArchiveNoCommentsNoSection(t *testing.T) {
      s := archiveStore(t)
      it := New("AWIT-TEST0001", "lone", "b", nil, nil)
      it.SetStatus(StatusClosed)
      if err := s.Save(it); err != nil {
          t.Fatal(err)
      }
      before, _ := os.ReadFile(s.ItemPath("AWIT-TEST0001"))
      if err := s.Archive(it); err != nil {
          t.Fatal(err)
      }
      after, _ := os.ReadFile(s.ArchivePath("AWIT-TEST0001"))
      if !bytes.Equal(before, after) {
          t.Fatalf("no-comment archive must be byte-identical\nbefore:\n%s\nafter:\n%s", before, after)
      }
  }

  func TestArchiveIdempotentAfterCrash(t *testing.T) {
      s := archiveStore(t)
      it, _ := s.Load("AWIT-TEST0004")
      // Simulate a crash after the archive file was written but before deletes.
      if err := os.MkdirAll(s.ArchiveDir(), 0o755); err != nil {
          t.Fatal(err)
      }
      if err := os.WriteFile(s.ArchivePath("AWIT-TEST0004"), []byte("stale"), 0o644); err != nil {
          t.Fatal(err)
      }
      if err := s.Archive(it); err != nil {
          t.Fatal(err)
      }
      data, _ := os.ReadFile(s.ArchivePath("AWIT-TEST0004"))
      if bytes.Equal(data, []byte("stale")) {
          t.Fatal("archive file was not rebuilt")
      }
  }
  ```

  Run `go test ./pkg/item -run 'TestComments|TestArchive'`; expect
  `undefined: Comment` / `s.Comments` / `s.Archive` build failures.

- [ ] **Step 5: Implement `Comment`, `Comments`, `ArchiveDir`, `ArchivePath`, `Archive`.**

  In `pkg/item/comment.go` add the `Comment` type (§4.4) and:

  ```go
  // Comments lists comments/<id>/ sorted by filename. Missing directory
  // → nil, nil. A file is a comment when Split succeeds and the
  // frontmatter carries author and created; everything else is an
  // attachment.
  func (s *Store) Comments(id string) ([]Comment, error) {
      entries, err := os.ReadDir(s.CommentsDir(id))
      if errors.Is(err, os.ErrNotExist) {
          return nil, nil
      }
      if err != nil {
          return nil, err
      }
      out := make([]Comment, 0, len(entries))
      for _, e := range entries { // ReadDir is sorted by filename
          if e.IsDir() {
              continue
          }
          data, err := os.ReadFile(filepath.Join(s.CommentsDir(id), e.Name()))
          if err != nil {
              return nil, err
          }
          out = append(out, parseComment(e.Name(), data))
      }
      return out, nil
  }

  func parseComment(name string, data []byte) Comment {
      c := Comment{File: name, Attachment: true}
      front, body, err := Split(data)
      if err != nil {
          return c
      }
      var fm struct {
          Author  string `yaml:"author"`
          Created string `yaml:"created"`
      }
      if yaml.Unmarshal(front, &fm) != nil || fm.Author == "" || fm.Created == "" {
          return c
      }
      created, err := time.Parse(time.RFC3339, fm.Created)
      if err != nil {
          return c
      }
      return Comment{File: name, Author: fm.Author, Created: created.UTC(), Text: strings.TrimSpace(string(body))}
  }
  ```

  Create `pkg/item/archive.go`:

  ```go
  func (s *Store) ArchiveDir() string            { return filepath.Join(s.Dir, "archive") }
  func (s *Store) ArchivePath(id string) string  { return filepath.Join(s.ArchiveDir(), id+".md") }

  // Archive collapses it and its comments into ArchivePath(it.ID),
  // moves attachments to ArchiveDir()/<id>/, then removes the item file
  // and its comments directory. Eligibility is the caller's job.
  func (s *Store) Archive(it *Item) error {
      comments, err := s.Comments(it.ID)
      if err != nil {
          return err
      }
      // 1. rewrite refs: drop comment refs, redirect attachment refs.
      prefix := path.Join("../comments", it.ID) + "/"
      byFile := make(map[string]Comment, len(comments))
      for _, c := range comments {
          byFile[c.File] = c
      }
      refs := make([]string, 0, len(it.Refs))
      for _, r := range it.Refs {
          if !strings.HasPrefix(r, prefix) {
              refs = append(refs, r)
              continue
          }
          if c, ok := byFile[strings.TrimPrefix(r, prefix)]; ok && c.Attachment {
              refs = append(refs, path.Join("../archive", it.ID, c.File))
          }
          // comment refs and refs to missing files are dropped
      }
      it.SetRefs(refs)
      data, err := it.Bytes()
      if err != nil {
          return err
      }
      // 2. append the collapsed comments.
      var b bytes.Buffer
      b.Write(data)
      first := true
      for _, c := range comments {
          if c.Attachment {
              continue
          }
          if first {
              b.WriteString("\n## Comments\n")
              first = false
          }
          fmt.Fprintf(&b, "\n### %s %s\n\n%s\n", c.Created.Format(time.RFC3339), c.Author, c.Text)
      }
      // 3. write archive file, move attachments, delete originals.
      if err := os.MkdirAll(s.ArchiveDir(), 0o755); err != nil {
          return err
      }
      if err := config.WriteAtomic(s.ArchivePath(it.ID), b.Bytes()); err != nil {
          return err
      }
      for _, c := range comments {
          if !c.Attachment {
              continue
          }
          if err := moveFile(filepath.Join(s.CommentsDir(it.ID), c.File), filepath.Join(s.ArchiveDir(), it.ID, c.File)); err != nil {
              return err
          }
      }
      if err := os.Remove(s.ItemPath(it.ID)); err != nil && !errors.Is(err, os.ErrNotExist) {
          return err
      }
      return os.RemoveAll(s.CommentsDir(it.ID))
  }

  // moveFile renames src to dst, creating dst's directory; on rename
  // failure (cross-device) it copies atomically and removes src.
  func moveFile(src, dst string) error {
      if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
          return err
      }
      if err := os.Rename(src, dst); err == nil {
          return nil
      }
      data, err := os.ReadFile(src)
      if err != nil {
          return err
      }
      if err := config.WriteAtomic(dst, data); err != nil {
          return err
      }
      return os.Remove(src)
  }
  ```

  Body ends with `\n` already (item template and `Bytes()` guarantee
  it), so `"\n## Comments\n"` yields exactly one blank line before the
  heading. If `Bytes()` output does not end in `\n` for some fixture,
  add one before the section — the golden in Step 7 pins the result.
  Run the Step 4 tests, see pass. Commit
  `item: add Comments and Archive`.

- [ ] **Step 6: Failing CLI tests.**

  Create `internal/cli/archive_test.go`:

  ```go
  package cli

  import (
      "errors"
      "os"
      "path/filepath"
      "strings"
      "testing"
  )

  func TestArchiveDryRun(t *testing.T) {
      dir := copyFixture(t, "archive")
      code, stdout, stderr := run(t, "--repo", dir, "archive", "--dry-run")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
      golden(t, "archive-dry-run.golden", []byte(stdout))
      if _, err := os.Stat(filepath.Join(dir, ".awit", "archive")); !errors.Is(err, os.ErrNotExist) {
          t.Fatal("dry-run must not create archive/")
      }
  }

  func TestArchiveMovesAndValidateStaysGreen(t *testing.T) {
      dir := copyFixture(t, "archive")
      code, stdout, stderr := run(t, "--repo", dir, "archive")
      if code != 0 || stderr != "" {
          t.Fatalf("exit %d stderr %q", code, stderr)
      }
      want := "archived AWIT-TEST0004\narchived AWIT-TEST0005\nArchived 2 items\n"
      if stdout != want {
          t.Fatalf("stdout = %q, want %q", stdout, want)
      }
      for _, id := range []string{"AWIT-TEST0004", "AWIT-TEST0005"} {
          if _, err := os.Stat(filepath.Join(dir, ".awit", "items", id+".md")); !errors.Is(err, os.ErrNotExist) {
              t.Fatalf("%s still in items/", id)
          }
          if _, err := os.Stat(filepath.Join(dir, ".awit", "archive", id+".md")); err != nil {
              t.Fatalf("%s missing from archive/: %v", id, err)
          }
      }
      for _, id := range []string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0003"} {
          if _, err := os.Stat(filepath.Join(dir, ".awit", "items", id+".md")); err != nil {
              t.Fatalf("%s must stay: %v", id, err)
          }
      }
      data, err := os.ReadFile(filepath.Join(dir, ".awit", "archive", "AWIT-TEST0004.md"))
      if err != nil {
          t.Fatal(err)
      }
      golden(t, "archive-TEST0004.golden", data)

      code, out, _ := run(t, "--repo", dir, "validate")
      if code != 0 || !strings.HasPrefix(out, "PASS") {
          t.Fatalf("validate after archive: exit %d\n%s", code, out)
      }
      // Second run: nothing left to do, still exit 0.
      code, out, _ = run(t, "--repo", dir, "archive")
      if code != 0 || out != "Archived 0 items\n" {
          t.Fatalf("second run: exit %d out %q", code, out)
      }
  }

  func TestArchiveNothingOnClean(t *testing.T) {
      // clean: TEST0005 is closed but TEST0006 (in_progress) depends on it.
      dir := copyFixture(t, "clean")
      code, stdout, _ := run(t, "--repo", dir, "archive", "--dry-run")
      if code != 0 {
          t.Fatalf("exit %d", code)
      }
      want := "skip AWIT-TEST0005: dependant AWIT-TEST0006 not archivable\nWould archive 0 items\n"
      if stdout != want {
          t.Fatalf("stdout = %q, want %q", stdout, want)
      }
  }
  ```

  Create both golden files empty. Run
  `go test ./internal/cli -run TestArchive`; expect exit 2 /
  `No help topic for 'archive'` style failures.

- [ ] **Step 7: Implement the command and register it.**

  `internal/cli/archive.go`:

  ```go
  package cli

  import (
      "context"
      "fmt"
      "time"

      "github.com/eisenwinter/awit/pkg/graph"
      "github.com/eisenwinter/awit/pkg/item"
      "github.com/urfave/cli/v3"
  )

  var archiveCmd = &cli.Command{
      Name:  "archive",
      Usage: "Move closed items nothing depends on to .awit/archive, one collapsed file each",
      Flags: []cli.Flag{
          &cli.BoolFlag{Name: "dry-run", Usage: "print what would move; write nothing"},
      },
      Action: archiveAction,
  }

  func archiveAction(_ context.Context, cmd *cli.Command) error {
      s, err := openStore(cmd)
      if err != nil {
          return err
      }
      release, err := s.Lock(5 * time.Second)
      if err != nil {
          return err
      }
      defer release()
      g, err := loadGraph(s)
      if err != nil {
          return err
      }
      w := cmd.Root().Writer
      set := g.Archivable()
      if cmd.Bool("dry-run") {
          in := make(map[*graph.Node]bool, len(set))
          for _, n := range set {
              in[n] = true
              fmt.Fprintf(w, "would archive %s\n", n.Item.ID)
          }
          for _, n := range g.Closed() {
              if in[n] || n.Quarantined() {
                  continue
              }
              fmt.Fprintf(w, "skip %s: dependant %s not archivable\n", n.Item.ID, pinnedBy(n, in))
          }
          fmt.Fprintf(w, "Would archive %d items\n", len(set))
          return nil
      }
      for _, n := range set {
          if err := s.Archive(n.Item); err != nil {
              return fmt.Errorf("archive %s: %w", n.Item.ID, err)
          }
          fmt.Fprintf(w, "archived %s\n", n.Item.ID)
      }
      fmt.Fprintf(w, "Archived %d items\n", len(set))
      return nil
  }

  // pinnedBy returns the ID of the smallest Unblocks neighbour outside
  // the archive set (Unblocks is sorted by ID in Build).
  func pinnedBy(n *graph.Node, in map[*graph.Node]bool) string {
      for _, u := range n.Unblocks {
          if !in[u] {
              return u.Item.ID
          }
      }
      return "?" // unreachable: a closed non-quarantined node outside the set has such a neighbour
  }
  ```

  Check `g.Closed()` excludes quarantined nodes (§4.6 says "sorted ID
  asc" only); the `n.Quarantined()` guard above is harmless either
  way. Register `archiveCmd` after `releaseCmd` in `app.go`. Run
  `go test ./internal/cli -run TestArchive -update` once, **read both
  goldens** (`archive-TEST0004.golden` must show the rewritten `refs:`
  with a single `../archive/AWIT-TEST0004/20260916T091500Z-jan.log`
  entry and the two `###` blocks in order; `archive-dry-run.golden`
  must match the Summary), then run without `-update` and see pass.
  Commit `cli/archive: add archive command`.

- [ ] **Step 8: Full package check and self-dogfood.**

  ```bash
  gofmt -l pkg/graph pkg/item internal/cli
  go build ./... && go vet ./pkg/graph ./pkg/item ./internal/cli && go test ./pkg/graph ./pkg/item ./internal/cli
  go build -o bin/awit ./cmd/awit && bin/awit archive --dry-run
  ```

  The last line runs against this repo's own queue and must exit 0.
  Do **not** run it without `--dry-run` inside this ticket — archiving
  the project's tickets is the orchestrator's call.

## Acceptance Criteria

- [ ] `go test ./pkg/graph -run TestArchivable` → `ok`
- [ ] `go test ./pkg/item -run 'TestComments|TestArchive'` → `ok`
- [ ] `go test ./internal/cli -run TestArchive` → `ok` with committed
  goldens `archive-dry-run.golden` and `archive-TEST0004.golden`
- [ ] `bin/awit --repo testdata/fixtures/archive archive --dry-run` prints exactly the `--dry-run` block from the Summary
- [ ] On a copy of the `archive` fixture: `awit archive` prints
  `archived AWIT-TEST0004`, `archived AWIT-TEST0005`, `Archived 2
  items`; afterwards `items/` holds 0001–0003 only, `archive/` holds
  `AWIT-TEST0004.md`, `AWIT-TEST0005.md`,
  `AWIT-TEST0004/20260916T091500Z-jan.log`, `comments/AWIT-TEST0004`
  is gone, and `awit validate` prints `PASS`
- [ ] `awit archive` on a repo with nothing eligible prints
  `Archived 0 items` and exits 0
- [ ] `go vet ./... && staticcheck ./...` clean on touched packages

## Out of scope

- `unarchive` (use `git revert`).
- `show`/`list`/`prime` reading from `archive/`.
- Auto-archiving from `close` or `next`.
- Any archive index file or graph-side notion of "external closed".
- Rewriting other items' `deps`.
- Git commit of the archive move.

## Comments

### 2026-09-18T06:39:25Z agent/orchestrator

implemented
