package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/item"
)

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
	if !strings.Contains(stdout, "===== REF 1/1: .awit/items/nope.md =====\n[missing]\n===== END REF 1/1 =====\n") {
		t.Fatalf("missing block wrong:\n%s", stdout)
	}
}

func TestShowFullItemRefNoRecursion(t *testing.T) {
	dir := copyFixture(t, "loop")
	// 0003 refs 0002 (an item); 0002 refs nothing. A recursive
	// renderer would also expand 0001's spec ref - assert it does not.
	it := readItem(t, dir, "AWIT-TEST0003")
	it.SetRefs([]string{"AWIT-TEST0002.md"})
	saveItem(t, dir, it)
	code, stdout, stderr := run(t, "--repo", dir, "show", "--full", "AWIT-TEST0003")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if n := strings.Count(stdout, "===== REF "); n != 1 {
		// One header for the single top-level ref (the END marker is
		// "===== END REF ...", which does not contain "===== REF ").
		t.Fatalf("want exactly one ref header, got %d:\n%s", n, stdout)
	}
	if n := strings.Count(stdout, "===== END REF "); n != 1 {
		t.Fatalf("want exactly one ref end marker, got %d:\n%s", n, stdout)
	}
	if !strings.Contains(stdout, "===== REF 1/1: .awit/items/AWIT-TEST0002.md =====\n[AWIT-TEST0002] Add E2E auth tests\n") {
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
