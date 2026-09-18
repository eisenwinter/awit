package skill

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTargetsAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, tg := range Targets() {
		if tg.Dir == "" || tg.Path == "" || tg.Frontmatter == "" {
			t.Fatalf("incomplete target %+v", tg)
		}
		if !strings.HasPrefix(tg.Dir, ".") {
			t.Fatalf("Dir %q should be a dotted agent directory", tg.Dir)
		}
		if seen[tg.Dir] {
			t.Fatalf("duplicate target dir %q", tg.Dir)
		}
		seen[tg.Dir] = true
		if filepath.IsAbs(tg.Path) {
			t.Fatalf("Path %q must be relative to Dir", tg.Path)
		}
		// Every tool that reads SKILL.md requires the skill's name to match
		// the directory holding it, so the path always ends the same way.
		want := filepath.Join("skills", Name, "SKILL.md")
		if tg.Path != want {
			t.Fatalf("Path = %q, want %q", tg.Path, want)
		}
	}
}

func TestTargetsCoverTheKnownTools(t *testing.T) {
	want := []string{".claude", ".omp", ".opencode", ".agents", ".pi"}
	var got []string
	for _, tg := range Targets() {
		got = append(got, tg.Dir)
	}
	if len(got) != len(want) {
		t.Fatalf("Targets() dirs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Targets()[%d].Dir = %q, want %q (order is fixed so prompts and output are deterministic)", i, got[i], want[i])
		}
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	for _, tg := range Targets() {
		a, b := Render(tg), Render(tg)
		if !bytes.Equal(a, b) {
			t.Fatalf("%s: Render is not deterministic", tg.Dir)
		}
	}
}

func TestRenderShape(t *testing.T) {
	for _, tg := range Targets() {
		got := string(Render(tg))
		if !strings.HasPrefix(got, "---\n") {
			t.Fatalf("%s: rendered skill must start with a frontmatter fence:\n%.40s", tg.Dir, got)
		}
		if !strings.Contains(got, "\nname: "+Name+"\n") {
			t.Fatalf("%s: rendered skill must declare name: %s", tg.Dir, Name)
		}
		if !strings.Contains(got, "\ndescription: ") {
			t.Fatalf("%s: rendered skill must declare a description", tg.Dir)
		}
		if !strings.Contains(got, "# Driving awit") {
			t.Fatalf("%s: rendered skill is missing the body", tg.Dir)
		}
		if !strings.HasSuffix(got, "\n") {
			t.Fatalf("%s: rendered skill must end in a newline", tg.Dir)
		}
	}
}

func TestDetect(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{".omp", ".pi"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A regular file named like an agent directory is not a directory and
	// must not be detected.
	if err := os.WriteFile(filepath.Join(root, ".claude"), []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	var got []string
	for _, tg := range Detect(root) {
		got = append(got, tg.Dir)
	}
	want := []string{".omp", ".pi"}
	if len(got) != len(want) {
		t.Fatalf("Detect = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Detect[%d] = %q, want %q (Targets order preserved)", i, got[i], want[i])
		}
	}
}

func TestDetectEmpty(t *testing.T) {
	if got := Detect(t.TempDir()); len(got) != 0 {
		t.Fatalf("Detect on a bare repo = %v, want none", got)
	}
}

// TestDogfoodOmpCopyMatchesRenderer keeps this repo's own committed skill
// byte-identical to what `awit init` would seed. Without it the embedded
// asset and the file agents actually read here drift apart silently.
func TestDogfoodOmpCopyMatchesRenderer(t *testing.T) {
	var omp *Target
	for _, tg := range Targets() {
		if tg.Dir == ".omp" {
			t := tg
			omp = &t
			break
		}
	}
	if omp == nil {
		t.Fatal("no .omp target")
	}
	path := filepath.Join("..", "..", omp.Dir, omp.Path)
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(onDisk, Render(*omp)) {
		t.Fatalf("%s differs from skill.Render for the .omp target.\n"+
			"The embedded asset is the source of truth: edit "+
			"internal/skill/assets/driving-awit.body.md (or the target's "+
			"Frontmatter) and regenerate, do not edit the seeded copy.", path)
	}
}
