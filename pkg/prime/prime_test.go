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

func TestPrimePriorityInvariant(t *testing.T) {
	limited := render(t, load(t, "clean"), Options{MaxTokens: 40})
	top := "[AWIT-TEST0001] open Implement OAuth2 bearer token extraction | auth,p1 | Unblocks: 2\n"
	if !strings.Contains(limited, top) {
		t.Fatalf("top ready row must survive truncation as a complete row:\n%s", limited)
	}
	if strings.Contains(limited, "CRITICAL PATH") {
		t.Fatalf("critical path is contextual payload, shed before the top ready row:\n%s", limited)
	}
	if !strings.Contains(limited, "=== READY (3) ===") {
		t.Fatalf("header counts stay post-filter, pre-truncation:\n%s", limited)
	}
	if !strings.Contains(limited, "=== BLOCKED (2) ===") {
		t.Fatalf("header counts stay post-filter, pre-truncation:\n%s", limited)
	}
	if !strings.HasSuffix(limited, "(+4 more)\n") {
		t.Fatalf("dropped rows collapse into a trailing (+N more) line:\n%s", limited)
	}
	shown := strings.Count(limited, "[AWIT-TEST")
	if shown != 1 {
		t.Fatalf("shown %d ready+blocked rows, want exactly the top ready row", shown)
	}
}

func TestPrimeTopReadySurvivesTinyBudget(t *testing.T) {
	limited := render(t, load(t, "clean"), Options{MaxTokens: 1})
	top := "[AWIT-TEST0001] open Implement OAuth2 bearer token extraction | auth,p1 | Unblocks: 2\n"
	if !strings.Contains(limited, top) {
		t.Fatalf("MaxTokens=1 must keep the complete top ready row:\n%s", limited)
	}
	if strings.Contains(limited, "CRITICAL PATH") {
		t.Fatalf("no budget for contextual payload at MaxTokens=1:\n%s", limited)
	}
	if !strings.HasSuffix(limited, "\n") || strings.HasSuffix(limited, "\n\n") {
		t.Fatalf("output keeps exactly one terminal LF:\n%q", limited)
	}
}

func TestPrimeWarningsSurviveTinyBudget(t *testing.T) {
	limited := render(t, load(t, "cyclic"), Options{MaxTokens: 1})
	want := "[CYCLE] AWIT-TEST0001 -> AWIT-TEST0002 -> AWIT-TEST0003 -> AWIT-TEST0001 (excluded from next)\n" +
		"[CYCLE] AWIT-TEST0004 -> AWIT-TEST0004 (excluded from next)\n" +
		"[AWIT-TEST0005] open Update the changelog | - | Unblocks: 0\n"
	if limited != want {
		t.Fatalf("floor is warning details plus the top ready row, nothing else.\ngot:\n%s\nwant:\n%s", limited, want)
	}
	if EstimateTokens([]byte(limited)) <= 1 {
		t.Fatalf("floor of %d bytes must exceed the 1-token budget by design", len(limited))
	}
}

func TestPrimeLongTopRowIsNeverTruncated(t *testing.T) {
	long := strings.Repeat("x", 500)
	g := graph.Build([]*item.Item{
		item.New("AWIT-TEST0001", long, "brief", nil, nil),
		item.New("AWIT-TEST0002", "short", "brief", nil, nil),
	}, nil)
	out := render(t, g, Options{MaxTokens: 10})
	row := "[" + "AWIT-TEST0001" + "] open " + long + " | - | Unblocks: 0\n"
	if out != row {
		t.Fatalf("long top row is kept whole, never cut mid-row.\ngot:\n%s\nwant:\n%s", out, row)
	}
	if EstimateTokens([]byte(out)) <= 10 {
		t.Fatalf("floor must exceed the budget here, proving the soft-budget rule")
	}
}

func TestPrimeFilteredNoReadyTinyBudget(t *testing.T) {
	opts := Options{MaxTokens: 1, Labels: [][]string{{"p0"}}}
	out := render(t, load(t, "clean"), opts)
	if out != "" {
		t.Fatalf("no warnings, no ready row: every byte is sheddable, want empty output, got:\n%q", out)
	}
	if again := render(t, load(t, "clean"), opts); again != out {
		t.Fatalf("budgeted render must be deterministic:\n%q\n---\n%q", out, again)
	}
}

func TestPrimeBudgetFitsOrIsFloor(t *testing.T) {
	full := render(t, load(t, "clean"), Options{})
	fullTokens := EstimateTokens([]byte(full))
	floor := render(t, load(t, "clean"), Options{MaxTokens: 1})
	for max := 2; max <= fullTokens+4; max++ {
		out := render(t, load(t, "clean"), Options{MaxTokens: max})
		if out == floor {
			continue // documented soft floor: emitted even over budget
		}
		if got := EstimateTokens([]byte(out)); got > max {
			t.Fatalf("max=%d: %d-token output is neither within budget nor the floor:\n%s", max, got, out)
		}
		if strings.Contains(out, "(+") {
			if !strings.HasSuffix(out, " more)\n") {
				t.Fatalf("max=%d: omission notice must be one trailing line:\n%s", max, out)
			}
			var dropped int
			idx := strings.LastIndex(out, "(+")
			if _, err := fmt.Sscanf(out[idx:], "(+%d more)", &dropped); err != nil {
				t.Fatalf("max=%d: bad (+N more) suffix: %q", max, out[idx:])
			}
			// 5 = 3 ready + 2 blocked on clean; the suffix counts shed
			// rows, never removed headers or critical-path nodes.
			shown := strings.Count(out, "[AWIT-TEST")
			if shown+dropped != 5 {
				t.Fatalf("max=%d: shown %d + dropped %d != 5 ready+blocked rows", max, shown, dropped)
			}
		}
		if strings.Contains(out, "=== READY") && !strings.Contains(out, "=== READY (3) ===") {
			t.Fatalf("max=%d: ready header count must stay pre-truncation:\n%s", max, out)
		}
	}
	if got := render(t, load(t, "clean"), Options{MaxTokens: fullTokens}); got != full {
		t.Fatalf("budget that fits the full snapshot must emit it byte-for-byte")
	}
}
func TestPrimeWarningsNoReadySingleTerminalLF(t *testing.T) {
	g := graph.Build([]*item.Item{
		item.New("AWIT-TEST0001", "alpha", "brief", []string{"AWIT-TEST0002"}, nil),
		item.New("AWIT-TEST0002", "beta", "brief", []string{"AWIT-TEST0001"}, nil),
	}, nil)
	if n := len(g.Ready()); n != 0 {
		t.Fatalf("two-cycle fixture must have no ready rows, got %d", n)
	}
	out := render(t, g, Options{MaxTokens: 30})
	if !strings.Contains(out, "[CYCLE] ") {
		t.Fatalf("warnings must survive truncation:\n%s", out)
	}
	if !strings.HasSuffix(out, "\n") || strings.HasSuffix(out, "\n\n") {
		t.Fatalf("output keeps exactly one terminal LF:\n%q", out)
	}
	if got := EstimateTokens([]byte(out)); got > 30 {
		t.Fatalf("%d-token output exceeds budget 30:\n%s", got, out)
	}
}
