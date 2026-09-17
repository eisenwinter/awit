package gitx

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// git runs the real git binary in dir and fails the test on error.
func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRepo creates an initialised repository with one empty commit and a known
// identity. It skips the test when git is not installed, so the suite stays
// green on a machine without git (guide §1: git is executed, never linked).
func newRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	git(t, dir, "init", "-q")
	git(t, dir, "config", "user.email", "test@example.com")
	git(t, dir, "config", "user.name", "Test User")
	// A global commit.gpgsign=true on the developer's machine would make every
	// commit in these tests prompt or fail; pin it off for this repository.
	git(t, dir, "config", "commit.gpgsign", "false")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", "init")
	return dir
}

// notARepo returns a temp dir that is not inside any repository, or skips.
// On some machines the system temp directory itself lives inside a checkout;
// in that case the "outside a repo" behaviour cannot be observed here.
func notARepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	if root, err := Root(dir); err == nil {
		t.Skipf("temp dir is inside a git repository (%s)", root)
	}
	return dir
}

func TestBranchInRepo(t *testing.T) {
	dir := newRepo(t)
	want := git(t, dir, "symbolic-ref", "--short", "HEAD")
	if want == "" {
		t.Fatal("symbolic-ref returned an empty branch name")
	}
	if got := Branch(dir); got != want {
		t.Fatalf("Branch() = %q, want %q", got, want)
	}
}

func TestBranchOutsideRepo(t *testing.T) {
	dir := notARepo(t)
	if got := Branch(dir); got != "" {
		t.Fatalf("Branch() = %q, want \"\" outside a repository", got)
	}
}

func TestBranchDetached(t *testing.T) {
	dir := newRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "a.txt")
	git(t, dir, "commit", "-q", "-m", "second")
	git(t, dir, "checkout", "-q", "--detach", "HEAD")
	if got := git(t, dir, "rev-parse", "--abbrev-ref", "HEAD"); got != "HEAD" {
		t.Fatalf("precondition: --abbrev-ref HEAD = %q, want \"HEAD\" after --detach", got)
	}
	if got := Branch(dir); got != "" {
		t.Fatalf("Branch() = %q, want \"\" when detached", got)
	}
}

func TestUserName(t *testing.T) {
	dir := newRepo(t)
	if got := UserName(dir); got != "Test User" {
		t.Fatalf("UserName() = %q, want %q", got, "Test User")
	}
}

// TestUserNameOutsideRepo is deliberately a weak assertion. `git config
// user.name` falls back to the user's global and system config, so outside a
// repository the result is "" on a CI runner with no global identity but a
// real name on a developer machine. Asserting either value would make the test
// fail for somebody. What is worth pinning is the contract that matters to
// callers: the call returns a plain trimmed string and never panics or hangs.
func TestUserNameOutsideRepo(t *testing.T) {
	dir := notARepo(t)
	got := UserName(dir)
	if got != strings.TrimSpace(got) {
		t.Fatalf("UserName() = %q, want no surrounding whitespace", got)
	}
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("UserName() = %q, want a single line", got)
	}
}

func TestRoot(t *testing.T) {
	dir := newRepo(t)
	got, err := Root(dir)
	if err != nil {
		t.Fatalf("Root() error = %v", err)
	}
	// Compare through EvalSymlinks on both sides: t.TempDir() is under
	// /var/folders/... on macOS (a symlink to /private/var/...) and under a
	// short 8.3-style path on some Windows setups, while git prints the fully
	// resolved worktree path. Without resolving both, the strings differ even
	// though they name the same directory.
	wantResolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	gotResolved, err := filepath.EvalSymlinks(filepath.FromSlash(got))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(gotResolved) != filepath.Clean(wantResolved) {
		t.Fatalf("Root() = %q (resolved %q), want %q", got, gotResolved, wantResolved)
	}
}

func TestRootOutsideRepo(t *testing.T) {
	dir := notARepo(t)
	got, err := Root(dir)
	if err == nil {
		t.Fatalf("Root() = %q, want an error outside a repository", got)
	}
	if got != "" {
		t.Fatalf("Root() = %q, want \"\" alongside the error", got)
	}
	if !strings.Contains(err.Error(), "rev-parse --show-toplevel") {
		t.Fatalf("Root() error = %v, want it to name the failing git command", err)
	}
}

func TestCommitOnlyStagesGivenPaths(t *testing.T) {
	dir := newRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The path is absolute, exactly as item.Path is (guide §4.3).
	if err := Commit(dir, []string{filepath.Join(dir, "a.txt")}, "awit: test"); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if got := git(t, dir, "log", "-1", "--pretty=%s"); got != "awit: test" {
		t.Fatalf("subject = %q, want %q", got, "awit: test")
	}
	if got := git(t, dir, "status", "--porcelain"); got != "?? b.txt" {
		t.Fatalf("status = %q, want %q (b.txt must stay untracked)", got, "?? b.txt")
	}
	if got := git(t, dir, "show", "--name-only", "--pretty=format:", "HEAD"); got != "a.txt" {
		t.Fatalf("committed files = %q, want %q", got, "a.txt")
	}
}

func TestCommitError(t *testing.T) {
	dir := notARepo(t)
	if err := Commit(dir, []string{filepath.Join(dir, "a.txt")}, "awit: test"); err == nil {
		t.Fatal("Commit() error = nil, want an error outside a repository")
	}
}

func TestCommitNoPaths(t *testing.T) {
	dir := newRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", "a.txt")
	// An empty pathspec would let git commit the already-staged a.txt, so a
	// claim commit could carry unrelated work. Commit must refuse instead.
	if err := Commit(dir, nil, "awit: test"); err == nil {
		t.Fatal("Commit() error = nil, want an error for an empty path list")
	}
	if got := git(t, dir, "log", "-1", "--pretty=%s"); got != "init" {
		t.Fatalf("subject = %q, want %q (nothing new must be committed)", got, "init")
	}
}
