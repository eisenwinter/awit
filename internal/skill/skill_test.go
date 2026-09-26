package skill_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/internal/cli"
	"github.com/eisenwinter/awit/internal/skill"
)

func TestTargetsAreWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, tg := range skill.Targets() {
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
		want := filepath.Join("skills", skill.Name, "SKILL.md")
		if tg.Path != want {
			t.Fatalf("Path = %q, want %q", tg.Path, want)
		}
	}
}

func TestTargetsCoverTheKnownTools(t *testing.T) {
	want := []string{".claude", ".omp", ".opencode", ".agents", ".pi"}
	var got []string
	for _, tg := range skill.Targets() {
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
	for _, tg := range skill.Targets() {
		a, b := skill.Render(tg), skill.Render(tg)
		if !bytes.Equal(a, b) {
			t.Fatalf("%s: Render is not deterministic", tg.Dir)
		}
	}
}

func TestRenderShape(t *testing.T) {
	for _, tg := range skill.Targets() {
		got := string(skill.Render(tg))
		if !strings.HasPrefix(got, "---\n") {
			t.Fatalf("%s: rendered skill must start with a frontmatter fence:\n%.40s", tg.Dir, got)
		}
		if !strings.Contains(got, "\nname: "+skill.Name+"\n") {
			t.Fatalf("%s: rendered skill must declare name: %s", tg.Dir, skill.Name)
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
	for _, tg := range skill.Detect(root) {
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
	if got := skill.Detect(t.TempDir()); len(got) != 0 {
		t.Fatalf("Detect on a bare repo = %v, want none", got)
	}
}

func ompTarget(t *testing.T) skill.Target {
	t.Helper()
	for _, tg := range skill.Targets() {
		if tg.Dir == ".omp" {
			return tg
		}
	}
	t.Fatal("no .omp target")
	return skill.Target{}
}

func committedOmpPath(t *testing.T) string {
	t.Helper()
	omp := ompTarget(t)
	return filepath.Join("..", "..", omp.Dir, omp.Path)
}

// TestDogfoodOmpCopyMatchesRenderer keeps this repo's own committed skill
// byte-identical to what `awit init` would seed. Without it the embedded
// asset and the file agents actually read here drift apart silently.
func TestDogfoodOmpCopyMatchesRenderer(t *testing.T) {
	omp := ompTarget(t)
	path := committedOmpPath(t)
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(onDisk, skill.Render(omp)) {
		t.Fatalf("%s differs from skill.Render for the .omp target.\n"+
			"The embedded asset is the source of truth: edit "+
			"internal/skill/assets/driving-awit.body.md (or the target's "+
			"Frontmatter) and regenerate, do not edit the seeded copy.", path)
	}
}

func runAwit(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = cli.Main(args, strings.NewReader(""), &out, &errb)
	return code, out.String(), errb.String()
}

// TestDogfoodOmpSyncRestoresStaleCopy drives the real `awit skill sync`
// against an isolated stale copy of this repo's .omp skill. Expected bytes
// always come from skill.Render so the scenario stays valid when the
// embedded source changes.
func TestDogfoodOmpSyncRestoresStaleCopy(t *testing.T) {
	omp := ompTarget(t)
	want := skill.Render(omp)
	shown := filepath.ToSlash(filepath.Join(omp.Dir, omp.Path))

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, omp.Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runAwit(t, "--repo", root, "init", "--no-skills")
	if code != 0 {
		t.Fatalf("init: exit %d stderr %q", code, stderr)
	}

	dest := filepath.Join(root, omp.Dir, omp.Path)
	committed, err := os.ReadFile(committedOmpPath(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		t.Fatal(err)
	}
	stale := append([]byte("# stale in-scenario copy\n"), committed...)
	if err := os.WriteFile(dest, stale, 0o644); err != nil {
		t.Fatal(err)
	}

	pre, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(pre, want) {
		t.Fatal("pre-sync: in-scenario .omp copy still matches skill.Render")
	}

	code, stdout, stderr := runAwit(t, "--repo", root, "skill", "sync")
	if code != 0 || stderr != "" {
		t.Fatalf("sync: exit %d stderr %q", code, stderr)
	}
	if stdout != "updated "+shown+"\n" {
		t.Fatalf("sync stdout = %q, want updated %s", stdout, shown)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("after sync: %s is not byte-identical to skill.Render", shown)
	}

	code, stdout, stderr = runAwit(t, "--repo", root, "skill", "sync")
	if code != 0 || stderr != "" {
		t.Fatalf("second sync: exit %d stderr %q", code, stderr)
	}
	if stdout != "current "+shown+"\n" {
		t.Fatalf("second sync stdout = %q, want current %s", stdout, shown)
	}
	again, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, want) {
		t.Fatalf("second sync: %s bytes changed", shown)
	}

}
