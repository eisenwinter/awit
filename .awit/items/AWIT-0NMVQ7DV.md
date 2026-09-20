---
id: AWIT-0NMVQ7DV
title: 'rollout: verify the manual block end to end in a scratch checkout'
brief: >-
  Run the full block lifecycle smoke in a disposable checkout and confirm docs and skill parity, without touching real queue state or tracker labels.
status: in_progress
deps: [AWIT-0NMVPMDJ]
labels: [phase6, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
claimed_at: "2026-09-20T19:19:49Z"
---
## Summary

Operator/orchestrator rollout verification for the manual-block feature (MB1-MB3): prove the whole lifecycle in a disposable checkout with the real binary, confirm read-surface truthfulness and skill parity, and retire the TICKETS.md/CLAUDE.md per-ticket suppression workaround pattern where it appears in consuming checkouts. No live tracker-label changes; stale tracker labels are never trusted or silently modified.

## Context (read first)

- `AGENTS.md`; MB1-MB3 (`AWIT-0NMVMSDQ`, `AWIT-0NMVNPDQ`, `AWIT-0NMVPMDJ`) — read all three via `awit show`.
- Plan smoke scenario §G and acceptance list; reviewed strings `agent://ProseBlockStrings`.
- The original "live EEEEE-F1c" reference is origin-checkout noise: do not hunt it in this repo. The general case is: any consuming checkout that suppresses a zero-dep item via out-of-band notes gets a real `blocked_reason` instead.

## Files

- No production code. Only: disposable temp checkout (never this repo's real `.awit` store), plus the consuming checkout's redundant queue-suppression notes if such a case is confirmed during rollout (remove only the redundant scheduling instructions after the item holds a real reason; preserve decision history and ordinary ticket references).

## Steps

- [ ] Build the binary; create a disposable temp repo (`awit init`, claim commits disabled per existing policy).
- [ ] Create A (no deps) and B depending on A; record canonical IDs.
- [ ] Claim A, then `block` A with a concrete release condition. Confirm A is open, unclaimed, carries the reason, retains deps/body; B still depends on A.
- [ ] Ranked `next`: expect exit 1 with the existing `No ready items` message. `next --claim <A>`: expect refusal naming the reason + unblock command. `next <A>` without claim: inspectable.
- [ ] Unbounded `prime`, `list --blocked`, `show` (human + JSON): A only manually blocked, B dependency-blocked, no quarantine warning. `validate`: PASS.
- [ ] `release` A: block persists, still not offered. `unblock` A: A ready, B still waits. Claim/close A normally; B becomes ready.
- [ ] Block an item with an open dep, then unblock: still dependency-blocked. Exercise `close` and `update --status closed` on blocked items; no obsolete reason returns on later `release`.
- [ ] With tracker tools unavailable, repeat block/unblock on an item with a valid external mapping: local ops succeed with no tool discovery or network.
- [ ] Smoke root/block/unblock/next/list/update/import help and introduced-label warnings (tracker stubs for import only).
- [ ] Confirm generated skill parity (`TestDogfoodOmpCopyMatchesRenderer`) and that docs no longer route around the queue with side notes. Do NOT unblock a real held item merely to test resumption.

## Acceptance Criteria

- Full lifecycle proven in the scratch checkout: block → excluded everywhere selectable → visible everywhere inspectable → release preserves → unblock resumes → close clears.
- A held item keeps real deps, is not offered by ranked `next`/`--claim`/`prime READY`, remains inspectable via `show`/`list --blocked`/unbounded `prime`, and `validate` is PASS without new faults.
- No real `.awit` state mutated for the smoke; no tracker label created, removed, or inferred.
- Where a consuming checkout suppressed selection via side notes, the item now carries an explicit local reason + release condition and the redundant suppression notes are removed (history preserved).

## Out of scope

New behavior, label sync, auto-migration of label-only items, expiry, permissions; unblocking real holds for testing.
