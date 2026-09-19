---
id: AWIT-0NHDBNDS
title: 'external: check body drift and push byte-exact bodies through tea'
brief: >-
  Add explicit external body checks and local-to-remote body push for linked Gitea issues. Compare raw body bytes and account for tea's file-field newline behavior instead of passing Markdown through --body.
status: closed
deps: [AWIT-0ND56S3G, AWIT-0ND5753G, AWIT-0NHDBCDN, AWIT-0NHDBJDR]
labels: [phase5, p1]
refs_base: repo
refs: [.awit/comments/AWIT-0NHDBNDS/20260919T164324Z-orchestrator.md]
---
## Summary

Add `awit external check [key] [--tea-login name]` and `awit external push-body <key> [--tea-login name]`. Keep ordinary `awit validate` offline. Check is read-only; push-body is an explicit local-canonical repair action.

## Context (read first)

- Draft 2’s `internal/teax`, safe login selection, HTTP-status handling, and key resolution.
- `Item.Body` is every byte after the frontmatter closing fence, not `show --full`, comments, refs, or rendered Markdown.
- Tea’s supported `-F @file` reader removes one terminal LF; read the source pinned in the investigation. The compatibility behavior must be exercised with a real tea process.

## Files

- Add `internal/cli/external.go`, `internal/cli/external_test.go`.
- Extend `internal/teax/teax.go`, `internal/teax/teax_test.go`.
- Register command in `internal/cli/app.go`.
- Update README external workflow, `docs/schema.md` body/external contract, guide command/API sections, spec and embedded skill troubleshooting.

## Interfaces

```go
func (c *Client) SetBody(ctx context.Context, number int64, body []byte) error

type ExternalCheckRow struct {
    ID     string `json:"id"`
    URL    string `json:"url,omitempty"`
    Result string `json:"result"` // match | drift | error
    Detail string `json:"detail,omitempty"`
}
```

Read:

```text
tea api --login <login> --repo <owner/repo> --include -X GET repos/<owner>/<repo>/issues/127
```

Write:

```text
tea api --login <login> --repo <owner/repo> --include -X PATCH -F body=@<absolute-transport-file> repos/<owner>/<repo>/issues/127
```

- The transport file contains exactly `Item.Body()` followed by **one extra LF**. Supported tea strips that single transport LF, leaving the original body intact, including empty bodies, zero/one/multiple terminal newlines, and CRLF. Do not use shell substitution, `strings.TrimSpace`, `tea issues edit --body`, or JSON pretty-printer output as the body.
- Write the private temporary transport payload via temp-then-rename, with mode 0600; close before tea opens it on Windows; delete it on every exit. Do not store it as a tracked ref.
- Require successful HTTP status and a valid issue response with matching issue number/URL. Verify the returned body with `bytes.Equal`; if the response omits body, GET that same issue and compare. A mismatch is an error, never success. Document the tested tea version/build with its one-LF behavior; a changed behavior that fails compatibility verification must not be advertised as supported.
- Production does not make HTTP requests directly. A local HTTP server is permitted in transport tests to observe the actual tea-generated request.
- `check` processes linked active items in canonical-ID order and reports every match/drift/error. Invalid external metadata becomes an error row; an explicitly selected unlinked item errors. The all-items form ignores genuinely unlinked items and reports the number checked, including zero.
- Compare body bytes only. Different whitespace, line endings, final newlines, or leading blank lines constitute drift. Frontmatter, title, labels, comments, and remote state are not compared by this command.
- Plain output uses one `MATCH`, `DRIFT`, or `ERROR` line naming each item and URL, plus deterministic totals. JSON is an array of the row objects; do not mix human lines into stdout. Exit 1 if any row is drift/error, otherwise 0. An authentication/read failure is not mislabeled body drift.
- `push-body` takes a local snapshot under the store lock and serializes against local state-changing/push operations until its bounded request completes. It never changes local content, remote state/title/labels, or Git history. It refuses ambiguous duplicate external links.

## Steps

- [ ] Write a failing `TestTeaBodyRoundTrip` using the real supported tea binary against an isolated local server with isolated tea configuration. Cases: empty body, no terminal LF, multiple LFs, CRLF, leading blank lines, Unicode, backticks, quotes, comma, `@`, and literal `null`. Assert the request-decoded body and read-back body equal the input, not just that argv contains `-F`.
- [ ] Run `go test ./internal/teax -run TestTeaBodyRoundTrip -count=1 -v` with the integration prerequisite enabled; demonstrate the failure when the unadapted body file is used. Keep the integration test opt-in but mandatory for release acceptance of this feature.
- [ ] Add failing command tests for equal/drift/error aggregation, deterministic ordering, JSON purity, invalid mapping, and read-only checks; run `go test ./internal/cli -run 'ExternalCheck|ExternalPushBody' -count=1`.
- [ ] Implement `SetBody`, the one-LF transport adaptation, response verification, command outputs, and explicit body-only behavior.
- [ ] Rerun the scoped tests and live disposable-issue scenario; update supported-tea and exact-byte documentation, then hand evidence to the orchestrator.

## Acceptance Criteria

- `go test ./internal/teax ./internal/cli -run 'TeaBodyRoundTrip|ExternalCheck|ExternalPushBody' -count=1 -v` passes; the real-tea transport test is not skipped in the recorded acceptance run.
- `awit external check` exits 1 and names every drifted item; errors remain distinct from drift. No local item bytes or remote fields change.
- `awit external push-body <id> --tea-login sandbox`, followed by `awit external check <id> --tea-login sandbox`, yields MATCH for all byte-edge cases on a disposable Gitea issue.
- `awit validate` works with tea absent and networking disabled.
- Demonstrate that a supported tea returning HTTP 403 with process exit zero is rejected and that remote body mismatch is not reported as success.

## Out of scope

Automatic body pushing on local edits, fetching remote bodies into existing items, title/label synchronization, general diff rendering, background polling.

