---
id: AWIT-0NPYAWT2
title: 'awit: dogfood the guide section 6 body template'
brief: >-
  Guide section 6 declares seven mandatory body sections but create emits the two-section default, so every item filed in this repo starts out violating the documented format. Adds .awit/templates/workitem.md and points config.yaml template: at it.
status: closed
deps: [AWIT-0NPYAVTA]
labels: [phase7, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

`plan/implementation-guide.md` §6 declares seven mandatory work item body sections in a
fixed order, but `create` in this repo emits the two-section default skeleton. Every item
filed here starts out violating the repo's own documented format unless the author had read
§6. Adding `.awit/templates/workitem.md` and pointing `config.yaml template:` at it closes
that, and makes `awit template` emit what this repo actually requires.

Configuration only — no Go changes.

## Context (read first)

- `plan/implementation-guide.md` §6 — the seven mandatory sections and their order, and the rule that conflict-marker bytes quoted in a body must be broken up or the item quarantines.
- `pkg/config/config.go:143-152` — `validateTemplatePath`: the value must be repo-root-relative with forward slashes; absolute paths and lexical `..` fail config load.
- Depends on `awit template` existing, which is how Step 3 is verified.

## Files

- `.awit/templates/workitem.md` — new.
- `.awit/config.yaml` — add `template:`.

## Interfaces

None.

## Steps

- [ ] Create `.awit/templates/workitem.md` with exactly these bytes — leading blank line included, matching how `item.DefaultBody` separates the body from the closing frontmatter fence:

```
(blank line)
## Summary
(blank)
## Context (read first)
(blank)
## Files
(blank)
## Interfaces
(blank)
## Steps
(blank)
## Acceptance Criteria
(blank)
## Out of scope
```

  i.e. the literal file starts with `\n## Summary\n\n## Context (read first)\n\n## Files\n\n## Interfaces\n\n## Steps\n\n## Acceptance Criteria\n\n## Out of scope\n`.

- [ ] Append to `.awit/config.yaml`:

```yaml
template: .awit/templates/workitem.md
```

- [ ] Run `awit template` — expect the seven section headings in order, exit 0. A `template .awit/templates/workitem.md: …` error means the path or the `template:` value is wrong
- [ ] Run `awit validate` — expect `PASS`. The file sits under `.awit/templates/`, not `.awit/items/`, so it is never scanned as an item
- [ ] Run `go test ./...` — PASS. Tests use `initRepo`/`copyFixture` against temp directories and never read the real `.awit/config.yaml`. A failure here means a test reads the repo's own config and must be reported, not worked around
- [ ] Commit: `awit: dogfood the guide section 6 body template`

## Acceptance Criteria

- `awit template` prints the seven §6 section headings in order, exit 0.
- `awit validate` prints `PASS`.
- `go test ./...` passes.
- A fresh `awit create` in this repo produces an item body carrying all seven sections.

## Out of scope

- Rewriting existing items' bodies to match.
- Any `validate` check that a body carries the sections.


## Comments

### 2026-09-21T14:04:42Z agent/orchestrator

repo config now points template: at .awit/templates/workitem.md so create and awit template emit the seven guide section 6 sections

### 2026-09-21T14:04:42Z agent/orchestrator

Implemented by DogfoodWorker: template file (exact 113-byte spec layout) + one config.yaml line. Verified by reviewer agent (scratch build): template prints seven section 6 headings, validate PASS, temp-dir create smoke carries all seven sections; SATISFIED zero findings. Worker deviation accepted: spec smoke id AWIT-SMOK0001 is invalid base32, minted id used in throwaway repo instead.
