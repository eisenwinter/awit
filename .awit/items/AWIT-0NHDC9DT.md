---
id: AWIT-0NHDC9DT
title: 'config: warn on labels outside the declared repository vocabulary'
brief: >-
  Add optional config labels as an advisory vocabulary and warn about unknown labels introduced through create/update. Keep free-form labels usable and preserve observed label-count semantics.
status: closed
deps: [AWIT-0ND56A3G, AWIT-0ND56H3G, AWIT-0NF68SDG, AWIT-0ND5763G]
labels: [phase5, p2]
refs_base: repo
refs: [.awit/comments/AWIT-0NHDC9DT/20260919T154852Z-orchestrator.md]
---
## Summary

Support:

```yaml
labels: [phase0, phase1, phase2, phase3, phase4, phase5, p0, p1, p2]
```

This declares recommended names, not an allowlist that rejects work.

## Context (read first)

- `mergeLabels`, `updateAction` label additions/removals, `labelCounts`.
- Guide §2 currently says labels are never declared; replace that statement in living docs and code comments.

## Files

- Modify `pkg/config/config.go`, tests; `internal/cli/create.go`, `update.go`, `label.go` comments and associated tests.
- Update schema config/labels, guide §2/4.2, README label guidance, spec Data model, skill vocabulary/Adding work items.

## Interfaces

- Add `Config.Labels []string` with `yaml:"labels,omitempty"`.
- Missing or empty list disables vocabulary warnings. Entries must be nonempty strings with no leading/trailing whitespace or control characters; duplicate names are deduplicated in memory. Matching is case-sensitive, like existing labels; no automatic lowercasing or renaming.
- On successful create, compare all final labels, including default_labels, against the configured vocabulary.
- On successful update, compare only labels newly introduced by that invocation and retained after --unlabel. Do not repeatedly warn about pre-existing unknown labels during unrelated updates, or warn about labels removed in the same command.
- Print once per successful command, sorted/deduplicated: `warning: unknown labels: <comma-space-list> (declare them in .awit/config.yaml labels)` on stderr. Save normally and retain exit 0. Failed mutations do not print a misleading success-path advisory.
- `awit label` still reports actual use counts, includes used undeclared labels, excludes unused declared labels, and preserves count-desc/label-asc ordering and current closed/quarantine behavior.
- Import preserves source labels without changing them or merging default_labels; this ticket adds warnings only to create/update as requested.

## Steps

- [ ] Write failing tests for create defaults, new update additions, add-then-remove, repeated existing labels, disabled vocabulary, exact case, JSON stdout, and actual label counts.
- [ ] Run `go test ./pkg/config ./internal/cli -run 'DeclaredLabels|UnknownLabels|LabelCounts' -count=1`; observe failures.
- [ ] Implement config validation and one small CLI warning helper reused by create/update. Compute introduced/retained labels from the actual mutation outcome.
- [ ] Verify warning-only behavior leaves labels stored and queryable; avoid a second vocabulary/count data model.
- [ ] Rerun targeted tests, update contradictory living docs/comments, and deliver evidence for orchestrator commit.

## Acceptance Criteria

- Scoped tests above pass.
- With vocabulary `[p1]`, `awit create Test --brief 'A test item.' -l typo` exits 0, stores typo, and emits the exact advisory on stderr without corrupting JSON stdout.
- `awit update <id> -l auth --unlabel auth` does not warn for auth if it is absent in the final labels.
- `awit label --state all --format json` includes used undeclared labels and excludes declared-but-unused names.
- No quarantine, validation FAIL, or automatic label correction is introduced.

## Out of scope

Strict allowlist enforcement, automatic typo correction, color-coded priorities, declared zero-count rows, remote label synchronization.

