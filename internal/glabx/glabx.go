// Package glabx is the one concrete subprocess boundary to the `glab`
// GitLab CLI. It is not a provider interface and it never speaks HTTP
// itself: every operation shells out with an explicit argument vector, no
// shell, a disconnected stdin, separate stdout/stderr capture, and a bounded
// operation deadline. glab is pre-authenticated by the operator; this
// package never logs in, selects logins, reads tokens, or writes
// configuration. Response headers and body content are never forwarded into
// errors.
//
// Supported glab: 1.118.0 (raw -F @file transport pinned by
// TestGlabBodyRoundTrip; the transport file carries exactly the body bytes,
// no LF adaptation).
package glabx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/eisenwinter/awit/pkg/item"
)

// opTimeout bounds every glab subprocess invocation.
const opTimeout = 30 * time.Second

// Issue is one GitLab issue as fetched through glab. Number is the decoded
// iid, never GitLab's database-wide id.
type Issue struct {
	Number int64
	Title  string
	Body   []byte // raw decoded description; null maps to empty
	Labels []string
	State  string // normalized open or closed
	URL    string // validated web_url
}

// Client is a verified glab session for one GitLab installation and project.
// Host is the link host, retaining any explicit port; every request passes
// the bare hostname to glab --hostname (glab rejects host:port there) and
// pins the child GITLAB_HOST to it.
type Client struct {
	Host    string
	Repo    string
	BaseURL string
}

// hostname returns the bare host without a port for glab --hostname, which
// rejects host:port values.
func (c *Client) hostname() string {
	u, err := url.Parse("https://" + c.Host)
	if err != nil {
		return c.Host
	}
	return u.Hostname()
}

// run executes glab with an explicit argv, no shell, disconnected stdin and
// separate stdout/stderr capture, bounded by opTimeout. The child inherits
// the process environment with prompting disabled, the default host pinned
// to host, and HTTP debug output stripped so a ambient GLAB_DEBUG_HTTP=1
// cannot leak response bodies into captured output.
func run(ctx context.Context, host string, args ...string) (stdout, stderr []byte, err error) {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "glab", args...)
	cmd.Stdin = nil

	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	cmd.Env = childEnv(host)
	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return out.Bytes(), errb.Bytes(), fmt.Errorf("glab %s timed out after %s", args[0], opTimeout)
	}
	return out.Bytes(), errb.Bytes(), err
}

// childEnv pins the child to host and disables prompting and HTTP debug
// output. Token material is never added: authentication comes from the
// operator's pre-authenticated glab configuration.
func childEnv(host string) []string {
	env := os.Environ()
	out := make([]string, 0, len(env)+2)
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		switch k {
		case "GLAB_DEBUG_HTTP":
			continue
		case "GITLAB_HOST", "GLAB_NO_PROMPT":
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "GITLAB_HOST="+host, "GLAB_NO_PROMPT=1")
	return out
}

// diag renders subprocess diagnostics without forwarding arbitrary
// subprocess output: stdout carries the --include headers and response body
// and is never included. stderr is glab's own diagnostic (config, network,
// usage); only its first line, capped, is kept.
func diag(op, host string, stderr []byte, err error) error {
	line, _, _ := bytes.Cut(stderr, []byte("\n"))
	msg := strings.TrimSpace(string(line))
	if len(msg) > 200 {
		msg = msg[:200] + "…"
	}
	if msg == "" {
		return fmt.Errorf("glab %s on %s: %w", op, host, err)
	}
	return fmt.Errorf("glab %s on %s: %v: %s", op, host, err, msg)
}

// splitInclude parses the --include block glab prints on stdout: an HTTP
// status line, header lines, a blank line, then the bare JSON payload. It
// returns the status code and the payload.
func splitInclude(stdout []byte) (status int, payload []byte, err error) {
	line, rem, _ := bytes.Cut(stdout, []byte("\n"))
	line = []byte(strings.TrimRight(string(line), "\r"))
	if !bytes.HasPrefix(line, []byte("HTTP/")) {
		return 0, nil, fmt.Errorf("no HTTP status line in glab stdout (is this glab with --include support?)")
	}
	fields := strings.Fields(string(line))
	if len(fields) < 2 {
		return 0, nil, fmt.Errorf("malformed HTTP status line %q", strings.TrimSpace(string(line)))
	}
	status, err = strconv.Atoi(fields[1])
	if err != nil {
		return 0, nil, fmt.Errorf("malformed HTTP status line %q", strings.TrimSpace(string(line)))
	}
	for len(rem) > 0 {
		l, r, _ := bytes.Cut(rem, []byte("\n"))
		if len(bytes.TrimSpace(l)) == 0 {
			return status, r, nil
		}
		rem = r
	}
	return status, nil, nil
}

// api runs `glab api` and returns the HTTP status code and response payload.
// Flags always precede the endpoint. glab prints the --include status block
// on stdout followed by the bare response body; unlike tea, glab exits
// nonzero on HTTP errors, so the process exit is checked first.
func (c *Client) api(ctx context.Context, method, endpoint string, extra ...string) (int, []byte, error) {
	host := c.hostname()
	args := []string{"api", "--hostname", host, "--include", "--method", method}
	args = append(args, extra...)
	args = append(args, endpoint)
	out, stderr, err := run(ctx, host, args...)
	op := "api " + method + " " + endpoint
	if err != nil {
		if status, _, serr := splitInclude(out); serr == nil {
			return 0, nil, fmt.Errorf("glab %s on %s: HTTP %d (%v)", op, c.Host, status, err)
		}
		return 0, nil, diag(op, c.Host, stderr, err)
	}
	status, payload, serr := splitInclude(out)
	if serr != nil {
		return 0, nil, fmt.Errorf("glab %s on %s: %w", op, c.Host, serr)
	}
	return status, payload, nil
}

// configGet reads one named non-secret glab setting for host through the
// read-only lookup (`environment, then local, then global` per glab): unset
// prints nothing with exit 0. Only non-secret settings are ever requested;
// token values are never read.
func configGet(ctx context.Context, host, key string) (string, error) {
	if _, err := exec.LookPath("glab"); err != nil {
		return "", fmt.Errorf("glab executable not found in PATH; install glab to work with GitLab issues")
	}
	out, stderr, err := run(ctx, host, "config", "get", key, "--host", host)
	if err != nil {
		return "", diag("config get "+key, host, stderr, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// normalizeSubfolder trims the slashes glab ignores, so `gitlab`,
// `/gitlab` and `gitlab/` compare equal.
func normalizeSubfolder(s string) string {
	return strings.Trim(strings.TrimSpace(s), "/")
}

// gitlabPath splits a GitLab issue URL path into its segments and locates
// the `/-/` resource marker: `<repo...> / - / issues|work_items / <iid>`.
// It returns the segments before the marker and the marker tail.
func gitlabPath(path string) (before []string, kind, id string, err error) {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	for i := range segs {
		if segs[i] != "-" {
			continue
		}
		if i+2 == len(segs)-1 && (segs[i+1] == "issues" || segs[i+1] == "work_items") {
			return segs[:i], segs[i+1], segs[i+2], nil
		}
	}
	return nil, "", "", fmt.Errorf("url path must end in /repo/-/issues/id or /repo/-/work_items/id")
}

// ParseIssueURL resolves a GitLab issue or work_items link to its stored
// mapping. Only the two issue-link shapes are accepted. The host's effective
// installation subfolder comes from the read-only glab configuration lookup
// (`glab config get subfolder --host <URL host>`; GITLAB_SUBFOLDER wins
// inside glab); unset means a root install. Only that verified prefix is
// stripped, on a segment boundary; without one every segment stays.
func ParseIssueURL(ctx context.Context, raw string) (item.External, error) {
	if _, err := exec.LookPath("glab"); err != nil {
		return item.External{}, fmt.Errorf("glab executable not found in PATH; install glab to work with GitLab issues")
	}
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return item.External{}, fmt.Errorf("not an absolute URL: %q", raw)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return item.External{}, fmt.Errorf("not an absolute HTTP(S) URL: %q", raw)
	}
	before, _, idStr, err := gitlabPath(u.Path)
	if err != nil {
		return item.External{}, fmt.Errorf("not a GitLab issue URL %q: %v", raw, err)
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return item.External{}, fmt.Errorf("not a GitLab issue URL %q: invalid issue id %q", raw, idStr)
	}
	sub, err := configGet(ctx, u.Hostname(), "subfolder")
	if err != nil {
		return item.External{}, err
	}
	segs := before
	if sub = normalizeSubfolder(sub); sub != "" {
		want := strings.Split(sub, "/")
		if len(segs) < len(want) {
			return item.External{}, fmt.Errorf("issue URL %q does not start with the configured subfolder %q; check glab config get subfolder --host %s", raw, sub, u.Hostname())
		}
		for i := range want {
			if segs[i] != want[i] {
				return item.External{}, fmt.Errorf("issue URL %q does not start with the configured subfolder %q; check glab config get subfolder --host %s", raw, sub, u.Hostname())
			}
		}
		segs = segs[len(want):]
	}
	if len(segs) < 2 {
		return item.External{}, fmt.Errorf("not a GitLab issue URL %q: repo must have at least two path segments", raw)
	}
	ext := item.External{Tracker: "gitlab", Repo: strings.Join(segs, "/"), ID: id, URL: raw}
	if err := item.ValidateExternal(ext); err != nil {
		return item.External{}, err
	}
	return ext, nil
}

// IssueBase validates the complete GitLab mapping and normalizes it to its
// installation base: scheme://host (case-normalized) plus any path prefix
// before the exact repo plus issue/work_items suffix. Port and
// installation-path case are retained; different schemes, ports, or prefixes
// never compare equal.
func IssueBase(issue item.External) (string, error) {
	if err := item.ValidateExternal(issue); err != nil {
		return "", err
	}
	u, err := url.Parse(issue.URL)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("not an absolute URL: %q", issue.URL)
	}
	before, kind, idStr, err := gitlabPath(u.Path)
	if err != nil {
		return "", fmt.Errorf("invalid external: %v", err)
	}
	_ = kind
	repoSegs := strings.Split(issue.Repo, "/")
	if len(before) < len(repoSegs) {
		return "", fmt.Errorf("invalid external: url path does not end in /%s/-/issues|work_items/%d", issue.Repo, issue.ID)
	}
	rest := before[len(before)-len(repoSegs):]
	for i := range repoSegs {
		if rest[i] != repoSegs[i] {
			return "", fmt.Errorf("invalid external: url path does not end in /%s/-/issues|work_items/%d", issue.Repo, issue.ID)
		}
	}
	if idStr != strconv.FormatInt(issue.ID, 10) {
		return "", fmt.Errorf("invalid external: url path does not end in /%s/-/issues|work_items/%d", issue.Repo, issue.ID)
	}
	base := strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
	if prefix := strings.Join(before[:len(before)-len(repoSegs)], "/"); prefix != "" {
		base += "/" + prefix
	}
	return base, nil
}

// Open verifies glab is usable for the given GitLab issue mapping: the
// executable exists, the effective subfolder, API host, and API protocol for
// the link host match the URL's installation base, and the pre-authenticated
// setup answers GET user with a positive numeric id. It never logs in,
// selects logins, reads tokens, or writes configuration.
func Open(ctx context.Context, issue item.External) (*Client, error) {
	if _, err := exec.LookPath("glab"); err != nil {
		return nil, fmt.Errorf("glab executable not found in PATH; install glab to work with GitLab issues")
	}
	base, err := IssueBase(issue)
	if err != nil {
		return nil, err
	}
	bu, err := url.Parse(base)
	if err != nil || bu.Host == "" {
		return nil, fmt.Errorf("invalid installation base %q", base)
	}
	host := bu.Host
	hostname := bu.Hostname()
	prefix := strings.Trim(bu.Path, "/")

	sub, err := configGet(ctx, hostname, "subfolder")
	if err != nil {
		return nil, err
	}
	if normalizeSubfolder(sub) != prefix {
		return nil, fmt.Errorf("glab subfolder for %s is %q but the issue URL needs %q; check glab config get subfolder --host %s (GITLAB_SUBFOLDER overrides file configuration)", host, normalizeSubfolder(sub), prefix, hostname)
	}
	rawHost, err := configGet(ctx, hostname, "api_host")
	if err != nil {
		return nil, err
	}
	if rawHost == "" {
		if port := bu.Port(); port != "" && !isDefaultPort(bu.Scheme, port) {
			return nil, fmt.Errorf("issue URL has explicit port %q which glab --hostname cannot address; set api_host for %s (glab config get api_host --host %s, or GITLAB_API_HOST)", port, host, hostname)
		}
	} else if rawHost != host {
		return nil, fmt.Errorf("glab api_host for %s is %q, not the issue host; refusing to target another instance (glab config get api_host --host %s, or GITLAB_API_HOST)", host, rawHost, hostname)
	}
	rawProto, err := configGet(ctx, hostname, "api_protocol")
	if err != nil {
		return nil, err
	}
	proto := rawProto
	if proto == "" {
		proto = "https"
	}
	if !strings.EqualFold(proto, bu.Scheme) {
		return nil, fmt.Errorf("glab api_protocol for %s is %q but the issue URL is %q; check glab config get api_protocol --host %s", host, proto, bu.Scheme, hostname)
	}

	c := &Client{Host: host, Repo: issue.Repo, BaseURL: base}
	status, payload, err := c.api(ctx, "GET", "user")
	if err != nil {
		return nil, fmt.Errorf("glab auth check failed for %s: %v; check glab auth status --hostname %s and the existing pre-authenticated setup", host, err, hostname)
	}
	if status < 200 || status > 299 {
		return nil, fmt.Errorf("glab auth check failed for %s: HTTP %d; check glab auth status --hostname %s and the existing pre-authenticated setup", host, status, hostname)
	}
	var user struct {
		ID *int64 `json:"id"`
	}
	if err := json.Unmarshal(payload, &user); err != nil || user.ID == nil || *user.ID <= 0 {
		return nil, fmt.Errorf("glab auth check failed for %s: invalid user response; check glab auth status --hostname %s and the existing pre-authenticated setup", host, hostname)
	}
	return c, nil
}

func isDefaultPort(scheme, port string) bool {
	return (scheme == "https" && port == "443") || (scheme == "http" && port == "80")
}

// projectEndpoint builds the explicit single-segment encoded project issue
// endpoint: the full project path is escaped once, never split into API path
// segments or double-escaped. The installation subfolder belongs to glab's
// verified API base, not the project parameter.
func (c *Client) projectEndpoint(number int64) string {
	return "projects/" + url.PathEscape(c.Repo) + "/issues/" + strconv.FormatInt(number, 10)
}

// GetIssue fetches one issue by iid. Required JSON fields are a positive
// matching iid, string title, present string-or-null description, an array
// of string labels, a wire state of opened|closed, and a web_url that
// verifies against the complete repo, iid, and installation base. The global
// id is ignored entirely.
func (c *Client) GetIssue(ctx context.Context, number int64) (Issue, error) {
	endpoint := c.projectEndpoint(number)
	status, payload, err := c.api(ctx, "GET", endpoint)
	if err != nil {
		return Issue{}, err
	}
	switch {
	case status == 404:
		return Issue{}, fmt.Errorf("gitlab issue %s!%d not found at %s", c.Repo, number, c.BaseURL)
	case status < 200 || status > 299:
		return Issue{}, fmt.Errorf("glab api GET %s on %s: HTTP %d", endpoint, c.Host, status)
	}
	return c.decodeIssue(number, payload)
}

// decodeIssue validates one issue payload: full identity plus every required
// field, even after exit 0 and status 200.
func (c *Client) decodeIssue(number int64, payload []byte) (Issue, error) {
	var present map[string]json.RawMessage
	if err := json.Unmarshal(payload, &present); err != nil {
		return Issue{}, fmt.Errorf("malformed issue response: %w", err)
	}
	var raw struct {
		Iid         *int64    `json:"iid"`
		Title       *string   `json:"title"`
		Description *string   `json:"description"`
		Labels      *[]string `json:"labels"`
		State       *string   `json:"state"`
		WebURL      *string   `json:"web_url"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return Issue{}, fmt.Errorf("malformed issue response: %w", err)
	}
	if raw.Iid == nil || *raw.Iid <= 0 || *raw.Iid != number {
		return Issue{}, fmt.Errorf("malformed issue response: missing or mismatched iid (want %d)", number)
	}
	if raw.Title == nil {
		return Issue{}, fmt.Errorf("malformed issue response: missing title")
	}
	if _, ok := present["description"]; !ok {
		return Issue{}, fmt.Errorf("malformed issue response: missing description")
	}
	if raw.Labels == nil {
		return Issue{}, fmt.Errorf("malformed issue response: missing labels")
	}
	if raw.State == nil {
		return Issue{}, fmt.Errorf("malformed issue response: missing state")
	}
	state, err := normalizeState(*raw.State)
	if err != nil {
		return Issue{}, fmt.Errorf("malformed issue response: %w", err)
	}
	if raw.WebURL == nil {
		return Issue{}, fmt.Errorf("malformed issue response: missing web_url")
	}
	webExt := item.External{Tracker: "gitlab", Repo: c.Repo, ID: number, URL: *raw.WebURL}
	webBase, err := IssueBase(webExt)
	if err != nil {
		return Issue{}, fmt.Errorf("malformed issue response: web_url %q does not identify %s!%d on %s", *raw.WebURL, c.Repo, number, c.BaseURL)
	}
	if webBase != c.BaseURL {
		return Issue{}, fmt.Errorf("malformed issue response: web_url %q is not on %s", *raw.WebURL, c.BaseURL)
	}
	iss := Issue{Number: number, Title: *raw.Title, State: state, URL: *raw.WebURL}
	if raw.Description != nil {
		iss.Body = []byte(*raw.Description)
	}
	iss.Labels = append(iss.Labels, (*raw.Labels)...)
	return iss, nil
}

// normalizeState maps the GitLab wire states to the local open|closed.
func normalizeState(s string) (string, error) {
	switch s {
	case "opened":
		return "open", nil
	case "closed":
		return "closed", nil
	default:
		return "", fmt.Errorf("unsupported issue state %q", s)
	}
}

// validateBody refuses bodies GitLab would execute instead of store. Any
// line beginning at column zero with a slash followed by a lowercase ASCII
// letter is quick-action shaped (conservative prefix rule ^/[a-z]+,
// arguments and unknown commands included). There is deliberately no
// Markdown parsing and no exemption for fenced code blocks: a slash line at
// column zero inside a fence is refused too, so the diagnostic names fenced
// blocks and requires the slash line to move away from column zero. awit
// never rewrites the body to make it safe. Invalid UTF-8 is refused rather
// than letting JSON encoding replace bytes and change the payload.
func validateBody(body []byte) error {
	if !utf8.Valid(body) {
		return fmt.Errorf("body is not valid UTF-8; glab would change bytes on write")
	}
	for _, line := range bytes.Split(body, []byte("\n")) {
		if len(line) >= 2 && line[0] == '/' && line[1] >= 'a' && line[1] <= 'z' {
			cmd := line[1:]
			if i := bytes.IndexAny(cmd, " \t\r"); i >= 0 {
				cmd = cmd[:i]
			}
			short := string(cmd)
			if len(short) > 20 {
				short = short[:20] + "…"
			}
			return fmt.Errorf("body contains a line starting with \"/%s\" at column zero, which GitLab would execute as a quick action instead of storing it; move the slash line away from column zero (even inside fenced code blocks, which awit does not exempt) and retry - awit never rewrites the body", short)
		}
	}
	return nil
}

// transportFile writes exactly body to a private temporary file: glab sends
// -F file values byte-exact, so no LF is added or stripped and no
// body-plus-newline copy is allocated. The file is written temp-then-rename
// with mode 0600 and closed before glab opens it; cleanup deletes it and
// must run on every exit path.
func transportFile(body []byte) (path string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "awit-glab-body-*")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { os.RemoveAll(dir) }
	tmp, err := os.CreateTemp(dir, ".tmp-*") // mode 0600
	if err != nil {
		cleanup()
		return "", nil, err
	}
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		cleanup()
		return "", nil, err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	final := filepath.Join(dir, "body")
	if err := os.Rename(tmp.Name(), final); err != nil {
		cleanup()
		return "", nil, err
	}
	return final, cleanup, nil
}

// SetBody replaces the description of the given issue iid with exactly the
// provided bytes. Quick-action-shaped bodies are refused before any file or
// subprocess exists. The request carries only the description field; title,
// labels, and state are never included. Success requires a 2xx status,
// verified identity, and a byte-exact description - against the PUT
// response, or a GET of the same issue when the response omits the changed
// field. A mismatch is an error, never success or rollback.
func (c *Client) SetBody(ctx context.Context, number int64, body []byte) error {
	if err := validateBody(body); err != nil {
		return err
	}
	path, cleanup, err := transportFile(body)
	if err != nil {
		return err
	}
	defer cleanup()
	endpoint := c.projectEndpoint(number)
	status, respBody, err := c.api(ctx, "PUT", endpoint, "-F", "description=@"+path)
	if err != nil {
		return err
	}
	if status < 200 || status > 299 {
		return fmt.Errorf("glab api PUT %s on %s: HTTP %d", endpoint, c.Host, status)
	}
	return c.verifyBody(ctx, number, body, respBody)
}

// verifyBody confirms the remote took exactly the pushed bytes. A present
// wrong identity, malformed JSON, a wrongly typed field, or a present
// description mismatch is an error; only a valid partial response missing
// the changed field (or an empty 2xx body) falls back to a GET.
func (c *Client) verifyBody(ctx context.Context, number int64, want, respBody []byte) error {
	if len(bytes.TrimSpace(respBody)) > 0 {
		var present map[string]json.RawMessage
		if err := json.Unmarshal(respBody, &present); err != nil {
			return fmt.Errorf("malformed issue response: %w", err)
		}
		if err := c.checkResponseIdentity(number, present); err != nil {
			return err
		}
		if rawDesc, ok := present["description"]; ok {
			if string(rawDesc) == "null" {
				if len(want) != 0 {
					return fmt.Errorf("push verification failed for issue !%d: remote description is null, want %d bytes", number, len(want))
				}
				return nil
			}
			var desc string
			if err := json.Unmarshal(rawDesc, &desc); err != nil {
				return fmt.Errorf("malformed issue response: description is not a string")
			}
			if !bytes.Equal([]byte(desc), want) {
				return fmt.Errorf("push verification failed for issue !%d: remote description (%d bytes) does not equal the local bytes (%d bytes)", number, len(desc), len(want))
			}
			return nil
		}
	}
	iss, err := c.GetIssue(ctx, number)
	if err != nil {
		return fmt.Errorf("push verification failed for issue !%d: %w", number, err)
	}
	if iss.Number != number {
		return fmt.Errorf("push verification failed for issue !%d: GET returned issue !%d", number, iss.Number)
	}
	if !bytes.Equal(iss.Body, want) {
		return fmt.Errorf("push verification failed for issue !%d: remote description (%d bytes) does not equal the local bytes (%d bytes)", number, len(iss.Body), len(want))
	}
	return nil
}

// checkResponseIdentity rejects a PUT response that contradicts the target:
// a present wrong iid or a web_url that does not identify this exact
// repo/iid/base is an error, never hidden with a fallback.
func (c *Client) checkResponseIdentity(number int64, present map[string]json.RawMessage) error {
	if rawIid, ok := present["iid"]; ok {
		var iid int64
		if err := json.Unmarshal(rawIid, &iid); err != nil {
			return fmt.Errorf("malformed issue response: iid is not an integer")
		}
		if iid != number {
			return fmt.Errorf("push verification failed for issue !%d: the response did not confirm the issue number", number)
		}
	}
	if rawURL, ok := present["web_url"]; ok {
		var webURL string
		if err := json.Unmarshal(rawURL, &webURL); err != nil {
			return fmt.Errorf("malformed issue response: web_url is not a string")
		}
		webBase, err := IssueBase(item.External{Tracker: "gitlab", Repo: c.Repo, ID: number, URL: webURL})
		if err != nil || webBase != c.BaseURL {
			return fmt.Errorf("push verification failed for issue !%d: response URL %q is not on %s", number, webURL, c.BaseURL)
		}
	}
	return nil
}

// SetState replaces the state of the given issue iid with exactly "open" or
// "closed" (state_event reopen|close). It never touches the description,
// title, or labels, never reads the remote state to decide what to write,
// and never retries. Verification mirrors SetBody: normalized returned or
// read-back state plus full identity, or an error.
func (c *Client) SetState(ctx context.Context, number int64, state string) error {
	var event string
	switch state {
	case "open":
		event = "reopen"
	case "closed":
		event = "close"
	default:
		return fmt.Errorf("invalid state %q: must be open or closed", state)
	}
	endpoint := c.projectEndpoint(number)
	status, respBody, err := c.api(ctx, "PUT", endpoint, "-f", "state_event="+event)
	if err != nil {
		return err
	}
	if status < 200 || status > 299 {
		return fmt.Errorf("glab api PUT %s on %s: HTTP %d", endpoint, c.Host, status)
	}
	return c.verifyState(ctx, number, state, respBody)
}

// verifyState confirms the remote took exactly the pushed state, normalized
// to open|closed. Fallback rules mirror verifyBody.
func (c *Client) verifyState(ctx context.Context, number int64, want string, respBody []byte) error {
	if len(bytes.TrimSpace(respBody)) > 0 {
		var present map[string]json.RawMessage
		if err := json.Unmarshal(respBody, &present); err != nil {
			return fmt.Errorf("malformed issue response: %w", err)
		}
		if err := c.checkResponseIdentity(number, present); err != nil {
			return err
		}
		if rawState, ok := present["state"]; ok {
			var wire string
			if err := json.Unmarshal(rawState, &wire); err != nil {
				return fmt.Errorf("malformed issue response: state is not a string")
			}
			got, err := normalizeState(wire)
			if err != nil {
				return fmt.Errorf("malformed issue response: %w", err)
			}
			if got != want {
				return fmt.Errorf("push verification failed for issue !%d: remote state %q does not equal %q", number, got, want)
			}
			return nil
		}
	}
	iss, err := c.GetIssue(ctx, number)
	if err != nil {
		return fmt.Errorf("push verification failed for issue !%d: %w", number, err)
	}
	if iss.Number != number {
		return fmt.Errorf("push verification failed for issue !%d: GET returned issue !%d", number, iss.Number)
	}
	if iss.State != want {
		return fmt.Errorf("push verification failed for issue !%d: remote state %q does not equal %q", number, iss.State, want)
	}
	return nil
}
