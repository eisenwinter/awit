---
id: AWIT-0NJ69JDP
title: 'item: accept additive GitLab issue metadata and subgroup URLs'
brief: >-
  Extend structured external metadata to GitLab issues while preserving valid Gitea files and behavior. Accept subgroup projects and both GitLab issue URL shapes without migrating existing items or weakening optional-metadata handling.
status: closed
deps: []
labels: [phase5, p0]
refs_base: repo
refs: [.awit/comments/AWIT-0NJ69JDP/20260919T182922Z-orchestrator.md]
---
## Summary

Extend `external.tracker` from `gitea` to `gitea|gitlab`, keeping the four-field mapping and all existing identity, YAML-preservation, warning, and quarantine contracts. This is additive-only: no existing item rewrite, migration, legacy-scalar conversion, or changed Gitea grammar.

## Context (read first)

- `glab-assessment.md` §§D.1, D.4, J.
- `.awit/archive/AWIT-0NHDBCDN.md`.
- `AGENTS.md`; `plan/implementation-guide.md` §§1–5, especially §4.3.
- `pkg/item/item.go`: `validRepoSegment`, `parseRepo`, `validateExternalURL`, `ValidateExternal`, `parseExternalID`, `parseExternalNode`, `SetExternal`.
- `internal/cli/create.go:parseExternalMapping`; metadata flags in `create.go` and `update.go`.
- `docs/schema.md`: Optional key `external`; `plan/awit-implementation-plan.md`: Data model.

## Files

- Modify `pkg/item/item.go`, `pkg/item/item_test.go`.
- Modify external metadata help in `internal/cli/create.go`, `internal/cli/update.go`; extend their existing external-metadata command tests.
- Exercise existing formatter/show/validate paths; change production formatting only if a demonstrated Gitea-only assumption prevents existing data-driven output.
- Paired docs: `docs/schema.md` Optional known keys and Optional key `external`; `plan/implementation-guide.md` §2 external mapping and §4.3; `plan/awit-implementation-plan.md` Data model and create command row; `README.md` create/update metadata descriptions.
- Embedded skill behavior does not change in this schema-only work item; import/push guidance is owned by GL3/GL4. Archived work items remain unchanged.

## Interfaces

Preserve these exact signatures and fields:

```go
type External struct {
    Tracker string `json:"tracker"`
    Repo    string `json:"repo"`
    ID      int64  `json:"id"`
    URL     string `json:"url"`
}

func ValidateExternal(e External) error
func (it *Item) SetExternal(e *External) error
func (it *Item) SetBody(body []byte)
```

Validation rules:

1. `tracker` is exactly lowercase `gitea` or `gitlab`. Required-field types, duplicate-owned-key rejection, positive base-10 int64 parsing, and preservation of extra nested keys remain as implemented.
2. Gitea uses the existing two-segment `parseRepo` and existing URL-validation behavior, unchanged. Do not broaden or tighten Gitea acceptance incidentally.
3. GitLab repo has **at least two** slash-separated segments. Reject leading/trailing slash, empty segments, `.` and `..`, Unicode whitespace/control characters, and any segment containing backslash or one of `/:?#@[]%`. Ordinary dots inside a name are allowed. Preserve case and spelling; do not clean or lowercase project paths.
4. GitLab URL must be absolute HTTP(S), have a host, and have no userinfo, query—including an empty `?`—or fragment—including an empty `#`. Its decoded path must end exactly in `/<complete repo>/-/issues/<canonical decimal ID>` or `/<complete repo>/-/work_items/<canonical decimal ID>`.
5. A prefix before that exact suffix is the installation path. Validate the entire path: no empty internal segments, dot segments, backslashes, whitespace/control characters, trailing slash, or encoded separator `%2f`/`%5c` in any case. Retain the existing conservative rejection of encoded dots `%2e`; reject nested encoded delimiter tricks rather than repeatedly decoding them. Match the full repo and ID, not a fixed four-segment suffix. Prefix segments follow the same safe-segment rules.
6. `external.id` means Gitea `number` or GitLab **`iid`**, never either product’s database-wide `id`. There is no new `project_id`, resource-kind field, or stored remote state.
7. `work_items` is accepted as an issue-link spelling, not a promise to import every GitLab work-item type. GL2 must prove the link resolves through the Issues API.
8. Missing external metadata remains valid. Invalid mappings and legacy scalars still preserve source YAML, set `ExternalProblem`, leave `External == nil`, warn on validate, and do not quarantine an otherwise valid local item. Remote operations refuse invalid metadata.
9. No-op parse/serialize and identical `SetExternal` preserve original bytes. Status/body mutations preserve unknown keys and do not normalize stored URLs.

State terminology in the paired docs: local statuses remain `open|in_progress|closed`; GitLab wire `opened` maps to common/local `open`, wire `closed` to `closed`. State conversion belongs to GL2, not YAML parsing.

## Steps

- [ ] **RED — add `TestExternalGitLabValidation` and `TestExternalGitLabRoundTrip`.** Cover subgroup and two-segment repos, both URL shapes, nested installation prefix, wrong repo/iid suffix, empty/dot segments, encoded separators/dots, credentials/query/fragment, unsupported tracker, nonpositive/overflow ID, malformed YAML fields, and old scalar preservation. Include existing valid Gitea fixtures as unchanged compatibility cases.

  ```go
  e := External{
      Tracker: "gitlab", Repo: "group/sub/project", ID: 127,
      URL: "https://forge.example/apps/gitlab/group/sub/project/-/work_items/127",
  }
  if err := ValidateExternal(e); err != nil {
      t.Fatalf("valid GitLab issue mapping: %v", err)
  }
  ```

- [ ] **Run RED:** `go test ./pkg/item -run 'ExternalGitLab|External.*RoundTrip' -count=1 -v`. Record the failure caused by the current Gitea-only validator; fixture/setup failures are not the RED proof.
- [ ] **RED — extend command coverage with `TestExternalGitLabMetadataCLI`.** In `initRepo(t)`, call existing `run`/`Main` for create, show/list JSON, update with an identical mapping, and clear-external. Invalid/partial mappings must exit 2 without changing any item. With both tracker executables unavailable, all metadata-only operations and validate must still work.
- [ ] **Run RED:** `go test ./internal/cli -run 'ExternalGitLabMetadataCLI' -count=1 -v`; record GitLab mapping rejection.
- [ ] **GREEN — implement only the tracker-dependent validation branch and truthful metadata help.** Reuse current YAML parsing/setters, formatter fields, and warning path. Keep Gitea validation on its existing branch. Update guide §4.3 and schema rules in the same change.
- [ ] **Run GREEN:** `go test ./pkg/item ./pkg/format ./internal/cli -run 'External|RoundTrip|UnknownKey' -count=1`. Perform the temporary-repository acceptance sequence, finish paired docs, and hand the scoped evidence to the orchestrator for its commit.

## Acceptance Criteria

- The scoped GREEN command passes, including unchanged Gitea and raw-source preservation cases.
- Using a built candidate `awit` and a fresh temporary directory `$R`, run:

  ```sh
  awit --repo "$R" init --prefix AWIT --no-skills
  awit --repo "$R" create Linked --brief 'A linked GitLab issue.' --alias GL-SCHEMA --external-tracker gitlab --external-repo group/sub/project --external-id 127 --external-url https://forge.example/apps/gitlab/group/sub/project/-/work_items/127
  awit --repo "$R" show GL-SCHEMA
  awit --repo "$R" --format json list GL-SCHEMA
  awit --repo "$R" validate
  awit --repo "$R" update GL-SCHEMA --clear-external
  ```

  Expect a normal minted AWIT ID, the exact GitLab mapping, no invalid-external warning, successful validation absent other faults, and removal of only external metadata on clear. No tea/glab process or commit occurs.
- Replacing the repo with `group//project`, adding a URL query, or supplying a mismatched iid exits 2 with no item mutation. Legacy `external: gitlab#42` still warns rather than quarantines and round-trips unchanged.
- Guide/schema describe additive compatibility and both URL shapes; they do not claim import or push availability before their owning work items land.

## Out of scope

Subprocess integration, import, remote writes, MRs, non-issue work items, migration, automatic URL normalization, new schema fields, or changes to Gitea validation.


