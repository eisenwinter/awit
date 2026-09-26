// Package teax is the one concrete subprocess boundary to the `tea` Gitea
// CLI. It is not a provider interface and it never speaks HTTP itself:
// every operation shells out with an explicit argument vector, no shell, a
// disconnected stdin, separate stdout/stderr capture, and a bounded
// operation deadline. Tokens and response headers are never forwarded into
// errors or logs.
package teax

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/eisenwinter/awit/pkg/item"
)

// opTimeout bounds every tea subprocess invocation.
const opTimeout = 30 * time.Second

// Issue is one Gitea issue as fetched through tea. Number is the repository
// issue number, never Gitea's database-wide id.
type Issue struct {
	Number int64
	Title  string
	Body   []byte
	Labels []string
	State  string
	URL    string
}

// Client is a verified tea session for one Gitea installation and
// repository.
type Client struct {
	Login   string
	Repo    string
	BaseURL string
}

// run executes tea with an explicit argv, no shell, disconnected stdin and
// separate stdout/stderr capture, bounded by opTimeout.
func run(ctx context.Context, args ...string) (stdout, stderr []byte, err error) {
	ctx, cancel := context.WithTimeout(ctx, opTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "tea", args...)
	cmd.Stdin = nil
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return out.Bytes(), errb.Bytes(), fmt.Errorf("tea %s timed out after %s", args[0], opTimeout)
	}
	return out.Bytes(), errb.Bytes(), err
}

// diag renders subprocess diagnostics: the exit error plus trimmed stderr.
// stdout - which may carry response headers or, from a misbehaving tea,
// token material - is never forwarded.
func diag(op string, stderr []byte, err error) error {
	msg := strings.TrimSpace(string(stderr))
	if len(msg) > 300 {
		msg = msg[:300] + "…"
	}
	if msg == "" {
		return fmt.Errorf("tea %s: %w", op, err)
	}
	return fmt.Errorf("tea %s: %v: %s", op, err, msg)
}

// splitInclude parses the --include block tea prints on **stderr**: an HTTP
// status line, header lines, a blank line. It returns the status code and
// the remaining stderr (tea diagnostics) with the block removed, so response
// headers are never forwarded into errors. tea exits zero even on HTTP
// errors, so the status line is the authoritative success signal.
func splitInclude(stderr []byte) (status int, rest []byte, err error) {
	line, rem, _ := bytes.Cut(stderr, []byte("\n"))
	if !bytes.HasPrefix(line, []byte("HTTP/")) {
		return 0, nil, fmt.Errorf("no HTTP status line in tea stderr (is this tea with --include support?)")
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

// api runs `tea api` and returns the HTTP status code and response body.
// Flags always precede the endpoint. tea prints the --include status block
// on stderr and the bare response body on stdout.
func (c *Client) api(ctx context.Context, method, endpoint string, extra ...string) (int, []byte, error) {
	args := []string{"api", "--login", c.Login, "--repo", c.Repo, "--include", "-X", method}
	args = append(args, extra...)
	args = append(args, endpoint)
	out, stderr, err := run(ctx, args...)
	status, rest, serr := splitInclude(stderr)
	if err != nil {
		if serr != nil {
			// No include block at all: forward the raw diagnostics.
			rest = stderr
		}
		return 0, nil, diag("api "+method+" "+endpoint, rest, err)
	}
	if serr != nil {
		return 0, nil, fmt.Errorf("tea api %s %s: %w", method, endpoint, serr)
	}
	return status, out, nil
}

// IssueBase normalizes an issue URL to its installation base:
// scheme://host (lowercased) plus any path prefix before
// /owner/repo/issues/<number>. This is the value tea logins are matched
// against and part of the import duplicate identity.
func IssueBase(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return "", fmt.Errorf("not an absolute URL: %q", raw)
	}
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segs) < 4 {
		return "", fmt.Errorf("issue URL %q must end in /owner/repo/issues/<number>", raw)
	}
	base := strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host)
	if prefix := strings.Join(segs[:len(segs)-4], "/"); prefix != "" {
		base += "/" + prefix
	}
	return base, nil
}

// normalizeLoginURL normalizes a tea login URL the same way IssueBase
// normalizes the installation part of an issue URL.
func normalizeLoginURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return strings.TrimRight(strings.ToLower(raw), "/")
	}
	return strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host) + strings.TrimRight(u.Path, "/")
}

type loginEntry struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	// Token and Default are deliberately not modelled: tokens are never
	// read into awit, and being the default login is never a reason to
	// select one.
}

// Open verifies tea is usable for the given external issue mapping: the
// executable exists, the api subcommand is supported, exactly one login
// (or the explicitly named one) matches the issue's installation base, and
// that login authenticates (GET user). It never selects a login merely
// because it is the default.
func Open(ctx context.Context, issue item.External, login string) (*Client, error) {
	if _, err := exec.LookPath("tea"); err != nil {
		return nil, fmt.Errorf("tea executable not found in PATH; install tea and run tea login add")
	}
	if _, stderr, err := run(ctx, "api", "--help"); err != nil {
		return nil, diag("api (the installed tea is too old for the api subcommand; upgrade tea)", stderr, err)
	}
	base, err := IssueBase(issue.URL)
	if err != nil {
		return nil, err
	}
	out, stderr, err := run(ctx, "login", "list", "--output", "json")
	if err != nil {
		return nil, diag("login list (run tea login add)", stderr, err)
	}
	var logins []loginEntry
	if err := json.Unmarshal(out, &logins); err != nil {
		return nil, fmt.Errorf("could not parse tea login list output: %w", err)
	}
	var name string
	if login != "" {
		found := false
		for _, l := range logins {
			if l.Name != login {
				continue
			}
			found = true
			if normalizeLoginURL(l.URL) != base {
				return nil, fmt.Errorf("tea login %q points at %s, not %s", login, normalizeLoginURL(l.URL), base)
			}
		}
		if !found {
			return nil, fmt.Errorf("tea login %q not found; run tea login add", login)
		}
		name = login
	} else {
		var matches []string
		for _, l := range logins {
			if normalizeLoginURL(l.URL) == base {
				matches = append(matches, l.Name)
			}
		}
		switch len(matches) {
		case 0:
			return nil, fmt.Errorf("no tea login matches %s; run tea login add", base)
		case 1:
			name = matches[0]
		default:
			sort.Strings(matches)
			return nil, fmt.Errorf("multiple tea logins match %s (%s); pass --tea-login to choose one", base, strings.Join(matches, ", "))
		}
	}
	c := &Client{Login: name, Repo: issue.Repo, BaseURL: base}
	status, _, err := c.api(ctx, "GET", "user")
	if err != nil {
		return nil, err
	}
	switch {
	case status == 401 || status == 403:
		return nil, fmt.Errorf("tea login %q was rejected by %s (HTTP %d); run tea login add to refresh credentials", name, base, status)
	case status < 200 || status > 299:
		return nil, fmt.Errorf("tea api GET user: unexpected HTTP %d from %s", status, base)
	}
	return c, nil
}

// GetIssue fetches one issue by repository issue number. Required JSON
// fields are number, title and state; a null body maps to empty.
func (c *Client) GetIssue(ctx context.Context, number int64) (Issue, error) {
	owner, name, _ := strings.Cut(c.Repo, "/")
	endpoint := "repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/issues/" + strconv.FormatInt(number, 10)
	status, body, err := c.api(ctx, "GET", endpoint)
	if err != nil {
		return Issue{}, err
	}
	switch {
	case status == 404:
		return Issue{}, fmt.Errorf("gitea issue %s#%d not found at %s", c.Repo, number, c.BaseURL)
	case status < 200 || status > 299:
		return Issue{}, fmt.Errorf("tea api GET %s: HTTP %d", endpoint, status)
	}
	var raw struct {
		Number *int64  `json:"number"`
		Title  *string `json:"title"`
		Body   *string `json:"body"`
		State  *string `json:"state"`
		Labels []struct {
			Name *string `json:"name"`
		} `json:"labels"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return Issue{}, fmt.Errorf("malformed issue response: %w", err)
	}
	if raw.Number == nil {
		return Issue{}, fmt.Errorf("malformed issue response: missing number")
	}
	if raw.Title == nil {
		return Issue{}, fmt.Errorf("malformed issue response: missing title")
	}
	if raw.State == nil {
		return Issue{}, fmt.Errorf("malformed issue response: missing state")
	}
	iss := Issue{Number: *raw.Number, Title: *raw.Title, State: *raw.State, URL: raw.HTMLURL}
	if raw.Body != nil {
		iss.Body = []byte(*raw.Body)
	}
	for _, l := range raw.Labels {
		if l.Name == nil {
			return Issue{}, fmt.Errorf("malformed issue response: label without a name")
		}
		iss.Labels = append(iss.Labels, *l.Name)
	}
	return iss, nil
}

// transportFile writes body plus exactly one transport LF to a private
// temporary file: tea's -F @file reader strips one terminal LF, so the extra
// LF leaves the original body intact (including empty bodies, zero or more
// terminal newlines, and CRLF). The file is written temp-then-rename with
// mode 0600 and closed before tea opens it; cleanup deletes it and must run
// on every exit path.
func transportFile(body []byte) (path string, cleanup func(), err error) {
	dir, err := os.MkdirTemp("", "awit-tea-body-*")
	if err != nil {
		return "", nil, err
	}
	cleanup = func() { os.RemoveAll(dir) }
	tmp, err := os.CreateTemp(dir, ".tmp-*") // mode 0600
	if err != nil {
		cleanup()
		return "", nil, err
	}
	payload := make([]byte, 0, len(body)+1)
	payload = append(payload, body...)
	payload = append(payload, '\n')
	if _, err := tmp.Write(payload); err != nil {
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

// SetBody replaces the body of the given repository issue number with
// exactly the provided bytes. It never touches title, labels, or state.
// A 2xx status is required (tea exits zero on HTTP errors), the response
// must confirm the issue number (and installation, when it carries a URL),
// and the remote body must equal the pushed bytes - verified against the
// PATCH response, or with a GET of the same issue when the response omits
// the body. A mismatch is an error, never success.
func (c *Client) SetBody(ctx context.Context, number int64, body []byte) error {
	payload, cleanup, err := transportFile(body)
	if err != nil {
		return err
	}
	defer cleanup()
	owner, name, _ := strings.Cut(c.Repo, "/")
	endpoint := "repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/issues/" + strconv.FormatInt(number, 10)
	status, respBody, err := c.api(ctx, "PATCH", endpoint, "-F", "body=@"+payload)
	if err != nil {
		return err
	}
	if status < 200 || status > 299 {
		return fmt.Errorf("tea api PATCH %s: HTTP %d", endpoint, status)
	}
	return c.verifyBody(ctx, number, body, respBody)
}

// verifyBody confirms the remote took exactly the pushed bytes. A response
// without a body field falls back to a GET of the same issue.
func (c *Client) verifyBody(ctx context.Context, number int64, want, respBody []byte) error {
	if len(bytes.TrimSpace(respBody)) > 0 {
		var raw struct {
			Number  *int64  `json:"number"`
			Body    *string `json:"body"`
			HTMLURL string  `json:"html_url"`
		}
		if err := json.Unmarshal(respBody, &raw); err != nil {
			return fmt.Errorf("malformed issue response: %w", err)
		}
		if raw.Number == nil || *raw.Number != number {
			return fmt.Errorf("push verification failed for issue #%d: the response did not confirm the issue number", number)
		}
		if raw.HTMLURL != "" {
			base, err := IssueBase(raw.HTMLURL)
			if err != nil || base != c.BaseURL {
				return fmt.Errorf("push verification failed for issue #%d: response URL %q is not on %s", number, raw.HTMLURL, c.BaseURL)
			}
		}
		if raw.Body != nil {
			if !bytes.Equal([]byte(*raw.Body), want) {
				return fmt.Errorf("push verification failed for issue #%d: remote body (%d bytes) does not equal the local bytes (%d bytes)", number, len(*raw.Body), len(want))
			}
			return nil
		}
	}
	iss, err := c.GetIssue(ctx, number)
	if err != nil {
		return fmt.Errorf("push verification failed for issue #%d: %w", number, err)
	}
	if iss.Number != number {
		return fmt.Errorf("push verification failed for issue #%d: GET returned issue #%d", number, iss.Number)
	}
	if !bytes.Equal(iss.Body, want) {
		return fmt.Errorf("push verification failed for issue #%d: remote body (%d bytes) does not equal the local bytes (%d bytes)", number, len(iss.Body), len(want))
	}
	return nil
}

// SetState replaces the state of the given repository issue number with
// exactly "open" or "closed" (PATCH -f state=<want>). It never touches
// title, labels, or body. A 2xx status is required (tea exits zero on HTTP
// errors), the response must confirm the issue number (and installation,
// when it carries a URL), and the remote state must equal the pushed
// state - verified against the PATCH response, or with a GET of the same
// issue when the response omits the state. A mismatch is an error, never
// success. The remote is never read to decide what to write.
func (c *Client) SetState(ctx context.Context, number int64, state string) error {
	if state != "open" && state != "closed" {
		return fmt.Errorf("invalid state %q: must be open or closed", state)
	}
	owner, name, _ := strings.Cut(c.Repo, "/")
	endpoint := "repos/" + url.PathEscape(owner) + "/" + url.PathEscape(name) + "/issues/" + strconv.FormatInt(number, 10)
	status, respBody, err := c.api(ctx, "PATCH", endpoint, "-f", "state="+state)
	if err != nil {
		return err
	}
	if status < 200 || status > 299 {
		return fmt.Errorf("tea api PATCH %s: HTTP %d", endpoint, status)
	}
	return c.verifyState(ctx, number, state, respBody)
}

// verifyState confirms the remote took exactly the pushed state. A response
// without a state field falls back to a GET of the same issue.
func (c *Client) verifyState(ctx context.Context, number int64, want string, respBody []byte) error {
	if len(bytes.TrimSpace(respBody)) > 0 {
		var raw struct {
			Number  *int64  `json:"number"`
			State   *string `json:"state"`
			HTMLURL string  `json:"html_url"`
		}
		if err := json.Unmarshal(respBody, &raw); err != nil {
			return fmt.Errorf("malformed issue response: %w", err)
		}
		if raw.Number == nil || *raw.Number != number {
			return fmt.Errorf("push verification failed for issue #%d: the response did not confirm the issue number", number)
		}
		if raw.HTMLURL != "" {
			base, err := IssueBase(raw.HTMLURL)
			if err != nil || base != c.BaseURL {
				return fmt.Errorf("push verification failed for issue #%d: response URL %q is not on %s", number, raw.HTMLURL, c.BaseURL)
			}
		}
		if raw.State != nil {
			if *raw.State != want {
				return fmt.Errorf("push verification failed for issue #%d: remote state %q does not equal %q", number, *raw.State, want)
			}
			return nil
		}
	}
	iss, err := c.GetIssue(ctx, number)
	if err != nil {
		return fmt.Errorf("push verification failed for issue #%d: %w", number, err)
	}
	if iss.Number != number {
		return fmt.Errorf("push verification failed for issue #%d: GET returned issue #%d", number, iss.Number)
	}
	if iss.State != want {
		return fmt.Errorf("push verification failed for issue #%d: remote state %q does not equal %q", number, iss.State, want)
	}
	return nil
}
