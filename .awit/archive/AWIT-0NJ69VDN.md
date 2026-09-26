---
id: AWIT-0NJ69VDN
title: "import: snapshot GitLab issues and preserve tracker-aware identity"
brief: >-
  Extend issue import to GitLab issue and work_items URLs through glab, retaining iid, exact description, labels, and initial state. Make duplicate identity tracker-aware while preserving aliases, subgroup lookup, atomic saving, and Gitea import behavior.
status: closed
deps: [AWIT-0NJ69JDP, AWIT-0NJ69QDP]
labels: [phase5, p0]
refs_base: repo
refs: []
---

## Summary

Reuse the existing import workflow for GitLab with tracker dispatch and correct installation/project/iid identity. Extend active/archive duplicate detection and shared duplicate-link checks without introducing a new lookup syntax or an inbound synchronization path.

## Context (read first)

- Approved GL1/GL2 interfaces; `glab-assessment.md` §§D.1, D.4, J.
- `.awit/archive/AWIT-0NHDBJDR.md` and `AWIT-0NHDBNDS.md`.
- `internal/cli/import.go`: `parseIssueURL`, `importAction`, `validateImportCandidate`, `refuseDuplicateImport`, `sameImportIdentity`.
- `internal/cli/external.go:duplicateExternalLinks`; `internal/cli/app.go:resolveItemID`, `parseExternalKey`.
- Guide §§4.3, 4.11–4.13, 5; schema lookup/import contracts.

## Files

- Modify `internal/cli/import.go`, `internal/cli/import_test.go`.
- Add the small private read-dispatch/base helpers to existing `internal/cli/external.go`; extend duplicate identity tests in `internal/cli/external_test.go`.
- Update subgroup examples/comments in `internal/cli/app.go`; extend existing lookup tests. No parser rewrite.
- Docs: `README.md` import row plus GitLab example; `docs/schema.md` Item lookup keys and external import paragraphs; guide §4.11 helper/lookup contracts; spec CLI command matrix and lookup paragraph; embedded skill Quick reference import/lookup rows and rendered `.omp` skill.

## Interfaces

Preserve `resolveItemID` and the canonical-only Store API. Change the private import parser’s signature and migrate all its callsites/tests in this work item:

```go
func parseIssueURL(ctx context.Context, raw string) (item.External, error)
func refuseDuplicateImport(s *item.Store, ext item.External) error
func sameImportIdentity(base string, want item.External, have *item.External) bool
func duplicateExternalLinks(items []*item.Item, want item.External) []string
func resolveItemID(items []*item.Item, key string) (string, error)

// CLI-owned snapshot, not a public provider/transport abstraction:
type externalIssue struct {
    Number int64
    Title  string
    Body   []byte
    Labels []string
    State  string
    URL    string
}

func externalBase(ext item.External) (string, error)
func getExternalIssue(ctx context.Context, ext item.External, teaLogin string) (externalIssue, error)
```

- `externalBase` dispatches to existing `teax.IssueBase(ext.URL)` for Gitea and `glabx.IssueBase(ext)` for GitLab. Preserve existing Gitea base semantics exactly.
- `getExternalIssue` opens the matching concrete client, fetches once, and converts its same-shaped Issue into `externalIssue`. Conversion copies slice headers, not body buffers. No transport abstraction or persistent client cache.
- `--tea-login` remains Gitea-only. On a GitLab target it is ignored, never interpreted as a GitLab login or passed to glab; document this so mixed commands can carry the flag for their Gitea rows.
- URL recognition checks the explicit GitLab `/-/issues/` or `/-/work_items/` shape before the legacy Gitea branch and delegates GitLab prefix resolution to `glabx.ParseIssueURL`. Do not detect a product from `gitlab.com` alone. Reject MR and other `/-/` resource shapes before they can fall through to Gitea parsing.
- Pure malformed input remains usage exit 2; missing glab, config/auth/read failures, duplicate identity, and invalid fetched content are operational exit 1.

Import flow:

1. Check required brief/arity/alias and URL grammar. For GitLab resolve the verified subfolder, then open/fetch before locking.
2. Gitea behavior stays unchanged. GitLab snapshot uses returned `iid` as Number; normalized `open|closed` selects local status. No remote assignee/claim is inferred. The stored URL is the validated **input URL**, matching the current import behavior; response `web_url` proves identity and may use the other accepted spelling.
3. Preserve decoded description exactly, including empty/null and all supported byte edges. Labels use current first-seen exact-name deduplication, with no splitting/trimming/case normalization or default-label merge. Reject malformed GitLab label entries in glabx; preserve existing Gitea treatment of empty names.
4. Under `Store.Lock`, rescan active and archived items; duplicate identity is `(tracker, normalized installation base, exact repo, iid)`. `issues` and `work_items` links to the same GitLab issue collide. Gitea/GitLab with the same host/repo/number do not collide; different hosts/prefixes remain distinct.
5. Continue refusing files whose identity cannot be inspected, with their path named. Do not claim uniqueness when an invalid external mapping might conceal the same link; keep missing external metadata distinct from invalid metadata during the duplicate scan. Preserve archive exclusion from the graph.
6. Mint only after uniqueness checks, call existing setters and `validateImportCandidate`, then `Store.Save`. Conflict-marker bodies must fail before save rather than create a quarantined item or be rewritten. Import makes no remote mutation and no commit.
7. `sameImportIdentity` must compare `Tracker` in addition to repo/iid/base. Reuse it from `duplicateExternalLinks` so body/state commands cannot retain the old cross-tracker collision bug.
8. Alias and subgroup-qualified lookup use `resolveItemID` unchanged: canonical ID first, alias case-insensitive, then repo-qualified/bare issue key. Ambiguity across trackers/hosts names sorted canonical IDs; aliases and canonical IDs disambiguate. Dependency edges remain canonical AWIT IDs.

## Steps

- [ ] **RED - add `TestImportGitLabFaithful`, `TestImportGitLabURLs`, `TestImportGitLabDuplicateIdentity`, and `TestExternalLookupGitLab`.** Use `glabxtest.Install` and existing `initRepo`/`run` helpers. A fetched fixture must deliberately separate iid from global id:

  ```json
  {
    "id": 987654,
    "iid": 127,
    "title": "Imported issue",
    "description": "Intro\r\nlast  ",
    "labels": ["area::api", "comma,label", "area::api"],
    "state": "opened",
    "web_url": "https://forge.example/group/sub/project/-/work_items/127"
  }
  ```

  Assert stored ID 127, exact description, exact first-seen labels, no defaults, input URL preservation, and local open. Add closed/null cases, both URL forms, installation prefixes, issue-only rejection, invalid auth/schema/identity, candidate conflict markers, missing brief on repeated Main calls, and no remote PUT.

- [ ] **Run RED:** `go test ./internal/cli -run 'ImportGitLab|ExternalLookupGitLab' -count=1 -v`; record the current GitLab URL/import rejection before dispatch changes.
- [ ] **RED - prove identity boundaries.** Exercise active and archived same-link refusal-including alternate URL spelling-and successful imports for same repo/iid on another tracker/host/prefix. For `duplicateExternalLinks`, assert only exact tracker/base/repo/iid matches are returned. Test uninspectable archives and ambiguous lookup without local writes.
- [ ] **Run RED:** `go test ./internal/cli -run 'ImportGitLabDuplicateIdentity|ExternalDuplicateTrackerIdentity' -count=1 -v`.
- [ ] **GREEN - add the small read/base dispatch helpers and migrate import/identity callers.** Keep network-before-lock, uniqueness-under-lock, candidate validation, and atomic save. Preserve all Gitea branches and the existing subgroup key parser. Update helper signatures in guide §4.11.
- [ ] **Run GREEN:** `go test ./internal/cli ./pkg/item -run 'Import|Alias|ExternalLookup|ExternalDuplicate' -count=1 -v`. Perform the temporary-directory CLI sequence, update paired docs and rendered skill, and hand evidence to the orchestrator for commit.

## Acceptance Criteria

- The focused command passes with portable stubs on Linux and Windows; existing Gitea imports remain unchanged.
- In a fresh temporary initialized repository `$R`, with `$GL_ISSUE_URL` naming an authorized existing GitLab issue and a pre-authenticated glab host:

  ```sh
  awit --repo "$R" init --prefix AWIT --no-skills
  awit --repo "$R" import "$GL_ISSUE_URL" --brief 'Imported GitLab issue.' --alias GL-IMPORT
  awit --repo "$R" show GL-IMPORT
  awit --repo "$R" --format json list GL-IMPORT
  awit --repo "$R" show 'group/sub/project#127'
  awit --repo "$R" import "$GL_ISSUE_URL" --brief 'Duplicate must refuse.'
  awit --repo "$R" validate
  ```

  Use the actual repo/iid in the qualified lookup. Expect a minted canonical ID, exact snapshot fields, alias/qualified lookup selecting that item, duplicate exit 1 naming it, and successful local validation. Compare `Item.Body()` to decoded API `description`, not `show --full` text.

- Subprocess-backed temporary-repo tests cover archive duplicates, equal numbers across trackers/hosts, sorted ambiguity, both URL spellings, and a prefixed subgroup installation. Same GitLab issue under either URL shape is one identity.
- Every refusal leaves item/archive bytes unchanged. No import performs PUT, creates remote issues, or commits.
- `go test ./internal/skill -run 'Render|DogfoodOmpCopyMatchesRenderer' -count=1` passes after regeneration.

## Out of scope

Re-import-as-update, body/state command routing, new alias grammar, new external-key syntax, arbitrary work-item/MR import, remote creation, default-label changes, persistent indexes, and inbound synchronization.

## Comments

### 2026-09-19T19:09:02Z agent/orchestrator

implemented
