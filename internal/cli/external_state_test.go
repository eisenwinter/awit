package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/internal/glabx/glabxtest"
	"github.com/eisenwinter/awit/internal/teax/teaxtest"
	"github.com/eisenwinter/awit/pkg/item"
)

func stubIssueState(t *testing.T, stubDir string, n int) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(stubDir, fmt.Sprintf("repos_owner_repo_issues_%d.json", n)))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		State *string `json:"state"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.State == nil {
		return ""
	}
	return *m.State
}

func stubArgvLog(t *testing.T, stubDir string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(stubDir, "argv.log"))
	if err != nil {
		return nil
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func hasStatePatch(t *testing.T, stubDir, want string) bool {
	t.Helper()
	for _, line := range stubArgvLog(t, stubDir) {
		if strings.Contains(line, "PATCH") && strings.Contains(line, want) {
			return true
		}
	}
	return false
}

func TestExternalStateClosePushesClosed(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no warning on success", stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusClosed {
		t.Fatalf("local status = %q, want closed", got.Status)
	}
	if got := stubIssueState(t, stub, 127); got != "closed" {
		t.Fatalf("remote state = %q, want closed", got)
	}
}

func TestExternalStateReleasePushesOpen(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"closed","body":"body\n"}`)
	code, _, _ := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox", "--no-push")
	if code != 0 {
		t.Fatalf("setup close --no-push: exit %d", code)
	}
	// Reset the stub remote to closed to prove release reopens it.
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"closed","body":"body\n"}`)
	code, stdout, stderr := run(t, "--repo", repo, "release", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "reopened AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no warning", stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusOpen {
		t.Fatalf("local status = %q, want open", got.Status)
	}
	if got := stubIssueState(t, stub, 127); got != "open" {
		t.Fatalf("remote state = %q, want open", got)
	}
}

func TestExternalStateUpdateStatusClosedPushes(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
	code, stdout, stderr := run(t, "--repo", repo, "update", "AWIT-TEST0001", "--status", "closed", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "updated AWIT-TEST0001: status=closed\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if got := stubIssueState(t, stub, 127); got != "closed" {
		t.Fatalf("remote state = %q, want closed", got)
	}
}

func TestExternalStateUpdateStatusInProgressPushesOpen(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"closed","body":"body\n"}`)
	code, _, stderr := run(t, "--repo", repo, "update", "AWIT-TEST0001", "--status", "in_progress", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusInProgress {
		t.Fatalf("local status = %q, want in_progress", got.Status)
	}
	if got := stubIssueState(t, stub, 127); got != "open" {
		t.Fatalf("remote state = %q, want open for in_progress", got)
	}
}

func TestExternalStateNonStatusUpdateNeverPushes(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
	code, _, stderr := run(t, "--repo", repo, "update", "AWIT-TEST0001", "--title", "New", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if hasStatePatch(t, stub, "state=") {
		t.Fatalf("non-status update must not PATCH state: %v", stubArgvLog(t, stub))
	}
	if got := stubIssueState(t, stub, 127); got != "open" {
		t.Fatalf("remote state changed to %q", got)
	}
}

func TestExternalStateSameStatusRetryPushes(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
	code, _, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("setup close: exit %d stderr %q", code, stderr)
	}
	// Remote drifted back to open; repeating the explicit status retries the push.
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
	code, stdout, stderr := run(t, "--repo", repo, "update", "AWIT-TEST0001", "--status", "closed", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "updated AWIT-TEST0001: status=closed\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if got := stubIssueState(t, stub, 127); got != "closed" {
		t.Fatalf("remote state = %q, want closed after retry", got)
	}
}

func TestExternalStatePush403KeepsLocalExitZero(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	if err := os.WriteFile(filepath.Join(stub, "repos_owner_repo_issues_127.status"), []byte("403"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTeaIssue(t, stub, 127, `{"message":"Forbidden"}`)
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (local success); stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q, want ordinary confirmation", stdout)
	}
	want := "warning: AWIT-TEST0001 saved locally; external state push failed:"
	if !strings.Contains(stderr, want) {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if !strings.Contains(stderr, "403") {
		t.Fatalf("stderr = %q, want the HTTP status named", stderr)
	}
	if !strings.Contains(stderr, "retry with awit update AWIT-TEST0001 --status closed") {
		t.Fatalf("stderr = %q, want the retry hint", stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusClosed {
		t.Fatalf("local status = %q, want closed preserved", got.Status)
	}
}

func TestExternalStateReleaseFailureKeepsLocal(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"closed","body":"body\n"}`)
	code, _, _ := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox", "--no-push")
	if code != 0 {
		t.Fatalf("setup close: exit %d", code)
	}
	if err := os.WriteFile(filepath.Join(stub, "repos_owner_repo_issues_127.status"), []byte("403"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTeaIssue(t, stub, 127, `{"message":"Forbidden"}`)
	code, stdout, stderr := run(t, "--repo", repo, "release", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr %q", code, stderr)
	}
	if stdout != "reopened AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "retry with awit update AWIT-TEST0001 --status open") {
		t.Fatalf("stderr = %q, want open retry hint", stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusOpen {
		t.Fatalf("local status = %q, want open preserved", got.Status)
	}
}

func TestExternalStateLocalSaveFailureNeverPushes(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
	itemsDir := filepath.Join(repo, ".awit", "items")
	if err := os.Chmod(itemsDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(itemsDir, 0o755) })
	probe, err := os.CreateTemp(itemsDir, ".probe-*")
	if err == nil {
		_ = probe.Close()
		_ = os.Remove(probe.Name())
		t.Skip("filesystem does not honour read-only directory")
	}
	code, stdout, _ := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 1 {
		t.Fatalf("exit %d, want 1; stdout %q", code, stdout)
	}
	if strings.Contains(stdout, "closed") {
		t.Fatalf("must not confirm success: %q", stdout)
	}
	if hasStatePatch(t, stub, "state=") {
		t.Fatalf("failed local save must never push: %v", stubArgvLog(t, stub))
	}
}

func TestExternalStateNoPushNeverExecutesTea(t *testing.T) {
	repo := initRepo(t)
	teaxtest.HideTea(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--no-push")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if strings.Contains(stderr, "external state push") {
		t.Fatalf("stderr = %q, want no push warning under --no-push", stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusClosed {
		t.Fatalf("local status = %q", got.Status)
	}
}

func TestExternalStateNoPushWithMalformedMetadata(t *testing.T) {
	repo := initRepo(t)
	teaxtest.HideTea(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", "external: gitlab#42\n", []byte("body\n"))
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--no-push")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if strings.Contains(stderr, "external state push") {
		t.Fatalf("--no-push must skip even malformed metadata: %q", stderr)
	}
}

func TestExternalStateInvalidMetadataWarns(t *testing.T) {
	repo, _ := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", "external: gitlab#42\n", []byte("body\n"))
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "warning: AWIT-TEST0001 saved locally; external state push failed:") {
		t.Fatalf("stderr = %q, want malformed-metadata warning", stderr)
	}
	if !strings.Contains(stderr, "retry with awit update AWIT-TEST0001 --status closed") {
		t.Fatalf("stderr = %q, want retry hint", stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusClosed {
		t.Fatalf("local status = %q", got.Status)
	}
}

func TestExternalStateMissingTeaWarns(t *testing.T) {
	repo := initRepo(t)
	teaxtest.HideTea(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001")
	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "warning: AWIT-TEST0001 saved locally; external state push failed:") {
		t.Fatalf("stderr = %q, want missing-tea warning", stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusClosed {
		t.Fatalf("local status = %q", got.Status)
	}
}

func TestExternalStateUnlinkedNoPushNoWarning(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", "", []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"x"}`)
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want silence for unlinked items", stderr)
	}
	if hasStatePatch(t, stub, "state=") {
		t.Fatalf("unlinked close must not push: %v", stubArgvLog(t, stub))
	}
}

func TestExternalStateCloseWithReasonPreservesCommentAndPushes(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001",
		"--reason", "done", "--author", "jane", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	got := readItem(t, repo, "AWIT-TEST0001")
	if got.Status != item.StatusClosed || len(got.Refs) != 1 {
		t.Fatalf("status=%q refs=%v, want closed with one comment ref", got.Status, got.Refs)
	}
	if got := stubIssueState(t, stub, 127); got != "closed" {
		t.Fatalf("remote state = %q, want closed", got)
	}
}

func TestExternalStateUpdateStatusOpenPushesOpen(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"closed","body":"body\n"}`)
	code, stdout, stderr := run(t, "--repo", repo, "update", "AWIT-TEST0001", "--status", "open", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "updated AWIT-TEST0001: status=open\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if got := stubIssueState(t, stub, 127); got != "open" {
		t.Fatalf("remote state = %q, want open", got)
	}
}

func TestExternalStateNoPushWithStubMakesNoRequest(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--no-push", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want silence under --no-push", stderr)
	}
	if hasStatePatch(t, stub, "state=") {
		t.Fatalf("--no-push must make no remote request: %v", stubArgvLog(t, stub))
	}
	code, _, stderr = run(t, "--repo", repo, "update", "AWIT-TEST0001", "--status", "open", "--no-push", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("update --no-push: exit %d stderr %q", code, stderr)
	}
	if hasStatePatch(t, stub, "state=") {
		t.Fatalf("update --no-push must make no remote request: %v", stubArgvLog(t, stub))
	}
}

func TestExternalStateAmbiguousLinkWarns(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("one\n"))
	writeExternalItem(t, repo, "AWIT-TEST0002", giteaExt("owner/repo", 127), []byte("two\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"x"}`)
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "warning: AWIT-TEST0001 saved locally; external state push failed:") {
		t.Fatalf("stderr = %q, want ambiguous-link warning", stderr)
	}
	if !strings.Contains(stderr, "AWIT-TEST0001") || !strings.Contains(stderr, "AWIT-TEST0002") {
		t.Fatalf("stderr = %q, want both ids named", stderr)
	}
	if !strings.Contains(stderr, "retry with awit update AWIT-TEST0001 --status closed") {
		t.Fatalf("stderr = %q, want retry hint", stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusClosed {
		t.Fatalf("local status = %q", got.Status)
	}
}

// --- GitLab state routing (AWIT-0NJ6A0DG) ---

func TestExternalStateGitLabClosePushesClosed(t *testing.T) {
	repo, _, glab := gitlabCheckRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", gitlabExtYAML(gitlabSubProject, 127, gitlabIssuesURL), []byte("body\n"))
	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("T", quote("body\n"), "opened", gitlabIssuesURL, `[]`))
	// --tea-login names a login GitLab must ignore, not select.
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "no-such-login")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no warning on success", stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusClosed {
		t.Fatalf("local status = %q, want closed", got.Status)
	}
	if got := readGlabStubIssue(t, glab, gitlabSubIssueKey); got.State != "closed" {
		t.Fatalf("remote state = %q, want closed on the wire", got.State)
	}
}

func TestExternalStateGitLabReleasePushesOpen(t *testing.T) {
	repo, _, glab := gitlabCheckRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", gitlabExtYAML(gitlabSubProject, 127, gitlabIssuesURL), []byte("body\n"))
	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("T", quote("body\n"), "opened", gitlabIssuesURL, `[]`))
	code, _, _ := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--no-push")
	if code != 0 {
		t.Fatalf("setup close --no-push: exit %d", code)
	}
	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("T", quote("body\n"), "closed", gitlabIssuesURL, `[]`))
	code, stdout, stderr := run(t, "--repo", repo, "release", "AWIT-TEST0001")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "reopened AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no warning", stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusOpen {
		t.Fatalf("local status = %q, want open", got.Status)
	}
	if got := readGlabStubIssue(t, glab, gitlabSubIssueKey); got.State != "opened" {
		t.Fatalf("remote state = %q, want opened on the wire", got.State)
	}
}

func TestExternalStateGitLabUpdateTriggers(t *testing.T) {
	repo, _, glab := gitlabCheckRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", gitlabExtYAML(gitlabSubProject, 127, gitlabIssuesURL), []byte("body\n"))
	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("T", quote("body\n"), "opened", gitlabIssuesURL, `[]`))

	code, stdout, stderr := run(t, "--repo", repo, "update", "AWIT-TEST0001", "--status", "closed")
	if code != 0 {
		t.Fatalf("explicit closed: exit %d stderr %q", code, stderr)
	}
	if stdout != "updated AWIT-TEST0001: status=closed\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if got := readGlabStubIssue(t, glab, gitlabSubIssueKey); got.State != "closed" {
		t.Fatalf("remote state = %q, want closed", got.State)
	}

	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("T", quote("body\n"), "closed", gitlabIssuesURL, `[]`))
	code, _, stderr = run(t, "--repo", repo, "update", "AWIT-TEST0001", "--status", "in_progress")
	if code != 0 {
		t.Fatalf("explicit in_progress: exit %d stderr %q", code, stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusInProgress {
		t.Fatalf("local status = %q, want in_progress", got.Status)
	}
	if got := readGlabStubIssue(t, glab, gitlabSubIssueKey); got.State != "opened" {
		t.Fatalf("remote state = %q, want opened for in_progress", got.State)
	}

	// Same-status retry reopens a remote that drifted back.
	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("T", quote("body\n"), "closed", gitlabIssuesURL, `[]`))
	code, _, stderr = run(t, "--repo", repo, "update", "AWIT-TEST0001", "--status", "in_progress")
	if code != 0 {
		t.Fatalf("same-status retry: exit %d stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no warning on retry success", stderr)
	}
	if got := readGlabStubIssue(t, glab, gitlabSubIssueKey); got.State != "opened" {
		t.Fatalf("remote state = %q, want opened after retry", got.State)
	}

	// A non-status update never pushes.
	puts := countGlabPUTs(t, glab)
	code, _, stderr = run(t, "--repo", repo, "update", "AWIT-TEST0001", "--title", "New")
	if code != 0 {
		t.Fatalf("title update: exit %d stderr %q", code, stderr)
	}
	if n := countGlabPUTs(t, glab); n != puts {
		t.Fatalf("non-status update performed a remote PUT (%d -> %d)", puts, n)
	}
	if got := readGlabStubIssue(t, glab, gitlabSubIssueKey); got.State != "opened" {
		t.Fatalf("remote state changed to %q", got.State)
	}
}

func TestExternalStateGitLabPush403KeepsLocalExitZero(t *testing.T) {
	repo, _, glab := gitlabCheckRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", gitlabExtYAML(gitlabSubProject, 127, gitlabIssuesURL), []byte("body\n"))
	if err := os.WriteFile(filepath.Join(glab, gitlabSubIssueKey+".status"), []byte("403"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeGitLabIssue(t, glab, gitlabSubIssueKey, `{"message":"Forbidden"}`)
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (local success); stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q, want ordinary confirmation", stdout)
	}
	if !strings.Contains(stderr, "warning: AWIT-TEST0001 saved locally; external state push failed:") {
		t.Fatalf("stderr = %q, want the retry warning", stderr)
	}
	if !strings.Contains(stderr, "403") {
		t.Fatalf("stderr = %q, want the HTTP status named", stderr)
	}
	if !strings.Contains(stderr, "retry with awit update AWIT-TEST0001 --status closed") {
		t.Fatalf("stderr = %q, want the retry hint", stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusClosed {
		t.Fatalf("local status = %q, want closed preserved", got.Status)
	}
}

func TestExternalStateGitLabSchemaFailureWarns(t *testing.T) {
	repo, _, glab := gitlabCheckRepo(t)
	// Omit state from the PUT response so verification falls back to a
	// GET, whose payload is missing its description.
	t.Setenv("GLAB_STUB_PATCH_OMIT_STATE", "1")
	writeExternalItem(t, repo, "AWIT-TEST0001", gitlabExtYAML(gitlabSubProject, 127, gitlabIssuesURL), []byte("body\n"))
	writeGitLabIssue(t, glab, gitlabSubIssueKey, `{"iid":127,"title":"T","labels":[],"state":"opened","web_url":`+quote(gitlabIssuesURL)+`}`)
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001")
	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "warning: AWIT-TEST0001 saved locally; external state push failed:") {
		t.Fatalf("stderr = %q, want a schema-failure warning, never success", stderr)
	}
	if !strings.Contains(stderr, "missing description") {
		t.Fatalf("stderr = %q, want the schema failure named", stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusClosed {
		t.Fatalf("local status = %q, want closed preserved", got.Status)
	}
}

func TestExternalStateGitLabRetryAfterAuthRepair(t *testing.T) {
	repo, _, glab := gitlabCheckRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", gitlabExtYAML(gitlabSubProject, 127, gitlabIssuesURL), []byte("body\n"))
	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("T", quote("body\n"), "opened", gitlabIssuesURL, `[]`))
	if err := os.WriteFile(filepath.Join(glab, "user.json"), []byte(`{"username":"nobody"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001",
		"--reason", "done", "--author", "jane")
	if code != 0 {
		t.Fatalf("broken-auth close: exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "warning: AWIT-TEST0001 saved locally; external state push failed:") {
		t.Fatalf("stderr = %q, want the auth-failure warning", stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusClosed || len(got.Refs) != 1 {
		t.Fatalf("status=%q refs=%v, want closed with one reason comment", got.Status, got.Refs)
	}

	// Repairing auth and repeating the current status retries without a second comment.
	writeGitLabUser(t, glab)
	code, stdout, stderr = run(t, "--repo", repo, "update", "AWIT-TEST0001", "--status", "closed")
	if code != 0 {
		t.Fatalf("successful local save must remain success: %s", stderr)
	}
	if stdout != "updated AWIT-TEST0001: status=closed\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no warning after repair", stderr)
	}
	if got := readGlabStubIssue(t, glab, gitlabSubIssueKey); got.State != "closed" {
		t.Fatalf("remote state = %q, want closed on the wire", got.State)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusClosed || len(got.Refs) != 1 {
		t.Fatalf("status=%q refs=%v, want closed with still one reason comment", got.Status, got.Refs)
	}
}

func TestExternalStateGitLabMissingGlabWarns(t *testing.T) {
	repo := initRepo(t)
	glabxtest.HideGlab(t)
	teaDir := teaxtest.Install(t)
	writeTeaLogins(t, teaDir, [2]string{"sandbox", "https://forge.example"})
	writeExternalItem(t, repo, "AWIT-TEST0001", gitlabExtYAML(gitlabSubProject, 127, gitlabIssuesURL), []byte("body\n"))
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001")
	if code != 0 {
		t.Fatalf("exit %d, want 0; stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "glab executable not found") {
		t.Fatalf("stderr = %q, want the missing-glab warning (never a tea fallback)", stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusClosed {
		t.Fatalf("local status = %q", got.Status)
	}
}

func TestExternalStateGitLabInvalidMetadataWarns(t *testing.T) {
	repo, _, _ := gitlabCheckRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", "external: gitlab#42\n", []byte("body\n"))
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "warning: AWIT-TEST0001 saved locally; external state push failed:") {
		t.Fatalf("stderr = %q, want malformed-metadata warning", stderr)
	}
	if !strings.Contains(stderr, "retry with awit update AWIT-TEST0001 --status closed") {
		t.Fatalf("stderr = %q, want retry hint", stderr)
	}
}

func TestExternalStateGitLabNoPushSkipsAll(t *testing.T) {
	repo := initRepo(t)
	teaxtest.HideTea(t)
	glabxtest.HideGlab(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", gitlabExtYAML(gitlabSubProject, 127, gitlabIssuesURL), []byte("body\n"))
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--no-push")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if strings.Contains(stderr, "external state push") {
		t.Fatalf("stderr = %q, want no push warning under --no-push", stderr)
	}
	if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusClosed {
		t.Fatalf("local status = %q", got.Status)
	}
	// --no-push returns before metadata validation, even for malformed metadata.
	writeExternalItem(t, repo, "AWIT-TEST0002", "external: gitlab#42\n", []byte("body\n"))
	code, stdout, stderr = run(t, "--repo", repo, "close", "AWIT-TEST0002", "--no-push")
	if code != 0 {
		t.Fatalf("malformed --no-push: exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0002\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if strings.Contains(stderr, "external state push") {
		t.Fatalf("--no-push must skip even malformed metadata: %q", stderr)
	}
}

func writeExternalPushConfig(t *testing.T, repo, extra string) {
	t.Helper()
	writeDefaultLabels(t, repo, []byte("prefix: AWIT\nstale_claim: 2h\n"+extra))
}

func TestExternalPushPolicyConfigFalseKeepsCloseLocal(t *testing.T) {
	repo, stub := externalRepo(t)
	writeDefaultLabels(t, repo, []byte("prefix: AWIT\nstale_claim: 2h\nexternal_push: false\n"))
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
	code, _, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 0 || readItem(t, repo, "AWIT-TEST0001").Status != item.StatusClosed {
		t.Fatalf("local close failed: exit %d, stderr %q", code, stderr)
	}
	if len(stubArgvLog(t, stub)) != 0 {
		t.Fatal("config-disabled close executed tea")
	}
	if !strings.Contains(stderr, "skipped by config external_push: false") {
		t.Fatalf("missing config skip diagnostic: %q", stderr)
	}
}

func TestExternalPushPolicyConfigFalseKeepsReleaseAndUpdateLocal(t *testing.T) {
	t.Run("release", func(t *testing.T) {
		repo, stub := externalRepo(t)
		writeExternalPushConfig(t, repo, "external_push: false\n")
		writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
		writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"closed","body":"body\n"}`)
		code, _, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--no-push")
		if code != 0 {
			t.Fatalf("setup close --no-push: exit %d stderr %q", code, stderr)
		}
		code, stdout, stderr := run(t, "--repo", repo, "release", "AWIT-TEST0001", "--tea-login", "sandbox")
		if code != 0 {
			t.Fatalf("exit %d stderr %q", code, stderr)
		}
		if stdout != "reopened AWIT-TEST0001\n" {
			t.Fatalf("stdout = %q", stdout)
		}
		if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusOpen || got.Assignee != "" {
			t.Fatalf("release must still clear the claim: status=%q assignee=%q", got.Status, got.Assignee)
		}
		if len(stubArgvLog(t, stub)) != 0 {
			t.Fatalf("config-disabled release executed tea: %v", stubArgvLog(t, stub))
		}
		if !strings.Contains(stderr, "skipped by config external_push: false") {
			t.Fatalf("missing config skip diagnostic: %q", stderr)
		}
		if !strings.Contains(stderr, "push with awit update AWIT-TEST0001 --status open --push=true") {
			t.Fatalf("skip hint = %q", stderr)
		}
		if got := stubIssueState(t, stub, 127); got != "closed" {
			t.Fatalf("remote state = %q, want unchanged closed", got)
		}
	})
	t.Run("update status closed", func(t *testing.T) {
		repo, stub := externalRepo(t)
		writeExternalPushConfig(t, repo, "external_push: false\n")
		writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
		writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
		code, stdout, stderr := run(t, "--repo", repo, "update", "AWIT-TEST0001", "--status", "closed", "--tea-login", "sandbox")
		if code != 0 {
			t.Fatalf("exit %d stderr %q", code, stderr)
		}
		if stdout != "updated AWIT-TEST0001: status=closed\n" {
			t.Fatalf("stdout = %q", stdout)
		}
		if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusClosed {
			t.Fatalf("local status = %q", got.Status)
		}
		if len(stubArgvLog(t, stub)) != 0 {
			t.Fatalf("config-disabled update executed tea: %v", stubArgvLog(t, stub))
		}
		want := "warning: AWIT-TEST0001 saved locally; external state push skipped by config external_push: false; push with awit update AWIT-TEST0001 --status closed --push=true\n"
		if stderr != want {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
		if got := stubIssueState(t, stub, 127); got != "open" {
			t.Fatalf("remote state = %q, want unchanged open", got)
		}
	})
}

func TestExternalPushPolicyExplicitTrueOverridesConfigFalse(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalPushConfig(t, repo, "external_push: false\n")
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--push=true", "--no-push=false", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no skip warning when --push=true", stderr)
	}
	if got := stubIssueState(t, stub, 127); got != "closed" {
		t.Fatalf("remote state = %q, want closed", got)
	}
}

func TestExternalPushPolicySameStatusOverrideRetry(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalPushConfig(t, repo, "external_push: false\n")
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
	code, _, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("config-disabled close: exit %d stderr %q", code, stderr)
	}
	if got := stubIssueState(t, stub, 127); got != "open" {
		t.Fatalf("remote state = %q after skipped close", got)
	}
	code, stdout, stderr := run(t, "--repo", repo, "update", "AWIT-TEST0001", "--status", "closed", "--push=true", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("override retry: exit %d stderr %q", code, stderr)
	}
	if stdout != "updated AWIT-TEST0001: status=closed\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no warning on explicit retry", stderr)
	}
	if got := stubIssueState(t, stub, 127); got != "closed" {
		t.Fatalf("remote state = %q, want closed after --push=true retry", got)
	}
}

func TestExternalPushPolicyExplicitFalseAndNoPushAreSilent(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"push false", []string{"--push=false"}},
		{"no-push", []string{"--no-push"}},
		{"push false with config false", []string{"--push=false"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, stub := externalRepo(t)
			if strings.Contains(tc.name, "config false") {
				writeExternalPushConfig(t, repo, "external_push: false\n")
			}
			writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
			writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
			args := append([]string{"--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox"}, tc.args...)
			code, stdout, stderr := run(t, args...)
			if code != 0 {
				t.Fatalf("exit %d stderr %q", code, stderr)
			}
			if stdout != "closed AWIT-TEST0001\n" {
				t.Fatalf("stdout = %q", stdout)
			}
			if stderr != "" {
				t.Fatalf("explicit disable must be silent: %q", stderr)
			}
			if len(stubArgvLog(t, stub)) != 0 {
				t.Fatalf("explicit disable executed tea: %v", stubArgvLog(t, stub))
			}
			if got := readItem(t, repo, "AWIT-TEST0001"); got.Status != item.StatusClosed {
				t.Fatalf("local status = %q", got.Status)
			}
		})
	}
}

func TestExternalPushPolicyNoPushFalseIsNeutral(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalPushConfig(t, repo, "external_push: false\n")
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
	code, _, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--no-push=false", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if len(stubArgvLog(t, stub)) != 0 {
		t.Fatalf("--no-push=false must not override config false: %v", stubArgvLog(t, stub))
	}
	if !strings.Contains(stderr, "skipped by config external_push: false") {
		t.Fatalf("neutral --no-push=false must keep the config skip: %q", stderr)
	}
}

func TestExternalPushPolicyOmittedAndTrueStillPush(t *testing.T) {
	for _, extra := range []string{"", "external_push: true\n"} {
		name := "omitted"
		if extra != "" {
			name = "true"
		}
		t.Run(name, func(t *testing.T) {
			repo, stub := externalRepo(t)
			if extra != "" {
				writeExternalPushConfig(t, repo, extra)
			}
			writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
			writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
			code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox")
			if code != 0 {
				t.Fatalf("exit %d stderr %q", code, stderr)
			}
			if stdout != "closed AWIT-TEST0001\n" || stderr != "" {
				t.Fatalf("stdout = %q stderr = %q", stdout, stderr)
			}
			if got := stubIssueState(t, stub, 127); got != "closed" {
				t.Fatalf("remote state = %q, want closed", got)
			}
		})
	}
}

func TestExternalPushPolicyFlagConflicts(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		stderr string
	}{
		{"push true plus no-push", []string{"close", "AWIT-TEST0001", "--push=true", "--no-push"}, "--push and --no-push cannot be combined\n"},
		{"push false plus no-push", []string{"close", "AWIT-TEST0001", "--push=false", "--no-push"}, "--push and --no-push cannot be combined\n"},
		{"invalid value", []string{"close", "AWIT-TEST0001", "--push=yes"}, "invalid --push value \"yes\": use --push=true or --push=false\n"},
		{"invalid on status update", []string{"update", "AWIT-TEST0001", "--status", "closed", "--push=bogus"}, "invalid --push value \"bogus\": use --push=true or --push=false\n"},
		{"invalid on non-status update", []string{"update", "AWIT-TEST0001", "--title", "New", "--push=yes"}, "invalid --push value \"yes\": use --push=true or --push=false\n"},
		{"conflict on non-status update", []string{"update", "AWIT-TEST0001", "--title", "New", "--push=true", "--no-push"}, "--push and --no-push cannot be combined\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo, stub := externalRepo(t)
			writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
			writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
			before := itemFileBytes(t, repo)
			args := append([]string{"--repo", repo}, tc.args...)
			args = append(args, "--tea-login", "sandbox")
			code, stdout, stderr := run(t, args...)
			if code != 2 {
				t.Fatalf("exit %d, want 2 (stdout %q stderr %q)", code, stdout, stderr)
			}
			if stderr != tc.stderr {
				t.Fatalf("stderr = %q, want %q", stderr, tc.stderr)
			}
			after := itemFileBytes(t, repo)
			if after["AWIT-TEST0001.md"] != before["AWIT-TEST0001.md"] {
				t.Fatal("usage error must precede any item write")
			}
			if len(stubArgvLog(t, stub)) != 0 {
				t.Fatalf("usage error executed tea: %v", stubArgvLog(t, stub))
			}
		})
	}
}

func TestExternalPushPolicyNoLeakAcrossMainCalls(t *testing.T) {
	t.Run("explicit --push=false does not leak", func(t *testing.T) {
		repo, stub := externalRepo(t)
		writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
		writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
		code, _, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--push=false", "--tea-login", "sandbox")
		if code != 0 {
			t.Fatalf("first close: exit %d stderr %q", code, stderr)
		}
		if got := stubIssueState(t, stub, 127); got != "open" {
			t.Fatalf("first close must skip the push: remote=%q", got)
		}
		code, _, stderr = run(t, "--repo", repo, "update", "AWIT-TEST0001", "--status", "closed", "--tea-login", "sandbox")
		if code != 0 {
			t.Fatalf("second update: exit %d stderr %q", code, stderr)
		}
		if got := stubIssueState(t, stub, 127); got != "closed" {
			t.Fatalf("second call must push; remote=%q", got)
		}
	})
	t.Run("--no-push does not leak", func(t *testing.T) {
		repo, stub := externalRepo(t)
		writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
		writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
		code, _, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--no-push", "--tea-login", "sandbox")
		if code != 0 {
			t.Fatalf("first close: exit %d stderr %q", code, stderr)
		}
		code, _, stderr = run(t, "--repo", repo, "update", "AWIT-TEST0001", "--status", "closed", "--tea-login", "sandbox")
		if code != 0 {
			t.Fatalf("second update: exit %d stderr %q", code, stderr)
		}
		if got := stubIssueState(t, stub, 127); got != "closed" {
			t.Fatalf("second call must push; remote=%q", got)
		}
	})
	t.Run("--push=true does not override config false later", func(t *testing.T) {
		repo, stub := externalRepo(t)
		writeExternalPushConfig(t, repo, "external_push: false\n")
		writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("one\n"))
		writeExternalItem(t, repo, "AWIT-TEST0002", giteaExt("owner/repo", 128), []byte("two\n"))
		writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"one\n"}`)
		writeTeaIssue(t, stub, 128, `{"number":128,"title":"U","state":"open","body":"two\n"}`)
		code, _, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--push=true", "--tea-login", "sandbox")
		if code != 0 {
			t.Fatalf("first close: exit %d stderr %q", code, stderr)
		}
		if got := stubIssueState(t, stub, 127); got != "closed" {
			t.Fatalf("explicit true must push; remote=%q", got)
		}
		code, _, stderr = run(t, "--repo", repo, "close", "AWIT-TEST0002", "--tea-login", "sandbox")
		if code != 0 {
			t.Fatalf("second close: exit %d stderr %q", code, stderr)
		}
		if !strings.Contains(stderr, "skipped by config external_push: false") {
			t.Fatalf("second close must follow config: %q", stderr)
		}
		if got := stubIssueState(t, stub, 128); got != "open" {
			t.Fatalf("second close must not push; remote=%q", got)
		}
	})
}

func TestExternalPushPolicyUnlinkedAndNonStatusStaySilent(t *testing.T) {
	t.Run("unlinked close", func(t *testing.T) {
		repo, stub := externalRepo(t)
		writeExternalPushConfig(t, repo, "external_push: false\n")
		writeExternalItem(t, repo, "AWIT-TEST0001", "", []byte("body\n"))
		writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"x"}`)
		code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox")
		if code != 0 {
			t.Fatalf("exit %d stderr %q", code, stderr)
		}
		if stdout != "closed AWIT-TEST0001\n" {
			t.Fatalf("stdout = %q", stdout)
		}
		if stderr != "" {
			t.Fatalf("unlinked close must not emit a skip warning: %q", stderr)
		}
		if len(stubArgvLog(t, stub)) != 0 {
			t.Fatalf("unlinked close executed tea: %v", stubArgvLog(t, stub))
		}
	})
	t.Run("non-status update", func(t *testing.T) {
		repo, stub := externalRepo(t)
		writeExternalPushConfig(t, repo, "external_push: false\n")
		writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
		writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
		code, stdout, stderr := run(t, "--repo", repo, "update", "AWIT-TEST0001", "--title", "New", "--push=true", "--tea-login", "sandbox")
		if code != 0 {
			t.Fatalf("exit %d stderr %q", code, stderr)
		}
		if stdout != "updated AWIT-TEST0001: title=New\n" {
			t.Fatalf("stdout = %q", stdout)
		}
		if stderr != "" {
			t.Fatalf("non-status update must not emit a skip warning: %q", stderr)
		}
		if len(stubArgvLog(t, stub)) != 0 {
			t.Fatalf("--push=true must not turn a title update into a push: %v", stubArgvLog(t, stub))
		}
		if got := stubIssueState(t, stub, 127); got != "open" {
			t.Fatalf("remote state changed to %q", got)
		}
	})
}

func TestExternalPushPolicyMalformedAndAmbiguousConfigSkip(t *testing.T) {
	t.Run("malformed", func(t *testing.T) {
		repo, stub := externalRepo(t)
		writeExternalPushConfig(t, repo, "external_push: false\n")
		writeExternalItem(t, repo, "AWIT-TEST0001", "external: gitlab#42\n", []byte("body\n"))
		code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox")
		if code != 0 {
			t.Fatalf("exit %d stderr %q", code, stderr)
		}
		if stdout != "closed AWIT-TEST0001\n" {
			t.Fatalf("stdout = %q", stdout)
		}
		if !strings.Contains(stderr, "skipped by config external_push: false") {
			t.Fatalf("stderr = %q, want config skip", stderr)
		}
		if strings.Contains(stderr, "push failed") {
			t.Fatalf("config skip must not emit a second failure warning: %q", stderr)
		}
		if len(stubArgvLog(t, stub)) != 0 {
			t.Fatalf("config skip executed tea: %v", stubArgvLog(t, stub))
		}
	})
	t.Run("ambiguous", func(t *testing.T) {
		repo, stub := externalRepo(t)
		writeExternalPushConfig(t, repo, "external_push: false\n")
		writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("one\n"))
		writeExternalItem(t, repo, "AWIT-TEST0002", giteaExt("owner/repo", 127), []byte("two\n"))
		writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"x"}`)
		code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox")
		if code != 0 {
			t.Fatalf("exit %d stderr %q", code, stderr)
		}
		if stdout != "closed AWIT-TEST0001\n" {
			t.Fatalf("stdout = %q", stdout)
		}
		if !strings.Contains(stderr, "skipped by config external_push: false") {
			t.Fatalf("stderr = %q, want config skip before duplicate-link validation", stderr)
		}
		if strings.Contains(stderr, "ambiguous") {
			t.Fatalf("config skip must precede duplicate-link validation: %q", stderr)
		}
		if len(stubArgvLog(t, stub)) != 0 {
			t.Fatalf("config skip executed tea: %v", stubArgvLog(t, stub))
		}
	})
}

func TestExternalPushPolicyCloseReasonPreserved(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalPushConfig(t, repo, "external_push: false\n")
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001",
		"--reason", "done", "--author", "jane", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	got := readItem(t, repo, "AWIT-TEST0001")
	if got.Status != item.StatusClosed || len(got.Refs) != 1 {
		t.Fatalf("status=%q refs=%v, want closed with one comment ref", got.Status, got.Refs)
	}
	if !strings.Contains(stderr, "skipped by config external_push: false") {
		t.Fatalf("stderr = %q, want config skip", stderr)
	}
	if len(stubArgvLog(t, stub)) != 0 {
		t.Fatalf("config-disabled close-with-reason executed tea: %v", stubArgvLog(t, stub))
	}
}

func TestExternalPushPolicyGitLabConfigFalseAndOverride(t *testing.T) {
	repo, _, glab := gitlabCheckRepo(t)
	writeExternalPushConfig(t, repo, "external_push: false\n")
	writeExternalItem(t, repo, "AWIT-TEST0001", gitlabExtYAML(gitlabSubProject, 127, gitlabIssuesURL), []byte("body\n"))
	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("T", quote("body\n"), "opened", gitlabIssuesURL, `[]`))

	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001")
	if code != 0 {
		t.Fatalf("config-disabled close: exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "skipped by config external_push: false") {
		t.Fatalf("stderr = %q, want config skip", stderr)
	}
	if len(stubArgvLog(t, glab)) != 0 {
		t.Fatalf("config-disabled close executed glab: %v", stubArgvLog(t, glab))
	}
	if got := readGlabStubIssue(t, glab, gitlabSubIssueKey); got.State != "opened" {
		t.Fatalf("remote state = %q, want opened", got.State)
	}

	code, stdout, stderr = run(t, "--repo", repo, "update", "AWIT-TEST0001", "--status", "closed", "--push=true")
	if code != 0 {
		t.Fatalf("override retry: exit %d stderr %q", code, stderr)
	}
	if stdout != "updated AWIT-TEST0001: status=closed\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want no warning on explicit retry", stderr)
	}
	if got := readGlabStubIssue(t, glab, gitlabSubIssueKey); got.State != "closed" {
		t.Fatalf("remote state = %q, want closed after --push=true", got.State)
	}

	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("T", quote("body\n"), "closed", gitlabIssuesURL, `[]`))
	puts := countGlabPUTs(t, glab)
	code, _, stderr = run(t, "--repo", repo, "update", "AWIT-TEST0001", "--title", "New")
	if code != 0 {
		t.Fatalf("title update: exit %d stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("non-status update must stay silent: %q", stderr)
	}
	if n := countGlabPUTs(t, glab); n != puts {
		t.Fatalf("non-status update performed a remote PUT (%d -> %d)", puts, n)
	}
}

func TestExternalPushPolicyGitLabExplicitFalseSilent(t *testing.T) {
	repo, _, glab := gitlabCheckRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", gitlabExtYAML(gitlabSubProject, 127, gitlabIssuesURL), []byte("body\n"))
	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("T", quote("body\n"), "opened", gitlabIssuesURL, `[]`))
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--push=false")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Fatalf("--push=false must be silent: %q", stderr)
	}
	if len(stubArgvLog(t, glab)) != 0 {
		t.Fatalf("--push=false executed glab: %v", stubArgvLog(t, glab))
	}
	if got := readGlabStubIssue(t, glab, gitlabSubIssueKey); got.State != "opened" {
		t.Fatalf("remote state = %q, want opened", got.State)
	}
}

func TestExternalPushPolicyCommitFalseStillPushes(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalPushConfig(t, repo, "commit: false\n")
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "closed AWIT-TEST0001\n" || stderr != "" {
		t.Fatalf("stdout = %q stderr = %q", stdout, stderr)
	}
	if got := stubIssueState(t, stub, 127); got != "closed" {
		t.Fatalf("commit: false must not disable state pushing; remote=%q", got)
	}
}

func TestExternalPushPolicyDoesNotAffectClaimCommit(t *testing.T) {
	gitLookPath(t)
	dir := gitClaimRepo(t, "external_push: false\n")
	code, _, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--agent", "test", "--seed", "1")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if subject := gitRun(t, dir, "log", "-1", "--pretty=%s"); subject != "awit: claim AWIT-TEST0001" {
		t.Fatalf("external_push: false must not alter claim commits; HEAD = %q", subject)
	}
}

func TestExternalPushPolicyPushBodyUnaffected(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalPushConfig(t, repo, "external_push: false\n")
	body := []byte("keep me\n")
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), body)
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"old"}`)
	code, stdout, stderr := run(t, "--repo", repo, "external", "push-body", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("push-body: exit %d stderr %q", code, stderr)
	}
	want := fmt.Sprintf("pushed body for AWIT-TEST0001 to https://forge.example/owner/repo/issues/127 (%d bytes)\n", len(body))
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestExternalPushPolicyLocalSaveFailureNeverPushes(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalPushConfig(t, repo, "external_push: false\n")
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
	itemsDir := filepath.Join(repo, ".awit", "items")
	if err := os.Chmod(itemsDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(itemsDir, 0o755) })
	probe, err := os.CreateTemp(itemsDir, ".probe-*")
	if err == nil {
		_ = probe.Close()
		_ = os.Remove(probe.Name())
		t.Skip("filesystem does not honour read-only directory")
	}
	code, stdout, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--push=true", "--tea-login", "sandbox")
	if code != 1 {
		t.Fatalf("exit %d, want 1; stdout %q stderr %q", code, stdout, stderr)
	}
	if strings.Contains(stdout, "closed") {
		t.Fatalf("must not confirm success: %q", stdout)
	}
	if len(stubArgvLog(t, stub)) != 0 {
		t.Fatalf("failed local save must never push: %v", stubArgvLog(t, stub))
	}
}
