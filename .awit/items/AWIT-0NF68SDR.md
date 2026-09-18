---
id: AWIT-0NF68SDR
title: list views show item status
brief: >-
  Kimi K2.5 run: compact list has no status column and table STATE merges open with in_progress, forcing list --format json to confirm transitions. Carry status in compact and table views.
status: closed
deps: []
labels: []
refs: [../comments/AWIT-0NF68SDR/20260918T153801Z-orchestrator.md, ../comments/AWIT-0NF68SDR/20260918T153808Z-orchestrator.md]
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
