---
id: AWIT-0NHDBCDN
title: 'item: support structured Gitea external metadata without losing source bytes'
brief: >-
  Promote the reserved external key to structured Gitea metadata editable on create and update. Preserve unknown YAML keys and unchanged source bytes, warn rather than quarantine on invalid optional integration metadata, and display valid links.
status: open
deps: [AWIT-0ND5753G, AWIT-0ND56H3G, AWIT-0ND56M3G, AWIT-0ND56F3G, AWIT-0ND56S3G]
labels: [phase5, p0]
refs: []
---
## Summary

Make `external` a supported frontmatter field:

```yaml
external:
  tracker: gitea
  repo: owner/repo
  id: 127
  url: https://forge.example/owner/repo/issues/127
```

`id` means the repository issue **number**, not Gitea’s database-wide issue ID. Retain ordinary AWIT identity, filenames, dependencies, and graph behavior.

## Context (read first)

- `plan/implementation-guide.md` §§2, 4.3, 4.7, 5, 9.
- `plan/awit-implementation-plan.md`: Data model and Phase 5.
- `AWIT-0ND5753G`: supersede its living contract, not its closed historical body.
- `pkg/item/item.go`: `Parse`, `Bytes`, `New`, `findKey`, setters.
- `internal/cli/create.go:createAction`, `update.go:updateAction`, `show.go:defaultView`, `app.go:toEntry`, `validate.go:validateAction`.

## Files

- Modify `pkg/item/item.go`, `pkg/item/item_test.go`.
- Modify `pkg/format/format.go`, formatter tests/goldens covering metadata.
- Modify `internal/cli/{create,update,show,app,validate}.go` and their relevant tests.
- Update `docs/schema.md`, `README.md`, both plan documents, and relevant embedded skill wording.

## Interfaces

Add these interfaces without renaming guide APIs:

```go
type External struct {
    Tracker string `json:"tracker"`
    Repo    string `json:"repo"`
    ID      int64  `json:"id"`
    URL     string `json:"url"`
}

// Add to Item:
// External *External
// ExternalProblem string // derived diagnostic; never serialized
func ValidateExternal(e External) error
func (it *Item) SetExternal(e *External) error // nil removes external
func (it *Item) SetBody(body []byte)          // owns a copy; no normalization
```

- Create/update flags: `--external-tracker`, `--external-repo`, `--external-id`, `--external-url`. Require all four together; partial mappings are usage errors, exit 2, before a write. Parse `--external-id` from a string so absent and zero differ without `IsSet`.
- Update additionally supports `--clear-external`; mutually exclusive with mapping flags. An unchanged identical mapping is a no-op.
- `SetExternal` edits the four owned subnodes in place, retaining extra subkeys, comments, mapping style, and unknown top-level keys. Clearing explicitly removes the whole field.
- Add `External *item.External` to format entries with `json:"external,omitempty"`. Existing entries without external metadata retain their output. Compact suffix: ` | External: gitea owner/repo#127`; table adds an EXTERNAL column only when any displayed row has a link. Show adds `external: gitea owner/repo#127 <url>`; JSON carries the structured object. Do not change ranking or prime’s payload solely to expose this field.

Validation:
- Mapping only; required four keys have the specified scalar types, no duplicate owned keys, tracker exactly `gitea`, positive base-10 integer representable by int64.
- Repo has exactly two nonempty path segments; reject whitespace/control characters, `.`/`..`, slash/backslash within a segment, and URL delimiter tricks. Build API endpoint segments using URL escaping, not interpolation of unchecked data.
- URL is absolute HTTP(S), with a host, no userinfo/query/fragment. Its decoded path must end exactly in `/<owner>/<repo>/issues/<id>` matching the other fields. An installation prefix before that suffix is allowed. Reject dot segments and encoded path separators; do not rewrite input URLs on unrelated edits.
- Missing `external` is valid. An invalid optional mapping, unsupported tracker, or old reserved scalar populates `ExternalProblem` and leaves the YAML intact; `External` is nil. It **does not quarantine an otherwise usable local item**. `validate` reports `WARN  <id>: invalid external: <reason>`; in JSON mode put this advisory on stderr without changing the existing fault-array schema. Exit remains 0 unless actual graph faults exist.
- Remote commands must refuse invalid metadata; they must never guess missing fields or silently skip a requested item.

## Steps

- [ ] Write failing tests for noncanonical YAML/CRLF no-op round-trip, external mapping read/write, extra nested keys, invalid types, old scalar preservation, and invalid create/update doing no write. Delete `TestItemHasNoExternalField`; it asserts the superseded implementation prohibition. Move generic unknown-key coverage to a genuinely unknown field.
- [ ] Run `go test ./pkg/item ./pkg/format ./internal/cli -run 'External|RoundTrip|UnknownKey' -count=1`; record the relevant failures.
- [ ] Implement `External`, strict setter validation, lenient optional-field parsing, flags, presentation, and warnings. Preserve original parsed bytes until a real setter changes content; mark all existing mutation paths dirty. `Bytes()` returns unchanged source bytes when clean and node-rendered frontmatter plus exact body when dirty. `New` starts dirty. `SetBody` is the shared body boundary used by import and templates.
- [ ] Prove a status-only edit preserves unknown external subkeys and raw body; prove parse→Bytes with no mutation is byte-identical even for source formatting the YAML encoder would otherwise change.
- [ ] Run the scoped command above to green; update the living contracts and send the orchestrator the acceptance evidence for its commit.

## Acceptance Criteria

- `go test ./pkg/item ./pkg/format ./internal/cli -run 'External|RoundTrip|UnknownKey' -count=1` passes.
- In a temporary initialized repository: `awit create Linked --brief 'A linked issue.' --external-tracker gitea --external-repo owner/repo --external-id 127 --external-url https://forge.example/owner/repo/issues/127` writes the mapping; `awit show <returned-id>` and `awit list --format json` expose it.
- `awit update <id> --clear-external` removes it without changing the body or unrelated keys.
- An old `external: gitlab#42` remains locally selectable and round-trips; `awit validate` warns, does not quarantine it, and exits 0 absent other faults.
- Invalid CLI mappings exit 2 with no item write. No tea process or Git commit occurs.

## Out of scope

Import, remote requests, alias lookup, GitHub/GitLab support, automatic conversion of reserved scalars, arbitrary YAML source-span editing after real mutations.

