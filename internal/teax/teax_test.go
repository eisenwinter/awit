package teax

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/internal/teax/teaxtest"
	"github.com/eisenwinter/awit/pkg/item"
)

func testExternal() item.External {
	return item.External{
		Tracker: "gitea",
		Repo:    "owner/repo",
		ID:      127,
		URL:     "https://forge.example/owner/repo/issues/127",
	}
}

func writeLogins(t *testing.T, dir string, logins ...[2]string) {
	t.Helper()
	type login struct {
		Name    string `json:"name"`
		URL     string `json:"url"`
		Token   string `json:"token"`
		Default bool   `json:"default"`
	}
	var rows []login
	for i, l := range logins {
		rows = append(rows, login{Name: l[0], URL: l[1], Token: "SECRET-TOKEN", Default: i == 0})
	}
	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "logins.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func argvLog(t *testing.T, dir string) [][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "argv.log"))
	if err != nil {
		return nil
	}
	var out [][]string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var args []string
		if err := json.Unmarshal([]byte(line), &args); err != nil {
			t.Fatalf("argv.log line %q: %v", line, err)
		}
		out = append(out, args)
	}
	return out
}

func TestTeaOpenSelectsMatchingLogin(t *testing.T) {
	dir := teaxtest.Install(t)
	writeLogins(t, dir, [2]string{"other", "https://elsewhere.example"}, [2]string{"sandbox", "https://forge.example"})
	c, err := Open(context.Background(), testExternal(), "")
	if err != nil {
		t.Fatal(err)
	}
	if c.Login != "sandbox" || c.Repo != "owner/repo" || c.BaseURL != "https://forge.example" {
		t.Fatalf("client = %+v", c)
	}
	// The capability probe and the auth verification must both have run,
	// with flags before the endpoint and an explicit argv (no shell).
	var sawHelp, sawUser bool
	for _, args := range argvLog(t, dir) {
		if strings.Join(args, " ") == "api --help" {
			sawHelp = true
		}
		if len(args) > 0 && args[0] == "api" && args[len(args)-1] == "user" {
			sawUser = true
			joined := strings.Join(args, " ")
			for _, want := range []string{"--login sandbox", "--repo owner/repo", "--include", "-X GET"} {
				if !strings.Contains(joined, want) {
					t.Fatalf("auth check argv %v missing %q", args, want)
				}
			}
		}
	}
	if !sawHelp || !sawUser {
		t.Fatalf("argv.log missing capability probe or auth check: %v", argvLog(t, dir))
	}
}

func TestTeaOpenExplicitLogin(t *testing.T) {
	dir := teaxtest.Install(t)
	writeLogins(t, dir, [2]string{"first", "https://forge.example"}, [2]string{"second", "https://forge.example"})
	c, err := Open(context.Background(), testExternal(), "second")
	if err != nil {
		t.Fatal(err)
	}
	if c.Login != "second" {
		t.Fatalf("login = %q", c.Login)
	}
}

func TestTeaOpenExplicitLoginWrongBase(t *testing.T) {
	dir := teaxtest.Install(t)
	writeLogins(t, dir, [2]string{"other", "https://elsewhere.example"})
	_, err := Open(context.Background(), testExternal(), "other")
	if err == nil || !strings.Contains(err.Error(), "other") {
		t.Fatalf("err = %v, want login base mismatch naming the login", err)
	}
}

func TestTeaOpenNoMatchingLogin(t *testing.T) {
	dir := teaxtest.Install(t)
	writeLogins(t, dir, [2]string{"other", "https://elsewhere.example"})
	_, err := Open(context.Background(), testExternal(), "")
	if err == nil || !strings.Contains(err.Error(), "tea login add") {
		t.Fatalf("err = %v, want actionable tea login add hint", err)
	}
}

func TestTeaOpenAmbiguousLogins(t *testing.T) {
	dir := teaxtest.Install(t)
	writeLogins(t, dir, [2]string{"first", "https://forge.example"}, [2]string{"second", "https://forge.example"})
	_, err := Open(context.Background(), testExternal(), "")
	if err == nil || !strings.Contains(err.Error(), "--tea-login") {
		t.Fatalf("err = %v, want --tea-login hint", err)
	}
	if !strings.Contains(err.Error(), "first") || !strings.Contains(err.Error(), "second") {
		t.Fatalf("err = %v, want both login names", err)
	}
}

func TestTeaOpenMissingExecutable(t *testing.T) {
	teaxtest.HideTea(t)
	_, err := Open(context.Background(), testExternal(), "")
	if err == nil || !strings.Contains(err.Error(), "install tea") {
		t.Fatalf("err = %v, want install tea hint", err)
	}
}

func TestTeaOpenTooOld(t *testing.T) {
	dir := teaxtest.Install(t)
	t.Setenv("TEA_STUB_NO_API", "1")
	writeLogins(t, dir, [2]string{"sandbox", "https://forge.example"})
	_, err := Open(context.Background(), testExternal(), "")
	if err == nil || !strings.Contains(err.Error(), "api") {
		t.Fatalf("err = %v, want missing api capability", err)
	}
}

// A 401 with subprocess exit zero must still fail: tea reports HTTP errors
// only via the --include status line.
func TestTeaOpenAuthExpired(t *testing.T) {
	dir := teaxtest.Install(t)
	writeLogins(t, dir, [2]string{"sandbox", "https://forge.example"})
	if err := os.WriteFile(filepath.Join(dir, "user.status"), []byte("401"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Open(context.Background(), testExternal(), "")
	if err == nil {
		t.Fatal("Open succeeded on HTTP 401")
	}
	if !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "tea login add") {
		t.Fatalf("err = %v, want HTTP status and re-login hint", err)
	}
	if strings.Contains(err.Error(), "SECRET-TOKEN") {
		t.Fatalf("err leaks token material: %v", err)
	}
}

func TestTeaOpenLoginListFailure(t *testing.T) {
	teaxtest.Install(t) // no logins.json: stub exits 1
	_, err := Open(context.Background(), testExternal(), "")
	if err == nil || !strings.Contains(err.Error(), "tea login") {
		t.Fatalf("err = %v, want login list failure", err)
	}
}

func openClient(t *testing.T) (*Client, string) {
	t.Helper()
	dir := teaxtest.Install(t)
	writeLogins(t, dir, [2]string{"sandbox", "https://forge.example"})
	c, err := Open(context.Background(), testExternal(), "")
	if err != nil {
		t.Fatal(err)
	}
	return c, dir
}

func TestTeaGetIssue(t *testing.T) {
	c, dir := openClient(t)
	body := "Line one.\n\nLine two with `code`.\n"
	issueJSON := `{"id": 987654, "number": 127, "title": "Fix header parsing", "body": ` + strconvQuote(body) + `,
		"state": "open", "labels": [{"id": 1, "name": "bug"}, {"id": 2, "name": "p1"}],
		"html_url": "https://forge.example/owner/repo/issues/127"}`
	if err := os.WriteFile(filepath.Join(dir, "repos_owner_repo_issues_127.json"), []byte(issueJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	iss, err := c.GetIssue(context.Background(), 127)
	if err != nil {
		t.Fatal(err)
	}
	if iss.Number != 127 {
		t.Fatalf("Number = %d, want the repository issue number 127 (database id 987654 ignored)", iss.Number)
	}
	if iss.Title != "Fix header parsing" || string(iss.Body) != body || iss.State != "open" {
		t.Fatalf("issue = %+v", iss)
	}
	if strings.Join(iss.Labels, ",") != "bug,p1" {
		t.Fatalf("labels = %v", iss.Labels)
	}
	if iss.URL != "https://forge.example/owner/repo/issues/127" {
		t.Fatalf("url = %q", iss.URL)
	}
	var sawGet bool
	for _, args := range argvLog(t, dir) {
		if args[len(args)-1] == "repos/owner/repo/issues/127" {
			sawGet = true
		}
	}
	if !sawGet {
		t.Fatalf("argv.log missing issue fetch: %v", argvLog(t, dir))
	}
}

func TestTeaGetIssueNullBody(t *testing.T) {
	c, dir := openClient(t)
	if err := os.WriteFile(filepath.Join(dir, "repos_owner_repo_issues_127.json"),
		[]byte(`{"number": 127, "title": "T", "body": null, "state": "closed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	iss, err := c.GetIssue(context.Background(), 127)
	if err != nil {
		t.Fatal(err)
	}
	if len(iss.Body) != 0 {
		t.Fatalf("null body must map to empty, got %q", iss.Body)
	}
}

func TestTeaGetIssueNotFound(t *testing.T) {
	c, dir := openClient(t)
	if err := os.WriteFile(filepath.Join(dir, "repos_owner_repo_issues_127.status"), []byte("404"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "repos_owner_repo_issues_127.json"), []byte(`{"message":"Not Found"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := c.GetIssue(context.Background(), 127)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("err = %v, want not found", err)
	}
}

func TestTeaGetIssueMalformedJSON(t *testing.T) {
	c, dir := openClient(t)
	if err := os.WriteFile(filepath.Join(dir, "repos_owner_repo_issues_127.json"), []byte(`not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := c.GetIssue(context.Background(), 127); err == nil {
		t.Fatal("malformed JSON must fail")
	}
}

func TestTeaGetIssueMissingNumber(t *testing.T) {
	c, dir := openClient(t)
	if err := os.WriteFile(filepath.Join(dir, "repos_owner_repo_issues_127.json"),
		[]byte(`{"title": "T", "state": "open"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := c.GetIssue(context.Background(), 127)
	if err == nil || !strings.Contains(err.Error(), "number") {
		t.Fatalf("err = %v, want missing number", err)
	}
}

func TestTeaGetIssueSubprocessFailure(t *testing.T) {
	c, _ := openClient(t)
	t.Setenv("TEA_STUB_API_EXIT", "3")
	t.Setenv("TEA_STUB_API_STDERR", "boom diagnostics")
	_, err := c.GetIssue(context.Background(), 127)
	if err == nil || !strings.Contains(err.Error(), "boom diagnostics") {
		t.Fatalf("err = %v, want subprocess diagnostics", err)
	}
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
