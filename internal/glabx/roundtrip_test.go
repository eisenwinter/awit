package glabx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/eisenwinter/awit/pkg/item"
)

// --- real glab transport (opt-in, mandatory for release acceptance) ---

// realGlabIssue is the isolated server's view of one issue.
type realGlabIssue struct {
	title       string
	description string
	labels      []string
	state       string // wire: opened|closed
}

// realGlabState is the isolated server's view of the issues glab touched.
type realGlabState struct {
	mu      sync.Mutex
	issues  map[int64]*realGlabIssue
	decoded map[int64]string // exact description strings glab sent per issue
	puts    int
	gets    int
}

// newRealGlabServer serves the GitLab API surface glab needs, locally and
// isolated: GET user, GET/PUT issue. The project parameter must arrive as
// one %2F-encoded segment (checked on the raw request URI). An optional
// /apps/gitlab installation prefix is stripped like glab's subfolder
// routing does. Issue 403 answers HTTP 403; issue 666 stores a corrupted
// description (a dishonest server); unknown issues 404.
func newRealGlabServer(t *testing.T) (*httptest.Server, *realGlabState) {
	t.Helper()
	st := &realGlabState{
		issues: map[int64]*realGlabIssue{
			1:  {title: "Root issue", description: "seed", labels: []string{"area::api", "p1"}, state: "opened"},
			2:  {title: "Prefixed issue", description: "seed", labels: []string{"docs"}, state: "closed"},
			71: {title: "Canary", description: "", labels: nil, state: "opened"},
			72: {title: "Canary control", description: "", labels: nil, state: "opened"},
		},
		decoded: map[int64]string{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.EscapedPath()
		prefix := ""
		if strings.HasPrefix(p, "/apps/gitlab/api/v4/") {
			prefix = "apps/gitlab"
			p = strings.TrimPrefix(p, "/apps/gitlab")
		}
		w.Header().Set("Content-Type", "application/json")
		rest, ok := strings.CutPrefix(p, "/api/v4/")
		if !ok {
			w.WriteHeader(404)
			fmt.Fprint(w, `{"message":"not the api base"}`)
			return
		}
		if rest == "user" {
			fmt.Fprint(w, `{"id":1,"username":"tester"}`)
			return
		}
		enc, ok := strings.CutPrefix(rest, "projects/")
		if !ok {
			w.WriteHeader(404)
			fmt.Fprintf(w, `{"message":"unknown path %s"}`, rest)
			return
		}
		// The project must travel as one encoded segment, never split.
		if !strings.Contains(r.RequestURI, "group%2Fsub%2Fproject") {
			w.WriteHeader(400)
			fmt.Fprint(w, `{"message":"project parameter is not one encoded segment"}`)
			return
		}
		dec, _ := url.PathUnescape(enc)
		proj, tail, _ := strings.Cut(dec, "/issues/")
		if proj != "group/sub/project" {
			w.WriteHeader(404)
			fmt.Fprint(w, `{"message":"unknown project"}`)
			return
		}
		n, err := strconv.ParseInt(tail, 10, 64)
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
		st.mu.Lock()
		defer st.mu.Unlock()
		iss, ok := st.issues[n]
		if !ok {
			w.WriteHeader(404)
			fmt.Fprint(w, `{"message":"404 Not found"}`)
			return
		}
		webURL := "http://" + r.Host + "/" + prefix
		webURL = strings.TrimSuffix(webURL, "/") + "/group/sub/project/-/issues/" + tail
		switch r.Method {
		case "PUT":
			st.puts++
			data, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(400)
				return
			}
			var req map[string]json.RawMessage
			if err := json.Unmarshal(data, &req); err != nil {
				w.WriteHeader(400)
				fmt.Fprint(w, `{"message":"bad json"}`)
				return
			}
			if rawDesc, ok := req["description"]; ok {
				var desc string
				if err := json.Unmarshal(rawDesc, &desc); err != nil {
					w.WriteHeader(400)
					fmt.Fprint(w, `{"message":"description is not a string"}`)
					return
				}
				st.decoded[n] = desc
				stored := desc
				if n == 666 {
					stored += "\nMUTATED BY SERVER"
				}
				iss.description = stored
			}
			if rawEvent, ok := req["state_event"]; ok {
				var event string
				if err := json.Unmarshal(rawEvent, &event); err != nil {
					w.WriteHeader(400)
					return
				}
				switch event {
				case "close":
					iss.state = "closed"
				case "reopen":
					iss.state = "opened"
				default:
					w.WriteHeader(422)
					fmt.Fprint(w, `{"message":"unknown state event"}`)
					return
				}
			}
			writeRealIssue(w, n, iss, webURL)
		case "GET":
			st.gets++
			writeRealIssue(w, n, iss, webURL)
		default:
			w.WriteHeader(405)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, st
}

func writeRealIssue(w http.ResponseWriter, n int64, iss *realGlabIssue, webURL string) {
	desc, _ := json.Marshal(iss.description)
	labels, _ := json.Marshal(iss.labels)
	title, _ := json.Marshal(iss.title)
	web, _ := json.Marshal(webURL)
	fmt.Fprintf(w, `{"id":987654,"iid":%d,"title":%s,"description":%s,"labels":%s,"state":%s,"web_url":%s}`,
		n, title, desc, labels, strconv.Quote(iss.state), web)
}

// writeRealGlabConfig isolates glab's configuration: a temp HOME holding one
// host section for the loopback server. Token and routing come only from
// this file; the operator's real configuration is untouched.
func writeRealGlabConfig(t *testing.T, srvURL, subfolder string) {
	t.Helper()
	u, err := url.Parse(srvURL)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	var cfg strings.Builder
	fmt.Fprintf(&cfg, "hosts:\n  127.0.0.1:\n    token: faketoken\n    api_protocol: http\n    api_host: %s\n", u.Host)
	if subfolder != "" {
		fmt.Fprintf(&cfg, "    subfolder: %s\n", subfolder)
	}
	dir := filepath.Join(home, ".config", "glab-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yml"), []byte(cfg.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
}

// TestGlabBodyRoundTrip exercises the byte-exact description transport
// against the real glab binary: isolated loopback HTTP server, isolated glab
// configuration, temp working directory. Opt-in via AWIT_TEST_REAL_GLAB=1
// but mandatory for release acceptance; when the flag is set glab must be
// present (a skip would silently fake the qualification).
func TestGlabBodyRoundTrip(t *testing.T) {
	if os.Getenv("AWIT_TEST_REAL_GLAB") != "1" {
		t.Skip("opt-in: set AWIT_TEST_REAL_GLAB=1 with a supported glab binary in PATH")
	}
	if _, err := exec.LookPath("glab"); err != nil {
		t.Fatal("AWIT_TEST_REAL_GLAB=1 is set but no glab binary is in PATH; release qualification cannot be skipped")
	}
	ver, err := exec.Command("glab", "--version").Output()
	if err != nil {
		t.Fatalf("glab --version: %v", err)
	}
	t.Logf("glab under test: %s", strings.TrimSpace(string(ver)))

	srv, st := newRealGlabServer(t)
	writeRealGlabConfig(t, srv.URL, "")
	t.Chdir(t.TempDir())
	// An unrelated ambient default host must never divert a host-scoped call.
	t.Setenv("GITLAB_HOST", "unrelated.example")
	ctx := context.Background()

	ext := item.External{Tracker: "gitlab", Repo: "group/sub/project", ID: 1, URL: srv.URL + "/group/sub/project/-/issues/1"}
	client, err := Open(ctx, ext)
	if err != nil {
		t.Fatalf("Open against the isolated server: %v", err)
	}

	for _, tc := range []struct{ name, body string }{
		{"empty", ""},
		{"leading blanks", "\n\nfirst\n"},
		{"no terminal LF", "line one\nline two"},
		{"single terminal LF", "line one\nline two\n"},
		{"multiple terminal LFs", "para\n\n\n"},
		{"lone terminal CR", "trail\r"},
		{"CRLF", "one\r\ntwo\r\n"},
		{"trailing spaces", "padded   \n"},
		{"unicode", "héllo wörld — 日本語\n"},
		{"quotes and backticks", "a `code` span, \"double\", 'single'\n"},
		{"comma and at", "a, b @mention\n"},
		{"literal null", "null"},
		{"digits", "127"},
		{"JSON-looking", "{\"a\": [1, 2]}"},
		{":namespace-looking", ":group/sub/project"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := client.SetBody(ctx, 1, []byte(tc.body)); err != nil {
				t.Fatalf("SetBody: %v", err)
			}
			st.mu.Lock()
			got := st.decoded[1]
			st.mu.Unlock()
			if got != tc.body {
				t.Fatalf("glab sent description %q, want byte-exact %q", got, tc.body)
			}
			iss, err := client.GetIssue(ctx, 1)
			if err != nil {
				t.Fatalf("GetIssue: %v", err)
			}
			if !bytes.Equal(iss.Body, []byte(tc.body)) {
				t.Fatalf("read-back %q, want %q", iss.Body, tc.body)
			}
			if iss.Title != "Root issue" || iss.State != "open" || strings.Join(iss.Labels, ",") != "area::api,p1" {
				t.Fatalf("body PUT changed title/labels/state: %+v", iss)
			}
		})
	}

	// The no-adaptation canary: a raw file travels byte-exact through glab's
	// -F reader, so SetBody must not add or strip anything. The
	// negative-control file proves the comparison is sensitive: one appended
	// LF changes the remote bytes.
	t.Run("raw file canary", func(t *testing.T) {
		raw := "canary body\nsecond line"
		f := filepath.Join(t.TempDir(), "body.raw")
		if err := os.WriteFile(f, []byte(raw), 0o600); err != nil {
			t.Fatal(err)
		}
		out, err := glabDirect(ctx, "api", "--hostname", "127.0.0.1", "--include", "--method", "PUT",
			"-F", "description=@"+f, "projects/group%2Fsub%2Fproject/issues/71")
		if err != nil {
			t.Fatalf("direct glab PUT: %v\n%s", err, out)
		}
		st.mu.Lock()
		got := st.decoded[71]
		st.mu.Unlock()
		if got != raw {
			t.Fatalf("raw file %q arrived as %q; the transport must not adapt bytes", raw, got)
		}
	})
	t.Run("negative control", func(t *testing.T) {
		raw := "canary body\nsecond line"
		f := filepath.Join(t.TempDir(), "body.lf")
		if err := os.WriteFile(f, []byte(raw+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		out, err := glabDirect(ctx, "api", "--hostname", "127.0.0.1", "--include", "--method", "PUT",
			"-F", "description=@"+f, "projects/group%2Fsub%2Fproject/issues/72")
		if err != nil {
			t.Fatalf("direct glab PUT: %v\n%s", err, out)
		}
		st.mu.Lock()
		got := st.decoded[72]
		st.mu.Unlock()
		if got == raw {
			t.Fatal("negative control failed: an appended LF left the remote bytes unchanged, the comparison is blind")
		}
		if got != raw+"\n" {
			t.Fatalf("negative control: remote %q, want exactly the LF-appended file", got)
		}
	})

	t.Run("HTTP403 is rejected", func(t *testing.T) {
		err := client.SetBody(ctx, 403, []byte("x"))
		if err == nil || !strings.Contains(err.Error(), "403") {
			t.Fatalf("err = %v, want HTTP 403 named", err)
		}
	})

	t.Run("unknown issue 404s", func(t *testing.T) {
		_, err := client.GetIssue(ctx, 404)
		if err == nil || !strings.Contains(err.Error(), "404") {
			t.Fatalf("err = %v, want HTTP 404 named", err)
		}
	})

	t.Run("quick-action refusal makes zero mutation requests", func(t *testing.T) {
		st.mu.Lock()
		before := st.puts
		old := st.issues[1].description
		st.mu.Unlock()
		err := client.SetBody(ctx, 1, []byte("looks fine\n/close\n"))
		if err == nil {
			t.Fatal("quick-action body must be refused")
		}
		st.mu.Lock()
		after := st.puts
		now := st.issues[1].description
		st.mu.Unlock()
		if after != before {
			t.Fatalf("%d mutation requests after refusal, want zero", after-before)
		}
		if now != old {
			t.Fatalf("refused body changed the remote: %q", now)
		}
	})

	t.Run("nested subfolder routing", func(t *testing.T) {
		writeRealGlabConfig(t, srv.URL, "apps/gitlab")
		sub := item.External{Tracker: "gitlab", Repo: "group/sub/project", ID: 2,
			URL: srv.URL + "/apps/gitlab/group/sub/project/-/issues/2"}
		sc, err := Open(ctx, sub)
		if err != nil {
			t.Fatalf("Open with subfolder: %v", err)
		}
		if sc.BaseURL != strings.ToLower(srv.URL)+"/apps/gitlab" {
			t.Fatalf("base = %q", sc.BaseURL)
		}
		iss, err := sc.GetIssue(ctx, 2)
		if err != nil {
			t.Fatalf("GetIssue through the prefix: %v", err)
		}
		if iss.Title != "Prefixed issue" || iss.State != "closed" {
			t.Fatalf("issue = %+v", iss)
		}
		if err := sc.SetBody(ctx, 2, []byte("prefixed write\n")); err != nil {
			t.Fatalf("SetBody through the prefix: %v", err)
		}
		st.mu.Lock()
		got := st.decoded[2]
		st.mu.Unlock()
		if got != "prefixed write\n" {
			t.Fatalf("prefixed write arrived as %q", got)
		}
	})
}

// glabDirect runs the real glab binary with the test's isolated environment
// (isolated HOME/config from t.Setenv, temp cwd). It bypasses the wrapper to
// pin the -F transport behavior itself.
func glabDirect(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "glab", args...)
	cmd.Stdin = nil
	cmd.Env = os.Environ()
	return cmd.CombinedOutput()
}
