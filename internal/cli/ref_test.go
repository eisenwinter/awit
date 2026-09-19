package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/item"
	"github.com/eisenwinter/awit/pkg/resolver"
)

func writeRepoFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func dualBaseRepo(t *testing.T) string {
	t.Helper()
	dir := initRepo(t)
	writeRepoFile(t, filepath.Join(dir, "notes.txt"), "ROOT\n")
	writeRepoFile(t, filepath.Join(dir, ".awit", "items", "notes.txt"), "ITEMS\n")
	writeRepoFile(t, filepath.Join(dir, "docs", "notes", "x.md"), "DOC\n")
	raw := "---\nid: AWIT-TEST0001\ntitle: Dual\nbrief: Dual-base refs.\nstatus: open\ndeps: []\nlabels: []\nrefs:\n  - notes.txt\n---\n\n## Summary\n"
	writeRepoFile(t, filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md"), raw)
	return dir
}

func TestRefAddStoresRootRelativePath(t *testing.T) {
	dir := dualBaseRepo(t)
	code, stdout, stderr := run(t, "--repo", dir, "ref", "add", "AWIT-TEST0001", "docs/notes/x.md")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if stdout != "ref added AWIT-TEST0001: docs/notes/x.md\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	it := readItem(t, dir, "AWIT-TEST0001")
	if it.RefsBase != "repo" {
		t.Fatalf("RefsBase = %q", it.RefsBase)
	}
	if len(it.Refs) != 2 || it.Refs[1] != "docs/notes/x.md" {
		t.Fatalf("refs = %v, want last docs/notes/x.md (not ../../docs/...)", it.Refs)
	}
	if it.Refs[0] != ".awit/items/notes.txt" {
		t.Fatalf("legacy ref not migrated: %v", it.Refs)
	}
	got := resolver.Resolve(dir, it.Refs)
	if string(got[0].Content) != "ITEMS\n" {
		t.Fatalf("notes.md resolved to %q, want ITEMS", got[0].Content)
	}
	if string(got[1].Content) != "DOC\n" {
		t.Fatalf("docs/notes/x.md resolved to %q", got[1].Content)
	}
}

func TestRefAddFromNestedCwdResolvesSameTarget(t *testing.T) {
	dir := dualBaseRepo(t)
	nested := filepath.Join(dir, "pkg", "item")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(nested)
	code, stdout, stderr := run(t, "--repo", dir, "ref", "add", "AWIT-TEST0001", "docs/notes/x.md")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if stdout != "ref added AWIT-TEST0001: docs/notes/x.md\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	it := readItem(t, dir, "AWIT-TEST0001")
	got := resolver.Resolve(dir, []string{it.Refs[len(it.Refs)-1]})
	want, err := filepath.Abs(filepath.Join(dir, "docs", "notes", "x.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Path != want {
		t.Fatalf("nested cwd retargeted to %q, want %q", got[0].Path, want)
	}
}

func TestRefAddAlreadyPresent(t *testing.T) {
	dir := dualBaseRepo(t)
	code, _, stderr := run(t, "--repo", dir, "ref", "add", "AWIT-TEST0001", "docs/notes/x.md")
	if code != 0 || stderr != "" {
		t.Fatalf("first add: exit %d stderr %q", code, stderr)
	}
	code, stdout, stderr := run(t, "--repo", dir, "ref", "add", "AWIT-TEST0001", "./docs/notes/x.md")
	if code != 0 || stderr != "" {
		t.Fatalf("second add: exit %d stderr %q", code, stderr)
	}
	if stdout != "ref already present AWIT-TEST0001: docs/notes/x.md\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	it := readItem(t, dir, "AWIT-TEST0001")
	n := 0
	for _, r := range it.Refs {
		if r == "docs/notes/x.md" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("duplicate refs: %v", it.Refs)
	}
}

func TestRefAddMissingTargetAllowed(t *testing.T) {
	dir := dualBaseRepo(t)
	code, stdout, stderr := run(t, "--repo", dir, "ref", "add", "AWIT-TEST0001", "docs/planned.md")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "ref added AWIT-TEST0001: docs/planned.md\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	it := readItem(t, dir, "AWIT-TEST0001")
	got := resolver.Resolve(dir, []string{"docs/planned.md"})
	if got[0].Err == nil {
		t.Fatal("planned ref should be missing")
	}
	if it.Refs[len(it.Refs)-1] != "docs/planned.md" {
		t.Fatalf("refs = %v", it.Refs)
	}
}

func TestRefAddRejectsAbsoluteAndEmpty(t *testing.T) {
	dir := dualBaseRepo(t)
	before, err := os.ReadFile(filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md"))
	if err != nil {
		t.Fatal(err)
	}
	code, _, stderr := run(t, "--repo", dir, "ref", "add", "AWIT-TEST0001", filepath.Join(dir, "docs", "notes", "x.md"))
	if code != 1 || !strings.Contains(stderr, "Error:") {
		t.Fatalf("absolute: exit %d stderr %q", code, stderr)
	}
	code, _, stderr = run(t, "--repo", dir, "ref", "add", "AWIT-TEST0001", "")
	if code == 0 {
		t.Fatalf("empty path succeeded, stderr %q", stderr)
	}
	after, err := os.ReadFile(filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("rejected add wrote the item:\n%s", after)
	}
}

func TestRefRmRemovesOnlyReference(t *testing.T) {
	dir := dualBaseRepo(t)
	code, _, stderr := run(t, "--repo", dir, "ref", "add", "AWIT-TEST0001", "docs/notes/x.md")
	if code != 0 || stderr != "" {
		t.Fatalf("add: exit %d stderr %q", code, stderr)
	}
	code, stdout, stderr := run(t, "--repo", dir, "ref", "rm", "AWIT-TEST0001", "docs/notes/x.md")
	if code != 0 || stderr != "" {
		t.Fatalf("rm: exit %d stderr %q", code, stderr)
	}
	if stdout != "ref removed AWIT-TEST0001: docs/notes/x.md\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(dir, "docs", "notes", "x.md")); err != nil {
		t.Fatalf("target deleted: %v", err)
	}
	it := readItem(t, dir, "AWIT-TEST0001")
	for _, r := range it.Refs {
		if r == "docs/notes/x.md" {
			t.Fatalf("ref still present: %v", it.Refs)
		}
	}
}

func TestRefRmAbsentDoesNotSave(t *testing.T) {
	dir := dualBaseRepo(t)
	before, err := os.ReadFile(filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md"))
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "--repo", dir, "ref", "rm", "AWIT-TEST0001", "docs/notes/x.md")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "Error:") {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	after, err := os.ReadFile(filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("absent rm migrated or wrote:\n%s", after)
	}
}

func TestRefAddWalkedUpNote(t *testing.T) {
	dir := dualBaseRepo(t)
	chdirSub(t, dir)
	code, _, stderr := run(t, "ref", "add", "AWIT-TEST0001", "docs/notes/x.md")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if !strings.HasPrefix(stderr, walkNotePrefix) || !strings.Contains(stderr, walkNoteSuffix) {
		t.Fatalf("stderr = %q, want walked-up note", stderr)
	}
}

func TestShowFullLegacyByteIdentical(t *testing.T) {
	dir := dualBaseRepo(t)
	path := filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "--repo", dir, "show", "--full", "AWIT-TEST0001")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("show --full mutated legacy item")
	}
	if !strings.Contains(stdout, "===== REF 1/1: notes.txt =====\nITEMS\n===== END REF 1/1 =====\n") {
		t.Fatalf("legacy show resolved the wrong file:\n%s", stdout)
	}
}

func TestUpdateMigratesLegacyRefsToFormerTargets(t *testing.T) {
	dir := dualBaseRepo(t)
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--brief", "Updated summary.")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	it := readItem(t, dir, "AWIT-TEST0001")
	if it.RefsBase != "repo" {
		t.Fatalf("RefsBase = %q", it.RefsBase)
	}
	if len(it.Refs) != 1 || it.Refs[0] != ".awit/items/notes.txt" {
		t.Fatalf("refs = %v", it.Refs)
	}
	got := resolver.Resolve(dir, it.Refs)
	if string(got[0].Content) != "ITEMS\n" {
		t.Fatalf("migrated update resolved %q, want ITEMS", got[0].Content)
	}
	code, stdout, stderr := run(t, "--repo", dir, "show", "--full", "AWIT-TEST0001")
	if code != 0 || stderr != "" {
		t.Fatalf("show: exit %d stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "===== REF 1/1: .awit/items/notes.txt =====\nITEMS\n") {
		t.Fatalf("show after update:\n%s", stdout)
	}
}

func TestAttachThenArchiveRetainsBytesAndResolvability(t *testing.T) {
	dir := initRepo(t)
	raw := "---\nid: AWIT-TEST0004\ntitle: Done\nbrief: Closed work.\nstatus: closed\ndeps: []\nlabels: []\nrefs: []\n---\n\n## Summary\n"
	writeRepoFile(t, filepath.Join(dir, ".awit", "items", "AWIT-TEST0004.md"), raw)
	src := filepath.Join(t.TempDir(), "notes.log")
	writeRepoFile(t, src, "KEEPME\n")
	code, stdout, stderr := run(t, "--repo", dir, "comment", "--author", "jan", "--file", src, "AWIT-TEST0004")
	if code != 0 || stderr != "" {
		t.Fatalf("attach: exit %d stderr %q", code, stderr)
	}
	ref := strings.TrimSpace(stdout)
	if !strings.HasPrefix(ref, ".awit/comments/AWIT-TEST0004/") {
		t.Fatalf("attach ref = %q", ref)
	}
	code, _, stderr = run(t, "--repo", dir, "archive")
	if code != 0 || stderr != "" {
		t.Fatalf("archive: exit %d stderr %q", code, stderr)
	}
	arch, err := os.ReadFile(filepath.Join(dir, ".awit", "archive", "AWIT-TEST0004.md"))
	if err != nil {
		t.Fatal(err)
	}
	it, err := item.Parse("x.md", arch)
	if err != nil {
		t.Fatal(err)
	}
	if it.RefsBase != "repo" || len(it.Refs) != 1 || !strings.HasPrefix(it.Refs[0], ".awit/archive/AWIT-TEST0004/") {
		t.Fatalf("archived refs = %v base %q", it.Refs, it.RefsBase)
	}
	got := resolver.Resolve(dir, it.Refs)
	if got[0].Err != nil || string(got[0].Content) != "KEEPME\n" {
		t.Fatalf("archived attachment resolve = %+v", got[0])
	}
	code, out, _ := run(t, "--repo", dir, "validate")
	if code != 0 || !strings.HasPrefix(out, "PASS") {
		t.Fatalf("validate after archive: exit %d\n%s", code, out)
	}
}
