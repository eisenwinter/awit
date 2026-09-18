---
id: AWIT-0NF68SDS
title: close prints closed ID confirmation
brief: >-
  Kimi K2.5 run: close prints the compact entry line with no status, so the model could not tell it closed anything. archive already prints archived ID per line; close should print closed ID the same way.
status: closed
deps: []
labels: []
refs: [../comments/AWIT-0NF68SDS/20260918T154423Z-orchestrator.md, ../comments/AWIT-0NF68SDS/20260918T154423Z-orchestrator-2.md]
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
