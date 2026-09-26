package ops

import (
	"bytes"
	"context"
	"fmt"

	"github.com/eisenwinter/awit/internal/glabx"
	"github.com/eisenwinter/awit/internal/teax"
	"github.com/eisenwinter/awit/pkg/item"
)

// ExternalCheckRow is one row of `awit external check` output. Result is
// match, drift, or error; an authentication or read failure is an error
// row, never drift.
type ExternalCheckRow struct {
	ID     string `json:"id"`
	URL    string `json:"url,omitempty"`
	Result string `json:"result"` // match | drift | error
	Detail string `json:"detail,omitempty"`
}

// CheckOne compares one linked item's raw body bytes against the remote
// issue body. It never writes locally or remotely. Only body bytes are
// compared: whitespace, line endings, final newlines, and leading blank
// lines all constitute drift; frontmatter, title, labels, comments, and
// remote state are ignored. Reads dispatch by tracker through
// GetExternalIssue; schema, auth, and identity failures are ERROR, never
// DRIFT.
func CheckOne(ctx context.Context, it *item.Item, login string) ExternalCheckRow {
	if it.ExternalProblem != "" {
		return ExternalCheckRow{ID: it.ID, Result: "error", Detail: it.ExternalProblem}
	}
	ext := it.External
	if ext == nil {
		return ExternalCheckRow{ID: it.ID, Result: "error", Detail: "invalid external: no external link"}
	}
	iss, err := GetExternalIssue(ctx, *ext, login)
	if err != nil {
		return ExternalCheckRow{ID: it.ID, URL: ext.URL, Result: "error", Detail: err.Error()}
	}
	if iss.Number != ext.ID {
		return ExternalCheckRow{ID: it.ID, URL: ext.URL, Result: "error",
			Detail: fmt.Sprintf("issue number mismatch: expected #%d but the server returned #%d", ext.ID, iss.Number)}
	}
	if bytes.Equal(it.Body(), iss.Body) {
		return ExternalCheckRow{ID: it.ID, URL: ext.URL, Result: "match"}
	}
	return ExternalCheckRow{ID: it.ID, URL: ext.URL, Result: "drift",
		Detail: fmt.Sprintf("remote body (%d bytes) does not equal the local bytes (%d bytes)", len(iss.Body), len(it.Body()))}
}

// CheckOneLine is CheckOne rendered as the single text line awit external
// check prints for that row: "MATCH <id> <url>", "DRIFT <id> <url>[: detail]",
// "ERROR <id> <url>: detail" or "ERROR <id>: detail" when there is no URL.
func CheckOneLine(ctx context.Context, it *item.Item, login string) string {
	r := CheckOne(ctx, it, login)
	switch r.Result {
	case "match":
		return fmt.Sprintf("MATCH %s %s", r.ID, r.URL)
	case "drift":
		if r.Detail != "" {
			return fmt.Sprintf("DRIFT %s %s: %s", r.ID, r.URL, r.Detail)
		}
		return fmt.Sprintf("DRIFT %s %s", r.ID, r.URL)
	default:
		if r.URL != "" {
			return fmt.Sprintf("ERROR %s %s: %s", r.ID, r.URL, r.Detail)
		}
		return fmt.Sprintf("ERROR %s: %s", r.ID, r.Detail)
	}
}

// ExternalIssue is the CLI-owned snapshot of a remote issue. It is not a
// public provider or transport abstraction.
type ExternalIssue struct {
	Number int64
	Title  string
	Body   []byte
	Labels []string
	State  string
	URL    string
}

// ExternalBase dispatches to teax.IssueBase for Gitea and glabx.IssueBase
// for GitLab. Gitea base semantics are preserved exactly.
func ExternalBase(ext item.External) (string, error) {
	switch ext.Tracker {
	case "gitea":
		return teax.IssueBase(ext.URL)
	case "gitlab":
		return glabx.IssueBase(ext)
	default:
		return "", fmt.Errorf("invalid external: tracker must be gitea or gitlab")
	}
}

// GetExternalIssue opens the matching concrete client, fetches once, and
// converts the same-shaped Issue into ExternalIssue. Conversion copies
// slice headers, not body buffers. --tea-login is Gitea-only and is never
// passed to glab. There is no transport abstraction or persistent client
// cache.
func GetExternalIssue(ctx context.Context, ext item.External, teaLogin string) (ExternalIssue, error) {
	switch ext.Tracker {
	case "gitea":
		client, err := teax.Open(ctx, ext, teaLogin)
		if err != nil {
			return ExternalIssue{}, err
		}
		iss, err := client.GetIssue(ctx, ext.ID)
		if err != nil {
			return ExternalIssue{}, err
		}
		return ExternalIssue{Number: iss.Number, Title: iss.Title, Body: iss.Body, Labels: iss.Labels, State: iss.State, URL: iss.URL}, nil
	case "gitlab":
		client, err := glabx.Open(ctx, ext)
		if err != nil {
			return ExternalIssue{}, err
		}
		iss, err := client.GetIssue(ctx, ext.ID)
		if err != nil {
			return ExternalIssue{}, err
		}
		return ExternalIssue{Number: iss.Number, Title: iss.Title, Body: iss.Body, Labels: iss.Labels, State: iss.State, URL: iss.URL}, nil
	default:
		return ExternalIssue{}, fmt.Errorf("unsupported tracker %q", ext.Tracker)
	}
}
