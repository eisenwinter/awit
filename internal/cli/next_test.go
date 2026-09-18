package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eisenwinter/awit/pkg/item"
)

func TestNextTopByUnblocks(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--seed", "1")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q stdout %q", code, stderr, stdout)
	}
	want := "[AWIT-TEST0001] open Implement OAuth2 bearer token extraction | auth,p1 | Unblocks: 2\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestNextSeededTieBreak(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "Alpha", "A.", nil)
	seedItem(t, dir, "AWIT-TEST0002", "Beta", "B.", nil)

	code1, out1, err1 := run(t, "--repo", dir, "--format", "compact", "next", "--seed", "1")
	if code1 != 0 || err1 != "" {
		t.Fatalf("seed 1: exit %d stderr %q", code1, err1)
	}
	code1b, out1b, err1b := run(t, "--repo", dir, "--format", "compact", "next", "--seed", "1")
	if code1b != 0 || err1b != "" {
		t.Fatalf("seed 1 repeat: exit %d stderr %q", code1b, err1b)
	}
	if out1 != out1b {
		t.Fatalf("same seed must be deterministic: %q vs %q", out1, out1b)
	}
	if !strings.HasPrefix(out1, "[AWIT-TEST0001]") && !strings.HasPrefix(out1, "[AWIT-TEST0002]") {
		t.Fatalf("seed 1 picked %q, want 0001 or 0002", out1)
	}

	_, out7, err7 := run(t, "--repo", dir, "--format", "compact", "next", "--seed", "7")
	if err7 != "" {
		t.Fatalf("seed 7 stderr %q", err7)
	}
	if !strings.HasPrefix(out7, "[AWIT-TEST0001]") && !strings.HasPrefix(out7, "[AWIT-TEST0002]") {
		t.Fatalf("seed 7 picked %q, want 0001 or 0002", out7)
	}
}

func TestNextLabelFilterEmpty(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "-l", "p0", "--seed", "1")
	if code != 1 {
		t.Fatalf("exit %d, want 1 stdout %q stderr %q", code, stdout, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if stderr != "No ready items (labels: p0)\n" {
		t.Fatalf("stderr = %q, want %q", stderr, "No ready items (labels: p0)\n")
	}
}

func TestNextClaimNoCommit(t *testing.T) {
	dir := copyFixture(t, "clean")
	before := time.Now().UTC().Add(-time.Second).Truncate(time.Second)
	code, _, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--no-commit", "--agent", "claude", "--seed", "1")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	got := readItem(t, dir, "AWIT-TEST0001")
	if got.Status != item.StatusInProgress {
		t.Fatalf("status = %q, want in_progress", got.Status)
	}
	if got.Assignee != "agent/claude" {
		t.Fatalf("assignee = %q, want agent/claude", got.Assignee)
	}
	if got.ClaimedAt == nil {
		t.Fatal("claimed_at is nil")
	}
	if got.ClaimedAt.Before(before) || got.ClaimedAt.After(time.Now().UTC().Add(time.Second)) {
		t.Fatalf("claimed_at = %v out of range", got.ClaimedAt)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Fatalf("fixture must not be a git repo; --no-commit must not require one: %v", err)
	}
}

func gitLookPath(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
}

func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func TestNextClaimCommits(t *testing.T) {
	gitLookPath(t)
	dir := copyFixture(t, "clean")
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.name", "tester")
	gitRun(t, dir, "config", "user.email", "tester@example.com")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-q", "-m", "init")

	code, _, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--agent", "claude", "--seed", "1")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	subject := gitRun(t, dir, "log", "-1", "--pretty=%s")
	if subject != "awit: claim AWIT-TEST0001" {
		t.Fatalf("subject = %q, want %q", subject, "awit: claim AWIT-TEST0001")
	}
	got := readItem(t, dir, "AWIT-TEST0001")
	if got.Status != item.StatusInProgress || got.Assignee != "agent/claude" {
		t.Fatalf("status=%q assignee=%q", got.Status, got.Assignee)
	}
	files := gitRun(t, dir, "show", "--name-only", "--pretty=format:", "HEAD")
	if !strings.Contains(files, "AWIT-TEST0001.md") {
		t.Fatalf("committed files = %q, want the claimed item", files)
	}
}

func TestNextClaimWithoutAgent(t *testing.T) {
	dir := copyFixture(t, "clean")
	t.Setenv("AWIT_AGENT", "")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--no-commit", "--seed", "1")
	if code != 1 {
		t.Fatalf("exit %d, want 1 stdout %q stderr %q", code, stdout, stderr)
	}
	if stderr != "Error: no agent identity; pass --agent or set AWIT_AGENT\n" {
		t.Fatalf("stderr = %q", stderr)
	}
	got := readItem(t, dir, "AWIT-TEST0001")
	if got.Status != item.StatusOpen || got.Assignee != "" {
		t.Fatalf("item was claimed without an agent: status=%q assignee=%q", got.Status, got.Assignee)
	}
}

func TestNextJSON(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "next", "--seed", "1")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	var row struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Status   string `json:"status"`
		State    string `json:"state"`
		Unblocks int    `json:"unblocks"`
	}
	if err := json.Unmarshal([]byte(stdout), &row); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	if row.ID != "AWIT-TEST0001" || row.State != "ready" || row.Unblocks != 2 {
		t.Fatalf("row = %+v, want 0001 ready unblocks 2", row)
	}
	if !strings.HasSuffix(stdout, "\n") {
		t.Fatal("json must end with a newline")
	}
}

func TestNextNoReadyUnfiltered(t *testing.T) {
	dir := initRepo(t)
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--seed", "1")
	if code != 1 || stdout != "" || stderr != "No ready items\n" {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}
