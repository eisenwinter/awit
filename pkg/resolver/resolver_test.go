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

func TestResolveRepoRootBase(t *testing.T) {
	tmp, itemsDir := layout(t)
	itemsNotes := filepath.Join(itemsDir, "notes.md")
	rootNotes := filepath.Join(tmp, "notes.md")
	writeFile(t, itemsNotes, "ITEMS\n")
	writeFile(t, rootNotes, "ROOT\n")

	legacy := Resolve(itemsDir, []string{"notes.md"})
	if string(legacy[0].Content) != "ITEMS\n" {
		t.Fatalf("items-base notes.md = %q, want ITEMS", legacy[0].Content)
	}
	migrated := Resolve(tmp, []string{".awit/items/notes.md"})
	if migrated[0].Err != nil {
		t.Fatalf("repo-base resolve: %v", migrated[0].Err)
	}
	if migrated[0].Path != legacy[0].Path {
		t.Fatalf("target changed: %q -> %q", legacy[0].Path, migrated[0].Path)
	}
	if string(migrated[0].Content) != "ITEMS\n" {
		t.Fatalf("repo-base content = %q, want ITEMS not ROOT", migrated[0].Content)
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
