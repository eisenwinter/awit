package item

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLockIsGitignored(t *testing.T) {
	root := t.TempDir()
	s, err := Init(root, "AWIT")
	if err != nil {
		t.Fatal(err)
	}
	rel, err := s.Lock(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer rel()
	gi, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, line := range strings.Split(strings.TrimSuffix(string(gi), "\n"), "\n") {
		if line == ".awit/.lock" {
			found++
		}
	}
	if found != 1 {
		t.Fatalf(".gitignore .awit/.lock count = %d, want 1\n%s", found, gi)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, ".lock")); err != nil {
		t.Fatalf("lock file not created: %v", err)
	}
}

func TestStoreLockTimeoutMessage(t *testing.T) {
	root := t.TempDir()
	s, err := Init(root, "AWIT")
	if err != nil {
		t.Fatal(err)
	}
	rel, err := s.Lock(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer rel()
	_, err = s.Lock(50 * time.Millisecond)
	if err == nil {
		t.Fatal("second Lock: want timeout")
	}
	want := "another awit process holds .awit/.lock (waited 50ms)"
	if err.Error() != want {
		t.Fatalf("err = %q, want %q", err.Error(), want)
	}
}
