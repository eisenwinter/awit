---
id: AWIT-0ND56E3G
title: 'pkg/item Store: find, load-all with broken detection, atomic save, mint, comments'
brief: >-
  Add pkg/item.Store: Find/Open/Init, LoadAll with quarantine reasons, atomic Save via config.WriteAtomic, Mint, and comment/attachment files with sanitised names.
status: closed
deps: [AWIT-0ND56D3G, AWIT-0ND5693G, AWIT-0ND56A3G, AWIT-0ND56C3G]
labels: [phase1, p0]
refs_base: repo
refs:
  - plan/implementation-guide.md
  - plan/awit-implementation-plan.md
---

## Summary
After this ticket `pkg/item/store.go` and `pkg/item/comment.go` exist. `Store` locates and initialises `.awit/`, loads every `items/*.md` as either an `Item` or a `Broken` (never panicking on a bad file), writes items through `config.WriteAtomic`, mints IDs via `id.Mint`, and appends comments/attachments under `comments/<id>/` with a forward-slash `refs` entry. CLI commands are not wired yet. `testdata/fixtures` do not exist — every test builds a tree in `t.TempDir()`.

## Context (read first)
- **guide §4.4 `pkg/item` — Store** — exact signatures. Copy them; do not rename.
- **guide §4.2** — `config.WriteAtomic(path string, data []byte) error` is how `Save` (and comment files) hit disk. Do not reimplement temp-rename.
- **guide §4.1** — `id.Mint(prefix, now, worker, exists)`, `id.Worker`, `id.Valid`. Mint 50 IDs at **distinct seconds**; 4 random bits mean 16 IDs per timestamp, so the same `now` fifty times hits `ErrExhausted`.
- **guide §4.5** — `gitx.Branch(s.Root)` returns `""` outside a repo; `Mint` still works.
- **guide §1** — never hardcode `/` in filesystem paths (`filepath`). `refs` inside frontmatter are always forward slashes: build them with `path.Join`, **never** `filepath.Join`.
- **guide §2** — `Init` gitignores only `.awit/.lock` (forward slashes, git convention). Comment filename `<YYYYMMDDTHHMMSSZ>-<author><ext>`; collision `-2`, `-3` before the extension. Duplicate ID = two `items/` stems equal case-insensitively (`strings.ToUpper`). Comment file format: frontmatter `author` + `created` (RFC3339 UTC), blank line, text. `--file` copies bytes verbatim, no frontmatter.
- **guide §5** — stdlib `testing`, `t.TempDir()`. Comparing refs: forward slashes, no `\\`.
- **spec Data model / Quarantine** — conflict markers, parse errors, id mismatch, duplicate ids all become `Broken`; the CLI never panics on a bad file.
- Deps must be `status: closed` before you start: `AWIT-0ND56D3G` (Parse/Bytes/New/SetRefs/HasConflictMarkers/Broken/Reason*), `AWIT-0ND5693G` (id), `AWIT-0ND56A3G` (config), `AWIT-0ND56C3G` (gitx.Branch).

## Files
- Create: `pkg/item/store.go`
- Create: `pkg/item/comment.go`
- Create: `pkg/item/store_test.go`
- Create: `pkg/item/comment_test.go`
- Modify: none of the files from `AWIT-0ND56D3G` unless a compile error forces a tiny unexported helper; do not change Parse/Bytes behaviour.

## Interfaces
- Consumes: `item.Parse`, `Item.Bytes`, `Item.New`, `Item.SetRefs`, `HasConflictMarkers`, `Broken`, `Reason*` from this package; `config.Default`, `config.Load`, `config.Config.Write`, `config.WriteAtomic`; `id.Mint`, `id.Worker`, `id.Valid`; `gitx.Branch`.
- Produces (verbatim from guide §4.4):

```go
package item

const DirName = ".awit"

type Store struct {
    Root   string
    Dir    string
    Config config.Config
}

func Find(start string) (*Store, error)
func Open(repoRoot string) (*Store, error)
func Init(repoRoot, prefix string) (*Store, error)

var ErrNotFound = errors.New("no .awit directory found (run awit init)")
var ErrExists   = errors.New(".awit already exists")

func (s *Store) ItemsDir() string
func (s *Store) CommentsDir(id string) string
func (s *Store) ItemPath(id string) string
func (s *Store) Exists(id string) bool
func (s *Store) Load(id string) (*Item, error)
func (s *Store) LoadAll() ([]*Item, []Broken, error)
func (s *Store) Save(it *Item) error
func (s *Store) Mint(now time.Time) (string, error)
func (s *Store) AddComment(it *Item, author string, now time.Time, text string) (ref string, err error)
func (s *Store) AttachFile(it *Item, author string, now time.Time, src string) (ref string, err error)
func CommentFileName(now time.Time, author, ext string) string
func SanitizeAuthor(author string) string

type BrokenError struct{ Broken Broken }
func (e *BrokenError) Error() string
```

- Produces (package-private):

```go
func ensureGitignore(repoRoot string) error
func uniqueCommentFile(dir, stampAuthor, ext string) (filename string, err error)
```

## Steps

- [ ] **Step 1: Failing tests for `SanitizeAuthor` and `CommentFileName`.**
  Create `pkg/item/comment_test.go`:

```go
package item

import (
	"testing"
	"time"
)

func TestSanitizeAuthor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want string
	}{
		{"agent/Claude Opus", "claude-opus"},
		{"agent/", "anon"},
		{"Jan", "jan"},
		{"Foo_Bar.baz", "foo_bar.baz"},
		{"@@@", "anon"},
		{"", "anon"},
		{"agent/agent/x", "agent-x"},
		{"--ok--", "ok"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := SanitizeAuthor(tt.in); got != tt.want {
				t.Fatalf("SanitizeAuthor(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCommentFileName(t *testing.T) {
	now := time.Date(2026, 9, 17, 14, 32, 5, 0, time.UTC)
	got := CommentFileName(now, "agent/Claude Opus", ".md")
	want := "20260917T143205Z-claude-opus.md"
	if got != want {
		t.Fatalf("CommentFileName = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2: Run it, see it fail.**

```sh
go test ./pkg/item -run 'TestSanitizeAuthor|TestCommentFileName' -v
```

  Expected: `undefined: SanitizeAuthor`, `undefined: CommentFileName`, `[build failed]`.

- [ ] **Step 3: Implement `comment.go` sanitiser + filename (AddComment later).**
  Create `pkg/item/comment.go` with at least:

```go
package item

import (
	"path"
	"strings"
	"time"
	"unicode"
)

func SanitizeAuthor(author string) string {
	s := strings.TrimPrefix(author, "agent/")
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if ok {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := b.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	out = strings.Trim(out, "-")
	if out == "" {
		return "anon"
	}
	return out
}

func CommentFileName(now time.Time, author, ext string) string {
	return now.UTC().Format("20060102T150405Z") + "-" + SanitizeAuthor(author) + ext
}

func uniqueCommentFile(dir, stampAuthor, ext string) (string, error) {
	name := stampAuthor + ext
	p := filepath.Join(dir, name)
	if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
		return name, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		// Stat nil err means exists — fall through to suffix.
	}
	if _, err := os.Stat(p); err == nil {
		for n := 2; n < 10000; n++ {
			name = stampAuthor + "-" + strconv.Itoa(n) + ext
			p = filepath.Join(dir, name)
			if _, err := os.Stat(p); errors.Is(err, os.ErrNotExist) {
				return name, nil
			}
		}
		return "", fmt.Errorf("item: comment filename collisions in %s", dir)
	}
	if errors.Is(err, os.ErrNotExist) {
		return name, nil
	}
	return "", err
}
```

  Write a clean version of `uniqueCommentFile` (the sketch above is the algorithm; gofmt-valid code with one `os.Stat` per candidate, first hit of `ErrNotExist` wins, first name has **no** `-1`). Need imports: `errors`, `fmt`, `os`, `path/filepath`, `strconv`. `path` is imported for `AddComment` in Step 11. Drop unused `unicode`/`path` until then if the compiler complains.

  Order for SanitizeAuthor: strip leading `"agent/"` (once, case-sensitive), then lower, then map every rune not in `[a-z0-9._-]` to `-`, collapse `--+` to `-`, trim `-`, empty → `"anon"`.

- [ ] **Step 4: Pass sanitiser tests, commit.**

```sh
go test ./pkg/item -run 'TestSanitizeAuthor|TestCommentFileName' -v
```

  Expected: `--- PASS: TestSanitizeAuthor`, `--- PASS: TestCommentFileName`, `ok`.

```sh
gofmt -w pkg/item/comment.go pkg/item/comment_test.go
git add pkg/item/comment.go pkg/item/comment_test.go
git commit -m "item: SanitizeAuthor and CommentFileName"
```

- [ ] **Step 5: Failing tests for Init, Open, Find.**
  Create `pkg/item/store_test.go`:

```go
package item

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eisenwinter/awit/pkg/id"
)

func TestInitCreatesLayoutAndGitignore(t *testing.T) {
	root := t.TempDir()
	s, err := Init(root, "AWIT")
	if err != nil {
		t.Fatal(err)
	}
	if s.Root != root {
		t.Fatalf("Root = %q, want %q", s.Root, root)
	}
	for _, p := range []string{
		filepath.Join(root, ".awit"),
		filepath.Join(root, ".awit", "items"),
		filepath.Join(root, ".awit", "comments"),
		filepath.Join(root, ".awit", "config.yaml"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
	}
	gi, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(strings.TrimSuffix(string(gi), "\n"), "\n") {
		if line == ".awit/.lock" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf(".gitignore .awit/.lock count = %d, want 1\n%s", n, gi)
	}
	if _, err := Init(root, "AWIT"); !errors.Is(err, ErrExists) {
		t.Fatalf("second Init err = %v, want ErrExists", err)
	}
}

func TestInitAppendsToExistingGitignore(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(p, []byte("dist/"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Init(root, "AWIT"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("dist/\n.awit/.lock\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("gitignore = %q, want %q", got, want)
	}
}

func TestOpenMissing(t *testing.T) {
	_, err := Open(t.TempDir())
	if err == nil {
		t.Fatal("Open missing .awit: want error")
	}
}

func TestFindWalksUp(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root, "AWIT"); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	s, err := Find(nested)
	if err != nil {
		t.Fatal(err)
	}
	if s.Root != root {
		t.Fatalf("Find Root = %q, want %q", s.Root, root)
	}
}

func TestFindNotFound(t *testing.T) {
	_, err := Find(t.TempDir())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Find err = %v, want ErrNotFound", err)
	}
}
```

- [ ] **Step 6: Run it, see it fail.**

```sh
go test ./pkg/item -run 'TestInit|TestOpenMissing|TestFind' -v
```

  Expected: `undefined: Init`, `undefined: Open`, `undefined: Find`, `undefined: ErrExists`, `undefined: ErrNotFound`, `[build failed]`.

- [ ] **Step 7: Implement `store.go` Init/Open/Find/path helpers.**
  Create `pkg/item/store.go`. Required behaviour:

  - `DirName = ".awit"`.
  - `Init`: if `repoRoot/.awit` exists (Stat succeeds) → `ErrExists`. `MkdirAll` `items` and `comments` at `0o755`. `config.Default(prefix).Write(dir)`. `ensureGitignore(repoRoot)`. Return `Open(repoRoot)`.
  - `ensureGitignore`: read `.gitignore` (missing file → empty). If non-empty and not ending in `\n`, append `\n` first. If any line equals `.awit/.lock`, write only if you added that newline; otherwise return. Else append `.awit/.lock\n`. Write via `config.WriteAtomic`.
  - `Open`: `dir := filepath.Join(repoRoot, DirName)`. If not a directory, wrap the Stat error (so `TestOpenMissing` sees a non-nil error). `cfg, err := config.Load(dir)`. `return &Store{Root: repoRoot, Dir: dir, Config: cfg}, nil`.
  - `Find`: `filepath.Abs(start)`, then loop: if `filepath.Join(cur, DirName)` is a directory, `return Open(cur)`. `parent := filepath.Dir(cur)`; if `parent == cur` → `ErrNotFound`. Else `cur = parent`.
  - `ItemsDir` = `filepath.Join(s.Dir, "items")`.
  - `CommentsDir(id)` = `filepath.Join(s.Dir, "comments", id)`.
  - `ItemPath(id)` = `filepath.Join(s.ItemsDir(), id+".md")`.
  - `Exists(id)` = `os.Stat(s.ItemPath(id)) == nil`.

```go
func (s *Store) Mint(now time.Time) (string, error) {
	return id.Mint(s.Config.Prefix, now, id.Worker(s.Root, gitx.Branch(s.Root)), s.Exists)
}
```

  Stub `Load`/`LoadAll`/`Save` so the file compiles if you add them empty, or wait for Step 9 — tests in this step do not call them.

- [ ] **Step 8: Pass layout tests, commit.**

```sh
go test ./pkg/item -run 'TestInit|TestOpenMissing|TestFind|TestSanitizeAuthor|TestCommentFileName' -v
```

  Expected: all PASS. `ok  	github.com/eisenwinter/awit/pkg/item`.

```sh
gofmt -w pkg/item/store.go
git add pkg/item/store.go pkg/item/store_test.go
git commit -m "item: Store Init, Open, Find"
```

- [ ] **Step 9: Failing tests for LoadAll, Load, Save, Mint.**
  Append to `pkg/item/store_test.go`:

```go
func initStore(t *testing.T) *Store {
	t.Helper()
	s, err := Init(t.TempDir(), "AWIT")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func writeItemFile(t *testing.T, s *Store, name, body string) {
	t.Helper()
	p := filepath.Join(s.ItemsDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func minimal(id string) string {
	return "---\nid: " + id + "\ntitle: T\nstatus: open\n---\n"
}

func TestLoadAllSortedAndClean(t *testing.T) {
	s := initStore(t)
	writeItemFile(t, s, "AWIT-TEST0002.md", minimal("AWIT-TEST0002"))
	writeItemFile(t, s, "AWIT-TEST0001.md", minimal("AWIT-TEST0001"))
	items, broken, err := s.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(broken) != 0 {
		t.Fatalf("broken = %#v", broken)
	}
	if len(items) != 2 || items[0].ID != "AWIT-TEST0001" || items[1].ID != "AWIT-TEST0002" {
		t.Fatalf("items = %#v", idsOf(items))
	}
}

func idsOf(items []*Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

func TestLoadAllClassifiesBroken(t *testing.T) {
	s := initStore(t)
	writeItemFile(t, s, "AWIT-TEST0001.md", minimal("AWIT-TEST0001"))
	writeItemFile(t, s, "AWIT-TEST0002.md", "---\nid: AWIT-TEST0002\ntitle: T\nstatus: open\n<<<<<<< HEAD\n---\n")
	writeItemFile(t, s, "AWIT-TEST0003.md", "---\nid: AWIT-TEST0003\ntitle: [unclosed\nstatus: open\n---\n")
	writeItemFile(t, s, "AWIT-TEST0004.md", minimal("AWIT-TEST0009"))
	items, broken, err := s.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ID != "AWIT-TEST0001" {
		t.Fatalf("items = %#v", idsOf(items))
	}
	if len(broken) != 3 {
		t.Fatalf("broken len = %d, %#v", len(broken), broken)
	}
	want := []struct {
		id string
		r  Reason
	}{
		{"AWIT-TEST0002", ReasonConflict},
		{"AWIT-TEST0003", ReasonParse},
		{"AWIT-TEST0004", ReasonIDMismatch},
	}
	for i, w := range want {
		if broken[i].ID != w.id || broken[i].Reason != w.r {
			t.Fatalf("broken[%d] = %s %s, want %s %s", i, broken[i].ID, broken[i].Reason, w.id, w.r)
		}
	}
}

func TestLoadAllDuplicateCaseInsensitive(t *testing.T) {
	s := initStore(t)
	writeItemFile(t, s, "AWIT-TEST0001.md", minimal("AWIT-TEST0001"))
	lower := filepath.Join(s.ItemsDir(), "awit-test0001.md")
	if err := os.WriteFile(lower, []byte(minimal("awit-test0001")), 0o644); err != nil {
		t.Fatal(err)
	}
	ents, err := os.ReadDir(s.ItemsDir())
	if err != nil {
		t.Fatal(err)
	}
	md := 0
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".md") {
			md++
		}
	}
	if md < 2 {
		t.Skip("case-insensitive filesystem")
	}
	items, broken, err := s.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %#v, want none", idsOf(items))
	}
	if len(broken) != 2 {
		t.Fatalf("broken = %#v", broken)
	}
	for _, b := range broken {
		if b.Reason != ReasonDuplicate {
			t.Fatalf("reason = %s, want DUPLICATE ID", b.Reason)
		}
	}
}

func TestLoadBrokenError(t *testing.T) {
	s := initStore(t)
	writeItemFile(t, s, "AWIT-TEST0001.md", "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\n<<<<<<< HEAD\n---\n")
	_, err := s.Load("AWIT-TEST0001")
	var be *BrokenError
	if !errors.As(err, &be) {
		t.Fatalf("err = %v, want *BrokenError", err)
	}
	if be.Broken.Reason != ReasonConflict {
		t.Fatalf("reason = %s", be.Broken.Reason)
	}
}

func TestLoadMissing(t *testing.T) {
	s := initStore(t)
	_, err := s.Load("AWIT-TEST0001")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want os.ErrNotExist", err)
	}
}

func TestSaveAtomicAndRoundTrip(t *testing.T) {
	s := initStore(t)
	it := New("AWIT-TEST0001", "Title", "Brief.", nil, []string{"auth"})
	if err := s.Save(it); err != nil {
		t.Fatal(err)
	}
	if it.Path != s.ItemPath("AWIT-TEST0001") {
		t.Fatalf("Path = %q, want %q", it.Path, s.ItemPath("AWIT-TEST0001"))
	}
	got, err := s.Load("AWIT-TEST0001")
	if err != nil {
		t.Fatal(err)
	}
	want, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := got.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(raw, want) {
		t.Fatalf("round-trip\ngot:\n%s\nwant:\n%s", raw, want)
	}
}

func TestMintUniqueAndValid(t *testing.T) {
	s := initStore(t)
	now := time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC)
	seen := map[string]bool{}
	for i := 0; i < 50; i++ {
		got, err := s.Mint(now.Add(time.Duration(i) * time.Second))
		if err != nil {
			t.Fatalf("Mint #%d: %v", i, err)
		}
		if !id.Valid("AWIT", got) {
			t.Fatalf("invalid id %q", got)
		}
		if seen[got] {
			t.Fatalf("duplicate %q", got)
		}
		seen[got] = true
		if s.Exists(got) {
			t.Fatalf("Mint must not create the file; Exists(%q) true", got)
		}
	}
}
```

- [ ] **Step 10: Run it, see Load/Save fail.**

```sh
go test ./pkg/item -run 'TestLoad|TestSave|TestMintUniqueAndValid' -v
```

  Expected: `undefined: LoadAll` / `undefined: Save` / `undefined: BrokenError` or FAIL if stubs return nil.

- [ ] **Step 11: Implement Load, LoadAll, Save, BrokenError.**
  `BrokenError.Error` returns `fmt.Sprintf("%s: %s: %s", e.Broken.Reason, e.Broken.ID, e.Broken.Detail)`.

  `Save`: `data, err := it.Bytes()`; `os.MkdirAll(s.ItemsDir(), 0o755)`; `config.WriteAtomic(s.ItemPath(it.ID), data)`; `it.Path = s.ItemPath(it.ID)`.

  `Load`: `os.ReadFile(s.ItemPath(id))`. If that error is non-nil, **return it unchanged** (`os.ErrNotExist` must pass through). Then:

  1. `HasConflictMarkers` → `&BrokenError{Broken: Broken{ID: id, Path: path, Reason: ReasonConflict, Detail: "conflict markers in file"}}`
  2. `Parse` error → `ReasonParse`, Detail = `err.Error()`
  3. `it.ID != stem` (stem = `strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))`) → `ReasonIDMismatch`
  4. else return `it, nil`

  `LoadAll` — implement **this algorithm**, not a different order:

```go
func (s *Store) LoadAll() ([]*Item, []Broken, error) {
	ents, err := os.ReadDir(s.ItemsDir())
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(ents, func(i, j int) bool { return ents[i].Name() < ents[j].Name() })

	type rec struct {
		stem   string
		path   string
		item   *Item
		broken *Broken
	}
	var files []rec
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		path := filepath.Join(s.ItemsDir(), e.Name())
		stem := strings.TrimSuffix(e.Name(), ".md")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		r := rec{stem: stem, path: path}
		switch {
		case HasConflictMarkers(data):
			r.broken = &Broken{ID: stem, Path: path, Reason: ReasonConflict, Detail: "conflict markers in file"}
		default:
			it, err := Parse(path, data)
			switch {
			case err != nil:
				r.broken = &Broken{ID: stem, Path: path, Reason: ReasonParse, Detail: err.Error()}
			case it.ID != stem:
				r.broken = &Broken{ID: stem, Path: path, Reason: ReasonIDMismatch,
					Detail: fmt.Sprintf("id %q != filename stem %q", it.ID, stem)}
			default:
				r.item = it
			}
		}
		files = append(files, r)
	}

	groups := map[string][]int{}
	for i, f := range files {
		key := strings.ToUpper(f.stem)
		groups[key] = append(groups[key], i)
	}
	for _, idxs := range groups {
		if len(idxs) < 2 {
			continue
		}
		for _, i := range idxs {
			files[i].item = nil
			files[i].broken = &Broken{
				ID: files[i].stem, Path: files[i].path,
				Reason: ReasonDuplicate, Detail: "duplicate id (case-insensitive)",
			}
		}
	}

	var items []*Item
	var broken []Broken
	for _, f := range files {
		if f.broken != nil {
			broken = append(broken, *f.broken)
			continue
		}
		items = append(items, f.item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	sort.Slice(broken, func(i, j int) bool { return broken[i].ID < broken[j].ID })
	return items, broken, nil
}
```

  Duplicate grouping **overrides** a previous reason: both colliding files become `ReasonDuplicate`. Directory read errors propagate; a bad file never makes `LoadAll` return a non-nil error.

- [ ] **Step 12: Pass load/save/mint, commit.**

```sh
go test ./pkg/item -run 'TestLoad|TestSave|TestMintUniqueAndValid|TestInit|TestFind' -v
```

  Expected: all PASS.

```sh
gofmt -w pkg/item/store.go pkg/item/store_test.go
git add pkg/item/store.go pkg/item/store_test.go
git commit -m "item: LoadAll quarantine, Save, Mint"
```

- [ ] **Step 13: Failing tests for AddComment and AttachFile.**
  Append to `pkg/item/store_test.go`:

```go
func TestAddCommentWritesFileAndRef(t *testing.T) {
	s := initStore(t)
	it := New("AWIT-TEST0001", "Title", "Brief.", nil, nil)
	if err := s.Save(it); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 17, 14, 32, 5, 0, time.UTC)
	ref, err := s.AddComment(it, "Jan", now, "  hello world  ")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(ref, `\`) || strings.Contains(ref, "\\") {
		t.Fatalf("ref has backslash: %q", ref)
	}
	if !strings.Contains(ref, "/") {
		t.Fatalf("ref missing /: %q", ref)
	}
	if ref != "../comments/AWIT-TEST0001/20260917T143205Z-jan.md" {
		t.Fatalf("ref = %q", ref)
	}
	p := filepath.Join(s.CommentsDir("AWIT-TEST0001"), "20260917T143205Z-jan.md")
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("---\nauthor: Jan\ncreated: 2026-09-17T14:32:05Z\n---\n\nhello world\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("comment file\ngot:\n%s\nwant:\n%s", got, want)
	}
	if len(it.Refs) != 1 || it.Refs[0] != ref {
		t.Fatalf("item refs = %#v", it.Refs)
	}
}

func TestAddCommentCollisionSuffix(t *testing.T) {
	s := initStore(t)
	it := New("AWIT-TEST0001", "Title", "Brief.", nil, nil)
	if err := s.Save(it); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 17, 14, 32, 5, 0, time.UTC)
	if _, err := s.AddComment(it, "jan", now, "one"); err != nil {
		t.Fatal(err)
	}
	ref2, err := s.AddComment(it, "jan", now, "two")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(ref2, "20260917T143205Z-jan-2.md") {
		t.Fatalf("collision ref = %q, want *-2.md", ref2)
	}
	p := filepath.Join(s.CommentsDir("AWIT-TEST0001"), "20260917T143205Z-jan-2.md")
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
}

func TestAttachFilePreservesExtension(t *testing.T) {
	s := initStore(t)
	it := New("AWIT-TEST0001", "Title", "Brief.", nil, nil)
	if err := s.Save(it); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(t.TempDir(), "shot.PNG")
	payload := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a}
	if err := os.WriteFile(src, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 17, 14, 32, 5, 0, time.UTC)
	ref, err := s.AttachFile(it, "jan", now, src)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(ref, ".PNG") && !strings.HasSuffix(ref, ".png") {
		// filepath.Ext keeps original case; this source is ".PNG"
		t.Fatalf("ext lost: %q", ref)
	}
	if !strings.HasSuffix(ref, "20260917T143205Z-jan.PNG") {
		t.Fatalf("ref = %q", ref)
	}
	got, err := os.ReadFile(filepath.Join(s.CommentsDir("AWIT-TEST0001"), "20260917T143205Z-jan.PNG"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mutated: %v", got)
	}
}
```

- [ ] **Step 14: Run it, see comments fail.**

```sh
go test ./pkg/item -run 'TestAddComment|TestAttachFile' -v
```

  Expected: `undefined: AddComment` / `undefined: AttachFile` or FAIL.

- [ ] **Step 15: Implement AddComment and AttachFile in `comment.go`.**

```go
func (s *Store) AddComment(it *Item, author string, now time.Time, text string) (string, error) {
	dir := s.CommentsDir(it.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	base := now.UTC().Format("20060102T150405Z") + "-" + SanitizeAuthor(author)
	filename, err := uniqueCommentFile(dir, base, ".md")
	if err != nil {
		return "", err
	}
	created := now.UTC().Format(time.RFC3339)
	body := "---\nauthor: " + author + "\ncreated: " + created + "\n---\n\n" + strings.TrimSpace(text) + "\n"
	if err := config.WriteAtomic(filepath.Join(dir, filename), []byte(body)); err != nil {
		return "", err
	}
	ref := path.Join("../comments", it.ID, filename) // path, not filepath
	refs := append(append([]string{}, it.Refs...), ref)
	it.SetRefs(refs)
	if err := s.Save(it); err != nil {
		return "", err
	}
	return ref, nil
}

func (s *Store) AttachFile(it *Item, author string, now time.Time, src string) (string, error) {
	dir := s.CommentsDir(it.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ext := filepath.Ext(src)
	base := now.UTC().Format("20060102T150405Z") + "-" + SanitizeAuthor(author)
	filename, err := uniqueCommentFile(dir, base, ext)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	if err := config.WriteAtomic(filepath.Join(dir, filename), data); err != nil {
		return "", err
	}
	ref := path.Join("../comments", it.ID, filename)
	refs := append(append([]string{}, it.Refs...), ref)
	it.SetRefs(refs)
	if err := s.Save(it); err != nil {
		return "", err
	}
	return ref, nil
}
```

  `path.Join` (import `"path"`) always uses `/`. Never `filepath.Join` for the ref string. Author in the comment YAML is the **original** `author` argument, not the sanitised form. AttachFile copies `src` bytes verbatim (no frontmatter) and keeps `filepath.Ext(src)` including case.

- [ ] **Step 16: Run the whole package, pass, commit.**

```sh
go test ./pkg/item -count=1
```

  Expected: `ok  	github.com/eisenwinter/awit/pkg/item`. `gofmt -l pkg/item` prints nothing.

```sh
gofmt -w pkg/item/*.go
git add pkg/item
git commit -m "item: AddComment, AttachFile, comment collision suffix"
```

## Acceptance Criteria
- `go test ./pkg/item -count=1` passes (frontmatter tests from AWIT-0ND56D3G still green).
- `TestInitCreatesLayoutAndGitignore` — second `Init` is `ErrExists`; `.gitignore` contains exactly one `.awit/.lock` line.
- `TestInitAppendsToExistingGitignore` — pre-existing `dist/` without trailing newline becomes `dist/\n.awit/.lock\n`.
- `TestLoadAllClassifiesBroken` — one clean item, three broken with `CONFLICT MARKERS`, `PARSE ERROR`, `ID MISMATCH` sorted by ID.
- `TestLoadAllDuplicateCaseInsensitive` skips on a case-insensitive FS; otherwise both files `DUPLICATE ID`.
- `TestLoadBrokenError` — `errors.As(err, **BrokenError)`.
- `TestLoadMissing` — `errors.Is(err, os.ErrNotExist)`.
- `TestMintUniqueAndValid` — 50 unique `id.Valid` IDs, files not created.
- `TestAddCommentWritesFileAndRef` — ref contains `/` and no `\`; file body is author/created frontmatter + trimmed text.
- `TestAddCommentCollisionSuffix` — same `now` twice → `…-jan-2.md`.
- `TestAttachFilePreservesExtension` — dest name keeps `.PNG`, bytes identical to source.
- Save goes through `config.WriteAtomic`. No new third-party deps.

## Out of scope
- CLI `awit init` / `awit comment` / `awit create` (later tickets call this Store).
- `testdata/fixtures/**` — do not create them.
- `pkg/lock`, graph, formatters.
- Changing Parse/Bytes/setter contracts from AWIT-0ND56D3G.
