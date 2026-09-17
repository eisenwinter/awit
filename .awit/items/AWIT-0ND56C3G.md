---
id: AWIT-0ND56C3G
title: 'internal/gitx: branch, user.name, root, commit'
brief: >-
  Add internal/gitx, a thin os/exec wrapper around the git binary exposing
  Branch, UserName, Root and Commit. It is the only place in awit that shells
  out to git, and it never links a git library.
status: closed
deps: []
labels: [phase0, p1]
refs:
  - ../../plan/implementation-guide.md
  - ../../plan/awit-implementation-plan.md
---

## Summary
After this ticket `internal/gitx/gitx.go` exists with exactly four exported functions — `Branch`, `UserName`, `Root`, `Commit` — plus one private `run` helper that executes `git -C <dir> <args...>` and captures stdout and stderr. `Branch` and `UserName` swallow errors and return `""` because their callers (ID worker hashing, comment author fallback) must work in a directory that is not a Git repository. `Root` and `Commit` return errors. `internal/gitx/gitx_test.go` drives the real `git` binary in `t.TempDir()` repositories and skips when `git` is not installed. No CLI wiring, no ID minting, no claim logic is added here.

## Context (read first)
- Guide §4.5 `internal/gitx` — the exact four signatures this ticket must produce. Copy them; do not rename.
- Guide §1: `git` is invoked via `os/exec`, **never linked**. Dependencies are limited to `github.com/urfave/cli/v3` and `gopkg.in/yaml.v3`; this package uses stdlib only.
- Guide §1: must compile and pass `go vet`, `staticcheck` and `go test ./...` on **Linux and Windows**; never hardcode `/` in filesystem paths — use `filepath`.
- Guide §2 decision 2 (worker hash input): hostname + worktree absolute path + branch name, FNV-1a 32, `% 64`; **"Branch missing (not a git repo) → empty string, still hashed."** That is why `Branch` returns `""` instead of an error: `id.Worker` must never fail because a user runs `awit` outside a repository.
- Guide §2 decision 4 (comment author source): `--author` → `AWIT_AGENT` → `config.agent_id` → `git config user.name` → error. The `user.name` step is `UserName`; it returns `""` so the resolution chain can fall through to its own error message.
- Guide §2 "Git commit on `--claim`": `git -C <root> add <itemfile>` then `git -C <root> commit -m "awit: claim <id>" -- <itemfile>`. `Commit` is that operation, generalised to a slice of paths.
- Guide §4.3: `Item.Path` is an **absolute** path on disk. Callers pass `it.Path` straight into `Commit`, so `Commit` must accept absolute paths. `git add --` and `git commit -- <pathspec>` both accept absolute paths as long as they are inside the worktree; Git resolves them against the worktree root itself, which is why no manual `filepath.Rel` conversion is needed (and why doing it by hand would be a portability bug on Windows, where `C:\...` paths and Git's internal forward-slash pathspecs differ).
- Guide §4.4: `Store.Mint` uses `id.Worker(s.Root, gitx.Branch(s.Root))` — the single consumer of `Branch`.
- Guide §5: tests use `testing` stdlib only, table-driven where it helps, `t.TempDir()` for the filesystem.
- Spec `plan/awit-implementation-plan.md` §"Phased plan" → "Phase 0 — skeleton" and §"Decisions" row *Claims*: "Soft claim: writes `status`, `assignee`, `claimed_at`, then commits (`awit: claim <id>`) unless `--no-commit`" — the committed file must be **only** the claimed item, otherwise a claim commit would drag unrelated staged work along. `Commit` therefore always passes an explicit pathspec to `git commit`.

## Files
- Create: `internal/gitx/gitx.go`
- Create: `internal/gitx/gitx_test.go`
- Modify: none. `go.mod` is untouched — this package imports only `bytes`, `errors`, `fmt`, `os/exec`, `strings` (and `os`, `path/filepath`, `testing` in the test).
- Fixtures/golden: none. Every test builds its repository at runtime with the real `git` binary in `t.TempDir()`.

## Interfaces
- Consumes: nothing. This package has no dependency on any other awit package, which is why its `deps` list is empty and it can be built in parallel with `pkg/id`, `pkg/config` and the CLI skeleton.
- Produces (verbatim from guide §4.5):
  ```go
  package gitx

  // Branch returns the current branch name for dir or "" when not a git repo / detached.
  func Branch(dir string) string
  // UserName returns `git config user.name` or "".
  func UserName(dir string) string
  // Root returns `git rev-parse --show-toplevel` or error.
  func Root(dir string) (string, error)
  // Commit stages the given paths (relative to or absolute within root) and commits only them.
  func Commit(root string, paths []string, message string) error
  ```
- Produces (package-private, introduced by this ticket, not in guide §4):
  ```go
  // run executes `git -C dir <args...>`, returning trimmed stdout, or an error
  // wrapping git's exit status and trimmed stderr.
  func run(dir string, args ...string) (string, error)

  // errNoPaths guards Commit against an empty pathspec, which would otherwise
  // commit whatever else happens to be staged in the worktree.
  var errNoPaths = errors.New("gitx: commit needs at least one path")
  ```
- Future consumers (**not** implemented here): `pkg/item` `Store.Mint` (`Branch`), `internal/cli` comment author resolution (`UserName`), `internal/cli` `next --claim` (`Commit`), `internal/cli` `openStore` fallbacks (`Root`).

## Steps

- [ ] **Step 1: Write the failing test for `run`, `Branch` and the repo helpers.**
  Create `internal/gitx/gitx_test.go` with the two helpers and the three branch tests:
  ```go
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
  ```
  Note why `TestBranchDetached` asserts the precondition first: `rev-parse --abbrev-ref HEAD` printing the literal string `HEAD` is the *only* signal that distinguishes a detached head from a branch called something else, so the test pins that contract before asserting the mapping to `""`.

- [ ] **Step 2: Run it, see it fail to compile.**
  ```bash
  go test ./internal/gitx -run TestBranch -v
  ```
  Expected failure (no `gitx.go` yet, so the package has no non-test file and the identifiers are undefined):
  ```text
  # github.com/eisenwinter/awit/internal/gitx [github.com/eisenwinter/awit/internal/gitx.test]
  internal/gitx/gitx_test.go:34:19: undefined: Root
  internal/gitx/gitx_test.go:47:14: undefined: Branch
  FAIL	github.com/eisenwinter/awit/internal/gitx [build failed]
  ```

- [ ] **Step 3: Implement `run`, `Branch`, `UserName`, `Root`.**
  Create `internal/gitx/gitx.go`:
  ```go
  // Package gitx wraps the git command line. It is the only package in awit that
  // shells out to git; nothing links a git library (guide §1).
  package gitx

  import (
  	"bytes"
  	"errors"
  	"fmt"
  	"os/exec"
  	"strings"
  )

  // run executes `git -C dir <args...>` and returns trimmed stdout.
  //
  // stdout and stderr are captured into separate buffers so that a failure can
  // report git's own diagnostic instead of an empty "exit status 128". The error
  // wraps the *exec.ExitError, so callers can still use errors.As on it.
  func run(dir string, args ...string) (string, error) {
  	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
  	var stdout, stderr bytes.Buffer
  	cmd.Stdout = &stdout
  	cmd.Stderr = &stderr
  	if err := cmd.Run(); err != nil {
  		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
  	}
  	return strings.TrimSpace(stdout.String()), nil
  }

  // Branch returns the current branch name for dir or "" when not a git repo /
  // detached. It never returns an error: id.Worker hashes the branch name and
  // must keep working outside a repository (guide §2, decision 2).
  func Branch(dir string) string {
  	out, err := run(dir, "rev-parse", "--abbrev-ref", "HEAD")
  	if err != nil {
  		return ""
  	}
  	// A detached HEAD makes --abbrev-ref print the literal "HEAD", which is not
  	// a branch name. Treat it like "no branch".
  	if out == "HEAD" {
  		return ""
  	}
  	return out
  }

  // UserName returns `git config user.name` or "". The empty string lets the
  // author-resolution chain fall through to its own error (guide §2, decision 4).
  func UserName(dir string) string {
  	out, err := run(dir, "config", "user.name")
  	if err != nil {
  		return ""
  	}
  	return out
  }

  // Root returns `git rev-parse --show-toplevel` or error.
  func Root(dir string) (string, error) {
  	return run(dir, "rev-parse", "--show-toplevel")
  }
  ```
  Two things to keep: `exec.Command("git", append([]string{"-C", dir}, args...)...)` — `-C dir` must come *before* the subcommand, and building a fresh slice avoids aliasing the caller's `args`. And `git` is resolved on `PATH` by `exec.Command`, which is what makes `exec.LookPath("git")` a valid skip guard in the tests.

- [ ] **Step 4: Run the branch tests, see them pass.**
  ```bash
  go test ./internal/gitx -run TestBranch -v
  ```
  Expected:
  ```text
  === RUN   TestBranchInRepo
  --- PASS: TestBranchInRepo (0.03s)
  === RUN   TestBranchOutsideRepo
  --- PASS: TestBranchOutsideRepo (0.01s)
  === RUN   TestBranchDetached
  --- PASS: TestBranchDetached (0.05s)
  PASS
  ok  	github.com/eisenwinter/awit/internal/gitx	0.09s
  ```
  Commit:
  ```bash
  gofmt -l internal/gitx
  git add internal/gitx
  git commit -m "gitx: run helper, Branch, UserName, Root"
  ```
  (`gofmt -l` must print nothing.)

- [ ] **Step 5: Write the failing tests for `UserName` and `Root`.**
  Append to `internal/gitx/gitx_test.go`:
  ```go
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
  ```
  `TestRootOutsideRepo` also pins the `run` error format from Step 3 (`git <args>: <exit err>: <stderr>`), which is the reason `run` joins the args into the message at all.
  ```bash
  go test ./internal/gitx -run 'TestUserName|TestRoot' -v
  ```
  Expected failure: `TestUserName` and `TestUserNameOutsideRepo` fail with `undefined: UserName` only if Step 3 was skipped; after Step 3 they compile, so the expected red here is limited to whatever is still missing. Run it and confirm the four tests pass (`UserName` and `Root` were implemented in Step 3, these tests lock their behaviour down):
  ```text
  --- PASS: TestUserName (0.03s)
  --- PASS: TestUserNameOutsideRepo (0.01s)
  --- PASS: TestRoot (0.03s)
  --- PASS: TestRootOutsideRepo (0.01s)
  ok  	github.com/eisenwinter/awit/internal/gitx	0.09s
  ```
  Commit:
  ```bash
  git add internal/gitx/gitx_test.go
  git commit -m "gitx: test UserName and Root"
  ```

- [ ] **Step 6: Write the failing tests for `Commit`.**
  Append to `internal/gitx/gitx_test.go`:
  ```go
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
  ```
  ```bash
  go test ./internal/gitx -run TestCommit -v
  ```
  Expected failure:
  ```text
  # github.com/eisenwinter/awit/internal/gitx [github.com/eisenwinter/awit/internal/gitx.test]
  internal/gitx/gitx_test.go:151:12: undefined: Commit
  FAIL	github.com/eisenwinter/awit/internal/gitx [build failed]
  ```

- [ ] **Step 7: Implement `Commit`.**
  Append to `internal/gitx/gitx.go`:
  ```go
  // errNoPaths guards against an empty pathspec. `git commit -m msg --` with no
  // paths commits everything already in the index, which would turn a claim
  // commit into a commit of unrelated staged work.
  var errNoPaths = errors.New("gitx: commit needs at least one path")

  // Commit stages the given paths (relative to or absolute within root) and
  // commits only them.
  //
  // Paths normally arrive as item.Path, which is absolute (guide §4.3). Git
  // accepts absolute pathspecs inside the worktree and resolves them against the
  // worktree root, so no filepath.Rel conversion is needed — and none should be
  // attempted, because hand-built relative paths break on Windows drive letters.
  //
  // The explicit pathspec on `git commit` is what makes the commit minimal: the
  // command commits those paths only, ignoring anything else in the index.
  func Commit(root string, paths []string, message string) error {
  	if len(paths) == 0 {
  		return errNoPaths
  	}
  	if _, err := run(root, append([]string{"add", "--"}, paths...)...); err != nil {
  		return fmt.Errorf("gitx: stage: %w", err)
  	}
  	if _, err := run(root, append([]string{"commit", "-q", "-m", message, "--"}, paths...)...); err != nil {
  		return fmt.Errorf("gitx: commit: %w", err)
  	}
  	return nil
  }
  ```
  `-q` keeps git's commit summary off stderr so a successful claim prints nothing; `--` terminates options so a path that begins with `-` is still treated as a path.

- [ ] **Step 8: Run the commit tests, see them pass.**
  ```bash
  go test ./internal/gitx -run TestCommit -v
  ```
  Expected:
  ```text
  === RUN   TestCommitOnlyStagesGivenPaths
  --- PASS: TestCommitOnlyStagesGivenPaths (0.12s)
  === RUN   TestCommitError
  --- PASS: TestCommitError (0.01s)
  === RUN   TestCommitNoPaths
  --- PASS: TestCommitNoPaths (0.09s)
  PASS
  ok  	github.com/eisenwinter/awit/internal/gitx	0.23s
  ```
  Commit:
  ```bash
  gofmt -l internal/gitx
  git add internal/gitx
  git commit -m "gitx: commit only the given paths"
  ```

- [ ] **Step 9: Run the whole package, build and vet.**
  ```bash
  go test ./internal/gitx -v
  go build ./...
  go vet ./...
  ```
  Expected: nine tests pass (`TestBranchInRepo`, `TestBranchOutsideRepo`, `TestBranchDetached`, `TestUserName`, `TestUserNameOutsideRepo`, `TestRoot`, `TestRootOutsideRepo`, `TestCommitOnlyStagesGivenPaths`, `TestCommitError`, `TestCommitNoPaths`), then `ok  	github.com/eisenwinter/awit/internal/gitx`; `go build` and `go vet` print nothing and exit `0`.
  Also confirm the skip path works, because CI images without git must stay green rather than fail:
  ```bash
  env PATH=/nonexistent go test ./internal/gitx -v 2>&1 | grep -c SKIP
  ```
  Expected: a non-zero count — every test reports `--- SKIP: ... git not installed`. (On Windows use `set PATH=` in `cmd` or `$env:PATH=''` in PowerShell; the `go` binary must still be reachable by absolute path.)

- [ ] **Step 10: Close ticket.**
  - Set `status: closed` in the frontmatter of `.awit/items/AWIT-0ND56C3G.md`.
  - Create `.awit/comments/AWIT-0ND56C3G/<YYYYMMDDTHHMMSSZ>-<author>.md` (UTC stamp, e.g. `20260917T152233Z-claude.md`):
    ```markdown
    ---
    author: agent/claude
    created: 2026-09-17T15:22:33Z
    ---

    Acceptance output:

    $ go test ./internal/gitx -v
    (all tests PASS; paste the real output here)

    $ go build ./...
    (no output, exit 0)

    $ go vet ./...
    (no output, exit 0)

    $ gofmt -l internal/gitx
    (no output)
    ```
  - Append the ref `../comments/AWIT-0ND56C3G/<file>.md` to this ticket's `refs` list (forward slashes, block style, after the two plan refs).
  - Commit:
    ```bash
    git add .awit/items/AWIT-0ND56C3G.md .awit/comments/AWIT-0ND56C3G
    git commit -m "tickets: close AWIT-0ND56C3G"
    ```

## Acceptance Criteria
- `go test ./internal/gitx -v` → all ten tests `PASS` (or uniformly `SKIP` with `git not installed` on a machine without git), final line `ok  	github.com/eisenwinter/awit/internal/gitx`, exit code `0`.
- `go test ./internal/gitx -run TestCommitOnlyStagesGivenPaths -v` → `PASS`; the test proves `git log -1 --pretty=%s` is `awit: test` and `git status --porcelain` is exactly `?? b.txt`.
- `go test ./internal/gitx -run TestBranchDetached -v` → `PASS`; `Branch` returns `""` for a detached HEAD.
- `go build ./...` → no output, exit code `0`.
- `go vet ./...` → no output, exit code `0`.
- `gofmt -l internal/gitx` → no output.
- `internal/gitx/gitx.go` exports exactly `Branch`, `UserName`, `Root`, `Commit`: `grep -c '^func [A-Z]' internal/gitx/gitx.go` → `4`.
- The package imports no third-party module: `go list -deps ./internal/gitx | grep -c eisenwinter` → `1` (only the package itself), and `grep -c 'urfave\|yaml' internal/gitx/gitx.go` → `0` with exit code `1`.

## Out of scope
- `id.Worker` / `id.WorkerFor` and any hashing of the branch name — `AWIT-0ND5693G` (`pkg/id`).
- `Store.Mint`, `Store.Save` and every other `pkg/item` symbol that will call into this package — `AWIT-0ND56E3G`.
- The `next --claim` flow (frontmatter write, `claimed_at`, `--no-commit`, the `awit: claim <id>` message) — `AWIT-0ND56X3G`. This ticket provides `Commit`; it does not build a message or decide when to call it.
- Comment author resolution (`--author` → `AWIT_AGENT` → `config.agent_id` → `UserName`) — `AWIT-0ND56Y3G` for the command, `AWIT-0ND56A3G` for `config.Agent`.
- Any CLI command, flag, or `internal/cli` file. `internal/gitx` has no knowledge of urfave/cli.
- Extra git operations: no `push`, `pull`, `status` parsing, `diff`, branch creation, stash, or worktree discovery beyond `Root`. Guide §4.5 lists four functions; four is the whole surface.
- Caching results across calls, or a package-level `Runner` interface to mock git. The tests drive the real binary, which is the only thing that proves the argument order is right.
- CI configuration for the new package — `AWIT-0ND56B3G` already runs `go test ./...` on both operating systems.
