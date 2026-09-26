package ops_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/internal/cli"
	"github.com/eisenwinter/awit/pkg/item"
)

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = cli.Main(args, strings.NewReader(""), &out, &errb)
	return code, out.String(), errb.String()
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func copyFixture(t *testing.T, name string) string {
	t.Helper()
	dst := t.TempDir()
	src := filepath.Join(repoRoot(t), "testdata", "fixtures", name)
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatalf("copyFixture %s: %v", name, err)
	}
	return dst
}

func readItem(t *testing.T, repo, id string) *item.Item {
	t.Helper()
	path := filepath.Join(repo, item.DirName, "items", id+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	it, err := item.Parse(path, data)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return it
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	code, _, stderr := run(t, "--repo", dir, "init")
	if code != 0 {
		t.Fatalf("init: exit %d stderr %q", code, stderr)
	}
	return dir
}

func openTestStore(t *testing.T, dir string) *item.Store {
	t.Helper()
	s, err := item.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
