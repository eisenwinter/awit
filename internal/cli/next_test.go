package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
func TestNextPrintExactID(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "AWIT-TEST0002")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q stdout %q", code, stderr, stdout)
	}
	if !strings.HasPrefix(stdout, "[AWIT-TEST0002]") || !strings.Contains(stdout, "Update database migration scripts") {
		t.Fatalf("stdout = %q, want the TEST0002 line", stdout)
	}
}

func TestNextClaimExactIDNoCommit(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, _, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--no-commit", "--agent", "claude", "AWIT-TEST0002")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	got := readItem(t, dir, "AWIT-TEST0002")
	if got.Status != item.StatusInProgress || got.Assignee != "agent/claude" || got.ClaimedAt == nil {
		t.Fatalf("status=%q assignee=%q claimed_at=%v", got.Status, got.Assignee, got.ClaimedAt)
	}
	top := readItem(t, dir, "AWIT-TEST0001")
	if top.Status != item.StatusOpen || top.Assignee != "" {
		t.Fatalf("top pick must be untouched: status=%q assignee=%q", top.Status, top.Assignee)
	}
}

func TestNextClaimExactIDCommits(t *testing.T) {
	gitLookPath(t)
	dir := copyFixture(t, "clean")
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.name", "tester")
	gitRun(t, dir, "config", "user.email", "tester@example.com")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-q", "-m", "init")

	code, _, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--agent", "claude", "AWIT-TEST0002")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if subject := gitRun(t, dir, "log", "-1", "--pretty=%s"); subject != "awit: claim AWIT-TEST0002" {
		t.Fatalf("subject = %q, want %q", subject, "awit: claim AWIT-TEST0002")
	}
}

func TestNextClaimIDRefusals(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
		id      string
		stderr  string
	}{
		{"blocked", "clean", "AWIT-TEST0003", "AWIT-TEST0003 is blocked by AWIT-TEST0001\n"},
		{"closed", "clean", "AWIT-TEST0005", "AWIT-TEST0005 is closed; awit release AWIT-TEST0005 to reopen it\n"},
		{"quarantined", "cyclic", "AWIT-TEST0001", quarantineWarning(4) + "AWIT-TEST0001 is quarantined [CYCLE]; run awit validate\n"},
		{"unknown", "clean", "AWIT-TEST0099", "Error: unknown item AWIT-TEST0099\n"},
		{"claimed", "clean", "AWIT-TEST0006", "AWIT-TEST0006 is claimed by agent/claude; awit release AWIT-TEST0006\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := copyFixture(t, tc.fixture)
			code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--no-commit", "--agent", "claude", tc.id)
			if code != 1 || stdout != "" || stderr != tc.stderr {
				t.Fatalf("exit %d stdout %q stderr %q, want stderr %q", code, stdout, stderr, tc.stderr)
			}
		})
	}
}

func TestNextIDHelp(t *testing.T) {
	code, stdout, stderr := run(t, "next", "--help")
	if code != 0 {
		t.Fatalf("next --help exit = %d, want 0 (stderr %q)", code, stderr)
	}
	for _, want := range []string{
		"Print the top unblocked item, or [id], optionally claiming it",
		"claim [id] or the pick: sets in_progress, commits (needs --agent or AWIT_AGENT)",
		"[id]",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("next --help missing %q; got:\n%s", want, stdout)
		}
	}
	code, stdout, stderr = run(t, "--help")
	if code != 0 {
		t.Fatalf("--help exit = %d, want 0 (stderr %q)", code, stderr)
	}
	want := "awit next --claim <id>              claim that exact item when ready"
	if !strings.Contains(stdout, want) {
		t.Errorf("--help missing %q; got:\n%s", want, stdout)
	}
}

// gitClaimRepo copies the clean fixture into a real git repository with one
// initial commit. configExtra, when non-empty, is appended to
// .awit/config.yaml before that commit, so commit-policy tests observe real
// history and staging instead of a mocked commit helper.
func gitClaimRepo(t *testing.T, configExtra string) string {
	t.Helper()
	dir := copyFixture(t, "clean")
	if configExtra != "" {
		path := filepath.Join(dir, ".awit", "config.yaml")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, []byte(configExtra)...), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	gitRun(t, dir, "init", "-q")
	gitRun(t, dir, "config", "user.name", "tester")
	gitRun(t, dir, "config", "user.email", "tester@example.com")
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func TestCommitPolicyConfigFalseSkipsCommitAndStaging(t *testing.T) {
	gitLookPath(t)
	dir := gitClaimRepo(t, "commit: false\n")

	code, _, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--agent", "test", "--seed", "1")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if subject := gitRun(t, dir, "log", "-1", "--pretty=%s"); subject != "init" {
		t.Fatalf("config commit: false must not add a commit; HEAD = %q", subject)
	}
	if staged := gitRun(t, dir, "diff", "--cached", "--name-only"); staged != "" {
		t.Fatalf("config commit: false must skip staging too; staged = %q", staged)
	}
	got := readItem(t, dir, "AWIT-TEST0001")
	if got.Status != item.StatusInProgress || got.Assignee != "agent/test" {
		t.Fatalf("claim must still be written: status=%q assignee=%q", got.Status, got.Assignee)
	}
}

func TestCommitPolicyExplicitTrueOverridesConfigFalse(t *testing.T) {
	gitLookPath(t)
	dir := gitClaimRepo(t, "commit: false\n")

	// --no-commit=false is neutral, so it neither conflicts with --commit
	// nor overrides the config; explicit true wins and the claim commits.
	code, _, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--commit=true", "--no-commit=false", "--agent", "test", "AWIT-TEST0002")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if subject := gitRun(t, dir, "log", "-1", "--pretty=%s"); subject != "awit: claim AWIT-TEST0002" {
		t.Fatalf("subject = %q, want %q", subject, "awit: claim AWIT-TEST0002")
	}
	files := gitRun(t, dir, "show", "--name-only", "--pretty=format:", "HEAD")
	if !strings.Contains(files, "AWIT-TEST0002.md") || strings.Contains(files, "TEST0001") {
		t.Fatalf("commit must touch only the claimed item; files = %q", files)
	}
	if staged := gitRun(t, dir, "diff", "--cached", "--name-only"); staged != "" {
		t.Fatalf("commit must leave nothing staged; staged = %q", staged)
	}
}

func TestCommitPolicyExplicitFalseOverridesDefault(t *testing.T) {
	gitLookPath(t)
	dir := gitClaimRepo(t, "")

	code, _, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--commit=false", "--agent", "test", "--seed", "1")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if subject := gitRun(t, dir, "log", "-1", "--pretty=%s"); subject != "init" {
		t.Fatalf("--commit=false must not add a commit; HEAD = %q", subject)
	}
	if staged := gitRun(t, dir, "diff", "--cached", "--name-only"); staged != "" {
		t.Fatalf("--commit=false must skip staging too; staged = %q", staged)
	}
	if got := readItem(t, dir, "AWIT-TEST0001"); got.Status != item.StatusInProgress {
		t.Fatalf("status = %q, want in_progress", got.Status)
	}
}

func TestCommitPolicyNoCommitFalseIsNeutral(t *testing.T) {
	gitLookPath(t)
	dir := gitClaimRepo(t, "commit: false\n")

	code, _, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--no-commit=false", "--agent", "test", "AWIT-TEST0002")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if subject := gitRun(t, dir, "log", "-1", "--pretty=%s"); subject != "init" {
		t.Fatalf("--no-commit=false must stay neutral; HEAD = %q", subject)
	}
	if got := readItem(t, dir, "AWIT-TEST0002"); got.Status != item.StatusInProgress {
		t.Fatalf("status = %q, want in_progress", got.Status)
	}
}

func TestCommitPolicyFlagConflicts(t *testing.T) {
	gitLookPath(t)
	tests := []struct {
		name   string
		args   []string
		stderr string
	}{
		{"commit true plus no-commit", []string{"--claim", "--commit=true", "--no-commit", "--agent", "test", "AWIT-TEST0002"}, "--commit and --no-commit cannot be combined\n"},
		{"commit false plus no-commit", []string{"--claim", "--commit=false", "--no-commit", "--agent", "test"}, "--commit and --no-commit cannot be combined\n"},
		{"invalid value", []string{"--claim", "--commit=yes", "--agent", "test"}, "invalid --commit value \"yes\": use --commit=true or --commit=false\n"},
		{"invalid value without claim", []string{"--commit=yes"}, "invalid --commit value \"yes\": use --commit=true or --commit=false\n"},
		{"conflict without claim", []string{"--commit=true", "--no-commit"}, "--commit and --no-commit cannot be combined\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := gitClaimRepo(t, "")
			args := append([]string{"--repo", dir, "--format", "compact", "next"}, tc.args...)
			code, stdout, stderr := run(t, args...)
			if code != 2 {
				t.Fatalf("exit %d, want 2 (stdout %q stderr %q)", code, stdout, stderr)
			}
			if stderr != tc.stderr {
				t.Fatalf("stderr = %q, want %q", stderr, tc.stderr)
			}
			if subject := gitRun(t, dir, "log", "-1", "--pretty=%s"); subject != "init" {
				t.Fatalf("usage error must precede any commit; HEAD = %q", subject)
			}
			if got := readItem(t, dir, "AWIT-TEST0001"); got.Status != item.StatusOpen || got.Assignee != "" {
				t.Fatalf("usage error must precede any write: status=%q assignee=%q", got.Status, got.Assignee)
			}
		})
	}
}

func TestCommitPolicyNoLeakAcrossMainCalls(t *testing.T) {
	gitLookPath(t)

	// The command tree is reused across Main calls, so an explicit policy
	// flag on one call must not bleed into later calls that omit it.
	assertHEAD := func(t *testing.T, dir, want string) {
		t.Helper()
		if subject := gitRun(t, dir, "log", "-1", "--pretty=%s"); subject != want {
			t.Fatalf("HEAD = %q, want %q", subject, want)
		}
	}

	t.Run("explicit --commit=false does not leak", func(t *testing.T) {
		dir := gitClaimRepo(t, "")
		code, _, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--commit=false", "--agent", "test", "--seed", "1")
		if code != 0 || stderr != "" {
			t.Fatalf("first claim: exit %d stderr %q", code, stderr)
		}
		code, _, stderr = run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--agent", "test", "AWIT-TEST0002")
		if code != 0 || stderr != "" {
			t.Fatalf("second claim: exit %d stderr %q", code, stderr)
		}
		assertHEAD(t, dir, "awit: claim AWIT-TEST0002")
	})

	t.Run("--no-commit does not leak", func(t *testing.T) {
		dir := gitClaimRepo(t, "")
		code, _, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--no-commit", "--agent", "test", "--seed", "1")
		if code != 0 || stderr != "" {
			t.Fatalf("first claim: exit %d stderr %q", code, stderr)
		}
		code, _, stderr = run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--agent", "test", "AWIT-TEST0002")
		if code != 0 || stderr != "" {
			t.Fatalf("second claim: exit %d stderr %q", code, stderr)
		}
		assertHEAD(t, dir, "awit: claim AWIT-TEST0002")
	})

	t.Run("--commit=true does not override config false later", func(t *testing.T) {
		dir := gitClaimRepo(t, "commit: false\n")
		code, _, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--commit=true", "--agent", "test", "--seed", "1")
		if code != 0 || stderr != "" {
			t.Fatalf("first claim: exit %d stderr %q", code, stderr)
		}
		code, _, stderr = run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--agent", "test", "AWIT-TEST0002")
		if code != 0 || stderr != "" {
			t.Fatalf("second claim: exit %d stderr %q", code, stderr)
		}
		assertHEAD(t, dir, "awit: claim AWIT-TEST0001")
	})
}

func TestCommitPolicyWithoutClaimWritesNothing(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--commit=false", "--seed", "1")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q stdout %q", code, stderr, stdout)
	}
	if !strings.HasPrefix(stdout, "[AWIT-TEST0001]") {
		t.Fatalf("stdout = %q, want the TEST0001 pick", stdout)
	}
	if got := readItem(t, dir, "AWIT-TEST0001"); got.Status != item.StatusOpen || got.Assignee != "" {
		t.Fatalf("policy flag without --claim must not write: status=%q assignee=%q", got.Status, got.Assignee)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
		t.Fatalf("fixture must stay git-free: %v", err)
	}
}

func TestNextWhyUniqueMax(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--seed", "1", "--why")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stdout %q stderr %q)", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "[AWIT-TEST0001]") {
		t.Fatalf("stdout = %q, want the TEST0001 pick", stdout)
	}
	want := "why: AWIT-TEST0001; unblocks=2; critical-path=yes; selection=max-unblocks; tie-break=none\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

func TestNextWhySeededTie(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "Alpha", "A.", nil)
	seedItem(t, dir, "AWIT-TEST0002", "Beta", "B.", nil)

	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--seed", "1", "--why")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stdout %q stderr %q)", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "[AWIT-TEST") {
		t.Fatalf("stdout = %q, want a pick", stdout)
	}
	winner := strings.SplitN(strings.TrimPrefix(stdout, "["), "]", 2)[0]
	// Whole-graph critical path over two independent roots is the
	// smaller-ID node alone, so only TEST0001 is a member.
	cp := "no"
	if winner == "AWIT-TEST0001" {
		cp = "yes"
	}
	want := "why: " + winner + "; unblocks=0; critical-path=" + cp + "; selection=max-unblocks; tie-break=pcg(seed=1,candidates=2)\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

func TestNextWhyLabelFilteredTieSize(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "Alpha", "A.", []string{"a"})
	seedItem(t, dir, "AWIT-TEST0002", "Beta", "B.", []string{"a"})
	seedItem(t, dir, "AWIT-TEST0003", "Gamma", "C.", []string{"b"})

	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "-l", "a", "--seed", "1", "--why")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stdout %q stderr %q)", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "[AWIT-TEST0001]") && !strings.HasPrefix(stdout, "[AWIT-TEST0002]") {
		t.Fatalf("stdout = %q, want 0001 or 0002", stdout)
	}
	winner := strings.SplitN(strings.TrimPrefix(stdout, "["), "]", 2)[0]
	cp := "no"
	if winner == "AWIT-TEST0001" {
		cp = "yes"
	}
	want := "why: " + winner + "; unblocks=0; critical-path=" + cp + "; selection=max-unblocks; tie-break=pcg(seed=1,candidates=2)\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

func TestNextWhyCriticalMembership(t *testing.T) {
	dir := copyFixture(t, "clean")
	// 0002 is ready but off the 0001 -> 0003 -> 0004 critical path.
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "-l", "db", "--seed", "1", "--why")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stdout %q stderr %q)", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "[AWIT-TEST0002]") {
		t.Fatalf("stdout = %q, want the TEST0002 pick", stdout)
	}
	want := "why: AWIT-TEST0002; unblocks=0; critical-path=no; selection=max-unblocks; tie-break=none\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

func TestNextWhyExplicitBlocked(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "AWIT-TEST0003", "--why")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stdout %q stderr %q)", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "[AWIT-TEST0003]") {
		t.Fatalf("stdout = %q, want the TEST0003 line", stdout)
	}
	// Exact lookup never claims a ranking win, even for a blocked item.
	want := "why: AWIT-TEST0003; unblocks=1; critical-path=yes; selection=explicit; tie-break=none\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

func TestNextWhyNoCandidates(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "-l", "p0", "--seed", "1", "--why")
	if code != 1 || stdout != "" {
		t.Fatalf("exit %d stdout %q, want 1 and empty", code, stdout)
	}
	if stderr != "No ready items (labels: p0)\n" {
		t.Fatalf("stderr = %q, want no why line", stderr)
	}
}

func TestNextWhyFailedClaim(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--no-commit", "--agent", "claude", "AWIT-TEST0003", "--why")
	if code != 1 || stdout != "" {
		t.Fatalf("exit %d stdout %q, want 1 and empty", code, stdout)
	}
	if stderr != "AWIT-TEST0003 is blocked by AWIT-TEST0001\n" {
		t.Fatalf("stderr = %q, want the refusal with no why line", stderr)
	}
}

func TestNextWhyJSONStdoutIdentical(t *testing.T) {
	dir := copyFixture(t, "clean")
	code1, out1, err1 := run(t, "--repo", dir, "--format", "json", "next", "--seed", "42")
	if code1 != 0 || err1 != "" {
		t.Fatalf("plain: exit %d stderr %q", code1, err1)
	}
	code2, out2, err2 := run(t, "--repo", dir, "--format", "json", "next", "--seed", "42", "--why")
	if code2 != 0 {
		t.Fatalf("why: exit %d stderr %q", code2, err2)
	}
	if out2 != out1 {
		t.Fatalf("--why changed JSON stdout:\n%s\nvs\n%s", out2, out1)
	}
	var row struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(out2), &row); err != nil {
		t.Fatalf("json: %v\n%s", err, out2)
	}
	if row.ID != "AWIT-TEST0001" {
		t.Fatalf("row.ID = %q, want AWIT-TEST0001", row.ID)
	}
	want := "why: AWIT-TEST0001; unblocks=2; critical-path=yes; selection=max-unblocks; tie-break=none\n"
	if err2 != want {
		t.Fatalf("stderr = %q, want %q", err2, want)
	}
}

func TestNextWhySeedReplay(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "Alpha", "A.", nil)
	seedItem(t, dir, "AWIT-TEST0002", "Beta", "B.", nil)

	code, out1, err1 := run(t, "--repo", dir, "--format", "compact", "next", "--why")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stdout %q stderr %q)", code, out1, err1)
	}
	si := strings.Index(err1, "seed=")
	cj := strings.Index(err1, ",candidates=")
	end := strings.Index(err1, ")\n")
	if si < 0 || cj < 0 || end < 0 || !strings.Contains(err1, "selection=max-unblocks; tie-break=pcg(") {
		t.Fatalf("cannot parse seed from %q", err1)
	}
	seed, serr := strconv.ParseInt(err1[si+len("seed="):cj], 10, 64)
	if serr != nil {
		t.Fatalf("cannot parse seed from %q: %v", err1, serr)
	}
	k, kerr := strconv.Atoi(err1[cj+len(",candidates=") : end])
	if kerr != nil {
		t.Fatalf("cannot parse candidates from %q: %v", err1, kerr)
	}
	if seed == 0 || k != 2 {
		t.Fatalf("seed=%d candidates=%d, want nonzero seed and 2", seed, k)
	}
	code, out2, err2 := run(t, "--repo", dir, "--format", "compact", "next", "--seed", strconv.FormatInt(seed, 10))
	if code != 0 || err2 != "" {
		t.Fatalf("replay: exit %d stderr %q", code, err2)
	}
	if out2 != out1 {
		t.Fatalf("replay with seed %d picked %q, want %q", seed, out2, out1)
	}
}

func TestNextWhyClaim(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--no-commit", "--agent", "claude", "--seed", "1", "--why")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stdout %q stderr %q)", code, stdout, stderr)
	}
	if !strings.HasPrefix(stdout, "[AWIT-TEST0001]") {
		t.Fatalf("stdout = %q, want the TEST0001 pick", stdout)
	}
	want := "why: AWIT-TEST0001; unblocks=2; critical-path=yes; selection=max-unblocks; tie-break=none\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	got := readItem(t, dir, "AWIT-TEST0001")
	if got.Status != item.StatusInProgress || got.Assignee != "agent/claude" {
		t.Fatalf("claim missing: status=%q assignee=%q", got.Status, got.Assignee)
	}
}
