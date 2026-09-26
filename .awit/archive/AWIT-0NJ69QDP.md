---
id: AWIT-0NJ69QDP
title: "glabx: wrap GitLab issues with verified byte-exact writes and quick-action refusal"
brief: >-
  Add a concrete pre-authenticated glab subprocess wrapper for issue reads, description writes, and state events. Refuse quick-action-shaped bodies before mutation and pin raw file transport with a real-glab compatibility canary.
status: closed
deps: [AWIT-0NJ69JDP]
labels: [phase5, p0]
refs_base: repo
refs: []
---

## Summary

Add `internal/glabx`, mirroring teax’s concrete process boundary while keeping GitLab-specific authentication, endpoints, JSON, status framing, and file transport independent. Own the quick-action safety restriction, subgroup/prefix resolution, exact delivery verification, and portable subprocess fixture consumed by later CLI work.

## Context (read first)

- `glab-assessment.md` §§D–J, especially the live glab 1.118.0 evidence in §J.
- `.awit/archive/AWIT-0NHDBJDR.md`, `AWIT-0NHDBNDS.md`, `AWIT-0NHDBQDT.md`; approved GL1 contract.
- `internal/teax/teax.go`; `internal/teax/teax_test.go:TestTeaBodyRoundTrip`; `internal/teax/teaxtest/teaxtest.go`; `internal/teax/testdata/teastub/main.go`.
- `plan/implementation-guide.md` §§1, 4.12, 5; glab configuration and config-get references linked in the assessment and handoff.

## Files

- Add `internal/glabx/glabx.go`, `internal/glabx/glabx_test.go`, `internal/glabx/setstate_test.go`.
- Add `internal/glabx/glabxtest/glabxtest.go` and `internal/glabx/testdata/glabstub/main.go` using the existing portable-helper pattern, not shell scripts.
- Preserve `internal/teax` behavior and API without a common transport extraction.
- Docs: add the exact glab contract under a new §4.13 in `plan/implementation-guide.md`, preserving §4.12; update §3 layout. Add GitLab transport/auth qualification notes to `docs/schema.md` external contract and `README.md` Install. Update `plan/awit-implementation-plan.md` external integration scope. Add pre-authentication and safe-body guidance to the embedded skill’s Setup/Common mistakes, clearly distinguishing the wrapper from CLI availability until GL3/GL4.
- Regenerate `.omp/skills/driving-awit/SKILL.md` from `internal/skill/assets/driving-awit.body.md` through `skill.Render`; do not hand-edit the generated copy.

## Interfaces

Add these exact package contracts:

```go
package glabx

type Issue struct {
    Number int64  // decoded iid; never global id
    Title  string
    Body   []byte // raw decoded description; null maps to empty
    Labels []string
    State  string // normalized open or closed
    URL    string // validated web_url
}

type Client struct {
    Host    string
    Repo    string
    BaseURL string
}

func ParseIssueURL(ctx context.Context, raw string) (item.External, error)
func IssueBase(issue item.External) (string, error)
func Open(ctx context.Context, issue item.External) (*Client, error)
func (c *Client) GetIssue(ctx context.Context, number int64) (Issue, error)
func (c *Client) SetBody(ctx context.Context, number int64, body []byte) error
func (c *Client) SetState(ctx context.Context, number int64, state string) error

// Private implementation boundaries, also named for focused tests:
func transportFile(body []byte) (path string, cleanup func(), err error)
func validateBody(body []byte) error
```

Portable test package:

```go
package glabxtest

func Install(t *testing.T) (scriptDir string)
func HideGlab(t *testing.T)
```

**URL/base/configuration:**

- `IssueBase` validates the complete GitLab mapping and removes the exact repo plus issue/work_items suffix. Normalize scheme and host case, retain port and installation-path case, and do not equate different schemes, ports, or prefixes.
- `ParseIssueURL` accepts only the two issue-link shapes. Resolve the host’s effective installation `subfolder` through read-only glab configuration lookup, accounting for the pinned release’s `GITLAB_SUBFOLDER` precedence. Use `glab config get subfolder --host <URL host>`; unset means root. Strip only the verified prefix on a segment boundary, then preserve all remaining namespace/project segments. Never guess that the first N segments form an installation prefix.
- For a stored mapping, the exact suffix supplies the expected installation base. `Open` must compare effective `subfolder`, API host, and API protocol against that base before a mutating API call can occur. Read only named non-secret settings; never parse credential files or request token values. Contradictory overrides fail with configuration guidance instead of silently targeting another instance. Split API-host deployments are not implicitly declared equivalent to a URL host.
- `--hostname` is always the link host, including port where present. Child `GITLAB_HOST` is pinned to that host; unrelated checkout remotes/default hosts never select the issue. Pass an explicit encoded endpoint, never `:id`/`:fullpath` placeholders. No configuration writes or login commands.

**Process/authentication:**

- `exec.LookPath("glab")`; missing executable gives an install-glab message. Use `exec.CommandContext`, explicit argv, nil stdin, separate stdout/stderr, and a 30-second per-process deadline. Respect earlier parent cancellation.
- `Open` performs `GET user`; require a successful status and valid user JSON with a positive numeric `id`, not an arbitrary success-looking error object. Auth failure says to check `glab auth status --hostname <host>` and the existing pre-authenticated setup. Never log in, select/create logins, access the keyring directly, or display tokens.
- Disable child HTTP debug output and prompting; suppress raw headers, body content, and untrusted credential-bearing diagnostics in errors. Keep operation/host/status/exit information actionable without forwarding arbitrary subprocess output.
- Ordinary HTTP errors produce nonzero process exit. Check that first; do not copy tea’s exit-zero recovery. Use `--include` for explicit 2xx verification, parsing its **stdout** status/header block and retaining only the JSON payload. Missing/malformed framing and non-2xx responses fail. Do not reuse tea’s stderr `splitInclude`.

**Exact API operations:**

```text
glab api --hostname <host> --include --method GET user
glab api --hostname <host> --include --method GET projects/group%2Fsub%2Fproject/issues/127
glab api --hostname <host> --include --method PUT -F description=@<absolute-private-file> projects/group%2Fsub%2Fproject/issues/127
glab api --hostname <host> --include --method PUT -f state_event=close projects/group%2Fsub%2Fproject/issues/127
glab api --hostname <host> --include --method PUT -f state_event=reopen projects/group%2Fsub%2Fproject/issues/127
```

Build the project parameter with `url.PathEscape(c.Repo)` **once for the full project path**; do not split it into API path segments or double-escape it. Installation subfolder belongs to glab’s verified API base, not the project parameter.

**Response schema and identity:**

- GET requires positive matching `iid`, string `title`, present string-or-null `description`, array-of-string `labels`, string `state`, and a valid `web_url`. Ignore global `id` entirely. Keep exact label spelling/order here; snapshot deduplication stays in import.
- Accept wire state `opened`→`open`, `closed`→`closed`; reject other GitLab states. Missing/wrongly typed fields and error JSON are failures, even after exit 0/status 200.
- Verify `web_url` using the expected complete repo, iid, and installation base. Either permitted web URL spelling may be returned. A matching iid and host alone are insufficient because iid repeats across projects.
- PUT responses must not contradict identity or the requested result. A valid partial response missing identity or the changed field triggers GET of the exact same issue for full verification; a present wrong identity, malformed JSON, wrong field type, or present mismatch is an error, not something to hide with a fallback. Empty 2xx response may use the same GET fallback.

**Body safety/transport:**

- Run `validateBody` before creating a transport file or executing PUT. Reject invalid UTF-8 instead of allowing JSON replacement characters to change bytes.
- Fail closed on any line beginning at column zero with slash followed by a lowercase ASCII letter: conservative prefix rule `(?m)^/[a-z]+`. Include commands with arguments, bare `/close` at EOF, LF and CRLF lines, and unknown commands. Do not maintain a GitLab command allowlist.
- Deliberately do **not** implement Markdown parsing or exemptions for fences: column-zero matches inside a fence are also refused. The diagnostic must point to fenced code blocks and explain that the slash line must not remain at column zero. Qualify the documented indented-in-fence example against the real server before recommending it. Never automatically fence, escape, indent, strip, or otherwise change a body.
- Safe payload files contain **exactly `body`**, with no added or stripped LF. Write directly without allocating a body-plus-newline copy. Private temporary directory, mode 0600 file, temp-then-rename, close before subprocess use, cleanup on every return path.
- Successful body delivery requires 2xx, verified identity, and `bytes.Equal` on the returned/read-back description. Never include title/labels/state fields in the description request. Mismatch is a failure, not success or automatic rollback.

**State:** accept common `open|closed` only; map `open` to `state_event=reopen`, `closed` to `state_event=close`. Verify normalized returned/read-back state and full identity. Do not read remote state to decide which state to write, and do not automatically retry.

## Steps

- [ ] **RED - create portable `glabxtest.Install`/glabstub and tests `TestGlabOpen`, `TestGlabIssueURL`, `TestGlabGetIssue`.** Script config settings, stdout status framing, user/issue JSON, and nonzero exits. Exercise nested prefix+subgroup, both link shapes, conflicting host/subfolder/protocol overrides, missing glab, invalid auth response, cancellation, 401/403/404/422/500, `id != iid`, wrong project/base, malformed/missing fields, null description, labels containing commas/Unicode, and normalized states. The stub must exit nonzero for HTTP failure by default.
- [ ] **Run RED:** `go test ./internal/glabx -run 'GlabOpen|GlabIssueURL|GlabGetIssue' -count=1 -v`; record the missing/unimplemented wrapper failures. Then implement only validated URL/config resolution, subprocess execution, auth, decoding, and GET; rerun to green.
- [ ] **RED - add `TestGlabSetBody`, `TestGlabQuickActionRefusal`, and `TestGlabSetState`.** Assert observed request payload/remote state, not only argv. Cover exact byte edges, UTF-8 refusal, private-file cleanup after failure, fallback GET, wrong identity/body/state, body-only/state-only requests, same-state events, and zero PUTs for unsafe bodies. A representative body-safety assertion is:

  ```go
  if err := validateBody([]byte("intro\r\n/close\r\n")); err == nil {
      t.Fatal("column-zero quick action must be refused")
  }
  ```

  The subprocess test must additionally prove the scripted remote issue is unchanged and no PUT occurred.

- [ ] **Run RED:** `go test ./internal/glabx -run 'GlabSetBody|GlabQuickActionRefusal|GlabSetState' -count=1 -v`; record failures before implementing mutations.
- [ ] **GREEN - implement guarded raw-file body PUT and verified state events.** Preserve the input bytes exactly. Keep quick-action refusal independent of server response behavior. Rerun the preceding focused tests.
- [ ] **Add `TestGlabBodyRoundTrip`, opt-in via `AWIT_TEST_REAL_GLAB=1`, using real glab against an isolated loopback HTTP server and isolated glab configuration.** Mirror `TestTeaBodyRoundTrip`, but require raw transport bytes. Include empty, leading blanks, no/one/multiple final LF, lone terminal CR, CRLF, trailing spaces, Unicode, quotes/backticks, comma, `@`, `null`, digits, JSON-looking text, and `:namespace`-looking text. Compare decoded request description and read-back bytes. Assert title/labels/state remain unchanged.
- [ ] **Prove the no-adaptation canary RED/GREEN.** Within `TestGlabBodyRoundTrip`, directly run glab on a raw file to establish preservation; also send a deliberately LF-appended negative-control file and prove it changes the remote bytes. Temporarily apply tea’s extra-LF adaptation to `SetBody`, run the focused real test and record its failure, then remove that mutation and record green. Keep the negative control and raw-file canary, not the incorrect implementation.
- [ ] **Qualify routing/status/safety with real glab.** Exercise root and nested-subfolder configurations, unrelated checkout/default-host settings, an encoded subgroup project, HTTP errors and include framing. A real-glab quick-action-refusal subtest must observe zero mutation requests and unchanged remote state; do not send `/close` directly to a live issue to re-prove §J. Use a separately authorized disposable issue only to qualify server round-trips and the documented safe fenced formatting.
- [ ] **Finish paired docs and rendered skill.** Record glab/server versions and exact non-skipped commands. Hand evidence to the orchestrator; do not claim unsupported versions or unexercised routing configurations.

## Acceptance Criteria

```sh
go test ./internal/glabx -run 'Glab' -count=1 -v
AWIT_TEST_REAL_GLAB=1 go test ./internal/glabx -run '^TestGlabBodyRoundTrip$' -count=1 -v
go test ./internal/skill -run 'Render|DogfoodOmpCopyMatchesRenderer' -count=1
```

- First command passes portable stub tests without glab/network prerequisites. Second command runs-not skips-with glab 1.118.0, reports the tested version, and proves decoded bytes, no-adaptation negative control, explicit status handling, prefix routing, and zero PUTs on quick-action refusal. If opt-in is explicitly enabled but glab is absent, fail rather than silently treating release qualification as complete.
- Temporary directories hold all stub/config/transport fixtures; tests never modify the user’s real glab configuration or keyring. No production HTTP client is introduced.
- On a separately authorized disposable GitLab issue, safe payloads round-trip exactly and leave title/labels/state unchanged; quick-action input is refused without changing any remote field. Record the server version, glab version, and body-case results. This is distinct from the loopback proof.
- Diagnostics guide missing glab/auth/config repair without login management or secrets. Every API request is host-scoped and uses a single encoded project parameter.
- This work item exposes a package API, not a new awit command. Actual temporary-repository CLI acceptance belongs to GL3/GL4; no wrapper-only command is added for testing.

## Out of scope

Shared transport/provider framework, HTTP client, token/keyring access, login creation/selection, MRs or arbitrary work-item resources, body rewriting, retries, daemon, remote creation, or changing tea’s LF/status/auth behavior.

## Comments

### 2026-09-19T18:52:57Z agent/orchestrator

implemented
