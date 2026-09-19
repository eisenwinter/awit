package item

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/eisenwinter/awit/pkg/resolver"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func absPath(t *testing.T, elem ...string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join(elem...))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestParseRefsBaseRepo(t *testing.T) {
	raw := "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nrefs_base: repo\nrefs:\n  - docs/a.md\n---\n"
	it, err := Parse("x.md", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if it.RefsBase != "repo" {
		t.Fatalf("RefsBase = %q, want repo", it.RefsBase)
	}
	if !slices.Equal(it.Refs, []string{"docs/a.md"}) {
		t.Fatalf("Refs = %v", it.Refs)
	}
}

func TestParseRefsBaseAbsentIsLegacy(t *testing.T) {
	raw := "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nrefs:\n  - notes.md\n---\n"
	it, err := Parse("x.md", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if it.RefsBase != "" {
		t.Fatalf("RefsBase = %q, want empty (historical items base)", it.RefsBase)
	}
}

func TestParseRefsBaseInvalidValue(t *testing.T) {
	_, err := Parse("x.md", []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nrefs_base: items\n---\n"))
	if err == nil || !strings.Contains(err.Error(), "refs_base") {
		t.Fatalf("err = %v, want parse error naming refs_base", err)
	}
}

func TestParseRefsBaseInvalidType(t *testing.T) {
	_, err := Parse("x.md", []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nrefs_base: [repo]\n---\n"))
	if err == nil || !strings.Contains(err.Error(), "refs_base") {
		t.Fatalf("err = %v, want parse error naming refs_base", err)
	}
}

func TestSetRefsBaseRepoInsertsBeforeRefs(t *testing.T) {
	it := New("AWIT-TEST0001", "T", "B.", nil, nil)
	if it.RefsBase != "repo" {
		t.Fatalf("New RefsBase = %q, want repo", it.RefsBase)
	}
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("refs_base: repo\nrefs:")) {
		t.Fatalf("refs_base must sit immediately before refs:\n%s", got)
	}
}

func TestSetRefsBaseEmptyDeletesKey(t *testing.T) {
	it := New("AWIT-TEST0001", "T", "B.", nil, nil)
	if err := it.SetRefsBase(""); err != nil {
		t.Fatal(err)
	}
	if it.RefsBase != "" {
		t.Fatalf("RefsBase = %q", it.RefsBase)
	}
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("refs_base:")) {
		t.Fatalf("refs_base key still present:\n%s", got)
	}
}

func TestSetRefsBaseRejectsOtherValues(t *testing.T) {
	it := New("AWIT-TEST0001", "T", "B.", nil, nil)
	if err := it.SetRefsBase("items"); err == nil {
		t.Fatal("want error")
	}
}

func TestNormalizeRefsAmbiguousLexicalPath(t *testing.T) {
	s := initStore(t)
	itemsNotes := filepath.Join(s.ItemsDir(), "notes.md")
	rootNotes := filepath.Join(s.Root, "notes.md")
	writeFile(t, itemsNotes, "ITEMS\n")
	writeFile(t, rootNotes, "ROOT\n")
	writeItemFile(t, s, "AWIT-TEST0001.md", "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nrefs:\n  - notes.md\n---\n")
	it, err := s.Load("AWIT-TEST0001")
	if err != nil {
		t.Fatal(err)
	}
	before := resolver.Resolve(s.ItemsDir(), it.Refs)
	if len(before) != 1 || before[0].Err != nil || string(before[0].Content) != "ITEMS\n" {
		t.Fatalf("legacy resolve = %+v, want ITEMS content", before)
	}
	wantPath := absPath(t, itemsNotes)
	if before[0].Path != wantPath {
		t.Fatalf("legacy path = %q, want %q", before[0].Path, wantPath)
	}

	if err := s.NormalizeRefs(it); err != nil {
		t.Fatal(err)
	}
	if it.RefsBase != "repo" {
		t.Fatalf("RefsBase = %q, want repo", it.RefsBase)
	}
	if !slices.Equal(it.Refs, []string{".awit/items/notes.md"}) {
		t.Fatalf("refs = %v, want [.awit/items/notes.md]", it.Refs)
	}
	after := resolver.Resolve(s.Root, it.Refs)
	if len(after) != 1 || after[0].Err != nil {
		t.Fatalf("migrated resolve = %+v", after)
	}
	if after[0].Path != before[0].Path {
		t.Fatalf("resolved target changed: %q -> %q", before[0].Path, after[0].Path)
	}
	if string(after[0].Content) != "ITEMS\n" {
		t.Fatalf("migrated content = %q, want ITEMS (not ROOT)", after[0].Content)
	}
}

func TestNormalizeRefsMissingStillRewrites(t *testing.T) {
	s := initStore(t)
	writeItemFile(t, s, "AWIT-TEST0001.md", "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nrefs:\n  - gone.md\n---\n")
	it, err := s.Load("AWIT-TEST0001")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.NormalizeRefs(it); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(it.Refs, []string{".awit/items/gone.md"}) {
		t.Fatalf("refs = %v", it.Refs)
	}
	got := resolver.Resolve(s.Root, it.Refs)
	if len(got) != 1 || got[0].Err == nil {
		t.Fatalf("want missing, got %+v", got)
	}
	want := absPath(t, s.ItemsDir(), "gone.md")
	if got[0].Path != want {
		t.Fatalf("missing path = %q, want %q", got[0].Path, want)
	}
}

func TestNormalizeRefsDirectItemFile(t *testing.T) {
	s := initStore(t)
	writeItemFile(t, s, "AWIT-TEST0001.md", "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nrefs:\n  - AWIT-TEST0002.md\n---\n")
	writeItemFile(t, s, "AWIT-TEST0002.md", "---\nid: AWIT-TEST0002\ntitle: Other\nstatus: open\n---\nbody-two\n")
	it, err := s.Load("AWIT-TEST0001")
	if err != nil {
		t.Fatal(err)
	}
	before := resolver.Resolve(s.ItemsDir(), it.Refs)
	if err := s.NormalizeRefs(it); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(it.Refs, []string{".awit/items/AWIT-TEST0002.md"}) {
		t.Fatalf("refs = %v", it.Refs)
	}
	after := resolver.Resolve(s.Root, it.Refs)
	if after[0].Path != before[0].Path {
		t.Fatalf("item-file target changed: %q -> %q", before[0].Path, after[0].Path)
	}
	if !bytes.Contains(after[0].Content, []byte("body-two")) {
		t.Fatalf("content = %q", after[0].Content)
	}
}

func TestNormalizeRefsOutsideRoot(t *testing.T) {
	s := initStore(t)
	writeItemFile(t, s, "AWIT-TEST0001.md", "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nrefs:\n  - ../../../outside.md\n---\n")
	it, err := s.Load("AWIT-TEST0001")
	if err != nil {
		t.Fatal(err)
	}
	before := filepath.Clean(filepath.Join(s.ItemsDir(), filepath.FromSlash("../../../outside.md")))
	if err := s.NormalizeRefs(it); err != nil {
		t.Fatal(err)
	}
	if len(it.Refs) != 1 || !strings.HasPrefix(it.Refs[0], "../") {
		t.Fatalf("refs = %v, want a ../ outside-root path", it.Refs)
	}
	got := filepath.Clean(filepath.Join(s.Root, filepath.FromSlash(it.Refs[0])))
	if got != before {
		t.Fatalf("outside target changed: %q -> %q (via %q)", before, got, it.Refs[0])
	}
}

func TestNormalizeRefsAlreadyRepoIsNoOp(t *testing.T) {
	s := initStore(t)
	writeItemFile(t, s, "AWIT-TEST0001.md", "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nrefs_base: repo\nrefs:\n  - docs/a.md\n---\n")
	it, err := s.Load("AWIT-TEST0001")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.NormalizeRefs(it); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(it.Refs, []string{"docs/a.md"}) {
		t.Fatalf("already-repo refs rewritten: %v", it.Refs)
	}
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nrefs_base: repo\nrefs:\n  - docs/a.md\n---\n")) {
		t.Fatalf("already-repo NormalizeRefs must not dirty:\n%s", got)
	}
}

func TestSaveMigratesAtomically(t *testing.T) {
	s := initStore(t)
	writeFile(t, filepath.Join(s.ItemsDir(), "notes.md"), "ITEMS\n")
	writeFile(t, filepath.Join(s.Root, "notes.md"), "ROOT\n")
	orig := []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nrefs:\n  - notes.md\n---\n")
	writeItemFile(t, s, "AWIT-TEST0001.md", string(orig))
	it, err := s.Load("AWIT-TEST0001")
	if err != nil {
		t.Fatal(err)
	}
	it.SetTitle("T2")
	if err := s.Save(it); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.ItemPath("AWIT-TEST0001"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("refs_base:")) != bytes.Contains(data, []byte(".awit/items/notes.md")) {
		t.Fatalf("partial migration (marker xor rewritten refs):\n%s", data)
	}
	if !bytes.Contains(data, []byte("refs_base: repo\n")) || !bytes.Contains(data, []byte("- .awit/items/notes.md\n")) {
		t.Fatalf("expected marker and rewritten ref together:\n%s", data)
	}
	if bytes.Contains(data, []byte("- notes.md\n")) {
		t.Fatalf("legacy ref survived:\n%s", data)
	}
	got := resolver.Resolve(s.Root, it.Refs)
	if string(got[0].Content) != "ITEMS\n" {
		t.Fatalf("saved item resolves to %q, want ITEMS", got[0].Content)
	}
}

func TestSaveLeavesFileWhenNormalizeFails(t *testing.T) {
	s := initStore(t)
	orig := []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nrefs:\n  - notes.md\n---\n")
	writeItemFile(t, s, "AWIT-TEST0001.md", string(orig))
	it, err := s.Load("AWIT-TEST0001")
	if err != nil {
		t.Fatal(err)
	}
	it.SetTitle("T2")
	s.Root = "relative-root"
	if err := s.Save(it); err == nil {
		t.Fatal("want NormalizeRefs error before write")
	}
	got, err := os.ReadFile(s.ItemPath("AWIT-TEST0001"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, orig) {
		t.Fatalf("partial write on normalize failure\ngot:\n%s", got)
	}
}

func TestAddCommentNormalizesBeforeAppend(t *testing.T) {
	s := initStore(t)
	writeFile(t, filepath.Join(s.ItemsDir(), "notes.md"), "ITEMS\n")
	writeFile(t, filepath.Join(s.Root, "notes.md"), "ROOT\n")
	writeItemFile(t, s, "AWIT-TEST0001.md", "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nrefs:\n  - notes.md\n---\n")
	it, err := s.Load("AWIT-TEST0001")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 17, 14, 32, 5, 0, time.UTC)
	ref, err := s.AddComment(it, "jan", now, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if ref != ".awit/comments/AWIT-TEST0001/20260917T143205Z-jan.md" {
		t.Fatalf("ref = %q", ref)
	}
	if !slices.Equal(it.Refs, []string{".awit/items/notes.md", ref}) {
		t.Fatalf("mixed-base list: %v", it.Refs)
	}
	if it.RefsBase != "repo" {
		t.Fatalf("RefsBase = %q", it.RefsBase)
	}
	got := resolver.Resolve(s.Root, it.Refs)
	if string(got[0].Content) != "ITEMS\n" {
		t.Fatalf("old ref retargeted: %q", got[0].Content)
	}
	if got[1].Err != nil || !bytes.Contains(got[1].Content, []byte("hello")) {
		t.Fatalf("comment resolve = %+v", got[1])
	}
}

func TestAttachFileNormalizesBeforeAppend(t *testing.T) {
	s := initStore(t)
	writeItemFile(t, s, "AWIT-TEST0001.md", "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nrefs:\n  - ../../docs/a.md\n---\n")
	src := filepath.Join(t.TempDir(), "shot.log")
	writeFile(t, src, "LOG\n")
	it, err := s.Load("AWIT-TEST0001")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 17, 14, 32, 5, 0, time.UTC)
	ref, err := s.AttachFile(it, "jan", now, src)
	if err != nil {
		t.Fatal(err)
	}
	if ref != ".awit/comments/AWIT-TEST0001/20260917T143205Z-jan.log" {
		t.Fatalf("ref = %q", ref)
	}
	if !slices.Equal(it.Refs, []string{"docs/a.md", ref}) {
		t.Fatalf("refs = %v", it.Refs)
	}
}

func TestArchiveAfterMigrationRewritesAttachmentRef(t *testing.T) {
	s := archiveStore(t)
	it, err := s.Load("AWIT-TEST0004")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Archive(it); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.ArchivePath("AWIT-TEST0004"))
	if err != nil {
		t.Fatal(err)
	}
	arch, err := Parse(s.ArchivePath("AWIT-TEST0004"), data)
	if err != nil {
		t.Fatal(err)
	}
	if arch.RefsBase != "repo" {
		t.Fatalf("RefsBase = %q", arch.RefsBase)
	}
	want := []string{".awit/archive/AWIT-TEST0004/20260916T091500Z-jan.log"}
	if !slices.Equal(arch.Refs, want) {
		t.Fatalf("refs = %v, want %v", arch.Refs, want)
	}
	attach := filepath.Join(s.ArchiveDir(), "AWIT-TEST0004", "20260916T091500Z-jan.log")
	got := resolver.Resolve(s.Root, arch.Refs)
	if got[0].Err != nil || got[0].Path != absPath(t, attach) {
		t.Fatalf("attachment resolve = %+v, want %s", got[0], attach)
	}
	body, err := os.ReadFile(attach)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("PASS TestHeaderGrammar")) {
		t.Fatalf("attachment bytes lost: %s", body)
	}
}
