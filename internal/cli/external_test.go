package cli

import (
	"context"
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

func TestExternalDuplicateTrackerIdentity(t *testing.T) {
	glIssues := item.External{Tracker: "gitlab", Repo: "group/project", ID: 127,
		URL: "https://forge.example/group/project/-/issues/127"}
	glWork := item.External{Tracker: "gitlab", Repo: "group/project", ID: 127,
		URL: "https://forge.example/group/project/-/work_items/127"}
	gitea := item.External{Tracker: "gitea", Repo: "group/project", ID: 127,
		URL: "https://forge.example/group/project/issues/127"}
	glHost := item.External{Tracker: "gitlab", Repo: "group/project", ID: 127,
		URL: "https://other.example/group/project/-/issues/127"}
	glPrefix := item.External{Tracker: "gitlab", Repo: "group/project", ID: 127,
		URL: "https://forge.example/gitlab/group/project/-/issues/127"}
	glIID := item.External{Tracker: "gitlab", Repo: "group/project", ID: 128,
		URL: "https://forge.example/group/project/-/issues/128"}
	glRepo := item.External{Tracker: "gitlab", Repo: "other/project", ID: 127,
		URL: "https://forge.example/other/project/-/issues/127"}
	items := []*item.Item{
		{ID: "AWIT-TEST0001", External: &glIssues},
		{ID: "AWIT-TEST0002", External: &glWork},
		{ID: "AWIT-TEST0003", External: &gitea},
		{ID: "AWIT-TEST0004", External: &glHost},
		{ID: "AWIT-TEST0005", External: &glPrefix},
		{ID: "AWIT-TEST0006", External: &glIID},
		{ID: "AWIT-TEST0007", External: &glRepo},
		{ID: "AWIT-TEST0008"},
	}
	got := duplicateExternalLinks(items, glIssues)
	if strings.Join(got, ",") != "AWIT-TEST0001,AWIT-TEST0002" {
		t.Fatalf("gitlab issues identity = %v, want TEST0001 and TEST0002 (work_items spelling)", got)
	}
	got = duplicateExternalLinks(items, glWork)
	if strings.Join(got, ",") != "AWIT-TEST0001,AWIT-TEST0002" {
		t.Fatalf("gitlab work_items identity = %v", got)
	}
	got = duplicateExternalLinks(items, gitea)
	if strings.Join(got, ",") != "AWIT-TEST0003" {
		t.Fatalf("gitea identity = %v, want only TEST0003 (same host/repo/number, different tracker)", got)
	}
	got = duplicateExternalLinks(items, glHost)
	if strings.Join(got, ",") != "AWIT-TEST0004" {
		t.Fatalf("other host = %v, want only TEST0004", got)
	}
	got = duplicateExternalLinks(items, glPrefix)
	if strings.Join(got, ",") != "AWIT-TEST0005" {
		t.Fatalf("other prefix = %v, want only TEST0005", got)
	}
	got = duplicateExternalLinks(items, glIID)
	if strings.Join(got, ",") != "AWIT-TEST0006" {
		t.Fatalf("other iid = %v, want only TEST0006", got)
	}
	got = duplicateExternalLinks(items, glRepo)
	if strings.Join(got, ",") != "AWIT-TEST0007" {
		t.Fatalf("other repo = %v, want only TEST0007", got)
	}
}

// --- GitLab routing (AWIT-0NJ6A0DG) ---

// gitlabCheckRepo installs both portable stubs: tea with one sandbox login
// and glab with an authenticated user, all on the same forge host.
func gitlabCheckRepo(t *testing.T) (repo, teaDir, glabDir string) {
	t.Helper()
	repo = initRepo(t)
	teaDir = teaxtest.Install(t)
	writeTeaLogins(t, teaDir, [2]string{"sandbox", "https://forge.example"})
	glabDir = glabxtest.Install(t)
	writeGitLabUser(t, glabDir)
	return repo, teaDir, glabDir
}

type glabStubIssueData struct {
	Title       string   `json:"title"`
	Description *string  `json:"description"`
	Labels      []string `json:"labels"`
	State       string   `json:"state"`
	WebURL      string   `json:"web_url"`
}

func readGlabStubIssue(t *testing.T, dir, key string) glabStubIssueData {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, key+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var iss glabStubIssueData
	if err := json.Unmarshal(data, &iss); err != nil {
		t.Fatal(err)
	}
	return iss
}

const gitlabOtherURL = "https://forge.example/group/other/-/issues/5"
const gitlabOtherKey = "projects_group%2Fother_issues_5"
const gitlabEmptyURL = "https://forge.example/group/empty/-/issues/6"
const gitlabEmptyKey = "projects_group%2Fempty_issues_6"

func TestExternalCheckGitLab(t *testing.T) {
	repo, _, glab := gitlabCheckRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", gitlabExtYAML(gitlabSubProject, 127, gitlabIssuesURL), []byte("same\n"))
	writeExternalItem(t, repo, "AWIT-TEST0002", gitlabExtYAML("group/other", 5, gitlabOtherURL), []byte("local\n"))
	writeExternalItem(t, repo, "AWIT-TEST0003", gitlabExtYAML("group/empty", 6, gitlabEmptyURL), []byte(""))
	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("T", quote("same\n"), "opened", gitlabIssuesURL, `[]`))
	writeGitLabIssue(t, glab, gitlabOtherKey, `{"id":1,"iid":5,"title":"T","description":"remote\n","labels":[],"state":"opened","web_url":`+quote(gitlabOtherURL)+`}`)
	writeGitLabIssue(t, glab, gitlabEmptyKey, `{"id":2,"iid":6,"title":"T","description":null,"labels":[],"state":"opened","web_url":`+quote(gitlabEmptyURL)+`}`)
	before := itemFileBytes(t, repo)

	code, stdout, stderr := run(t, "--repo", repo, "external", "check")
	if code != 1 {
		t.Fatalf("exit %d, want 1 (drift present); stdout %q stderr %q", code, stdout, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty on a clean run", stderr)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("want 3 rows + totals, got %d lines:\n%s", len(lines), stdout)
	}
	if lines[0] != "MATCH AWIT-TEST0001 "+gitlabIssuesURL {
		t.Fatalf("line 0 = %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "DRIFT AWIT-TEST0002 "+gitlabOtherURL) {
		t.Fatalf("line 1 = %q", lines[1])
	}
	if lines[2] != "MATCH AWIT-TEST0003 "+gitlabEmptyURL {
		t.Fatalf("line 2 = %q (null remote description must match an empty local body)", lines[2])
	}
	if lines[3] != "Checked 3 items: 2 match, 1 drift, 0 error" {
		t.Fatalf("totals = %q", lines[3])
	}
	after := itemFileBytes(t, repo)
	for name, b := range before {
		if after[name] != b {
			t.Fatalf("check is read-only but %s changed", name)
		}
	}
}

// A broken GitLab auth is an error row, never body drift.
func TestExternalCheckGitLabAuthErrorIsNotDrift(t *testing.T) {
	repo, _, glab := gitlabCheckRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", gitlabExtYAML(gitlabSubProject, 127, gitlabIssuesURL), []byte("mine\n"))
	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("T", quote("mine\n"), "opened", gitlabIssuesURL, `[]`))
	if err := os.WriteFile(filepath.Join(glab, "user.json"), []byte(`{"username":"nobody"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, _ := run(t, "--repo", repo, "external", "check")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.HasPrefix(stdout, "ERROR AWIT-TEST0001 ") || strings.Contains(stdout, "DRIFT") {
		t.Fatalf("stdout = %q, want an ERROR row, not drift", stdout)
	}
	if !strings.Contains(stdout, "auth") {
		t.Fatalf("error row must name the auth failure: %q", stdout)
	}
}

func TestExternalCheckMixedTrackers(t *testing.T) {
	repo, tea, glab := gitlabCheckRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("shared\n"))
	writeExternalItem(t, repo, "AWIT-TEST0002", gitlabExtYAML(gitlabSubProject, 127, gitlabWorkItemsURL), []byte("shared\n"))
	writeTeaIssue(t, tea, 127, `{"number":127,"title":"T","state":"open","body":"shared\n"}`)
	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("T", quote("shared\n"), "opened", gitlabWorkItemsURL, `[]`))

	code, stdout, stderr := run(t, "--repo", repo, "external", "check", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d, want 0; stdout %q stderr %q", code, stdout, stderr)
	}
	want := "MATCH AWIT-TEST0001 https://forge.example/owner/repo/issues/127\n" +
		"MATCH AWIT-TEST0002 " + gitlabWorkItemsURL + "\n" +
		"Checked 2 items: 2 match, 0 drift, 0 error\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}

	code, stdout, stderr = run(t, "--repo", repo, "--format", "json", "external", "check", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("json exit %d stderr %q", code, stderr)
	}
	var rows []ExternalCheckRow
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("stdout must be a pure JSON array: %v\n%s", err, stdout)
	}
	if len(rows) != 2 || rows[0].ID != "AWIT-TEST0001" || rows[1].ID != "AWIT-TEST0002" {
		t.Fatalf("rows not in canonical-ID order: %+v", rows)
	}
	if rows[0].Result != "match" || rows[1].Result != "match" {
		t.Fatalf("rows = %+v", rows)
	}

	// Drift on the GitLab side aggregates while the Gitea row stays byte-identical.
	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("T", quote("changed\n"), "opened", gitlabWorkItemsURL, `[]`))
	code, stdout, _ = run(t, "--repo", repo, "external", "check", "--tea-login", "sandbox")
	if code != 1 {
		t.Fatalf("exit %d, want 1 (drift)", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want 2 rows + totals, got:\n%s", stdout)
	}
	if lines[0] != "MATCH AWIT-TEST0001 https://forge.example/owner/repo/issues/127" {
		t.Fatalf("gitea row changed: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "DRIFT AWIT-TEST0002 "+gitlabWorkItemsURL) {
		t.Fatalf("line 1 = %q", lines[1])
	}
	if lines[2] != "Checked 2 items: 1 match, 1 drift, 0 error" {
		t.Fatalf("totals = %q", lines[2])
	}
}

func TestExternalPushBodyGitLab(t *testing.T) {
	repo, _, glab := gitlabCheckRepo(t)
	body := []byte("line one\r\n\r\nunicode é 🚀\n\nlast, no newline")
	writeExternalItem(t, repo, "AWIT-TEST0001", gitlabExtYAML(gitlabSubProject, 127, gitlabIssuesURL), body)
	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("Remote title", quote("old"), "opened", gitlabIssuesURL, `["area::api"]`))
	before := itemFileBytes(t, repo)
	remoteBefore := readGlabStubIssue(t, glab, gitlabSubIssueKey)

	code, stdout, stderr := run(t, "--repo", repo, "external", "push-body", "AWIT-TEST0001")
	if code != 0 {
		t.Fatalf("push-body: exit %d stderr %q", code, stderr)
	}
	want := fmt.Sprintf("pushed body for AWIT-TEST0001 to %s (%d bytes)\n", gitlabIssuesURL, len(body))
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	remoteAfter := readGlabStubIssue(t, glab, gitlabSubIssueKey)
	if remoteAfter.Description == nil || *remoteAfter.Description != string(body) {
		t.Fatalf("remote description = %q, want the exact local bytes", *remoteAfter.Description)
	}
	if remoteAfter.Title != remoteBefore.Title || remoteAfter.State != remoteBefore.State || remoteAfter.WebURL != remoteBefore.WebURL {
		t.Fatalf("body push changed title/state/url: before %+v after %+v", remoteBefore, remoteAfter)
	}
	if strings.Join(remoteAfter.Labels, ",") != strings.Join(remoteBefore.Labels, ",") {
		t.Fatalf("body push changed labels: before %q after %q", remoteBefore.Labels, remoteAfter.Labels)
	}
	code, stdout, _ = run(t, "--repo", repo, "external", "check", "AWIT-TEST0001")
	if code != 0 || !strings.HasPrefix(stdout, "MATCH AWIT-TEST0001 ") {
		t.Fatalf("check after push: exit %d stdout %q", code, stdout)
	}
	after := itemFileBytes(t, repo)
	if after["AWIT-TEST0001.md"] != before["AWIT-TEST0001.md"] {
		t.Fatal("push-body modified the local item file")
	}
}

func TestExternalPushBodyGitLabRefusesDuplicates(t *testing.T) {
	repo, _, glab := gitlabCheckRepo(t)
	writeExternalItem(t, repo, "AWIT-TEST0001", gitlabExtYAML(gitlabSubProject, 127, gitlabIssuesURL), []byte("one\n"))
	writeExternalItem(t, repo, "AWIT-TEST0002", gitlabExtYAML(gitlabSubProject, 127, gitlabWorkItemsURL), []byte("two\n"))
	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("T", quote("old"), "opened", gitlabIssuesURL, `[]`))
	code, _, stderr := run(t, "--repo", repo, "external", "push-body", "AWIT-TEST0001")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "AWIT-TEST0001") || !strings.Contains(stderr, "AWIT-TEST0002") {
		t.Fatalf("stderr = %q, want both duplicate ids", stderr)
	}
	if n := countGlabPUTs(t, glab); n != 0 {
		t.Fatalf("refused push performed %d remote PUT(s)", n)
	}
	if got := readGlabStubIssue(t, glab, gitlabSubIssueKey); got.Description == nil || *got.Description != "old" {
		t.Fatalf("remote body changed despite refusal: %+v", got)
	}
}

func TestExternalPushBodyGitLabQuickActionRefusal(t *testing.T) {
	repo, _, glab := gitlabCheckRepo(t)
	body := []byte("/close\n\nkeep me\n")
	writeExternalItem(t, repo, "AWIT-TEST0001", gitlabExtYAML(gitlabSubProject, 127, gitlabIssuesURL), body)
	writeGitLabIssue(t, glab, gitlabSubIssueKey, gitlabIssueJSON("T", quote("old"), "opened", gitlabIssuesURL, `[]`))
	before := itemFileBytes(t, repo)

	code, _, stderr := run(t, "--repo", repo, "external", "push-body", "AWIT-TEST0001")
	if code != 1 {
		t.Fatalf("exit %d, want 1 (quick-action refusal)", code)
	}
	if !strings.Contains(stderr, "quick action") {
		t.Fatalf("stderr = %q, want safe-format guidance", stderr)
	}
	if n := countGlabPUTs(t, glab); n != 0 {
		t.Fatalf("refused push performed %d remote PUT(s)", n)
	}
	after := itemFileBytes(t, repo)
	if after["AWIT-TEST0001.md"] != before["AWIT-TEST0001.md"] {
		t.Fatal("refused push modified the local item file")
	}
	got := readGlabStubIssue(t, glab, gitlabSubIssueKey)
	if got.Description == nil || *got.Description != "old" || got.State != "opened" {
		t.Fatalf("remote changed despite refusal: %+v", got)
	}
}

func TestSetExternalBodyUnsupportedTracker(t *testing.T) {
	ext := item.External{Tracker: "trac", Repo: "group/project", ID: 1, URL: "https://forge.example/group/project"}
	err := setExternalBody(context.Background(), ext, "", []byte("x"))
	if err == nil || !strings.Contains(err.Error(), `unsupported tracker "trac"`) {
		t.Fatalf("err = %v, want unsupported-tracker refusal before any mutation", err)
	}
}

func TestSetExternalStateUnsupportedTracker(t *testing.T) {
	ext := item.External{Tracker: "trac", Repo: "group/project", ID: 1, URL: "https://forge.example/group/project"}
	err := setExternalState(context.Background(), ext, "", "closed")
	if err == nil || !strings.Contains(err.Error(), `unsupported tracker "trac"`) {
		t.Fatalf("err = %v, want unsupported-tracker refusal before any mutation", err)
	}
}

// Merge-request URLs are not issue links: both dispatchers must refuse them
// before any remote mutation.
func TestSetExternalDispatchGitLabMRURLRefuses(t *testing.T) {
	glabDir := glabxtest.Install(t)
	writeGitLabUser(t, glabDir)
	ext := item.External{Tracker: "gitlab", Repo: "group/project", ID: 1, URL: "https://forge.example/group/project/-/merge_requests/1"}
	if err := setExternalBody(context.Background(), ext, "", []byte("x")); err == nil {
		t.Fatal("body push to an MR URL must be refused before any mutation")
	}
	if err := setExternalState(context.Background(), ext, "", "closed"); err == nil {
		t.Fatal("state push to an MR URL must be refused before any mutation")
	}
	if n := countGlabPUTs(t, glabDir); n != 0 {
		t.Fatalf("refused MR-URL push performed %d remote PUT(s)", n)
	}
}
