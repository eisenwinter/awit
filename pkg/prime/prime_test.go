package prime

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

var update = flag.Bool("update", false, "rewrite golden files")

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func load(t *testing.T, name string) *graph.Graph {
	t.Helper()
	st, err := item.Open(filepath.Join(repoRoot(t), "testdata", "fixtures", name))
	if err != nil {
		t.Fatal(err)
	}
	items, broken, err := st.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	return graph.Build(items, broken)
}

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join(repoRoot(t), "testdata", "golden", name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s\nhint: go test ./pkg/prime -update", got, want)
	}
}

func render(t *testing.T, g *graph.Graph, opts Options) string {
	t.Helper()
	var b bytes.Buffer
	if err := Render(&b, g, opts); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestPrimeDeterministic(t *testing.T) {
	a := render(t, load(t, "clean"), Options{})
	b := render(t, load(t, "clean"), Options{})
	if a != b {
		t.Fatalf("two renders differ:\n%s\n---\n%s", a, b)
	}
}

func TestPrimeGoldenClean(t *testing.T) {
	out := render(t, load(t, "clean"), Options{})
	if strings.Contains(out, "=== GRAPH WARNINGS ===") {
		t.Fatal("clean output must omit the warnings section")
	}
	golden(t, "prime-clean.golden", []byte(out))
}

func TestPrimeGoldenCyclic(t *testing.T) {
	out := render(t, load(t, "cyclic"), Options{})
	lines := strings.Split(out, "\n")
	if lines[0] != "=== GRAPH WARNINGS ===" {
		t.Fatalf("first line = %q", lines[0])
	}
	var warns []string
	for _, l := range lines[1:] {
		if l == "" {
			break
		}
		warns = append(warns, l)
	}
	if len(warns) != 2 {
		t.Fatalf("warnings = %v, want 2 lines", warns)
	}
	for _, w := range warns {
		if !strings.HasPrefix(w, "[CYCLE] ") || !strings.HasSuffix(w, " (excluded from next)") {
			t.Fatalf("warning = %q", w)
		}
	}
	if !strings.Contains(out, "=== READY (1) ===\n[AWIT-TEST0005] open Update the changelog | - | Unblocks: 0\n") {
		t.Fatalf("ready section missing 0005:\n%s", out)
	}
	if !strings.Contains(out, "=== BLOCKED (0) ===\n\n=== CRITICAL PATH (1) ===\nAWIT-TEST0005\n") {
		t.Fatalf("blocked/critical sections wrong:\n%s", out)
	}
	golden(t, "prime-cyclic.golden", []byte(out))
}

func TestPrimeMaxTokens(t *testing.T) {
	limited := render(t, load(t, "clean"), Options{MaxTokens: 40})
	if !strings.Contains(limited, "AWIT-TEST0001 -> AWIT-TEST0003 -> AWIT-TEST0004\n") {
		t.Fatalf("critical path must survive truncation:\n%s", limited)
	}
	if !strings.Contains(limited, "=== READY (3) ===") {
		t.Fatalf("header counts stay pre-truncation:\n%s", limited)
	}
	idx := strings.LastIndex(limited, "(+")
	if idx < 0 || !strings.HasSuffix(limited, " more)\n") {
		t.Fatalf("dropped lines must collapse into a trailing (+N more) line:\n%s", limited)
	}
	var dropped int
	if _, err := fmt.Sscanf(limited[idx:], "(+%d more)", &dropped); err != nil || dropped < 1 {
		t.Fatalf("bad (+N more) suffix: %q", limited[idx:])
	}
	shown := strings.Count(limited, "[AWIT-TEST")
	if shown+dropped != 5 {
		t.Fatalf("shown %d + dropped %d != 5 ready+blocked lines", shown, dropped)
	}
}
