package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
