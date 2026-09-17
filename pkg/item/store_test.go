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
