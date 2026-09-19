---
id: AWIT-0NF68SDR
title: list views show item status
brief: >-
  Kimi K2.5 run: compact list has no status column and table STATE merges open with in_progress, forcing list --format json to confirm transitions. Carry status in compact and table views.
status: closed
deps: []
labels: []
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

Evidence: same Kimi run — after closes printed without status, default
`list` could not confirm either: compact lines carry no status column, and
table STATE renders both `open` and `in_progress` as ready/blocked. Only
`--format json` distinguishes them.

Carry `status` in compact (`[ID] status Title | …` or appended token — keep
golden-stable choice in the diff) and add a STATUS column to table. JSON
already has it. Scope is display only; sorting/filtering untouched.

## Acceptance Criteria

- [ ] Compact and table `list` show each item's status distinctly
  (open vs in_progress vs closed visible without json).
- [ ] Golden files updated via `-update`; `validate` PASS.

## Comments

### 2026-09-18T15:38:01Z agent/orchestrator

dev DONE: status in compact ([ID] status Title) + STATUS table column; 6 goldens regenerated via -update, pure status insertions. Verified: build+vet clean, go test ./... -count=1 all ok, reviewer APPROVE (prime BLOCKED lines stay status-free: pre-existing bespoke template, out of scope). Live smoke: open vs in_progress distinct in both views.

### 2026-09-18T15:38:08Z agent/orchestrator

implemented
