package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/internal/teax/teaxtest"
)

// writeExternalItem writes an item file with exact body bytes; extYAML is
// extra frontmatter (typically the external mapping) and may be empty.
func writeExternalItem(t *testing.T, repo, id, extYAML string, body []byte) {
	t.Helper()
	content := "---\nid: " + id + "\ntitle: " + id + "\nstatus: open\n" + extYAML + "---\n" + string(body)
	path := filepath.Join(repo, ".awit", "items", id+".md")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func giteaExt(repo string, n int) string {
	return fmt.Sprintf(`external:
  tracker: gitea
  repo: %s
  id: %d
  url: https://forge.example/%s/issues/%d
`, repo, n, repo, n)
}

// externalRepo is a repo with the tea stub installed and one sandbox login.
func externalRepo(t *testing.T) (repo, stubDir string) {
	t.Helper()
	repo = initRepo(t)
	stubDir = teaxtest.Install(t)
	writeTeaLogins(t, stubDir, [2]string{"sandbox", "https://forge.example"})
	return repo, stubDir
}

func itemFileBytes(t *testing.T, repo string) map[string]string {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(repo, ".awit", "items"))
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	for _, e := range ents {
		data, err := os.ReadFile(filepath.Join(repo, ".awit", "items", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		m[e.Name()] = string(data)
	}
	return m
}

func TestExternalCheckAllMixed(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("shared body\n"))
	writeExternalItem(t, repo, "AWIT-TEST0002", giteaExt("owner/repo", 128), []byte("local bytes\n"))
	writeExternalItem(t, repo, "AWIT-TEST0003", "external: gitlab#42\n", []byte("b\n"))
	writeExternalItem(t, repo, "AWIT-TEST0004", "", []byte("unlinked\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"shared body\n"}`)
	writeTeaIssue(t, stub, 128, `{"number":128,"title":"T","state":"open","body":"remote bytes\n"}`)
	before := itemFileBytes(t, repo)

	code, stdout, stderr := run(t, "--repo", repo, "external", "check", "--tea-login", "sandbox")
	if code != 1 {
		t.Fatalf("exit %d, want 1 (drift and error present); stdout %q stderr %q", code, stdout, stderr)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("want 3 rows + totals, got %d lines:\n%s", len(lines), stdout)
	}
	if lines[0] != "MATCH AWIT-TEST0001 https://forge.example/owner/repo/issues/127" {
		t.Fatalf("line 0 = %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "DRIFT AWIT-TEST0002 https://forge.example/owner/repo/issues/128") {
		t.Fatalf("line 1 = %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "ERROR AWIT-TEST0003: invalid external:") {
		t.Fatalf("line 2 = %q", lines[2])
	}
	if lines[3] != "Checked 3 items: 1 match, 1 drift, 1 error" {
		t.Fatalf("totals = %q", lines[3])
	}
	after := itemFileBytes(t, repo)
	for name, b := range before {
		if after[name] != b {
			t.Fatalf("check is read-only but %s changed", name)
		}
	}
}

func TestExternalCheckMatchExitZero(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("same\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"same\n"}`)
	code, stdout, stderr := run(t, "--repo", repo, "external", "check", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "MATCH AWIT-TEST0001 https://forge.example/owner/repo/issues/127\nChecked 1 items: 1 match, 0 drift, 0 error\n" {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestExternalCheckJSONPurity(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("same\n"))
	writeExternalItem(t, repo, "AWIT-TEST0002", giteaExt("owner/repo", 128), []byte("local\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"same\n"}`)
	writeTeaIssue(t, stub, 128, `{"number":128,"title":"T","state":"open","body":"remote\n"}`)
	code, stdout, stderr := run(t, "--repo", repo, "--format", "json", "external", "check", "--tea-login", "sandbox")
	if code != 1 {
		t.Fatalf("exit %d, want 1 (drift present); stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
	var rows []ExternalCheckRow
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("stdout must be a pure JSON array: %v\n%s", err, stdout)
	}
	if len(rows) != 2 || rows[0].Result != "match" || rows[1].Result != "drift" {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].ID != "AWIT-TEST0001" || rows[1].ID != "AWIT-TEST0002" {
		t.Fatalf("rows not in canonical-ID order: %+v", rows)
	}
	if rows[1].URL != "https://forge.example/owner/repo/issues/128" {
		t.Fatalf("drift row URL = %q", rows[1].URL)
	}
}

func TestExternalCheckNoLinkedItems(t *testing.T) {
	repo, _ := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", "", []byte("plain\n"))
	code, stdout, stderr := run(t, "--repo", repo, "external", "check", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "Checked 0 items: 0 match, 0 drift, 0 error\n" {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestExternalCheckSingleKeyDrift(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("mine\n"))
	writeExternalItem(t, repo, "AWIT-TEST0002", giteaExt("owner/repo", 128), []byte("other\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"theirs\n"}`)
	writeTeaIssue(t, stub, 128, `{"number":128,"title":"T","state":"open","body":"other\n"}`)
	// External-key selection must work too.
	code, stdout, _ := run(t, "--repo", repo, "external", "check", "owner/repo#127", "--tea-login", "sandbox")
	if code != 1 {
		t.Fatalf("exit %d, want 1 (drift)", code)
	}
	if !strings.HasPrefix(stdout, "DRIFT AWIT-TEST0001 ") {
		t.Fatalf("stdout = %q", stdout)
	}
	if strings.Contains(stdout, "TEST0002") {
		t.Fatalf("a single-key check must not touch other items: %q", stdout)
	}
	if !strings.Contains(stdout, "Checked 1 items: 0 match, 1 drift, 0 error") {
		t.Fatalf("totals missing: %q", stdout)
	}
}

func TestExternalCheckUnlinkedKeyErrors(t *testing.T) {
	repo, _ := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", "", []byte("plain\n"))
	code, _, stderr := run(t, "--repo", repo, "external", "check", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "AWIT-TEST0001") || !strings.Contains(stderr, "no external") {
		t.Fatalf("stderr = %q, want the unlinked item named", stderr)
	}
}

// An authentication/read failure is an error row, never body drift.
func TestExternalCheckReadErrorIsNotDrift(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("mine\n"))
	if err := os.WriteFile(filepath.Join(stub, "repos_owner_repo_issues_127.status"), []byte("404"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTeaIssue(t, stub, 127, `{"message":"Not Found"}`)
	code, stdout, _ := run(t, "--repo", repo, "external", "check", "--tea-login", "sandbox")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.HasPrefix(stdout, "ERROR AWIT-TEST0001 ") || strings.Contains(stdout, "DRIFT") {
		t.Fatalf("stdout = %q, want an ERROR row, not drift", stdout)
	}
	if !strings.Contains(stdout, "not found") {
		t.Fatalf("error row must carry the read failure: %q", stdout)
	}
}

func TestExternalPushBodyRoundTrip(t *testing.T) {
	repo, stub := externalRepo(t)
	body := []byte("line one\r\n\r\nunicode é 🚀\n\nlast, no newline")
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), body)
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"old"}`)
	before := itemFileBytes(t, repo)

	code, stdout, stderr := run(t, "--repo", repo, "external", "push-body", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("push-body: exit %d stderr %q", code, stderr)
	}
	want := fmt.Sprintf("pushed body for AWIT-TEST0001 to https://forge.example/owner/repo/issues/127 (%d bytes)\n", len(body))
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	// The remote now matches, so check reports MATCH.
	code, stdout, _ = run(t, "--repo", repo, "external", "check", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 0 || !strings.HasPrefix(stdout, "MATCH AWIT-TEST0001 ") {
		t.Fatalf("check after push: exit %d stdout %q", code, stdout)
	}
	// push-body never changes local content.
	after := itemFileBytes(t, repo)
	if after["AWIT-TEST0001.md"] != before["AWIT-TEST0001.md"] {
		t.Fatal("push-body modified the local item file")
	}
}

func TestExternalPushBodyRefusesDuplicates(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("one\n"))
	writeExternalItem(t, repo, "AWIT-TEST0002", giteaExt("owner/repo", 127), []byte("two\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"old"}`)
	code, _, stderr := run(t, "--repo", repo, "external", "push-body", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "AWIT-TEST0001") || !strings.Contains(stderr, "AWIT-TEST0002") {
		t.Fatalf("stderr = %q, want both duplicate ids", stderr)
	}
	if got := teaIssueBody(t, stub, 127); got != "old" {
		t.Fatalf("remote body changed despite refusal: %q", got)
	}
}

func teaIssueBody(t *testing.T, stubDir string, n int) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(stubDir, fmt.Sprintf("repos_owner_repo_issues_%d.json", n)))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Body *string `json:"body"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.Body == nil {
		return ""
	}
	return *m.Body
}

// tea exits zero on HTTP errors; a 403 must fail the push.
func TestExternalPushBodyRejects403ExitZero(t *testing.T) {
	repo, stub := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("x"))
	if err := os.WriteFile(filepath.Join(stub, "repos_owner_repo_issues_127.status"), []byte("403"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTeaIssue(t, stub, 127, `{"message":"Forbidden"}`)
	code, _, stderr := run(t, "--repo", repo, "external", "push-body", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "403") {
		t.Fatalf("stderr = %q, want the HTTP status named", stderr)
	}
}

// A server that acknowledges but never stored the body is a failure, not success.
func TestExternalPushBodyVerificationMismatch(t *testing.T) {
	repo, stub := externalRepo(t)
	t.Setenv("TEA_STUB_PATCH_NO_STORE", "1")
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("new\n"))
	writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"old"}`)
	code, _, stderr := run(t, "--repo", repo, "external", "push-body", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "verification") {
		t.Fatalf("stderr = %q, want a verification failure", stderr)
	}
	if got := teaIssueBody(t, stub, 127); got != "old" {
		t.Fatalf("remote body = %q, want untouched", got)
	}
}

func TestExternalPushBodyUnlinked(t *testing.T) {
	repo, _ := externalRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", "", []byte("plain\n"))
	code, _, stderr := run(t, "--repo", repo, "external", "push-body", "AWIT-TEST0001", "--tea-login", "sandbox")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "no external") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestExternalPushBodyUsageError(t *testing.T) {
	repo, _ := externalRepo(t)
	code, _, _ := run(t, "--repo", repo, "external", "push-body")
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}
