package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/eisenwinter/awit/pkg/item"
)

func TestCreateLockTimeout(t *testing.T) {
	repo := initRepo(t)
	s, err := item.Open(repo)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := s.Lock(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer rel()
	code, stdout, stderr := run(t, "--repo", repo, "create", "--brief", "blocked by lock.", "Blocked")
	if code != 1 {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	want := "Error: another awit process holds .awit/.lock (waited 5s)\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

func TestConcurrentCreates(t *testing.T) {
	repo := initRepo(t)
	const n = 10
	type result struct {
		i      int
		code   int
		stderr string
	}
	ch := make(chan result, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		i := i
		go func() {
			defer wg.Done()
			code, _, stderr := run(t, "--repo", repo, "create", "--brief", "concurrent item.", fmt.Sprintf("Item %d", i))
			ch <- result{i: i, code: code, stderr: stderr}
		}()
	}
	wg.Wait()
	close(ch)
	for r := range ch {
		if r.code != 0 {
			t.Errorf("create %d: exit %d stderr %q", r.i, r.code, r.stderr)
		}
	}
	entries, err := os.ReadDir(filepath.Join(repo, item.DirName, "items"))
	if err != nil {
		t.Fatal(err)
	}
	var md []string
	seen := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) != ".md" {
			continue
		}
		if seen[name] {
			t.Errorf("duplicate filename %s", name)
		}
		seen[name] = true
		md = append(md, name)
	}
	if len(md) != n {
		t.Fatalf("item files = %d, want %d (%v)", len(md), n, md)
	}
}
