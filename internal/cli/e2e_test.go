package cli

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/item"
)

func loopGitAvailable(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
}

func loopGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func initLoopRepo(t *testing.T) string {
	t.Helper()
	loopGitAvailable(t)
	dir := copyFixture(t, "loop")
	loopGit(t, dir, "init", "-q")
	loopGit(t, dir, "config", "user.name", "tester")
	loopGit(t, dir, "config", "user.email", "tester@example.com")
	loopGit(t, dir, "add", ".")
	loopGit(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func loopMustRun(t *testing.T, args ...string) string {
	t.Helper()
	code, stdout, stderr := run(t, args...)
	if code != 0 {
		t.Fatalf("awit %s: exit %d stderr %q stdout %q", args, code, stderr, stdout)
	}
	if stderr != "" {
		t.Fatalf("awit %s: stderr %q", args, stderr)
	}
	return stdout
}

func TestAgentLoop(t *testing.T) {
	dir := initLoopRepo(t)
	repo := []string{"--repo", dir}

	start := loopMustRun(t, append(repo, "prime")...)
	golden(t, "e2e_prime_start.golden", []byte(start))

	nextOut := loopMustRun(t, append(repo, "--format", "compact", "next", "--claim", "--agent", "claude", "--seed", "7")...)
	if !strings.HasPrefix(nextOut, "[AWIT-TEST0001]") {
		t.Fatalf("next --claim = %q, want 0001", nextOut)
	}
	claimed := readItem(t, dir, "AWIT-TEST0001")
	if claimed.Status != item.StatusInProgress || claimed.Assignee != "agent/claude" {
		t.Fatalf("claimed status=%q assignee=%q", claimed.Status, claimed.Assignee)
	}
	if got := loopGit(t, dir, "log", "-1", "--pretty=%s"); got != "awit: claim AWIT-TEST0001" {
		t.Fatalf("claim commit = %q", got)
	}

	full := loopMustRun(t, append(repo, "show", "--full", "AWIT-TEST0001")...)
	if !strings.Contains(full, "URL-safe base64 alphabet") {
		t.Fatalf("show --full missing spec token grammar:\n%s", full)
	}
	if !strings.Contains(full, "invalid_token") {
		t.Fatalf("show --full missing spec 401 body:\n%s", full)
	}

	_ = loopMustRun(t, append(repo, "comment", "--author", "claude", "AWIT-TEST0001", "loop", "research")...)
	refsOnly := loopMustRun(t, append(repo, "show", "--refs-only", "AWIT-TEST0001")...)
	if !strings.Contains(refsOnly, "../../docs/spec.md") {
		t.Fatalf("refs-only missing spec ref:\n%s", refsOnly)
	}
	if !strings.Contains(refsOnly, "../comments/AWIT-TEST0001/") {
		t.Fatalf("refs-only missing comment ref:\n%s", refsOnly)
	}
	if strings.Contains(refsOnly, "\\") {
		t.Fatalf("refs-only contains a backslash:\n%s", refsOnly)
	}
	nRefs := 0
	if strings.Contains(refsOnly, "../../docs/spec.md") {
		nRefs++
	}
	if strings.Contains(refsOnly, "../comments/AWIT-TEST0001/") {
		nRefs++
	}
	if nRefs != 2 {
		t.Fatalf("refs-only should mention 2 refs, got %d\n%s", nRefs, refsOnly)
	}
	afterComment := readItem(t, dir, "AWIT-TEST0001")
	if len(afterComment.Refs) != 2 {
		t.Fatalf("item refs = %v, want 2", afterComment.Refs)
	}

	_ = loopMustRun(t, append(repo, "close", "AWIT-TEST0001")...)
	closed := readItem(t, dir, "AWIT-TEST0001")
	if closed.Status != item.StatusClosed {
		t.Fatalf("status after close = %q", closed.Status)
	}

	after := loopMustRun(t, append(repo, "prime")...)
	golden(t, "e2e_prime_after_0001.golden", []byte(after))

	val := loopMustRun(t, append(repo, "--format", "compact", "validate")...)
	if val != "PASS  3 items, 0 quarantined\n" {
		t.Fatalf("validate after 0001 close = %q", val)
	}

	next2 := loopMustRun(t, append(repo, "--format", "compact", "next", "--claim", "--agent", "claude", "--seed", "7")...)
	if !strings.HasPrefix(next2, "[AWIT-TEST0002]") {
		t.Fatalf("next 2 = %q, want 0002", next2)
	}
	_ = loopMustRun(t, append(repo, "close", "AWIT-TEST0002")...)

	next3 := loopMustRun(t, append(repo, "--format", "compact", "next", "--claim", "--agent", "claude", "--seed", "7")...)
	if !strings.HasPrefix(next3, "[AWIT-TEST0003]") {
		t.Fatalf("next 3 = %q, want 0003", next3)
	}
	_ = loopMustRun(t, append(repo, "close", "AWIT-TEST0003")...)

	code, stdout, stderr := run(t, append(repo, "--format", "compact", "next", "--seed", "7")...)
	if code != 1 || stdout != "" || stderr != "No ready items\n" {
		t.Fatalf("final next: exit %d stdout %q stderr %q", code, stdout, stderr)
	}

	drained := loopMustRun(t, append(repo, "prime")...)
	if !strings.Contains(drained, "=== READY (0) ===") {
		t.Fatalf("drained prime missing READY (0):\n%s", drained)
	}
	if !strings.Contains(drained, "=== BLOCKED (0) ===") {
		t.Fatalf("drained prime missing BLOCKED (0):\n%s", drained)
	}
	if strings.Contains(drained, "CRITICAL PATH") {
		t.Fatalf("drained prime still has CRITICAL PATH:\n%s", drained)
	}

	val2 := loopMustRun(t, append(repo, "--format", "compact", "validate")...)
	if val2 != "PASS  3 items, 0 quarantined\n" {
		t.Fatalf("validate drained = %q", val2)
	}

	again := loopMustRun(t, append(repo, "prime")...)
	if drained != again {
		t.Fatalf("prime not deterministic\nfirst:\n%s\nsecond:\n%s", drained, again)
	}
}
