package glabx

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/internal/glabx/glabxtest"
	"github.com/eisenwinter/awit/pkg/item"
)

func testExternal() item.External {
	return item.External{
		Tracker: "gitlab",
		Repo:    "group/project",
		ID:      127,
		URL:     "https://forge.example/group/project/-/issues/127",
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeConfig(t *testing.T, dir, key, value string) {
	t.Helper()
	writeFile(t, dir, "config-"+key, value)
}

func scriptUser(t *testing.T, dir, body string) {
	t.Helper()
	writeFile(t, dir, "user.json", body)
}

func scriptIssue(t *testing.T, dir, key, body string) {
	t.Helper()
	writeFile(t, dir, key+".json", body)
}

func scriptGlabIssue(t *testing.T, dir, body string) {
	t.Helper()
	scriptIssue(t, dir, "projects_group%2Fproject_issues_127", body)
}

func argvLog(t *testing.T, dir string) [][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "argv.log"))
	if err != nil {
		return nil
	}
	var out [][]string
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		var args []string
		if err := json.Unmarshal([]byte(line), &args); err != nil {
			t.Fatalf("argv.log line %q: %v", line, err)
		}
		out = append(out, args)
	}
	return out
}

func argvJoined(log [][]string) []string {
	var out []string
	for _, args := range log {
		out = append(out, strings.Join(args, " "))
	}
	return out
}

func openClient(t *testing.T) (*Client, string) {
	t.Helper()
	dir := glabxtest.Install(t)
	scriptUser(t, dir, `{"id": 42, "username": "tester"}`)
	c, err := Open(context.Background(), testExternal())
	if err != nil {
		t.Fatal(err)
	}
	return c, dir
}

func stubDescription(t *testing.T, dir, key string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, key+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Description *string `json:"description"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.Description == nil {
		return ""
	}
	return *m.Description
}

func stubStored(t *testing.T, dir, key string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, key+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// countPUTs counts api calls that perform a PUT: the mutation requests that
// must stay at zero when a body is refused.
func countPUTs(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	for _, args := range argvLog(t, dir) {
		if len(args) > 0 && args[0] == "api" && strings.Contains(strings.Join(args, " "), "--method PUT") {
			n++
		}
	}
	return n
}

// --- issue URLs and installation base ---

func TestGlabIssueURLRootInstall(t *testing.T) {
	glabxtest.Install(t)
	for _, tc := range []struct {
		name, url, repo string
		id              int64
	}{
		{"issues shape", "https://forge.example/group/project/-/issues/127", "group/project", 127},
		{"work_items shape", "https://forge.example/group/project/-/work_items/127", "group/project", 127},
		{"subgroup", "https://forge.example/group/sub/project/-/issues/9", "group/sub/project", 9},
		{"subgroup work_items", "https://forge.example/group/sub/project/-/work_items/9", "group/sub/project", 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ext, err := ParseIssueURL(context.Background(), tc.url)
			if err != nil {
				t.Fatal(err)
			}
			if ext.Tracker != "gitlab" || ext.Repo != tc.repo || ext.ID != tc.id || ext.URL != tc.url {
				t.Fatalf("external = %+v", ext)
			}
		})
	}
}

func TestGlabIssueURLNestedPrefix(t *testing.T) {
	dir := glabxtest.Install(t)
	writeConfig(t, dir, "subfolder", "apps/gitlab")
	ext, err := ParseIssueURL(context.Background(), "https://example.com/apps/gitlab/group/sub/project/-/work_items/9")
	if err != nil {
		t.Fatal(err)
	}
	if ext.Repo != "group/sub/project" || ext.ID != 9 {
		t.Fatalf("external = %+v", ext)
	}
}

func TestGlabIssueURLSubfolderTrimmed(t *testing.T) {
	dir := glabxtest.Install(t)
	writeConfig(t, dir, "subfolder", "/apps/gitlab/")
	ext, err := ParseIssueURL(context.Background(), "https://example.com/apps/gitlab/group/project/-/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	if ext.Repo != "group/project" {
		t.Fatalf("external = %+v", ext)
	}
}

func TestGlabIssueURLEnvSubfolderPrecedence(t *testing.T) {
	dir := glabxtest.Install(t)
	writeConfig(t, dir, "subfolder", "other/prefix")
	t.Setenv("GITLAB_SUBFOLDER", "apps/gitlab")
	ext, err := ParseIssueURL(context.Background(), "https://example.com/apps/gitlab/group/project/-/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	if ext.Repo != "group/project" {
		t.Fatalf("env GITLAB_SUBFOLDER must win over the config file: %+v", ext)
	}
}

func TestGlabIssueURLNeverGuessesPrefix(t *testing.T) {
	glabxtest.Install(t) // no subfolder configured: root install
	ext, err := ParseIssueURL(context.Background(), "https://example.com/apps/gitlab/group/project/-/issues/1")
	if err != nil {
		t.Fatal(err)
	}
	if ext.Repo != "apps/gitlab/group/project" {
		t.Fatalf("without a verified subfolder every segment stays: %+v", ext)
	}
}

func TestGlabIssueURLSubfolderMismatch(t *testing.T) {
	dir := glabxtest.Install(t)
	writeConfig(t, dir, "subfolder", "apps/gitlab")
	_, err := ParseIssueURL(context.Background(), "https://example.com/other/group/project/-/issues/1")
	if err == nil || !strings.Contains(err.Error(), "subfolder") {
		t.Fatalf("err = %v, want a subfolder diagnostic", err)
	}
}

func TestGlabIssueURLSubfolderSegmentBoundary(t *testing.T) {
	dir := glabxtest.Install(t)
	writeConfig(t, dir, "subfolder", "apps/gitlab")
	_, err := ParseIssueURL(context.Background(), "https://example.com/apps/gitlabx/group/project/-/issues/1")
	if err == nil {
		t.Fatal("a string-prefix match must not strip on a non-segment boundary")
	}
}

func TestGlabIssueURLRejects(t *testing.T) {
	glabxtest.Install(t)
	for _, tc := range []struct{ name, url string }{
		{"merge request", "https://forge.example/group/project/-/merge_requests/1"},
		{"snippet", "https://forge.example/group/project/-/snippets/1"},
		{"gitea shape", "https://forge.example/group/project/issues/127"},
		{"bare project", "https://forge.example/group/project"},
		{"non-numeric iid", "https://forge.example/group/project/-/issues/abc"},
		{"zero iid", "https://forge.example/group/project/-/issues/0"},
		{"single segment repo", "https://forge.example/project/-/issues/1"},
		{"not a URL", "not a url"},
		{"query", "https://forge.example/group/project/-/issues/127?x=1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseIssueURL(context.Background(), tc.url); err == nil {
				t.Fatalf("ParseIssueURL(%q) succeeded, want rejection", tc.url)
			}
		})
	}
}

func TestGlabIssueURLMissingGlab(t *testing.T) {
	glabxtest.HideGlab(t)
	_, err := ParseIssueURL(context.Background(), "https://forge.example/group/project/-/issues/127")
	if err == nil || !strings.Contains(err.Error(), "install glab") {
		t.Fatalf("err = %v, want install glab hint", err)
	}
}

func TestGlabIssueURLCancelled(t *testing.T) {
	glabxtest.Install(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ParseIssueURL(ctx, "https://forge.example/group/project/-/issues/127"); err == nil {
		t.Fatal("cancelled context must fail before any write")
	}
}

func TestGlabIssueBase(t *testing.T) {
	for _, tc := range []struct {
		name, url, repo, base string
		id                    int64
	}{
		{"root issues", "https://forge.example/group/project/-/issues/127", "group/project", "https://forge.example", 127},
		{"root work_items", "https://forge.example/group/project/-/work_items/127", "group/project", "https://forge.example", 127},
		{"subgroup", "https://forge.example/group/sub/project/-/issues/9", "group/sub/project", "https://forge.example", 9},
		{"prefix", "https://example.com/apps/gitlab/group/project/-/issues/1", "group/project", "https://example.com/apps/gitlab", 1},
		{"nested prefix", "https://example.com/a/b/group/sub/project/-/work_items/3", "group/sub/project", "https://example.com/a/b", 3},
		{"case and port", "HTTPS://Forge.Example:8443/Keep/Case/-/issues/5", "Keep/Case", "https://forge.example:8443", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, err := IssueBase(item.External{Tracker: "gitlab", Repo: tc.repo, ID: tc.id, URL: tc.url})
			if err != nil {
				t.Fatal(err)
			}
			if base != tc.base {
				t.Fatalf("base = %q, want %q", base, tc.base)
			}
		})
	}
}

func TestGlabIssueBaseDistinguishes(t *testing.T) {
	mk := func(url string) item.External {
		return item.External{Tracker: "gitlab", Repo: "group/project", ID: 127, URL: url}
	}
	for _, tc := range []struct{ name, a, b string }{
		{"scheme", "https://forge.example/group/project/-/issues/127", "http://forge.example/group/project/-/issues/127"},
		{"port", "https://forge.example/group/project/-/issues/127", "https://forge.example:8443/group/project/-/issues/127"},
		{"prefix", "https://example.com/apps/gitlab/group/project/-/issues/127", "https://example.com/other/group/project/-/issues/127"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ba, err := IssueBase(mk(tc.a))
			if err != nil {
				t.Fatal(err)
			}
			bb, err := IssueBase(mk(tc.b))
			if err != nil {
				t.Fatal(err)
			}
			if ba == bb {
				t.Fatalf("bases must differ: %q", ba)
			}
		})
	}
}

func TestGlabIssueBaseRejectsMismatch(t *testing.T) {
	ext := testExternal()
	ext.Repo = "group/other"
	if _, err := IssueBase(ext); err == nil {
		t.Fatal("repo/URL mismatch must fail")
	}
	ext = testExternal()
	ext.URL = "https://forge.example/group/project/-/issues/128"
	if _, err := IssueBase(ext); err == nil {
		t.Fatal("iid/URL mismatch must fail")
	}
}

// --- Open ---

func TestGlabOpen(t *testing.T) {
	dir := glabxtest.Install(t)
	scriptUser(t, dir, `{"id": 42, "username": "tester"}`)
	c, err := Open(context.Background(), testExternal())
	if err != nil {
		t.Fatal(err)
	}
	if c.Host != "forge.example" || c.Repo != "group/project" || c.BaseURL != "https://forge.example" {
		t.Fatalf("client = %+v", c)
	}
	var sawConfig, sawUser bool
	for _, args := range argvLog(t, dir) {
		joined := strings.Join(args, " ")
		if strings.HasPrefix(joined, "config get ") && strings.Contains(joined, "--host forge.example") {
			sawConfig = true
		}
		if len(args) > 0 && args[0] == "api" && args[len(args)-1] == "user" {
			sawUser = true
			for _, want := range []string{"--hostname forge.example", "--include", "--method GET"} {
				if !strings.Contains(joined, want) {
					t.Fatalf("auth check argv %v missing %q", args, want)
				}
			}
		}
	}
	if !sawConfig || !sawUser {
		t.Fatalf("argv.log missing read-only config lookup or auth check: %v", argvJoined(argvLog(t, dir)))
	}
}

func TestGlabOpenNestedPrefix(t *testing.T) {
	dir := glabxtest.Install(t)
	writeConfig(t, dir, "subfolder", "apps/gitlab")
	scriptUser(t, dir, `{"id": 7, "username": "tester"}`)
	ext := item.External{Tracker: "gitlab", Repo: "group/sub/project", ID: 9,
		URL: "https://example.com/apps/gitlab/group/sub/project/-/work_items/9"}
	c, err := Open(context.Background(), ext)
	if err != nil {
		t.Fatal(err)
	}
	if c.BaseURL != "https://example.com/apps/gitlab" || c.Repo != "group/sub/project" {
		t.Fatalf("client = %+v", c)
	}
}

func TestGlabOpenConflictingSubfolder(t *testing.T) {
	dir := glabxtest.Install(t)
	writeConfig(t, dir, "subfolder", "other/prefix")
	scriptUser(t, dir, `{"id": 42, "username": "tester"}`)
	_, err := Open(context.Background(), testExternal())
	if err == nil || !strings.Contains(err.Error(), "subfolder") {
		t.Fatalf("err = %v, want subfolder configuration guidance", err)
	}
}

func TestGlabOpenConflictingAPIHost(t *testing.T) {
	dir := glabxtest.Install(t)
	writeConfig(t, dir, "api_host", "elsewhere.example")
	scriptUser(t, dir, `{"id": 42, "username": "tester"}`)
	_, err := Open(context.Background(), testExternal())
	if err == nil || !strings.Contains(err.Error(), "api_host") {
		t.Fatalf("err = %v, want api_host configuration guidance", err)
	}
}

func TestGlabOpenConflictingAPIProtocol(t *testing.T) {
	dir := glabxtest.Install(t)
	writeConfig(t, dir, "api_protocol", "http")
	scriptUser(t, dir, `{"id": 42, "username": "tester"}`)
	_, err := Open(context.Background(), testExternal())
	if err == nil || !strings.Contains(err.Error(), "api_protocol") {
		t.Fatalf("err = %v, want api_protocol configuration guidance", err)
	}
}

func TestGlabOpenAPIHostWithPort(t *testing.T) {
	dir := glabxtest.Install(t)
	writeConfig(t, dir, "api_host", "127.0.0.1:18711")
	writeConfig(t, dir, "api_protocol", "http")
	scriptUser(t, dir, `{"id": 42, "username": "tester"}`)
	ext := item.External{Tracker: "gitlab", Repo: "group/project", ID: 127,
		URL: "http://127.0.0.1:18711/group/project/-/issues/127"}
	c, err := Open(context.Background(), ext)
	if err != nil {
		t.Fatal(err)
	}
	if c.Host != "127.0.0.1:18711" {
		t.Fatalf("client = %+v", c)
	}
	var sawBareHostname bool
	for _, args := range argvLog(t, dir) {
		if len(args) > 0 && args[0] == "api" && args[len(args)-1] == "user" &&
			strings.Contains(strings.Join(args, " "), "--hostname 127.0.0.1 ") {
			sawBareHostname = true
		}
	}
	if !sawBareHostname {
		t.Fatalf("glab --hostname cannot carry a port; want the bare host: %v", argvJoined(argvLog(t, dir)))
	}
}

func TestGlabOpenBarePortRefused(t *testing.T) {
	glabxtest.Install(t)
	ext := item.External{Tracker: "gitlab", Repo: "group/project", ID: 127,
		URL: "https://forge.example:8443/group/project/-/issues/127"}
	_, err := Open(context.Background(), ext)
	if err == nil || !strings.Contains(err.Error(), "api_host") {
		t.Fatalf("err = %v, want api_host guidance for explicit ports", err)
	}
}

func TestGlabOpenMissingGlab(t *testing.T) {
	glabxtest.HideGlab(t)
	_, err := Open(context.Background(), testExternal())
	if err == nil || !strings.Contains(err.Error(), "install glab") {
		t.Fatalf("err = %v, want install glab hint", err)
	}
}

func TestGlabOpenAuthFailures(t *testing.T) {
	for _, tc := range []struct {
		name, status, userJSON string
	}{
		{"unauthorized", "401", `{"message": "401 Unauthorized"}`},
		{"forbidden", "403", `{"message": "403 Forbidden"}`},
		{"server error", "500", `{"message": "Internal Server Error"}`},
		{"error object exit zero", "200", `{"message": "success-looking error"}`},
		{"missing id", "200", `{"username": "tester"}`},
		{"zero id", "200", `{"id": 0, "username": "tester"}`},
		{"string id", "200", `{"id": "42", "username": "tester"}`},
		{"malformed", "200", `not json`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := glabxtest.Install(t)
			writeFile(t, dir, "user.status", tc.status)
			scriptUser(t, dir, tc.userJSON)
			_, err := Open(context.Background(), testExternal())
			if err == nil {
				t.Fatal("Open succeeded on invalid auth")
			}
			if !strings.Contains(err.Error(), "glab auth status --hostname forge.example") {
				t.Fatalf("err = %v, want auth repair guidance", err)
			}
			if strings.Contains(err.Error(), "faketoken") {
				t.Fatalf("err leaks credential material: %v", err)
			}
		})
	}
}

func TestGlabOpenCancelled(t *testing.T) {
	dir := glabxtest.Install(t)
	scriptUser(t, dir, `{"id": 42}`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Open(ctx, testExternal()); err == nil {
		t.Fatal("cancelled context must fail before any write")
	}
}

// --- GetIssue ---

func TestGlabGetIssue(t *testing.T) {
	c, dir := openClient(t)
	key := "projects_group%2Fsub%2Fproject_issues_127"
	issueJSON := `{"id": 987654, "iid": 127, "title": "Fix header parsing", "description": "Line one.\n\nLine two.", "labels": ["bug", "p1"], "state": "opened", "web_url": "https://forge.example/group/sub/project/-/issues/127"}`
	scriptIssue(t, dir, key, issueJSON)
	cc := &Client{Host: c.Host, Repo: "group/sub/project", BaseURL: "https://forge.example"}
	iss, err := cc.GetIssue(context.Background(), 127)
	if err != nil {
		t.Fatal(err)
	}
	if iss.Number != 127 {
		t.Fatalf("Number = %d, want decoded iid 127 (global id 987654 ignored)", iss.Number)
	}
	if iss.Title != "Fix header parsing" || string(iss.Body) != "Line one.\n\nLine two." {
		t.Fatalf("issue = %+v", iss)
	}
	if strings.Join(iss.Labels, "|") != "bug|p1" {
		t.Fatalf("labels = %v", iss.Labels)
	}
	if iss.State != "open" {
		t.Fatalf("state = %q, want normalized open", iss.State)
	}
	var sawGet bool
	for _, args := range argvLog(t, dir) {
		if args[len(args)-1] == "projects/group%2Fsub%2Fproject/issues/127" {
			sawGet = true
			joined := strings.Join(args, " ")
			for _, want := range []string{"--hostname forge.example", "--include", "--method GET"} {
				if !strings.Contains(joined, want) {
					t.Fatalf("fetch argv %v missing %q", args, want)
				}
			}
			if strings.Contains(joined, ":fullpath") || strings.Contains(joined, ":id") {
				t.Fatalf("fetch argv %v must carry an explicit encoded endpoint, no placeholders", args)
			}
		}
	}
	if !sawGet {
		t.Fatalf("argv.log missing single-segment encoded fetch: %v", argvJoined(argvLog(t, dir)))
	}
}

func TestGlabGetIssueShapes(t *testing.T) {
	c, dir := openClient(t)
	base := "https://forge.example/group/project/-/issues/127"
	for _, tc := range []struct {
		name, body, wantBody, wantState string
		wantLabels                      []string
	}{
		{"null description", `{"id":1,"iid":127,"title":"T","description":null,"labels":[],"state":"closed","web_url":"` + base + `"}`, "", "closed", nil},
		{"work_items spelling", `{"id":1,"iid":127,"title":"T","description":"x","labels":[],"state":"closed","web_url":"https://forge.example/group/project/-/work_items/127"}`, "x", "closed", nil},
		{"comma and unicode labels", `{"id":1,"iid":127,"title":"T","description":"","labels":["area::api","comma,label","日本語"],"state":"opened","web_url":"` + base + `"}`, "", "open", []string{"area::api", "comma,label", "日本語"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scriptIssue(t, dir, "projects_group%2Fproject_issues_127", tc.body)
			iss, err := c.GetIssue(context.Background(), 127)
			if err != nil {
				t.Fatal(err)
			}
			if string(iss.Body) != tc.wantBody || iss.State != tc.wantState {
				t.Fatalf("issue = %+v", iss)
			}
			if strings.Join(iss.Labels, "|") != strings.Join(tc.wantLabels, "|") {
				t.Fatalf("labels = %v", iss.Labels)
			}
		})
	}
}

func TestGlabGetIssueHTTPFailures(t *testing.T) {
	for _, tc := range []struct{ name, status string }{
		{"not found", "404"},
		{"forbidden", "403"},
		{"unprocessable", "422"},
		{"server error", "500"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, dir := openClient(t)
			writeFile(t, dir, "projects_group%2Fproject_issues_127.status", tc.status)
			scriptIssue(t, dir, "projects_group%2Fproject_issues_127", `{"message":"no"}`)
			_, err := c.GetIssue(context.Background(), 127)
			if err == nil || !strings.Contains(err.Error(), tc.status) {
				t.Fatalf("err = %v, want HTTP %s named", err, tc.status)
			}
		})
	}
}

func TestGlabGetIssueIdentity(t *testing.T) {
	base := "https://forge.example/group/project/-/issues/127"
	good := func() string {
		return `{"id":1,"iid":127,"title":"T","description":"d","labels":[],"state":"opened","web_url":"` + base + `"}`
	}
	for _, tc := range []struct {
		name, mutate string
	}{
		{"iid mismatch", strings.Replace(good(), `"iid":127`, `"iid":128`, 1)},
		{"wrong project", strings.Replace(good(), "group/project/-", "group/other/-", 1)},
		{"wrong host", strings.Replace(good(), "https://forge.example/", "https://elsewhere.example/", 1)},
		{"unknown state", strings.Replace(good(), `"opened"`, `"reopened"`, 1)},
		{"missing iid", `{"id":1,"title":"T","description":"d","labels":[],"state":"opened","web_url":"` + base + `"}`},
		{"missing title", `{"id":1,"iid":127,"description":"d","labels":[],"state":"opened","web_url":"` + base + `"}`},
		{"missing description", `{"id":1,"iid":127,"title":"T","labels":[],"state":"opened","web_url":"` + base + `"}`},
		{"missing labels", `{"id":1,"iid":127,"title":"T","description":"d","state":"opened","web_url":"` + base + `"}`},
		{"missing state", `{"id":1,"iid":127,"title":"T","description":"d","labels":[],"web_url":"` + base + `"}`},
		{"missing web_url", `{"id":1,"iid":127,"title":"T","description":"d","labels":[],"state":"opened"}`},
		{"numeric description", strings.Replace(good(), `"description":"d"`, `"description":4`, 1)},
		{"string labels", strings.Replace(good(), `"labels":[]`, `"labels":"bug"`, 1)},
		{"non-string label", strings.Replace(good(), `"labels":[]`, `"labels":[3]`, 1)},
		{"error object", `{"message":"not found"}`},
		{"malformed", `not json`},
		{"empty", ``},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, dir := openClient(t)
			scriptIssue(t, dir, "projects_group%2Fproject_issues_127", tc.mutate)
			if _, err := c.GetIssue(context.Background(), 127); err == nil {
				t.Fatalf("GetIssue accepted %q", tc.mutate)
			}
		})
	}
}

// The database-wide id is never identity, even when it equals the iid.
func TestGlabGetIssueIgnoresGlobalID(t *testing.T) {
	c, dir := openClient(t)
	scriptIssue(t, dir, "projects_group%2Fproject_issues_127",
		`{"id":127,"iid":127,"title":"T","description":"d","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
	iss, err := c.GetIssue(context.Background(), 127)
	if err != nil {
		t.Fatal(err)
	}
	if iss.Number != 127 {
		t.Fatalf("Number = %d, want decoded iid 127", iss.Number)
	}
}

func TestGlabGetIssueMalformedFraming(t *testing.T) {
	c, dir := openClient(t)
	t.Setenv("GLAB_STUB_NO_INCLUDE", "1")
	scriptIssue(t, dir, "projects_group%2Fproject_issues_127",
		`{"id":1,"iid":127,"title":"T","description":"d","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
	if _, err := c.GetIssue(context.Background(), 127); err == nil {
		t.Fatal("missing stdout status framing must fail")
	}
}

func TestGlabGetIssueSubprocessFailure(t *testing.T) {
	c, _ := openClient(t)
	t.Setenv("GLAB_STUB_API_EXIT", "3")
	t.Setenv("GLAB_STUB_API_STDERR", "boom diagnostics")
	_, err := c.GetIssue(context.Background(), 127)
	if err == nil || !strings.Contains(err.Error(), "boom diagnostics") {
		t.Fatalf("err = %v, want subprocess diagnostics", err)
	}
}

func TestGlabGetIssueCancelled(t *testing.T) {
	c, dir := openClient(t)
	scriptIssue(t, dir, "projects_group%2Fproject_issues_127",
		`{"id":1,"iid":127,"title":"T","description":"d","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.GetIssue(ctx, 127); err == nil {
		t.Fatal("cancelled context must fail")
	}
}

// --- SetBody transport (stub-backed) ---

func TestGlabSetBodyByteEdges(t *testing.T) {
	c, dir := openClient(t)
	scriptGlabIssue(t, dir, `{"id":9,"iid":127,"title":"T","description":"old","labels":["kept"],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
	for _, tc := range []struct{ name, body string }{
		{"empty", ""},
		{"no terminal LF", "line one\nline two"},
		{"single terminal LF", "line one\nline two\n"},
		{"multiple terminal LFs", "para\n\n\n"},
		{"lone terminal CR", "trail\r"},
		{"CRLF", "one\r\ntwo\r\n"},
		{"leading blank lines", "\n\nfirst\n"},
		{"trailing spaces", "padded   \n"},
		{"unicode", "héllo wörld — 日本語\n"},
		{"backticks and quotes", "a `code` span, \"double\", 'single'\n"},
		{"comma and at", "a, b @mention\n"},
		{"literal null", "null"},
		{"digits", "127"},
		{"JSON-looking", `{"a": [1, 2]}`},
		{":namespace-looking", ":group/project"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := c.SetBody(context.Background(), 127, []byte(tc.body)); err != nil {
				t.Fatalf("SetBody: %v", err)
			}
			if got := stubDescription(t, dir, "projects_group%2Fproject_issues_127"); got != tc.body {
				t.Fatalf("stored description %q, want byte-exact %q", got, tc.body)
			}
			iss, err := c.GetIssue(context.Background(), 127)
			if err != nil {
				t.Fatalf("GetIssue: %v", err)
			}
			if string(iss.Body) != tc.body {
				t.Fatalf("read-back %q, want %q", iss.Body, tc.body)
			}
			stored := stubStored(t, dir, "projects_group%2Fproject_issues_127")
			if stored["title"] != "T" || stored["state"] != "opened" {
				t.Fatalf("body PUT must leave title/state unchanged: %v", stored)
			}
			if labels, _ := json.Marshal(stored["labels"]); string(labels) != `["kept"]` {
				t.Fatalf("body PUT must leave labels unchanged: %v", stored)
			}
		})
	}
}

func TestGlabSetBodyRequestShape(t *testing.T) {
	c, dir := openClient(t)
	scriptGlabIssue(t, dir, `{"id":9,"iid":127,"title":"T","description":"old","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
	if err := c.SetBody(context.Background(), 127, []byte("new")); err != nil {
		t.Fatal(err)
	}
	var sawPUT bool
	for _, args := range argvLog(t, dir) {
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, "--method PUT") || !strings.HasSuffix(joined, "projects/group%2Fproject/issues/127") {
			continue
		}
		sawPUT = true
		if !strings.Contains(joined, "--hostname forge.example") || !strings.Contains(joined, "--include") {
			t.Fatalf("PUT argv %v missing host scoping or include framing", args)
		}
		var file string
		for i, a := range args {
			if a == "-F" && i+1 < len(args) && strings.HasPrefix(args[i+1], "description=@") {
				file = strings.TrimPrefix(args[i+1], "description=@")
			}
			for _, forbidden := range []string{"title=", "labels=", "state=", "state_event="} {
				if strings.Contains(a, forbidden) {
					t.Fatalf("description PUT must be body-only; argv carries %q", a)
				}
			}
		}
		if file == "" {
			t.Fatalf("PUT argv %v has no description=@file field", args)
		}
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Fatalf("transport file %q must be cleaned up, still exists", file)
		}
	}
	if !sawPUT {
		t.Fatalf("argv.log missing encoded description PUT: %v", argvJoined(argvLog(t, dir)))
	}
}

// glab sends -F file values byte-exact, so the transport file carries
// exactly the body: no added or stripped LF, mode 0600, temp-then-rename,
// deleted on every exit.
func TestTransportFile(t *testing.T) {
	for _, body := range []string{"", "a\r\nb\n", "trail\n\n", "no-lf"} {
		path, cleanup, err := transportFile([]byte(body))
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != body {
			cleanup()
			t.Fatalf("payload %q, want exactly %q", data, body)
		}
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
			cleanup()
			t.Fatalf("mode %o, want 0600", fi.Mode().Perm())
		}
		cleanup()
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("transport file must be deleted on every exit: %v", err)
		}
	}
}

func TestGlabSetBodyRejectsHTTPError(t *testing.T) {
	for _, status := range []string{"401", "403", "404", "422", "500"} {
		t.Run(status, func(t *testing.T) {
			c, dir := openClient(t)
			writeFile(t, dir, "projects_group%2Fproject_issues_127.status", status)
			scriptGlabIssue(t, dir, `{"message":"no"}`)
			err := c.SetBody(context.Background(), 127, []byte("x"))
			if err == nil || !strings.Contains(err.Error(), status) {
				t.Fatalf("err = %v, want HTTP %s named", err, status)
			}
			if got := stubDescription(t, dir, "projects_group%2Fproject_issues_127"); got != "" {
				t.Fatalf("failed PUT must not change the remote: %q", got)
			}
		})
	}
}

func TestGlabSetBodyVerificationMismatch(t *testing.T) {
	c, dir := openClient(t)
	t.Setenv("GLAB_STUB_PATCH_NO_STORE", "1")
	scriptGlabIssue(t, dir, `{"id":9,"iid":127,"title":"T","description":"old","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
	err := c.SetBody(context.Background(), 127, []byte("new body\n"))
	if err == nil || !strings.Contains(err.Error(), "verification") {
		t.Fatalf("err = %v, want a verification failure", err)
	}
}

func TestGlabSetBodyGetFallback(t *testing.T) {
	c, dir := openClient(t)
	t.Setenv("GLAB_STUB_PATCH_OMIT_DESCRIPTION", "1")
	scriptGlabIssue(t, dir, `{"id":9,"iid":127,"title":"T","description":"old","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
	if err := c.SetBody(context.Background(), 127, []byte("via fallback\n")); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	if got := stubDescription(t, dir, "projects_group%2Fproject_issues_127"); got != "via fallback\n" {
		t.Fatalf("stored description %q", got)
	}
}

func TestGlabSetBodyGetFallbackMismatch(t *testing.T) {
	c, dir := openClient(t)
	t.Setenv("GLAB_STUB_PATCH_OMIT_DESCRIPTION", "1")
	t.Setenv("GLAB_STUB_PATCH_NO_STORE", "1")
	scriptGlabIssue(t, dir, `{"id":9,"iid":127,"title":"T","description":"old","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
	err := c.SetBody(context.Background(), 127, []byte("new body\n"))
	if err == nil || !strings.Contains(err.Error(), "verification") {
		t.Fatalf("err = %v, want a verification failure", err)
	}
}

func TestGlabSetBodyWrongIdentity(t *testing.T) {
	c, dir := openClient(t)
	t.Setenv("GLAB_STUB_PATCH_WRONG_IID", "1")
	scriptGlabIssue(t, dir, `{"id":9,"iid":127,"title":"T","description":"old","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
	err := c.SetBody(context.Background(), 127, []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "127") {
		t.Fatalf("err = %v, want the issue number named", err)
	}
}

func TestGlabSetBodyMalformedResponse(t *testing.T) {
	c, dir := openClient(t)
	t.Setenv("GLAB_STUB_PATCH_KEEP_RESPONSE", "1")
	writeFile(t, dir, "projects_group%2Fproject_issues_127.patch-response", `not json`)
	scriptGlabIssue(t, dir, `{"id":9,"iid":127,"title":"T","description":"x","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
	if err := c.SetBody(context.Background(), 127, []byte("x")); err == nil {
		t.Fatal("malformed PUT response must fail, not fall back silently")
	}
}

func TestGlabSetBodyInvalidUTF8(t *testing.T) {
	c, dir := openClient(t)
	scriptGlabIssue(t, dir, `{"id":9,"iid":127,"title":"T","description":"old","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
	before := countPUTs(t, dir)
	if err := c.SetBody(context.Background(), 127, []byte("bad \xff utf8")); err == nil {
		t.Fatal("invalid UTF-8 must be refused before mutation")
	}
	if n := countPUTs(t, dir); n != before {
		t.Fatalf("%d PUTs after refusal, want zero", n-before)
	}
	if got := stubDescription(t, dir, "projects_group%2Fproject_issues_127"); got != "old" {
		t.Fatalf("refused body must leave the remote unchanged: %q", got)
	}
}

func TestGlabSetBodyCleansTempAfterFailure(t *testing.T) {
	c, dir := openClient(t)
	writeFile(t, dir, "projects_group%2Fproject_issues_127.status", "403")
	scriptGlabIssue(t, dir, `{"message":"no"}`)
	_ = c.SetBody(context.Background(), 127, []byte("x"))
	leftovers, err := filepath.Glob(filepath.Join(os.TempDir(), "awit-glab-body-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("transport temp dirs leaked: %v", leftovers)
	}
}

// --- quick-action refusal ---

func TestGlabQuickActionRefusal(t *testing.T) {
	if err := validateBody([]byte("intro\r\n/close\r\n")); err == nil {
		t.Fatal("column-zero quick action must be refused")
	}
	for _, tc := range []struct {
		name, body string
		refused    bool
	}{
		{"bare close at EOF", "intro\n/close", true},
		{"close with arguments", "/close and more\n", true},
		{"unknown command", "text\n/frobnicate x\n", true},
		{"single letter", "/a", true},
		{"CRLF command", "intro\r\n/close\r\n", true},
		{"LF command", "intro\n/approve\n", true},
		{"inside fence still refused", "```\n/close\n```\n", true},
		{"indented is safe", "intro\n /close\n", false},
		{"mid-line is safe", "run /close now\n", false},
		{"bare slash is safe", "a/b\n/\n", false},
		{"slash digit is safe", "/1abc\n", false},
		{"uppercase is safe", "/Close\n", false},
		{"empty is safe", "", false},
		{"plain text is safe", "hello\nworld\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBody([]byte(tc.body))
			if tc.refused && err == nil {
				t.Fatalf("body %q must be refused", tc.body)
			}
			if !tc.refused && err != nil {
				t.Fatalf("body %q must be accepted: %v", tc.body, err)
			}
			if tc.refused && !strings.Contains(err.Error(), "column zero") {
				t.Fatalf("refusal must explain column zero, got: %v", err)
			}
			if tc.refused && !strings.Contains(err.Error(), "fenc") {
				t.Fatalf("refusal must point at fenced code blocks, got: %v", err)
			}
		})
	}
}

func TestGlabSetBodyRefusesUnsafeWithoutPUT(t *testing.T) {
	c, dir := openClient(t)
	scriptGlabIssue(t, dir, `{"id":9,"iid":127,"title":"T","description":"old","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
	before := countPUTs(t, dir)
	err := c.SetBody(context.Background(), 127, []byte("looks fine\n/close\n"))
	if err == nil {
		t.Fatal("quick-action body must be refused")
	}
	if n := countPUTs(t, dir); n != before {
		t.Fatalf("%d PUTs after refusal, want zero mutation requests", n-before)
	}
	if got := stubDescription(t, dir, "projects_group%2Fproject_issues_127"); got != "old" {
		t.Fatalf("refused body must leave the remote unchanged: %q", got)
	}
}
