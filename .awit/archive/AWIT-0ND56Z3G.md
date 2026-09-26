---
id: AWIT-0ND56Z3G
title: pkg/resolver
brief: >-
  Add pkg/resolver with Resolve (map every frontmatter ref relative to .awit/items/, read it, never fail as a whole) and IsItemRef (detect a resolved path that names another item file). Pure stdlib, no CLI wiring.
status: closed
deps: []
labels: [phase4, p1]
refs_base: repo
refs:
  - plan/implementation-guide.md
  - plan/awit-implementation-plan.md
---

## Summary

After this ticket `pkg/resolver/resolver.go` exists with the `Resolved` struct, `Resolve(itemsDir, refs)` and `IsItemRef(itemsDir, path)`. `Resolve` turns each forward-slash ref from an item's frontmatter into an absolute OS path anchored at `.awit/items/`, reads the file, and returns one `Resolved` per ref **in input order**; a missing or unreadable file is recorded in `Resolved.Err` (with `Content == nil`) and never aborts the whole call. `IsItemRef` reports whether an absolute path is a `.md` file directly inside `itemsDir` and returns its stem (the item ID) so `awit show --full` can render another item's default view instead of dumping its raw bytes and can stop recursion after one level. No command is changed here; `AWIT-0ND5703G` wires the package into `show`.

## Context (read first)

- Guide §4.9 - the exact exported surface. Copy the `Resolved` struct and `Resolve` signature verbatim. `IsItemRef` is an addition made by this ticket; its signature is fixed below.
- Guide §1: module `github.com/eisenwinter/awit`; stdlib only in this package (no `pkg/item` import - the resolver must not know about frontmatter). Never hardcode `/` in filesystem paths: refs in frontmatter are forward slashes, so every ref goes through `filepath.FromSlash` before it touches `filepath.Join`. Tests build expected paths with `filepath.Join` so they pass on Windows.
- Spec `plan/awit-implementation-plan.md` §Paths and platforms + §Phase 4: "relative-path resolution from `.awit/items/`, slash normalisation, missing-file reporting". A ref like `../../docs/spec.md` in `.awit/items/AWIT-X.md` means `<repo>/docs/spec.md`; a ref like `../comments/AWIT-X/2026...-jan.md` means `<repo>/.awit/comments/AWIT-X/2026...-jan.md`.
- Order of operations inside `Resolve` for each ref (keep exactly this order so error paths are predictable): `p := filepath.Clean(filepath.Join(itemsDir, filepath.FromSlash(ref)))` → `abs, err := filepath.Abs(p)` → `os.ReadFile(abs)`. `Path` is always set to the best path known at the point of failure (`p` if `Abs` failed, `abs` otherwise) so `show --refs-only` can print `[missing]` next to a real path.
- `IsItemRef` semantics: both arguments are made absolute with `filepath.Abs`; `filepath.Rel(absItemsDir, absPath)` must succeed, must not start with `..`, must not contain a path separator (items live flat in `items/`, no subdirectories), and must end in `.md` with a non-empty stem. Returns `("", false)` in every other case. Do not check whether the file exists - the caller already has `Resolved.Err` for that.
- Guide §5: tests are `testing` stdlib, table-driven where sensible, `t.TempDir()` for the filesystem. This package has no fixtures; each test writes the files it needs.
- `t.TempDir()` on macOS returns a path under a symlinked `/var`. `filepath.Abs` does not resolve symlinks, and neither does the test, so comparing `Resolved.Path` against `filepath.Join(tmp, ...)` is stable on all three OSes.

## Files

- Create: `pkg/resolver/resolver.go`
- Create: `pkg/resolver/resolver_test.go`

## Interfaces

- Consumes: stdlib only (`os`, `path/filepath`, `strings`).
- Produces (verbatim from guide §4.9 plus `IsItemRef`, this ticket):

  ```go
  package resolver

  type Resolved struct {
      Ref     string // as written in frontmatter
      Path    string // absolute, OS separators
      Content []byte // nil when Err != nil
      Err     error  // os.ErrNotExist etc.
  }

  // Resolve maps every ref relative to itemsDir (FromSlash applied) and reads it. Never returns an error itself.
  func Resolve(itemsDir string, refs []string) []Resolved

  // IsItemRef reports whether path names a ".md" file directly inside itemsDir and returns its stem (the item ID).
  func IsItemRef(itemsDir, path string) (id string, ok bool)
  ```

- Not produced: any CLI command, any rendering, any recursion over refs of refs. Those are `AWIT-0ND5703G`.

## Steps

- [ ] **Step 1: Write the failing tests.**

  Create `pkg/resolver/resolver_test.go`:

  ```go
  package resolver

  import (
  	"errors"
  	"os"
  	"path/filepath"
  	"testing"
  )

  // layout creates <tmp>/.awit/items and returns (tmp, itemsDir).
  func layout(t *testing.T) (string, string) {
  	t.Helper()
  	tmp := t.TempDir()
  	itemsDir := filepath.Join(tmp, ".awit", "items")
  	if err := os.MkdirAll(itemsDir, 0o755); err != nil {
  		t.Fatal(err)
  	}
  	return tmp, itemsDir
  }

  func writeFile(t *testing.T, path, content string) {
  	t.Helper()
  	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
  		t.Fatal(err)
  	}
  	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
  		t.Fatal(err)
  	}
  }

  func TestResolveRelativeUp(t *testing.T) {
  	tmp, itemsDir := layout(t)
  	spec := filepath.Join(tmp, "docs", "spec.md")
  	writeFile(t, spec, "The Authorization header is Bearer.\n")

  	got := Resolve(itemsDir, []string{"../../docs/spec.md"})
  	if len(got) != 1 {
  		t.Fatalf("len = %d, want 1", len(got))
  	}
  	r := got[0]
  	if r.Ref != "../../docs/spec.md" {
  		t.Fatalf("Ref = %q, want ../../docs/spec.md (verbatim)", r.Ref)
  	}
  	if r.Err != nil {
  		t.Fatalf("Err = %v, want nil", r.Err)
  	}
  	if r.Path != spec {
  		t.Fatalf("Path = %q, want %q", r.Path, spec)
  	}
  	if !filepath.IsAbs(r.Path) {
  		t.Fatalf("Path %q is not absolute", r.Path)
  	}
  	if string(r.Content) != "The Authorization header is Bearer.\n" {
  		t.Fatalf("Content = %q", r.Content)
  	}
  }

  func TestResolveMissing(t *testing.T) {
  	tmp, itemsDir := layout(t)
  	got := Resolve(itemsDir, []string{"../../docs/nope.md"})
  	if len(got) != 1 {
  		t.Fatalf("len = %d, want 1", len(got))
  	}
  	r := got[0]
  	if !errors.Is(r.Err, os.ErrNotExist) {
  		t.Fatalf("Err = %v, want os.ErrNotExist", r.Err)
  	}
  	if r.Content != nil {
  		t.Fatalf("Content = %q, want nil on error", r.Content)
  	}
  	want := filepath.Join(tmp, "docs", "nope.md")
  	if r.Path != want {
  		t.Fatalf("Path = %q, want %q (path is reported even when missing)", r.Path, want)
  	}
  }

  func TestResolveKeepsOrder(t *testing.T) {
  	tmp, itemsDir := layout(t)
  	writeFile(t, filepath.Join(tmp, "docs", "a.md"), "A\n")
  	writeFile(t, filepath.Join(tmp, ".awit", "comments", "AWIT-TEST0001", "20260917T143205Z-jan.md"), "---\nauthor: jan\n---\n\nnote\n")
  	refs := []string{
  		"../../docs/a.md",
  		"../../docs/missing.md",
  		"../comments/AWIT-TEST0001/20260917T143205Z-jan.md",
  	}
  	got := Resolve(itemsDir, refs)
  	if len(got) != len(refs) {
  		t.Fatalf("len = %d, want %d", len(got), len(refs))
  	}
  	for i, r := range got {
  		if r.Ref != refs[i] {
  			t.Fatalf("got[%d].Ref = %q, want %q (order must match input)", i, r.Ref, refs[i])
  		}
  	}
  	if got[0].Err != nil || string(got[0].Content) != "A\n" {
  		t.Fatalf("got[0] = %+v", got[0])
  	}
  	if !errors.Is(got[1].Err, os.ErrNotExist) || got[1].Content != nil {
  		t.Fatalf("got[1] = %+v, want ErrNotExist and nil Content", got[1])
  	}
  	if got[2].Err != nil || string(got[2].Content) != "---\nauthor: jan\n---\n\nnote\n" {
  		t.Fatalf("got[2] = %+v", got[2])
  	}
  	wantComment := filepath.Join(tmp, ".awit", "comments", "AWIT-TEST0001", "20260917T143205Z-jan.md")
  	if got[2].Path != wantComment {
  		t.Fatalf("got[2].Path = %q, want %q", got[2].Path, wantComment)
  	}
  	if empty := Resolve(itemsDir, nil); len(empty) != 0 {
  		t.Fatalf("Resolve(nil) len = %d, want 0", len(empty))
  	}
  }

  func TestResolveForwardSlashOnAllOS(t *testing.T) {
  	tmp, itemsDir := layout(t)
  	deep := filepath.Join(tmp, "docs", "sub", "deep.md")
  	writeFile(t, deep, "deep\n")
  	sibling := filepath.Join(itemsDir, "AWIT-TEST0002.md")
  	writeFile(t, sibling, "---\nid: AWIT-TEST0002\n---\n")

  	cases := []struct{ ref, want string }{
  		{"../../docs/sub/deep.md", deep},
  		{"./AWIT-TEST0002.md", sibling},
  		{"AWIT-TEST0002.md", sibling},
  		{"../items/AWIT-TEST0002.md", sibling},
  	}
  	for _, tc := range cases {
  		got := Resolve(itemsDir, []string{tc.ref})
  		if got[0].Err != nil {
  			t.Fatalf("%s: Err = %v", tc.ref, got[0].Err)
  		}
  		if got[0].Path != tc.want {
  			t.Fatalf("%s: Path = %q, want %q", tc.ref, got[0].Path, tc.want)
  		}
  		if got[0].Ref != tc.ref {
  			t.Fatalf("%s: Ref rewritten to %q", tc.ref, got[0].Ref)
  		}
  	}
  }

  func TestIsItemRef(t *testing.T) {
  	tmp, itemsDir := layout(t)
  	cases := []struct {
  		name   string
  		path   string
  		wantID string
  		wantOK bool
  	}{
  		{"item file", filepath.Join(itemsDir, "AWIT-TEST0002.md"), "AWIT-TEST0002", true},
  		{"item via dotdot", filepath.Join(itemsDir, "..", "items", "AWIT-TEST0003.md"), "AWIT-TEST0003", true},
  		{"doc outside items", filepath.Join(tmp, "docs", "spec.md"), "", false},
  		{"comment file", filepath.Join(tmp, ".awit", "comments", "AWIT-TEST0001", "20260917T143205Z-jan.md"), "", false},
  		{"subdirectory of items", filepath.Join(itemsDir, "sub", "AWIT-TEST0004.md"), "", false},
  		{"non-md in items", filepath.Join(itemsDir, "notes.txt"), "", false},
  		{"bare extension", filepath.Join(itemsDir, ".md"), "", false},
  		{"items dir itself", itemsDir, "", false},
  	}
  	for _, tc := range cases {
  		t.Run(tc.name, func(t *testing.T) {
  			id, ok := IsItemRef(itemsDir, tc.path)
  			if ok != tc.wantOK || id != tc.wantID {
  				t.Fatalf("IsItemRef(%q) = (%q, %v), want (%q, %v)", tc.path, id, ok, tc.wantID, tc.wantOK)
  			}
  		})
  	}
  }
  ```

- [ ] **Step 2: Run it, see it fail.**

  ```bash
  go test ./pkg/resolver -run 'TestResolve|TestIsItemRef' -v
  ```

  Expected failure (no non-test file in the package yet):

  ```text
  # github.com/eisenwinter/awit/pkg/resolver [github.com/eisenwinter/awit/pkg/resolver.test]
  pkg/resolver/resolver_test.go: undefined: Resolve
  pkg/resolver/resolver_test.go: undefined: IsItemRef
  FAIL	github.com/eisenwinter/awit/pkg/resolver [build failed]
  ```

  (Go may print `no non-test Go files` instead; either is the expected red.) Do not skip this step.

- [ ] **Step 3: Implement `resolver.go`.**

  Create `pkg/resolver/resolver.go`:

  ```go
  // Package resolver maps the forward-slash refs stored in item frontmatter to
  // files on disk. Refs are relative to the .awit/items/ directory of the repo
  // that owns the item, so "../../docs/spec.md" is <repo>/docs/spec.md and
  // "../comments/<id>/<file>" is <repo>/.awit/comments/<id>/<file>.
  package resolver

  import (
  	"os"
  	"path/filepath"
  	"strings"
  )

  // Resolved is the outcome of resolving one ref. Content is nil whenever Err
  // is non-nil; Path is always populated so callers can print where they looked.
  type Resolved struct {
  	Ref     string // as written in frontmatter
  	Path    string // absolute, OS separators
  	Content []byte // nil when Err != nil
  	Err     error  // os.ErrNotExist etc.
  }

  // Resolve maps every ref relative to itemsDir (FromSlash applied) and reads
  // it. The result has exactly one element per ref, in input order. Resolve
  // never returns an error itself; per-ref failures live in Resolved.Err.
  func Resolve(itemsDir string, refs []string) []Resolved {
  	out := make([]Resolved, 0, len(refs))
  	for _, ref := range refs {
  		p := filepath.Clean(filepath.Join(itemsDir, filepath.FromSlash(ref)))
  		abs, err := filepath.Abs(p)
  		if err != nil {
  			out = append(out, Resolved{Ref: ref, Path: p, Err: err})
  			continue
  		}
  		data, err := os.ReadFile(abs)
  		if err != nil {
  			out = append(out, Resolved{Ref: ref, Path: abs, Err: err})
  			continue
  		}
  		out = append(out, Resolved{Ref: ref, Path: abs, Content: data})
  	}
  	return out
  }

  // IsItemRef reports whether path names a ".md" file directly inside itemsDir
  // and returns its stem, which is the item ID. It does not touch the disk:
  // whether the file exists is the caller's concern (see Resolved.Err).
  func IsItemRef(itemsDir, path string) (id string, ok bool) {
  	absDir, err := filepath.Abs(itemsDir)
  	if err != nil {
  		return "", false
  	}
  	absPath, err := filepath.Abs(path)
  	if err != nil {
  		return "", false
  	}
  	rel, err := filepath.Rel(absDir, absPath)
  	if err != nil {
  		return "", false
  	}
  	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
  		return "", false
  	}
  	if strings.ContainsRune(rel, filepath.Separator) {
  		return "", false
  	}
  	stem, isMD := strings.CutSuffix(rel, ".md")
  	if !isMD || stem == "" {
  		return "", false
  	}
  	return stem, true
  }
  ```

  Implementation rules:
  - Keep the three-step order `Clean(Join(FromSlash))` → `Abs` → `ReadFile`. `filepath.Join` already cleans, but the explicit `Clean` documents the intent and costs nothing.
  - Never wrap the `os.ReadFile` error: callers use `errors.Is(r.Err, os.ErrNotExist)` and `*os.PathError` already carries the path.
  - `Resolve(itemsDir, nil)` returns a non-nil empty slice (`make(..., 0, 0)`), so JSON encoders print `[]` rather than `null`.
  - `IsItemRef` uses `filepath.Rel`, not string prefix matching, so `itemsDir/../items/X.md` is recognised and `itemsDir-other/X.md` is rejected.
  - Extension match is case-sensitive (`.md` only). Item files are always written as `<ID>.md` by `Store.Save`, so `.MD` is not an item.

- [ ] **Step 4: Run the tests, see them pass, commit.**

  ```bash
  go test ./pkg/resolver -run 'TestResolve|TestIsItemRef' -v
  ```

  Expected:

  ```text
  === RUN   TestResolveRelativeUp
  --- PASS: TestResolveRelativeUp
  === RUN   TestResolveMissing
  --- PASS: TestResolveMissing
  === RUN   TestResolveKeepsOrder
  --- PASS: TestResolveKeepsOrder
  === RUN   TestResolveForwardSlashOnAllOS
  --- PASS: TestResolveForwardSlashOnAllOS
  === RUN   TestIsItemRef
  --- PASS: TestIsItemRef
      --- PASS: TestIsItemRef/item_file
      --- PASS: TestIsItemRef/item_via_dotdot
      --- PASS: TestIsItemRef/doc_outside_items
      --- PASS: TestIsItemRef/comment_file
      --- PASS: TestIsItemRef/subdirectory_of_items
      --- PASS: TestIsItemRef/non-md_in_items
      --- PASS: TestIsItemRef/bare_extension
      --- PASS: TestIsItemRef/items_dir_itself
  PASS
  ok  	github.com/eisenwinter/awit/pkg/resolver
  ```

  Then:

  ```bash
  go vet ./pkg/resolver
  gofmt -l pkg/resolver
  git add pkg/resolver/resolver.go pkg/resolver/resolver_test.go
  git commit -m "resolver: resolve item refs relative to .awit/items"
  ```

  `gofmt -l pkg/resolver` must print nothing.

- [ ] **Step 5: Close ticket.**
  1. Run `go build ./... && go vet ./pkg/resolver && go test ./pkg/resolver -count=1` and copy the output.
  2. Create `.awit/comments/AWIT-0ND56Z3G/<YYYYMMDDTHHMMSSZ>-<author>.md` (UTC stamp, author sanitised to `[a-z0-9._-]`, e.g. `20260918T101500Z-claude.md`) with this shape:

     ```markdown
     ---
     author: <author>
     created: <YYYY-MM-DDTHH:MM:SSZ>
     ---

     Closed AWIT-0ND56Z3G. Acceptance output:

     <paste the go test -v output from step 1>
     ```

  3. In `.awit/items/AWIT-0ND56Z3G.md` append `  - ../comments/AWIT-0ND56Z3G/<that filename>` to the `refs:` block and change `status: open` to `status: closed`. Touch nothing else in the frontmatter.
  4. Commit:

     ```bash
     git add .awit/items/AWIT-0ND56Z3G.md .awit/comments/AWIT-0ND56Z3G/
     git commit -m "tickets: close AWIT-0ND56Z3G"
     ```

## Acceptance Criteria

- `go test ./pkg/resolver -count=1` passes; `go vet ./pkg/resolver` and `gofmt -l pkg/resolver` are silent.
- `Resolve(itemsDir, []string{"../../docs/spec.md"})` where `itemsDir = <tmp>/.awit/items` returns one element with `Path == filepath.Join(tmp, "docs", "spec.md")`, `Err == nil`, `Content` equal to the file bytes.
- A missing ref yields `errors.Is(r.Err, os.ErrNotExist) == true`, `Content == nil`, and `Path` set to the absolute path that was tried.
- `Resolve` returns exactly `len(refs)` results with `got[i].Ref == refs[i]`; a failure in the middle does not stop later refs.
- Refs are written with `/` and resolve to OS-native paths on Windows (test compares against `filepath.Join`).
- `IsItemRef(itemsDir, filepath.Join(itemsDir, "AWIT-TEST0002.md"))` is `("AWIT-TEST0002", true)`; paths outside `itemsDir`, in subdirectories, without `.md`, or with an empty stem return `("", false)`.
- `pkg/resolver` imports only `os`, `path/filepath`, `strings`.

## Out of scope

- Wiring into `awit show` (`--full`, `--refs-only`), delimiter rendering, one-level item-ref expansion - `AWIT-0ND5703G`.
- Following refs of refs, cycle detection between items' refs, caching, or size limits.
- Validating that refs stay inside the repository root (a ref may legitimately point anywhere on disk).
- Changing `pkg/item` or how refs are written (`Store.AddComment` / `AttachFile` already produce forward-slash refs).
