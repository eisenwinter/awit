---
id: AWIT-0NF68SDG
title: update echoes changed fields on success
brief: >-
  Kimi K2.5 run: update --status prints nothing on success, indistinguishable from a no-op. Echo the changed fields (one line each) so callers see what happened.
status: closed
deps: []
labels: []
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

Evidence: same Kimi run, transcript line 22 ("The close output didn't show,
but fine") after `update --status in_progress`, whose output was empty —
only the chained `comment` printed its ref path. Silent success reads as a
no-op to a weak model.

On success print one line per changed field, e.g.
`updated <id>: status=in_progress`. No `--format` contract on update output;
keep it to stdout, one line per field, deterministic order.

## Acceptance Criteria

- [ ] `awit update <id> --status in_progress` prints the changed field(s).
- [ ] No-change invocation (`nothing to update`) output unchanged.
- [ ] Existing update tests updated; `validate` PASS.

## Comments

### 2026-09-18T15:41:29Z agent/orchestrator

implemented

### 2026-09-18T15:41:29Z agent/orchestrator

dev DONE: update echoes updated <id>: field=value per flag-supplied field on stdout, fixed order. Verified: build+vet clean, go test ./... -count=1 all ok, reviewer APPROVE. Live smoke: change echoes, bare update still Error: nothing to update exit 1.
