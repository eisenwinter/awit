---
id: AWIT-0NHDBJDR
title: 'import: mint local items from Gitea issues and resolve human aliases'
brief: >-
  Import an existing Gitea issue through tea, retaining its number, URL, body, labels, and initial state. Add a separate alias field and deterministic lookup without weakening AWIT ID or dependency rules.
status: open
deps: [AWIT-0ND56E3G, AWIT-0NFAW5DT, AWIT-0ND56K3G, AWIT-0ND56J3G, AWIT-0NHDBCDN]
labels: [phase5, p0]
refs: []
---
## Summary

Add `awit import <issue-url> --brief <summary> [--alias DTRM-F21] [--tea-login name]`. This is the single import spelling; do not also add `create --from-external`.

Import is an explicit initial snapshot, not ongoing inbound synchronization. Re-importing a linked issue refuses without updating local content.

## Context (read first)

- Draft 1’s external schema and `Item.SetBody` contract.
- `createAction`, `mergeLabels`, `loadItem`, `showOne`, `nextAction`, `nextNode`, `refuseClaim`, `listAction`.
- `AWIT-0NFAW5DT`: exact `next [id]` behavior is already implemented.
- Guide §§4.3–4.6 and §5’s reusable urfave command pitfalls.
- Tea `cmd/api.go`, `modules/context/context.go`, and login-list output cited in the investigation.

## Files

- Add `internal/cli/import.go`, `internal/cli/import_test.go`.
- Add `internal/teax/teax.go`, `internal/teax/teax_test.go`: the one concrete tea subprocess wrapper.
- Modify `pkg/item/item.go`, its tests, and `pkg/format/format.go` for alias representation.
- Modify `internal/cli/{app,author,create,update,show,list,next,dep}.go` for registration and consistent identifier resolution.
- Update README Commands, schema identity/external sections, guide shared APIs/command matrix, spec, and embedded skill Quick reference.

## Interfaces

```go
// Add Item.Alias string and format.Entry.Alias string (`json:"alias,omitempty"`).
func (it *Item) SetAlias(alias string) error // empty clears

// Internal CLI helpers; keep Store.Load canonical-ID-only.
func resolveItemID(items []*item.Item, key string) (string, error)

// internal/teax, concrete wrapper rather than provider interface:
type Issue struct {
    Number int64
    Title  string
    Body   []byte
    Labels []string
    State  string
    URL    string
}
type Client struct { Login string; Repo string; BaseURL string }
func Open(ctx context.Context, issue item.External, login string) (*Client, error)
func (c *Client) GetIssue(ctx context.Context, number int64) (Issue, error)
```

Tea operations, with flags before the endpoint:

```text
tea login list --output json
tea api --login <login> --repo <owner/repo> --include -X GET user
tea api --login <login> --repo <owner/repo> --include -X GET repos/<escaped-owner>/<escaped-repo>/issues/127
```

`Open` uses `exec.LookPath("tea")`, checks the required `api` capability, selects a login whose normalized full base URL equals the base extracted from the issue URL, and verifies authentication with `GET user`. Explicit `--tea-login` must match that base. Without it, one matching login is selected; multiple matching logins require the flag. Never select a login merely because it is default or because the checkout has a remote. Missing executable says to install tea; absent/invalid credentials say to run `tea login add`. Never manage or print tokens.

Run subprocesses with `exec.CommandContext`, an explicit argument vector, no shell, disconnected stdin, separate stdout/stderr capture, and a bounded 30-second operation deadline. Validate subprocess success **and** the `--include` HTTP status; do not pass successful error JSON through as an issue. Capture diagnostics rather than forwarding raw response headers or authentication material.

Import mapping:
- Mint a normal configured-prefix AWIT ID. Ignore Gitea’s database `id`; store JSON `number` as `external.id`.
- Title is remote title. Body is the decoded JSON `body` string converted to UTF-8 bytes, without trimming or adding a skeleton. Null body maps to empty; malformed/missing required fields fail.
- Labels are exact issue label names, first-seen deduplicated. Do not merge local default labels into an imported historical snapshot; later `update -l` can change them.
- Remote `open`→local `open`; `closed`→local `closed`; other states refuse. No local claim/assignee is inferred. `--brief` remains mandatory; no heuristic sentence extraction.
- Validate the candidate complete serialized item before saving, including the existing whole-file conflict-marker rule. Refuse an issue body that would immediately quarantine the new item; explain that it must be reconciled before import, rather than silently altering its bytes.
- Fetch before locking. Under `Store.Lock`, rescan active items and archived item files for the same normalized installation base + repo + issue number, then mint and save. Duplicate import errors name the existing item/archive path and leave all bytes unchanged. If a file cannot be inspected for identity, refuse import with that path rather than claim uniqueness was established. Archive scanning is only an import-identity guard; archived items remain excluded from the graph.

Alias and lookup:
- Optional scalar `alias`, set by create/update/import using `--alias`; update offers `--clear-alias`. Accept `[A-Za-z][A-Za-z0-9._-]{0,127}`. No whitespace, slash, `#`, or AWIT-shaped alias that could collide with canonical IDs. Uniqueness is case-insensitive across active parseable items; duplicate manual aliases make alias lookup fail with sorted canonical IDs, never choose one arbitrarily. They do not alter dependency readiness. `validate` warns about invalid/duplicate optional aliases.
- Canonical IDs remain exact/case-sensitive and take precedence. Otherwise match alias case-insensitively. No prefix/fuzzy/title matching and no bare integer shorthand.
- Additionally resolve `owner/repo#127` and quoted `'#127'` from valid external metadata; bare `#127` succeeds only if unique across active items. Ambiguity is an error listing canonical IDs. Existing issue references in journals/commits/docs are not rewritten.
- `show <key>` and `next [key]` resolve through the same helper. `list [key]` optionally selects exactly one item before existing status/label/state filters; no key retains the current listing. Exact `next <key>` remains a lookup, not a rerank.
- Use the same resolution for mutation targets (`loadItem` consumers) and dependency CLI inputs; serialize dependency edges only as canonical AWIT IDs. Resolve under the mutation lock. Unknown/ambiguous keys never become filesystem paths.
- Display alias as an optional compact suffix and in JSON; show adds `alias: DTRM-F21`. Keep canonical IDs as primary output, filenames, and commit identifiers.

## Steps

- [ ] Write failing command tests for faithful import, Gitea number-versus-database-ID mapping, closed-state import, byte-exact body storage, duplicate import including archive, missing tea/auth, HTTP 401 with subprocess exit zero, and ambiguous lookup. Add alias tests for exact selection and mutation of the intended canonical file.
- [ ] Run `go test ./internal/cli ./pkg/item -run 'Import|Alias|ExternalLookup' -count=1`; observe failures before implementation.
- [ ] Implement the concrete `teax` subprocess boundary and use a portable helper executable in subprocess tests. Implement `importAction` with preflight/fetch, locked uniqueness checks, `SetBody`, and normal atomic save.
- [ ] Implement alias setters, CLI flags, `resolveItemID`, displays, and all named lookup callsites. Keep `id.Valid`, Store paths, and graph keys unchanged.
- [ ] Run `go test ./internal/teax ./internal/cli ./pkg/item -run 'Tea|Import|Alias|ExternalLookup' -count=1` to green; update docs and submit evidence for orchestrator commit.

## Acceptance Criteria

- The scoped test command above passes, including Windows-compatible subprocess tests.
- Against a disposable issue #127: `awit import https://forge.example/owner/repo/issues/127 --brief 'Imported issue.' --alias DTRM-F21 --tea-login sandbox` returns a newly minted AWIT ID, retains `external.id: 127`, and preserves the exact decoded body.
- `awit show DTRM-F21`, `awit list DTRM-F21 --format json`, and `awit next DTRM-F21` resolve the same canonical item. `awit next --claim DTRM-F21` retains current ready/claimed/closed refusal rules.
- A second identical import exits 1 naming the existing item, with no new file or update. `awit create ... --id DTRM-F21` remains invalid.
- Missing tea, no matching login, expired authentication, malformed responses, ambiguous aliases, and issue-not-found all fail before local creation with actionable messages. Import never writes remote state or commits.

## Out of scope

Remote issue creation, bulk import, pull-request import, re-import-as-update, remote content becoming authoritative after creation, alias substitution inside Markdown, persistent indexes.

