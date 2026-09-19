package teax

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
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

// --- SetBody transport (stub-backed) ---

// scriptTeaIssue seeds the stub's stored response for repos/owner/repo/issues/<n>.
func scriptTeaIssue(t *testing.T, dir string, n int64, bodyJSON string) {
	t.Helper()
	key := fmt.Sprintf("repos_owner_repo_issues_%d", n)
	if err := os.WriteFile(filepath.Join(dir, key+".json"), []byte(bodyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
}

// stubIssueBody reads back the body the stub currently stores for issue n.
func stubIssueBody(t *testing.T, dir string, n int64) string {
	t.Helper()
	key := fmt.Sprintf("repos_owner_repo_issues_%d", n)
	data, err := os.ReadFile(filepath.Join(dir, key+".json"))
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

func TestTeaSetBodyByteEdges(t *testing.T) {
	c, dir := openClient(t)
	scriptTeaIssue(t, dir, 127, `{"number":127,"title":"T","state":"open","body":"old"}`)
	for _, tc := range []struct{ name, body string }{
		{"empty", ""},
		{"no terminal LF", "line one\nline two"},
		{"single terminal LF", "line one\nline two\n"},
		{"multiple terminal LFs", "para\n\n\n"},
		{"CRLF", "one\r\ntwo\r\n"},
		{"leading blank lines", "\n\nfirst\n"},
		{"unicode", "héllo wörld — 日本語\n"},
		{"backticks and quotes", "a `code` span, \"double\", 'single'\n"},
		{"comma and at", "a, b @mention\n"},
		{"literal null", "null"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := c.SetBody(context.Background(), 127, []byte(tc.body)); err != nil {
				t.Fatalf("SetBody: %v", err)
			}
			if got := stubIssueBody(t, dir, 127); got != tc.body {
				t.Fatalf("stored body %q, want byte-exact %q", got, tc.body)
			}
			iss, err := c.GetIssue(context.Background(), 127)
			if err != nil {
				t.Fatalf("GetIssue: %v", err)
			}
			if !bytes.Equal(iss.Body, []byte(tc.body)) {
				t.Fatalf("read-back %q, want %q", iss.Body, tc.body)
			}
		})
	}
}

// tea exits zero on HTTP errors; the --include status must be validated.
func TestTeaSetBodyRejectsHTTPErrorStatusExitZero(t *testing.T) {
	c, dir := openClient(t)
	key := "repos_owner_repo_issues_127"
	if err := os.WriteFile(filepath.Join(dir, key+".status"), []byte("403"), 0o644); err != nil {
		t.Fatal(err)
	}
	scriptTeaIssue(t, dir, 127, `{"message":"Forbidden"}`)
	err := c.SetBody(context.Background(), 127, []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v, want HTTP 403 named", err)
	}
}

// A server that ignores the PATCH but answers 200 must not be reported as success.
func TestTeaSetBodyVerificationMismatch(t *testing.T) {
	c, dir := openClient(t)
	t.Setenv("TEA_STUB_PATCH_NO_STORE", "1")
	scriptTeaIssue(t, dir, 127, `{"number":127,"title":"T","state":"open","body":"old"}`)
	err := c.SetBody(context.Background(), 127, []byte("new body\n"))
	if err == nil || !strings.Contains(err.Error(), "verification") {
		t.Fatalf("err = %v, want a verification failure", err)
	}
}

// When the PATCH response omits the body, SetBody falls back to a GET.
func TestTeaSetBodyGetFallback(t *testing.T) {
	c, dir := openClient(t)
	t.Setenv("TEA_STUB_PATCH_OMIT_BODY", "1")
	scriptTeaIssue(t, dir, 127, `{"number":127,"title":"T","state":"open","body":"old"}`)
	if err := c.SetBody(context.Background(), 127, []byte("via fallback\n")); err != nil {
		t.Fatalf("SetBody: %v", err)
	}
	if got := stubIssueBody(t, dir, 127); got != "via fallback\n" {
		t.Fatalf("stored body %q", got)
	}
}

func TestTeaSetBodyGetFallbackMismatch(t *testing.T) {
	c, dir := openClient(t)
	t.Setenv("TEA_STUB_PATCH_OMIT_BODY", "1")
	t.Setenv("TEA_STUB_PATCH_NO_STORE", "1")
	scriptTeaIssue(t, dir, 127, `{"number":127,"title":"T","state":"open","body":"old"}`)
	err := c.SetBody(context.Background(), 127, []byte("new body\n"))
	if err == nil || !strings.Contains(err.Error(), "verification") {
		t.Fatalf("err = %v, want a verification failure", err)
	}
}

// The response must confirm the issue number that was patched.
func TestTeaSetBodyWrongNumberIdentity(t *testing.T) {
	c, dir := openClient(t)
	t.Setenv("TEA_STUB_PATCH_WRONG_NUMBER", "1")
	scriptTeaIssue(t, dir, 127, `{"number":127,"title":"T","state":"open","body":"old"}`)
	err := c.SetBody(context.Background(), 127, []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "127") {
		t.Fatalf("err = %v, want the issue number named", err)
	}
}

// A response URL on another installation fails verification.
func TestTeaSetBodyURLIdentityMismatch(t *testing.T) {
	c, dir := openClient(t)
	scriptTeaIssue(t, dir, 127, `{"number":127,"title":"T","state":"open","body":"old","html_url":"https://elsewhere.example/owner/repo/issues/127"}`)
	err := c.SetBody(context.Background(), 127, []byte("x"))
	if err == nil || !strings.Contains(err.Error(), "verification") {
		t.Fatalf("err = %v, want a verification failure naming the URL", err)
	}
}

// The transport file is body + exactly one LF, mode 0600, and cleanup
// deletes it.
func TestTransportFile(t *testing.T) {
	path, cleanup, err := transportFile([]byte("a\r\nb\n"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "a\r\nb\n\n" {
		t.Fatalf("payload %q, want body plus one transport LF", data)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && fi.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o, want 0600", fi.Mode().Perm())
	}
	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("transport file must be deleted on every exit: %v", err)
	}
}

// --- real tea transport (opt-in, mandatory for release acceptance) ---

// realTeaState is the isolated server's view of the issues tea touched.
type realTeaState struct {
	mu      sync.Mutex
	bodies  map[int64]string
	decoded map[int64]string // exact body strings tea sent per issue
}

// newRealTeaServer serves the Gitea API surface tea needs, locally and
// isolated: GET user, GET/PATCH issue. Issue 403 answers HTTP 403 (tea
// still exits zero); issue 666 stores a corrupted body (a dishonest
// server).
func newRealTeaServer(t *testing.T) (*httptest.Server, *realTeaState) {
	t.Helper()
	st := &realTeaState{bodies: map[int64]string{}, decoded: map[int64]string{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/user", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"login":"tester"}`)
	})
	mux.HandleFunc("/api/v1/repos/owner/repo/issues/", func(w http.ResponseWriter, r *http.Request) {
		tail := strings.TrimPrefix(r.URL.Path, "/api/v1/repos/owner/repo/issues/")
		n, err := strconv.ParseInt(tail, 10, 64)
		w.Header().Set("Content-Type", "application/json")
		if err != nil {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"message":"bad issue number"}`)
			return
		}
		if n == 403 {
			w.WriteHeader(403)
			fmt.Fprint(w, `{"message":"Forbidden"}`)
			return
		}
		issueURL := "http://" + r.Host + "/owner/repo/issues/" + tail
		st.mu.Lock()
		defer st.mu.Unlock()
		switch r.Method {
		case "PATCH":
			data, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(400)
				return
			}
			var req struct {
				Body *string `json:"body"`
			}
			if err := json.Unmarshal(data, &req); err != nil || req.Body == nil {
				w.WriteHeader(400)
				fmt.Fprint(w, `{"message":"missing body field"}`)
				return
			}
			st.decoded[n] = *req.Body
			stored := *req.Body
			if n == 666 {
				stored += "\nMUTATED BY SERVER"
			}
			st.bodies[n] = stored
			fmt.Fprintf(w, `{"number":%d,"title":"T","state":"open","body":%s,"html_url":%s}`,
				n, strconvQuote(stored), strconvQuote(issueURL))
		case "GET":
			fmt.Fprintf(w, `{"number":%d,"title":"T","state":"open","body":%s,"html_url":%s}`,
				n, strconvQuote(st.bodies[n]), strconvQuote(issueURL))
		default:
			w.WriteHeader(405)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, st
}

// writeRealTeaConfig isolates tea's configuration: an XDG config home in a
// temp dir holding one login for the test server.
func writeRealTeaConfig(t *testing.T, baseURL string) {
	t.Helper()
	dir := t.TempDir()
	cfg := "logins:\n    - name: sandbox\n      url: " + baseURL + "\n      token: faketoken\n      default: true\n      user: tester\n"
	if err := os.MkdirAll(filepath.Join(dir, "tea"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tea", "config.yml"), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
}

// TestTeaBodyRoundTrip exercises the byte-exact body transport against the
// real tea binary: isolated local HTTP server, isolated tea configuration.
// It is opt-in (AWIT_TEST_REAL_TEA=1) but mandatory for release acceptance
// of the external feature; a tea build whose -F @file reader stops stripping
// exactly one terminal LF fails here and must not be advertised as supported.
func TestTeaBodyRoundTrip(t *testing.T) {
	if os.Getenv("AWIT_TEST_REAL_TEA") != "1" {
		t.Skip("opt-in: set AWIT_TEST_REAL_TEA=1 with a supported tea binary in PATH")
	}
	if _, err := exec.LookPath("tea"); err != nil {
		t.Skip("tea binary not found in PATH")
	}
	ver, _, err := run(context.Background(), "--version")
	if err != nil {
		t.Fatalf("tea --version: %v", err)
	}
	t.Logf("tea under test: %s", strings.TrimSpace(string(ver)))

	srv, st := newRealTeaServer(t)
	writeRealTeaConfig(t, srv.URL)
	ext := item.External{Tracker: "gitea", Repo: "owner/repo", ID: 1, URL: srv.URL + "/owner/repo/issues/1"}
	client, err := Open(context.Background(), ext, "sandbox")
	if err != nil {
		t.Fatalf("Open against the isolated server: %v", err)
	}
	ctx := context.Background()

	for _, tc := range []struct{ name, body string }{
		{"empty", ""},
		{"no terminal LF", "line one\nline two"},
		{"single terminal LF", "line one\nline two\n"},
		{"multiple terminal LFs", "para\n\n\n"},
		{"CRLF", "one\r\ntwo\r\n"},
		{"leading blank lines", "\n\nfirst\n"},
		{"unicode", "héllo wörld — 日本語\n"},
		{"backticks and quotes", "a `code` span, \"double\", 'single'\n"},
		{"comma and at", "a, b @mention\n"},
		{"literal null", "null"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := client.SetBody(ctx, 1, []byte(tc.body)); err != nil {
				t.Fatalf("SetBody: %v", err)
			}
			st.mu.Lock()
			got := st.decoded[1]
			st.mu.Unlock()
			if got != tc.body {
				t.Fatalf("tea sent body %q, want byte-exact %q", got, tc.body)
			}
			iss, err := client.GetIssue(ctx, 1)
			if err != nil {
				t.Fatalf("GetIssue: %v", err)
			}
			if !bytes.Equal(iss.Body, []byte(tc.body)) {
				t.Fatalf("read-back %q, want %q", iss.Body, tc.body)
			}
		})
	}

	t.Run("HTTP403 exit zero is rejected", func(t *testing.T) {
		err := client.SetBody(ctx, 403, []byte("x"))
		if err == nil || !strings.Contains(err.Error(), "403") {
			t.Fatalf("err = %v, want HTTP 403 named (tea exits zero on HTTP errors)", err)
		}
	})

	t.Run("server-mutated body is not success", func(t *testing.T) {
		err := client.SetBody(ctx, 666, []byte("original\n"))
		if err == nil || !strings.Contains(err.Error(), "verification") {
			t.Fatalf("err = %v, want a verification failure", err)
		}
	})

	// The compatibility canary: with the raw body file (no transport LF
	// appended), tea must strip exactly one terminal LF. This is why the
	// transport file carries one extra LF, and it fails loudly if a tea
	// build changes that behavior.
	t.Run("unadapted file loses exactly one terminal LF", func(t *testing.T) {
		raw := "terminal newline\n"
		f := filepath.Join(t.TempDir(), "body.raw")
		if err := os.WriteFile(f, []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		_, _, err := run(ctx, "api", "--login", "sandbox", "--repo", "owner/repo", "--include",
			"-X", "PATCH", "-F", "body=@"+f, "repos/owner/repo/issues/2")
		if err != nil {
			t.Fatalf("direct tea PATCH: %v", err)
		}
		st.mu.Lock()
		got := st.decoded[2]
		st.mu.Unlock()
		if got == raw {
			t.Fatal("this tea did not strip the terminal LF; the transport adaptation is wrong for it")
		}
		if got != strings.TrimSuffix(raw, "\n") {
			t.Fatalf("tea stripped more than one LF: sent %q for file %q", got, raw)
		}
	})
}
