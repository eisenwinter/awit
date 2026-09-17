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
