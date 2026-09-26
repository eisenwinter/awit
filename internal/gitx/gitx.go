// Package gitx wraps the git command line. It is the only package in awit that
// shells out to git; nothing links a git library (spec Tech Stack).
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
// must keep working outside a repository (spec §2, ID Layout).
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
// author-resolution chain fall through to its own error (author rules: schema.md).
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

// errNoPaths guards against an empty pathspec. `git commit -m msg --` with no
// paths commits everything already in the index, which would turn a claim
// commit into a commit of unrelated staged work.
var errNoPaths = errors.New("gitx: commit needs at least one path")

// Commit stages the given paths (relative to or absolute within root) and
// commits only them.
//
// Paths normally arrive as item.Path, which is absolute. Git
// accepts absolute pathspecs inside the worktree and resolves them against the
// worktree root, so no filepath.Rel conversion is needed - and none should be
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
