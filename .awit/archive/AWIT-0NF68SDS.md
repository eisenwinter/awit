---
id: AWIT-0NF68SDS
title: close prints closed ID confirmation
brief: >-
  Kimi K2.5 run: close prints the compact entry line with no status, so the model could not tell it closed anything. archive already prints archived ID per line; close should print closed ID the same way.
status: closed
deps: []
labels: []
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

Evidence: Kimi K2.5 playground run (`/tmp/longcat/browserfetch`,
2026-09-18, transcript `history://KimiBrowserfetch` lines 25-28). Three
`awit close --reason` calls printed compact entry lines
(`[ID] Title | labels | Unblocks: 0`) with no status; the model wrote
"The closes printed but not confirmed" and dropped to
`list --format json | jq` to verify.

Precedent: `archive` prints one `archived <id>` line per item plus
`Archived N items` (guide §2). `close` should print one `closed <id>` line.
Keep stdout (close has no `--format` contract to protect); golden-test it.

## Acceptance Criteria

- [ ] `awit close <id>` prints `closed <id>`; output names the resulting state.
- [ ] Existing close tests updated; `validate` PASS.

## Comments

### 2026-09-18T15:44:23Z agent/orchestrator

implemented

### 2026-09-18T15:44:23Z agent/orchestrator

dev DONE: close prints closed <id> on stdout in both paths, error paths silent. Verified: build+vet clean, go test ./... -count=1 all ok, reviewer APPROVE (persistence via AddComment/Save unchanged). Live smoke: closed line printed, status closed persisted, list shows closed.
