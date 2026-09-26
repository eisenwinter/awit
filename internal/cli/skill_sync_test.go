package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func skillRepo(t *testing.T, dirs ...string) string {
	t.Helper()
	root := agentRepo(t, dirs...)
	code, _, stderr := run(t, "--repo", root, "init", "--no-skills")
	if code != 0 {
		t.Fatalf("init: exit %d stderr %q", code, stderr)
	}
	return root
}

func writeSkillFile(t *testing.T, root, dir, content string) string {
	t.Helper()
	p := skillPath(root, dir)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func writeCurrentSkill(t *testing.T, root, dir string) string {
	t.Helper()
	p := skillPath(root, dir)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, seededBytes(t, dir), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSkillSyncUpdatesStaleFile(t *testing.T) {
	root := skillRepo(t, ".claude")
	p := writeSkillFile(t, root, ".claude", "hand-edited\n")

	code, stdout, stderr := run(t, "--repo", root, "skill", "sync")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "updated .claude/skills/driving-awit/SKILL.md\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, seededBytes(t, ".claude")) {
		t.Fatal("stale file was not rewritten to skill.Render bytes")
	}
}

func TestSkillSyncCreatesMissingFile(t *testing.T) {
	root := skillRepo(t, ".claude")
	parent := filepath.Join(root, ".claude", "skills", "driving-awit")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := run(t, "--repo", root, "skill", "sync")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "created .claude/skills/driving-awit/SKILL.md\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	got, err := os.ReadFile(skillPath(root, ".claude"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, seededBytes(t, ".claude")) {
		t.Fatal("created file bytes differ from skill.Render")
	}
}

func TestSkillSyncCreatesMissingDestinationDirs(t *testing.T) {
	root := skillRepo(t, ".claude")

	code, stdout, stderr := run(t, "--repo", root, "skill", "sync")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "created .claude/skills/driving-awit/SKILL.md\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	got, err := os.ReadFile(skillPath(root, ".claude"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, seededBytes(t, ".claude")) {
		t.Fatal("created file bytes differ from skill.Render")
	}
}

func TestSkillSyncCurrentDoesNotRewrite(t *testing.T) {
	root := skillRepo(t, ".claude")
	p := writeCurrentSkill(t, root, ".claude")
	past := time.Date(2000, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(p, past, past); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := run(t, "--repo", root, "skill", "sync")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "current .claude/skills/driving-awit/SKILL.md\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	after, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("current file was rewritten")
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, seededBytes(t, ".claude")) {
		t.Fatal("current file bytes changed")
	}
}

func TestSkillSyncMixedTargetOrder(t *testing.T) {
	root := skillRepo(t, ".claude", ".omp", ".pi")
	writeSkillFile(t, root, ".claude", "stale\n")
	writeCurrentSkill(t, root, ".pi")

	code, stdout, stderr := run(t, "--repo", root, "skill", "sync")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	want := "" +
		"updated .claude/skills/driving-awit/SKILL.md\n" +
		"created .omp/skills/driving-awit/SKILL.md\n" +
		"current .pi/skills/driving-awit/SKILL.md\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	for _, dir := range []string{".claude", ".omp", ".pi"} {
		got, err := os.ReadFile(skillPath(root, dir))
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		if !bytes.Equal(got, seededBytes(t, dir)) {
			t.Fatalf("%s: bytes differ from skill.Render", dir)
		}
	}
}

func TestSkillSyncSecondRunIsCurrent(t *testing.T) {
	root := skillRepo(t, ".claude")
	code, stdout, stderr := run(t, "--repo", root, "skill", "sync")
	if code != 0 || stderr != "" {
		t.Fatalf("first: exit %d stderr %q", code, stderr)
	}
	if stdout != "created .claude/skills/driving-awit/SKILL.md\n" {
		t.Fatalf("first stdout = %q", stdout)
	}

	code, stdout, stderr = run(t, "--repo", root, "skill", "sync")
	if code != 0 || stderr != "" {
		t.Fatalf("second: exit %d stderr %q", code, stderr)
	}
	if stdout != "current .claude/skills/driving-awit/SKILL.md\n" {
		t.Fatalf("second stdout = %q", stdout)
	}
}

func TestSkillSyncRepoFlagIsolatesRoots(t *testing.T) {
	a := skillRepo(t, ".claude")
	b := skillRepo(t, ".claude")
	const keep = "keep-me\n"
	writeSkillFile(t, b, ".claude", keep)
	t.Chdir(t.TempDir())

	code, stdout, stderr := run(t, "--repo", a, "skill", "sync")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "created .claude/skills/driving-awit/SKILL.md\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	got, err := os.ReadFile(skillPath(a, ".claude"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, seededBytes(t, ".claude")) {
		t.Fatal("repo a was not created")
	}
	kept, err := os.ReadFile(skillPath(b, ".claude"))
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != keep {
		t.Fatalf("repo b was mutated: %q", kept)
	}
}

func TestSkillSyncNoDetectedTargetsExitsZero(t *testing.T) {
	root := initRepo(t)
	code, stdout, stderr := run(t, "--repo", root, "skill", "sync")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestSkillSyncReadErrorDestIsDirectory(t *testing.T) {
	root := skillRepo(t, ".claude", ".omp")
	p := skillPath(root, ".claude")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := run(t, "--repo", root, "skill", "sync")
	if code != 1 {
		t.Fatalf("exit %d, want 1 (stderr %q)", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("success line for failed write: %q", stdout)
	}
	if !strings.Contains(stderr, "Error:") {
		t.Fatalf("stderr = %q, want Error: prefix", stderr)
	}
	if !strings.Contains(stderr, ".claude/skills/driving-awit/SKILL.md") {
		t.Fatalf("stderr = %q, want dest path", stderr)
	}
	if _, err := os.Stat(skillPath(root, ".omp")); !os.IsNotExist(err) {
		t.Fatal("later target written after error")
	}
}

func TestSkillSyncWriteErrorWhenDestParentIsFile(t *testing.T) {
	root := skillRepo(t, ".claude", ".omp")
	skills := filepath.Join(root, ".claude", "skills")
	if err := os.WriteFile(skills, []byte("not a dir"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := run(t, "--repo", root, "skill", "sync")
	if code != 1 {
		t.Fatalf("exit %d, want 1 (stderr %q)", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("success line for failed write: %q", stdout)
	}
	if !strings.Contains(stderr, "Error:") {
		t.Fatalf("stderr = %q, want Error: prefix", stderr)
	}
	if !strings.Contains(stderr, ".claude/skills/driving-awit/SKILL.md") {
		t.Fatalf("stderr = %q, want dest path", stderr)
	}
	if _, err := os.Stat(skillPath(root, ".omp")); !os.IsNotExist(err) {
		t.Fatal("later target written after error")
	}
}

func TestSkillSyncStopsAtFirstErrorAfterSuccess(t *testing.T) {
	root := skillRepo(t, ".claude", ".omp", ".pi")
	writeCurrentSkill(t, root, ".claude")
	if err := os.MkdirAll(skillPath(root, ".omp"), 0o755); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := run(t, "--repo", root, "skill", "sync")
	if code != 1 {
		t.Fatalf("exit %d, want 1 (stderr %q)", code, stderr)
	}
	if stdout != "current .claude/skills/driving-awit/SKILL.md\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "Error:") || !strings.Contains(stderr, ".omp/skills/driving-awit/SKILL.md") {
		t.Fatalf("stderr = %q", stderr)
	}
	if _, err := os.Stat(skillPath(root, ".pi")); !os.IsNotExist(err) {
		t.Fatal("later target written after error")
	}
}

func TestSkillSyncHelpStatesRefreshAndOverwrite(t *testing.T) {
	code, stdout, stderr := run(t, "skill", "--help")
	if code != 0 {
		t.Fatalf("skill --help exit %d stderr %q", code, stderr)
	}
	lower := strings.ToLower(stdout)
	if !strings.Contains(lower, "refresh") {
		t.Fatalf("skill --help missing refresh:\n%s", stdout)
	}
	if !strings.Contains(lower, "sync") {
		t.Fatalf("skill --help missing sync:\n%s", stdout)
	}

	code, stdout, stderr = run(t, "skill", "sync", "--help")
	if code != 0 {
		t.Fatalf("skill sync --help exit %d stderr %q", code, stderr)
	}
	lower = strings.ToLower(stdout)
	if !strings.Contains(lower, "refresh") {
		t.Fatalf("skill sync --help missing refresh:\n%s", stdout)
	}
	if !strings.Contains(lower, "overwrite") && !strings.Contains(lower, "hand") {
		t.Fatalf("skill sync --help missing hand-edit overwrite:\n%s", stdout)
	}
}
