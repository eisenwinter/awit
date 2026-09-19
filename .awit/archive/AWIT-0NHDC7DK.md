---
id: AWIT-0NHDC7DK
title: 'create: use a configured repository item-body template'
brief: >-
  Let repositories configure a body template path while retaining the existing skeleton when no template is configured. Fail explicitly on an invalid configured template rather than silently creating the wrong work-item structure.
status: closed
deps: [AWIT-0ND56A3G, AWIT-0ND56H3G, AWIT-0NHDBCDN]
labels: [phase1, p1]
refs_base: repo
refs: []
---
## Summary

Add optional config `template: plan/workitem-template.md`. Normal create uses the template’s bytes as the item body; absent/empty config keeps the current skeleton.

## Context (read first)

- `Item.New` currently supplies `\n## Summary\n\n## Acceptance Criteria\n\n`.
- `createAction` creates/saves items and keeps --brief mandatory.
- Draft 1 defines `SetBody`; draft 2 import deliberately uses remote body instead of templates.

## Files

- Modify `pkg/config/config.go`, its tests; `internal/cli/create.go`, create tests.
- Update schema config/Body, guide §2 body-template decision and §4.2, README creation guidance, spec Data model/Phase 1, embedded skill Adding work items.

## Interfaces

- Add `Config.Template string` with `yaml:"template,omitempty"`.
- Config value is a forward-slash, repo-root-relative path, never cwd-relative or `.awit`-relative. Reject absolute paths and lexical escape above root. Do not add environment expansion, globbing, or template-language interpolation.
- Only create loads it; list/show/validate do not read the template file merely to open config. Syntactically invalid config values fail config load. Missing/unreadable/directory/non-UTF-8 configured files fail create before mint/save with the path and cause.
- Reject conflict-marker lines because the resulting item would quarantine. Treat the file as **body-only** content; do not parse it as frontmatter or merge arbitrary item fields.
- Preserve file bytes exactly, including empty content and terminal newlines. The closing frontmatter fence already separates the body; do not synthesize an extra leading newline for custom templates.
- No configured template means the current default skeleton verbatim. An explicitly configured broken template never silently falls back. Import always uses the issue body and never reads the template.

## Steps

- [ ] Add failing create tests for root-relative lookup from nested cwd, exact byte content, absent fallback, an intentionally empty template, and configured read/encoding/conflict-marker errors leaving no new item.
- [ ] Run `go test ./pkg/config ./internal/cli -run 'Template|Create' -count=1`; observe new failures.
- [ ] Implement config path validation and template read inside create; use `it.SetBody(templateBytes)` before the existing atomic save. Keep `Item.New`’s default unchanged for non-template callers.
- [ ] Prove templates cannot change frontmatter identity/status and that import still ignores local templates.
- [ ] Rerun tests, update docs with a body-only example, and give the orchestrator evidence.

## Acceptance Criteria

- Scoped tests above pass.
- With `template: plan/workitem-template.md`, `awit create 'Template check' --brief 'A template check.'` produces a body byte-equal to that file, from root or nested cwd.
- With no template config, the body equals the previous default skeleton.
- A missing configured path exits 1 naming the path and creates no item; an empty file is accepted as an empty body.

## Out of scope

A templating engine, per-label templates, variable expansion, changing frontmatter defaults, applying templates to imports or existing items.


## Comments

### 2026-09-19T15:38:16Z agent/orchestrator

implemented
