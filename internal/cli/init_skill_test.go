package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/internal/skill"
)

// agentRepo makes a bare directory carrying the given agent directories.
func agentRepo(t *testing.T, dirs ...string) string {
	t.Helper()
	root := t.TempDir()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func skillPath(root, dir string) string {
	return filepath.Join(root, dir, "skills", skill.Name, "SKILL.md")
}

// seededBytes is what a correctly seeded file must contain, byte for byte.
func seededBytes(t *testing.T, dir string) []byte {
	t.Helper()
	for _, tg := range skill.Targets() {
		if tg.Dir == dir {
			return skill.Render(tg)
		}
	}
	t.Fatalf("no target for %s", dir)
	return nil
}

func TestInitSeedsOnYes(t *testing.T) {
	root := agentRepo(t, ".claude", ".omp")
	code, stdout, stderr := runStdin(t, "y\ny\n", "--repo", root, "init")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	for _, d := range []string{".claude", ".omp"} {
		got, err := os.ReadFile(skillPath(root, d))
		if err != nil {
			t.Fatalf("%s: %v", d, err)
		}
		if !bytes.Equal(got, seededBytes(t, d)) {
			t.Fatalf("%s: seeded bytes differ from skill.Render", d)
		}
	}
	if !strings.Contains(stdout, ".claude") || !strings.Contains(stdout, ".omp") {
		t.Fatalf("stdout should name both directories:\n%s", stdout)
	}
}

func TestInitDeclineDoesNotSeed(t *testing.T) {
	root := agentRepo(t, ".claude", ".omp")
	// "n" then a bare newline: both mean no.
	code, _, stderr := runStdin(t, "n\n\n", "--repo", root, "init")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	for _, d := range []string{".claude", ".omp"} {
		if _, err := os.Stat(skillPath(root, d)); !os.IsNotExist(err) {
			t.Fatalf("%s: seeded despite a declined prompt", d)
		}
	}
}

// A closed or empty stdin (CI, `awit init < /dev/null`) reads EOF on the
// first prompt and must be treated as "no" for every target, never as a
// blocking read and never as consent.
func TestInitEmptyStdinSeedsNothing(t *testing.T) {
	root := agentRepo(t, ".claude", ".omp", ".pi")
	code, _, stderr := runStdin(t, "", "--repo", root, "init")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	for _, d := range []string{".claude", ".omp", ".pi"} {
		if _, err := os.Stat(skillPath(root, d)); !os.IsNotExist(err) {
			t.Fatalf("%s: seeded on EOF", d)
		}
	}
}

func TestInitSkillsFlagDoesNotPrompt(t *testing.T) {
	root := agentRepo(t, ".claude", ".agents")
	code, stdout, stderr := runStdin(t, "", "--repo", root, "init", "--skills")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if strings.Contains(stdout, "[y/N]") {
		t.Fatalf("--skills must not prompt:\n%s", stdout)
	}
	for _, d := range []string{".claude", ".agents"} {
		if _, err := os.Stat(skillPath(root, d)); err != nil {
			t.Fatalf("%s: not seeded under --skills: %v", d, err)
		}
	}
}

func TestInitNoSkillsFlag(t *testing.T) {
	root := agentRepo(t, ".claude", ".omp")
	code, stdout, stderr := runStdin(t, "y\ny\n", "--repo", root, "init", "--no-skills")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if strings.Contains(stdout, "[y/N]") {
		t.Fatalf("--no-skills must not prompt:\n%s", stdout)
	}
	if _, err := os.Stat(skillPath(root, ".claude")); !os.IsNotExist(err) {
		t.Fatal("--no-skills seeded anyway")
	}
}

func TestInitSkillsAndNoSkillsConflict(t *testing.T) {
	code, _, stderr := run(t, "--repo", t.TempDir(), "init", "--skills", "--no-skills")
	if code != 2 {
		t.Fatalf("exit %d, want 2 (stderr %q)", code, stderr)
	}
	if stderr != "Error: pass either --skills or --no-skills\n" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestInitExistingSkillIsKeptUnlessForced(t *testing.T) {
	root := agentRepo(t, ".claude")
	p := skillPath(root, ".claude")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	const mine = "---\nname: driving-awit\ndescription: hand written\n---\n\nmine\n"
	if err := os.WriteFile(p, []byte(mine), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runStdin(t, "y\n", "--repo", root, "init", "--skills")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != mine {
		t.Fatalf("an existing skill was overwritten without --force")
	}
	if !strings.Contains(stdout, "--force") {
		t.Fatalf("the skip should tell the user how to overwrite:\n%s", stdout)
	}
}

func TestInitForceOverwrites(t *testing.T) {
	root := agentRepo(t, ".claude")
	p := skillPath(root, ".claude")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runStdin(t, "", "--repo", root, "init", "--skills", "--force")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, seededBytes(t, ".claude")) {
		t.Fatal("--force did not rewrite the skill")
	}
}

// A repository with no agent directory must behave exactly as init did
// before this feature: one line, no prompt, no extra output.
func TestInitWithoutAgentDirsIsUnchanged(t *testing.T) {
	root := t.TempDir()
	abs, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runStdin(t, "y\n", "--repo", root, "init")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "Initialized .awit in "+abs+" (prefix AWIT)\n" {
		t.Fatalf("stdout = %q", stdout)
	}
}

// A file (not a directory) named like an agent directory is not a tool.
func TestInitIgnoresAgentNameThatIsAFile(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".claude"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runStdin(t, "y\n", "--repo", root, "init")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if strings.Contains(stdout, "[y/N]") {
		t.Fatalf("prompted for a plain file:\n%s", stdout)
	}
}
