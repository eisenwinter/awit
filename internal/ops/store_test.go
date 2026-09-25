package ops_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/eisenwinter/awit/internal/ops"
	"github.com/eisenwinter/awit/pkg/item"
)

func TestOpenPrecedence(t *testing.T) {
	root := copyFixture(t, "clean")
	other := copyFixture(t, "archive")
	t.Setenv("AWIT_REPO", other)
	s, err := ops.Open(root)
	if err != nil || s.Root != root {
		t.Fatalf("repo arg must win: %v %v", s, err)
	}
	s, err = ops.Open("")
	if err != nil || s.Root != other {
		t.Fatalf("AWIT_REPO must beat cwd: %v %v", s, err)
	}
	t.Setenv("AWIT_REPO", "")
	sub := filepath.Join(root, "sub")
	os.MkdirAll(sub, 0o755)
	t.Chdir(sub)
	s, err = ops.Open("")
	if err != nil || s.Root != root {
		t.Fatalf("walk-up: %v %v", s, err)
	}
}

func TestOpenWithoutRepo(t *testing.T) {
	t.Setenv("AWIT_REPO", "")
	t.Chdir(t.TempDir())
	if _, err := ops.Open(""); !errors.Is(err, item.ErrNotFound) {
		t.Fatalf("err = %v, want item.ErrNotFound", err)
	}
}

func TestWalkedUpNote(t *testing.T) {
	root := copyFixture(t, "clean")
	s := openTestStore(t, root)
	sub := filepath.Join(root, "sub")
	os.MkdirAll(sub, 0o755)
	t.Setenv("AWIT_REPO", "")
	t.Chdir(sub)
	want := "Note: no .awit in the current directory; using " + root + ". Run awit init here, or pass --repo / set AWIT_REPO."
	if got := ops.WalkedUpNote("", s); got != want {
		t.Fatalf("note = %q, want %q", got, want)
	}
	if got := ops.WalkedUpNote(root, s); got != "" {
		t.Fatalf("repo given: %q", got)
	}
	t.Setenv("AWIT_REPO", root)
	if got := ops.WalkedUpNote("", s); got != "" {
		t.Fatalf("AWIT_REPO set: %q", got)
	}
	t.Setenv("AWIT_REPO", "")
	t.Chdir(root)
	if got := ops.WalkedUpNote("", s); got != "" {
		t.Fatalf("cwd has .awit: %q", got)
	}
}
