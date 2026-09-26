---
id: AWIT-0P4XRXDG
title: Document skill sync across command references and the driving-awit skill
brief: >-
  README, usage, design matrix, embedded Quick reference, and help consistency for skill sync.
status: in_progress
deps: [AWIT-0P4XRXDX]
labels: []
refs_base: repo
refs: []
assignee: agent/orchestrator
claimed_at: "2026-09-26T21:01:05Z"
---
## Summary

Paired documentation cutover for `awit skill sync`, including the embedded driving-awit Quick reference row (source edit + dogfood re-proof).

## Context (read first)

- `README.md` command table, `docs/usage.md` command table + usage section, `docs/design-spec.md` §5 matrix (§7 only on real package change).
- Embedded source: `internal/skill/assets/driving-awit.body.md` Quick reference table (never edit generated copies).

## Files

- `README.md`, `docs/usage.md`, `docs/design-spec.md`, `internal/skill/assets/driving-awit.body.md`, command help as needed.

## Interfaces

- Documented: invocation, detected-target scope, current/updated/created records, inherited `--repo`, hand-edit overwrite, error behavior, exit 0. No dry-run/JSON.

## Steps

- [ ] RED (review): list missing/inaccurate entries across the inventories vs actual `--help`/behavior.
- [ ] GREEN: README + usage entries; §5 matrix entry (§7 only if packages changed); embedded Quick reference row at source; regenerate seeded `.omp` copy; stale-comment audit.
- [ ] Re-exercise dogfood scenario (or exact-byte smoke) against final embedded source; compare help examples to observed output.

## Acceptance Criteria

- All inventories contain the command with accurate semantics; design-spec §5 present; Quick reference row present and distributed byte-exactly by sync.
- Help and docs agree; no undocumented flags/schema.
- No generated-copy-only edits.

## Out of scope

New specs, unrelated cleanup, new architecture, project-wide validation (main agent runs it once at the end).
