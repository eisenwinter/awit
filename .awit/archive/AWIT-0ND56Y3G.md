---
id: AWIT-0ND56Y3G
title: awit comment (inline, --file, author resolution)
brief: >-
  Add `awit comment <id> [text...]` with `--file` (attach verbatim) and `--author`, stdin fallback when no text is given, and the shared author precedence. Prints the new forward-slash ref; JSON prints {"id","ref"}.
status: closed
deps: [AWIT-0ND56E3G, AWIT-0ND56G3G]
labels: [phase4, p1]
refs_base: repo
refs:
  - plan/implementation-guide.md
  - plan/awit-implementation-plan.md
---

## Summary
After this ticket `internal/cli/comment.go` registers `awit comment <id> [text...]`. The comment text is the positional args after the id joined by single spaces; when there is no text and no `--file` and stdin is not a terminal, the whole of stdin is the text. `--file <path>` copies a file verbatim into `.awit/comments/<id>/` (extension kept) instead of writing a Markdown comment; passing both text and `--file` is the error `pass either text or --file`. Whitespace-only text is the error `empty comment`. The author comes from `resolveAuthor` (flag → `AWIT_AGENT` → `config.agent_id` → git `user.name` → error). On success the command prints the ref that was appended to the item's `refs` (e.g. `../comments/AWIT-TEST0001/20260918T101500Z-jan.md`) followed by a newline; with `--format json` it prints `{"id": "...", "ref": "..."}`. All writing is done by `Store.AddComment` / `Store.AttachFile`, which already save the item.

## Context (read first)
- Guide §2 decision 4 (author precedence) and the "Comment file format" / "Comment filename" rows: frontmatter `author`, `created` (RFC3339 UTC) then a blank line then the text; attached files are copied verbatim with no frontmatter; filename `<YYYYMMDDTHHMMSSZ>-<sanitised author><ext>` with `-2`, `-3` on collision. All of that lives in `pkg/item` already — this ticket does **not** build filenames or frontmatter.
- Guide §4.4: `Store.AddComment(it, author, now, text) (ref, err)` and `Store.AttachFile(it, author, now, src) (ref, err)`. Both write the comment file, append the forward-slash ref to `it.Refs`, **save the item**, and return the ref. Do not call `s.Save` after them. `AttachFile` keeps `filepath.Ext(src)` (case included) and reads `src` as given, so relative `--file` paths are relative to the process cwd, not the repo.
- Guide §4.11: `openStore(cmd)`; global `--format` read via `detectFormat(cmd)`; errors returned from `Action` become `Error: <msg>` on stderr with exit 1. Stdin is `cmd.Root().Reader` (the root command sets `Reader: stdin`; subcommands inherit it). Tests inject stdin through `runStdin`.
- `AWIT-0ND56M3G` (`internal/cli/author.go`) defines the helpers this ticket calls. Their **actual** signatures (use these, not a `cmd`-taking variant):
  ```go
  func loadItem(s *item.Store, id string) (*item.Item, error)     // "unknown item <id>" / "<id>: <REASON>: <detail> (fix the file, then retry)"
  func resolveAuthor(flagAuthor, repoRoot string, cfg config.Config) (string, error)
  func withAgentPrefix(v string) string
  ```
  Precedence inside `resolveAuthor`: `--author` verbatim → `AWIT_AGENT` with `agent/` prefix unless the value already contains `/` → `config.agent_id` (same prefix rule) → `git config user.name` lowercased with spaces → `-` (no prefix) → error `no author; pass --author or set AWIT_AGENT`. Call it as `resolveAuthor(cmd.String("author"), s.Root, s.Config)`.
- `AWIT-0ND56H3G` (`internal/cli/create.go`) defines `detectFormat(cmd *cli.Command) (format.Format, error)`. `AWIT-0ND56F3G` provides `format.JSON` and `format.IsTerminal(*os.File) bool`. Both tickets are phase 1 and closed before phase 4 starts; if `detectFormat` is somehow absent, define it in `comment.go` with exactly this code:
  ```go
  func detectFormat(cmd *cli.Command) (format.Format, error) {
  	var stdout *os.File
  	if f, ok := cmd.Root().Writer.(*os.File); ok {
  		stdout = f
  	}
  	return format.Detect(cmd.Root().String("format"), stdout)
  }
  ```
- Argument order in tests: put flags **before** the positional args (`comment --author jan AWIT-TEST0001 some text`). urfave/cli v3 stops flag parsing at the first positional argument, so `comment AWIT-TEST0001 text --author jan` would make `--author` part of the text.
- Error ordering (tests depend on it): `comment needs an item id` → `pass either text or --file` → store open errors → `loadItem` errors → author errors → stdin read → `empty comment` → write errors. The text/file exclusivity check runs before touching the store so the user gets the usage mistake first.
- Test harness (`internal/cli/helpers_test.go`, from `AWIT-0ND56G3G`): `run(t, args...)`, `runStdin(t, stdin, args...)`, `copyFixture(t, "clean") string` (temp repo root, always pass `--repo`), `readItem(t, repo, id) *item.Item`. The `clean` fixture item `AWIT-TEST0001` has `refs: []`; `parse-error` fixture's `AWIT-TEST0001.md` is unparseable.
- Guide §1: refs are forward slashes even on Windows; the ref printed and stored must never contain `\`. `AddComment` uses `path.Join`, so this holds automatically — the test asserts it anyway.

## Files
- Create: `internal/cli/comment.go`
- Create: `internal/cli/comment_test.go`
- Modify: `internal/cli/app.go` — append `commentCmd` to the `Commands` slice in `newRoot`.

## Interfaces
- Consumes:
  ```go
  func openStore(cmd *cli.Command) (*item.Store, error)
  func loadItem(s *item.Store, id string) (*item.Item, error)
  func resolveAuthor(flagAuthor, repoRoot string, cfg config.Config) (string, error)
  func detectFormat(cmd *cli.Command) (format.Format, error)
  func (s *item.Store) AddComment(it *item.Item, author string, now time.Time, text string) (string, error)
  func (s *item.Store) AttachFile(it *item.Item, author string, now time.Time, src string) (string, error)
  func format.IsTerminal(f *os.File) bool
  const format.JSON format.Format
  ```
- Produces (package-private, this ticket):
  ```go
  var commentCmd *cli.Command
  func commentAction(_ context.Context, cmd *cli.Command) error
  func readStdinText(r io.Reader) (string, error) // "" when r is a terminal
  type commentJSON struct {
      ID  string `json:"id"`
      Ref string `json:"ref"`
  }
  ```
- Stdout contract: compact/table → `<ref>\n`; json → two-space-indented object `{"id","ref"}` with trailing newline.

## Steps

- [ ] **Step 1: Write the failing tests.**

  Create `internal/cli/comment_test.go`:

  ```go
  package cli

  import (
  	"encoding/json"
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  	"time"

  	"github.com/eisenwinter/awit/pkg/item"
  )

  // commentFiles lists the files in <repo>/.awit/comments/<id>, sorted by name.
  func commentFiles(t *testing.T, repo, id string) []string {
  	t.Helper()
  	ents, err := os.ReadDir(filepath.Join(repo, ".awit", "comments", id))
  	if err != nil {
  		t.Fatalf("read comments dir: %v", err)
  	}
  	names := make([]string, 0, len(ents))
  	for _, e := range ents {
  		names = append(names, e.Name())
  	}
  	return names
  }

  // refFile turns a printed ref ("../comments/<id>/<file>") into the file's path under repo.
  func refFile(repo, ref string) string {
  	return filepath.Join(repo, ".awit", "items", filepath.FromSlash(ref))
  }

  func TestCommentInline(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	t.Setenv("AWIT_AGENT", "")
  	const id = "AWIT-TEST0001"
  	code, stdout, stderr := run(t, "--repo", dir, "comment", "--author", "jan", id, "first", "note")
  	if code != 0 || stderr != "" {
  		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
  	}
  	ref := strings.TrimSuffix(stdout, "\n")
  	if !strings.HasPrefix(ref, "../comments/"+id+"/") || !strings.HasSuffix(ref, "-jan.md") {
  		t.Fatalf("ref = %q, want ../comments/%s/<stamp>-jan.md", ref, id)
  	}
  	if strings.Contains(ref, "\\") {
  		t.Fatalf("ref %q contains a backslash", ref)
  	}
  	it := readItem(t, dir, id)
  	if len(it.Refs) != 1 || it.Refs[0] != ref {
  		t.Fatalf("refs = %v, want [%s]", it.Refs, ref)
  	}
  	body, err := os.ReadFile(refFile(dir, ref))
  	if err != nil {
  		t.Fatal(err)
  	}
  	text := string(body)
  	if !strings.HasPrefix(text, "---\nauthor: jan\ncreated: ") {
  		t.Fatalf("comment frontmatter = %q", text)
  	}
  	createdLine := strings.SplitN(strings.TrimPrefix(text, "---\nauthor: jan\ncreated: "), "\n", 2)[0]
  	if _, err := time.Parse(time.RFC3339, createdLine); err != nil || !strings.HasSuffix(createdLine, "Z") {
  		t.Fatalf("created = %q, want RFC3339 UTC: %v", createdLine, err)
  	}
  	if !strings.HasSuffix(text, "\n---\n\nfirst note\n") {
  		t.Fatalf("comment body = %q, want to end with blank line + \"first note\\n\"", text)
  	}
  }

  func TestCommentFile(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	const id = "AWIT-TEST0001"
  	src := filepath.Join(t.TempDir(), "notes.txt")
  	want := "hello\nworld\n"
  	if err := os.WriteFile(src, []byte(want), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	code, stdout, stderr := run(t, "--repo", dir, "comment", "--author", "jan", "--file", src, id)
  	if code != 0 || stderr != "" {
  		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
  	}
  	ref := strings.TrimSuffix(stdout, "\n")
  	if !strings.HasPrefix(ref, "../comments/"+id+"/") || !strings.HasSuffix(ref, "-jan.txt") {
  		t.Fatalf("ref = %q, want ../comments/%s/<stamp>-jan.txt (extension kept)", ref, id)
  	}
  	got, err := os.ReadFile(refFile(dir, ref))
  	if err != nil {
  		t.Fatal(err)
  	}
  	if string(got) != want {
  		t.Fatalf("attached bytes = %q, want verbatim %q (no frontmatter)", got, want)
  	}
  	if it := readItem(t, dir, id); len(it.Refs) != 1 || it.Refs[0] != ref {
  		t.Fatalf("refs = %v, want [%s]", it.Refs, ref)
  	}
  }

  func TestCommentStdin(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	const id = "AWIT-TEST0001"
  	code, stdout, stderr := runStdin(t, "from stdin\nsecond line\n", "--repo", dir, "comment", "--author", "jan", id)
  	if code != 0 || stderr != "" {
  		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
  	}
  	ref := strings.TrimSuffix(stdout, "\n")
  	body, err := os.ReadFile(refFile(dir, ref))
  	if err != nil {
  		t.Fatal(err)
  	}
  	if !strings.HasSuffix(string(body), "\n---\n\nfrom stdin\nsecond line\n") {
  		t.Fatalf("comment = %q, want stdin text as body", body)
  	}

  	// Empty and whitespace-only stdin are refused; nothing is written.
  	for _, in := range []string{"", "  \n\t\n"} {
  		code, _, stderr := runStdin(t, in, "--repo", dir, "comment", "--author", "jan", id)
  		if code != 1 || stderr != "Error: empty comment\n" {
  			t.Fatalf("stdin %q: exit %d stderr %q, want 1 / Error: empty comment", in, code, stderr)
  		}
  	}
  	if it := readItem(t, dir, id); len(it.Refs) != 1 {
  		t.Fatalf("refs = %v, want exactly the one stdin comment", it.Refs)
  	}
  }

  func TestCommentBothTextAndFile(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	src := filepath.Join(t.TempDir(), "notes.txt")
  	if err := os.WriteFile(src, []byte("x\n"), 0o644); err != nil {
  		t.Fatal(err)
  	}
  	code, stdout, stderr := run(t, "--repo", dir, "comment", "--author", "jan", "--file", src, "AWIT-TEST0001", "some", "text")
  	if code != 1 || stdout != "" || stderr != "Error: pass either text or --file\n" {
  		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
  	}
  	if it := readItem(t, dir, "AWIT-TEST0001"); len(it.Refs) != 0 {
  		t.Fatalf("refs = %v, want none written", it.Refs)
  	}
  	if _, err := os.Stat(filepath.Join(dir, ".awit", "comments", "AWIT-TEST0001")); !os.IsNotExist(err) {
  		t.Fatalf("comments dir must not be created on a usage error (stat err = %v)", err)
  	}
  }

  func TestCommentAuthorFromEnv(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	t.Setenv("AWIT_AGENT", "claude")
  	const id = "AWIT-TEST0001"
  	code, stdout, stderr := run(t, "--repo", dir, "comment", id, "agent", "note")
  	if code != 0 || stderr != "" {
  		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
  	}
  	ref := strings.TrimSuffix(stdout, "\n")
  	if !strings.HasSuffix(ref, "-claude.md") {
  		t.Fatalf("ref = %q, want filename ending -claude.md (agent/ prefix stripped from filename)", ref)
  	}
  	body, err := os.ReadFile(refFile(dir, ref))
  	if err != nil {
  		t.Fatal(err)
  	}
  	if !strings.Contains(string(body), "\nauthor: agent/claude\n") {
  		t.Fatalf("comment = %q, want author: agent/claude in frontmatter", body)
  	}
  }

  func TestCommentTwoDistinctFiles(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	const id = "AWIT-TEST0001"
  	_, out1, err1 := run(t, "--repo", dir, "comment", "--author", "jan", id, "one")
  	_, out2, err2 := run(t, "--repo", dir, "comment", "--author", "jan", id, "two")
  	if err1 != "" || err2 != "" {
  		t.Fatalf("stderr %q / %q", err1, err2)
  	}
  	ref1, ref2 := strings.TrimSpace(out1), strings.TrimSpace(out2)
  	if ref1 == ref2 {
  		t.Fatalf("two comments in the same second must get distinct refs, both %q", ref1)
  	}
  	it := readItem(t, dir, id)
  	if len(it.Refs) != 2 || it.Refs[0] != ref1 || it.Refs[1] != ref2 {
  		t.Fatalf("refs = %v, want [%s %s]", it.Refs, ref1, ref2)
  	}
  	if files := commentFiles(t, dir, id); len(files) != 2 {
  		t.Fatalf("comment files = %v, want 2", files)
  	}
  	b1, _ := os.ReadFile(refFile(dir, ref1))
  	b2, _ := os.ReadFile(refFile(dir, ref2))
  	if !strings.HasSuffix(string(b1), "\n\none\n") || !strings.HasSuffix(string(b2), "\n\ntwo\n") {
  		t.Fatalf("bodies = %q / %q", b1, b2)
  	}
  }

  func TestCommentJSON(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "comment", "--author", "jan", "AWIT-TEST0001", "json", "note")
  	if code != 0 || stderr != "" {
  		t.Fatalf("exit %d stderr %q", code, stderr)
  	}
  	var got struct {
  		ID  string `json:"id"`
  		Ref string `json:"ref"`
  	}
  	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
  		t.Fatalf("stdout %q is not JSON: %v", stdout, err)
  	}
  	if got.ID != "AWIT-TEST0001" || !strings.HasPrefix(got.Ref, "../comments/AWIT-TEST0001/") {
  		t.Fatalf("json = %+v", got)
  	}
  	if !strings.HasSuffix(stdout, "\n") || !strings.Contains(stdout, "\n  \"id\"") {
  		t.Fatalf("json must be two-space indented with trailing newline: %q", stdout)
  	}
  }

  func TestCommentErrors(t *testing.T) {
  	t.Run("no id", func(t *testing.T) {
  		dir := copyFixture(t, "clean")
  		code, _, stderr := run(t, "--repo", dir, "comment", "--author", "jan")
  		if code != 1 || stderr != "Error: comment needs an item id\n" {
  			t.Fatalf("exit %d stderr %q", code, stderr)
  		}
  	})
  	t.Run("unknown item", func(t *testing.T) {
  		dir := copyFixture(t, "clean")
  		code, _, stderr := run(t, "--repo", dir, "comment", "--author", "jan", "AWIT-TEST0099", "x")
  		if code != 1 || stderr != "Error: unknown item AWIT-TEST0099\n" {
  			t.Fatalf("exit %d stderr %q", code, stderr)
  		}
  	})
  	t.Run("broken item", func(t *testing.T) {
  		dir := copyFixture(t, "parse-error")
  		code, _, stderr := run(t, "--repo", dir, "comment", "--author", "jan", "AWIT-TEST0001", "x")
  		if code != 1 || !strings.HasPrefix(stderr, "Error: AWIT-TEST0001: "+string(item.ReasonParse)+": ") || !strings.Contains(stderr, "(fix the file, then retry)") {
  			t.Fatalf("exit %d stderr %q", code, stderr)
  		}
  	})
  	t.Run("missing attachment", func(t *testing.T) {
  		dir := copyFixture(t, "clean")
  		code, _, stderr := run(t, "--repo", dir, "comment", "--author", "jan", "--file", filepath.Join(dir, "nope.txt"), "AWIT-TEST0001")
  		if code != 1 || !strings.HasPrefix(stderr, "Error: ") || !strings.Contains(stderr, "nope.txt") {
  			t.Fatalf("exit %d stderr %q", code, stderr)
  		}
  		if it := readItem(t, dir, "AWIT-TEST0001"); len(it.Refs) != 0 {
  			t.Fatalf("refs = %v, want none after failed attach", it.Refs)
  		}
  	})
  }
  ```

- [ ] **Step 2: Run it, see it fail.**

  ```bash
  go test ./internal/cli -run 'TestComment' -v
  ```

  Expected: every test fails because the command does not exist yet. urfave reports an unknown subcommand as a usage error (exit 2), so the first failure looks like:

  ```text
  === RUN   TestCommentInline
      comment_test.go: exit 2 stdout "" stderr "..."
  --- FAIL: TestCommentInline
  ```

  Do not skip this step.

- [ ] **Step 3: Implement `comment.go` and register the command.**

  Create `internal/cli/comment.go`:

  ```go
  package cli

  import (
  	"context"
  	"encoding/json"
  	"fmt"
  	"io"
  	"os"
  	"strings"
  	"time"

  	"github.com/eisenwinter/awit/pkg/format"
  	"github.com/urfave/cli/v3"
  )

  var commentCmd = &cli.Command{
  	Name:      "comment",
  	Usage:     "Write a timestamped comment or attach a file to an item",
  	ArgsUsage: "<id> [text...]",
  	Flags: []cli.Flag{
  		&cli.StringFlag{Name: "file", Usage: "copy this file into the item's comments instead of writing text"},
  		&cli.StringFlag{Name: "author", Usage: "author (default: AWIT_AGENT, config agent_id, git user.name)"},
  	},
  	Action: commentAction,
  }

  // commentJSON is the --format json shape: the item that received the
  // comment and the forward-slash ref that was appended to its refs.
  type commentJSON struct {
  	ID  string `json:"id"`
  	Ref string `json:"ref"`
  }

  func commentAction(_ context.Context, cmd *cli.Command) error {
  	id := cmd.Args().First()
  	if id == "" {
  		return fmt.Errorf("comment needs an item id")
  	}
  	text := strings.Join(cmd.Args().Tail(), " ")
  	file := cmd.String("file")
  	if file != "" && text != "" {
  		return fmt.Errorf("pass either text or --file")
  	}
  	s, err := openStore(cmd)
  	if err != nil {
  		return err
  	}
  	it, err := loadItem(s, id)
  	if err != nil {
  		return err
  	}
  	author, err := resolveAuthor(cmd.String("author"), s.Root, s.Config)
  	if err != nil {
  		return err
  	}
  	now := time.Now().UTC()
  	var ref string
  	if file != "" {
  		ref, err = s.AttachFile(it, author, now, file)
  	} else {
  		if text == "" {
  			text, err = readStdinText(cmd.Root().Reader)
  			if err != nil {
  				return err
  			}
  		}
  		if strings.TrimSpace(text) == "" {
  			return fmt.Errorf("empty comment")
  		}
  		ref, err = s.AddComment(it, author, now, text)
  	}
  	if err != nil {
  		return err
  	}
  	f, err := detectFormat(cmd)
  	if err != nil {
  		return err
  	}
  	if f == format.JSON {
  		enc := json.NewEncoder(cmd.Writer)
  		enc.SetIndent("", "  ")
  		return enc.Encode(commentJSON{ID: it.ID, Ref: ref})
  	}
  	_, err = fmt.Fprintln(cmd.Writer, ref)
  	return err
  }

  // readStdinText returns everything on r, or "" when r is an interactive
  // terminal (so a bare "awit comment <id>" in a shell fails with "empty
  // comment" instead of hanging on a read nobody knows is pending).
  func readStdinText(r io.Reader) (string, error) {
  	if r == nil {
  		return "", nil
  	}
  	if f, ok := r.(*os.File); ok && format.IsTerminal(f) {
  		return "", nil
  	}
  	b, err := io.ReadAll(r)
  	if err != nil {
  		return "", fmt.Errorf("read stdin: %w", err)
  	}
  	return string(b), nil
  }
  ```

  In `internal/cli/app.go`, inside `newRoot`, append `commentCmd` to the `Commands` slice (after whatever is already there; order of the slice only affects `--help` output).

  Implementation rules:
  - `AddComment` and `AttachFile` save the item themselves. Do not call `s.Save(it)` afterwards — a second save is harmless but doubles the temp-then-rename work and hides bugs in those two methods.
  - The `--file` path is passed to `AttachFile` unchanged. A relative path is resolved against the process cwd by `os.ReadFile`; do not join it with `s.Root`.
  - `text` keeps its internal newlines; `AddComment` trims leading/trailing whitespace and appends exactly one `\n`. The `empty comment` check therefore uses `strings.TrimSpace`.
  - Missing-file errors from `AttachFile` are returned unwrapped (`*os.PathError` already includes the path), which is what the `missing attachment` test asserts.
  - Nothing is written before all validation has passed: the store is opened, the item loaded and the author resolved before any `MkdirAll` happens inside the store methods.

- [ ] **Step 4: Run the tests, see them pass, commit.**

  ```bash
  go test ./internal/cli -run 'TestComment' -v
  ```

  Expected:

  ```text
  === RUN   TestCommentInline
  --- PASS: TestCommentInline
  === RUN   TestCommentFile
  --- PASS: TestCommentFile
  === RUN   TestCommentStdin
  --- PASS: TestCommentStdin
  === RUN   TestCommentBothTextAndFile
  --- PASS: TestCommentBothTextAndFile
  === RUN   TestCommentAuthorFromEnv
  --- PASS: TestCommentAuthorFromEnv
  === RUN   TestCommentTwoDistinctFiles
  --- PASS: TestCommentTwoDistinctFiles
  === RUN   TestCommentJSON
  --- PASS: TestCommentJSON
  === RUN   TestCommentErrors
      --- PASS: TestCommentErrors/no_id
      --- PASS: TestCommentErrors/unknown_item
      --- PASS: TestCommentErrors/broken_item
      --- PASS: TestCommentErrors/missing_attachment
  --- PASS: TestCommentErrors
  PASS
  ok  	github.com/eisenwinter/awit/internal/cli
  ```

  Then the whole package (existing command tests must still pass because `newRoot` changed):

  ```bash
  go test ./internal/cli -count=1
  gofmt -l internal/cli
  git add internal/cli/comment.go internal/cli/comment_test.go internal/cli/app.go
  git commit -m "cli/comment: inline text, --file attachments, author resolution"
  ```

  `gofmt -l internal/cli` must print nothing.

- [ ] **Step 5: Close ticket.**

  1. Run `go build ./... && go vet ./internal/cli && go test ./internal/cli -count=1 -run TestComment -v` and copy the output.
  2. With the new command, write the closing comment through awit itself (dogfooding) from the repo root:

     ```bash
     go run ./cmd/awit comment --author <you> AWIT-0ND56Y3G "Closed AWIT-0ND56Y3G. Acceptance output: <paste the output from step 1>"
     ```

     This creates `.awit/comments/AWIT-0ND56Y3G/<YYYYMMDDTHHMMSSZ>-<author>.md` and appends its ref to the ticket's `refs:` block. If `go run` is not possible in your environment, create the same file by hand with the frontmatter `author:` / `created:` (RFC3339 UTC), a blank line, and the text, then append `  - ../comments/AWIT-0ND56Y3G/<file>` to `refs:`.
  3. Change `status: open` to `status: closed` in `.awit/items/AWIT-0ND56Y3G.md`. Touch nothing else in the frontmatter.
  4. Commit:

     ```bash
     git add .awit/items/AWIT-0ND56Y3G.md .awit/comments/AWIT-0ND56Y3G/
     git commit -m "tickets: close AWIT-0ND56Y3G"
     ```

## Acceptance Criteria
- `go test ./internal/cli -run TestComment -count=1 -v` — all eight tests (and every `TestCommentErrors` subtest) PASS; `go test ./internal/cli -count=1` stays green.
- `awit comment --author jan AWIT-TEST0001 first note` on a copy of the `clean` fixture prints `../comments/AWIT-TEST0001/<stamp>-jan.md`, the item's `refs` gains exactly that entry, and the comment file is `---\nauthor: jan\ncreated: <RFC3339 UTC>\n---\n\nfirst note\n`.
- `awit comment --author jan --file notes.txt AWIT-TEST0001` copies the bytes verbatim to `<stamp>-jan.txt` and prints that ref.
- `printf 'from stdin\n' | awit comment --author jan AWIT-TEST0001` uses stdin as the text; empty or whitespace-only stdin → `Error: empty comment`, exit 1, nothing written.
- Text plus `--file` → `Error: pass either text or --file`, exit 1, no comments directory created.
- `AWIT_AGENT=claude awit comment AWIT-TEST0001 note` → file `<stamp>-claude.md` with `author: agent/claude`.
- Two comments by the same author in the same second produce two distinct refs and two files.
- `--format json` prints `{"id": "...", "ref": "..."}` two-space indented with a trailing newline.
- Unknown id → `Error: unknown item <id>`; unparseable item → `Error: <id>: PARSE ERROR: ... (fix the file, then retry)`.
- `gofmt -l internal/cli` prints nothing; `comment.go` does not import `internal/gitx` (author resolution is entirely inside `resolveAuthor`).

## Out of scope
- Comment filename/frontmatter generation, collision suffixes, ref formatting — all inside `pkg/item` (`AWIT-0ND56E3G`).
- `resolveAuthor` / `loadItem` bodies — `AWIT-0ND56M3G`.
- Rendering comments when showing an item (`show --full` resolves refs — `AWIT-0ND5703G`).
- Editing or deleting existing comments, `--reply`, threading, Markdown validation of the text.
- Git commits after commenting (only `next --claim` commits; guide §2 decision 3).
- Reading `--file` from stdin (`--file -`) or URLs.
