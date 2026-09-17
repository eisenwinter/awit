package format

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

// goldenPath resolves testdata/golden next to this package.
//
// NOTE: this deviates from ticket AWIT-0ND56F3G Step 8, whose verbatim helper
// resolves ../../testdata/golden at the repo root. Per orchestrator scope
// direction the five golden files live in-package at
// pkg/format/testdata/golden/ to avoid concurrent edits of the shared
// repo-root directory; the behaviour (flag-driven rewrite, bytes.Equal
// comparison, -update hint) is otherwise identical.
func goldenPath(name string) string {
	return filepath.Join("testdata", "golden", name)
}

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := goldenPath(name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run: go test ./... -update)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("golden %s mismatch\ngot:\n%s\nwant:\n%s\nrun: go test ./... -update", name, got, want)
	}
}
