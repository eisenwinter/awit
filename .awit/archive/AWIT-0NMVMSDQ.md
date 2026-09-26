---
id: AWIT-0NMVMSDQ
title: "item/graph: represent a manual block with blocked_reason"
brief: >-
  Persist an optional blocked_reason on items and exclude manually blocked nodes from Ready in graph classification, keeping edges and quarantine unchanged.
status: closed
deps: []
labels: [phase6, p0]
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

A `blocked` tracker label with zero open deps is unrepresentable today: `Graph.classify` marks any healthy non-closed zero-dep node ready regardless of labels. Add a durable local manual block - optional `blocked_reason` string on `Item` (non-empty = held) - and centralize exclusion in `Graph.classify` so ranked `next`, `prime`, and `list --blocked` all agree. No new status, no quarantine category, no graph edges.

## Context (read first)

- `AGENTS.md`; guide §§1, 2, 4.3, 4.6 and 5.
- `pkg/item/item.go:20-40,65-157` - `Item`, parse, setters; no block field today.
- `pkg/item/reason.go:8-25` - `Status`/`ParseStatus` accept only open/in_progress/closed; unchanged.
- `pkg/graph/rank.go:12-36` - `Graph.classify`, `depSatisfied`; loop starts `ready := true`.
- `pkg/graph/graph.go:93-151` - `Build`; `Node.OpenDepIDs` at 38-55 returns dep IDs only.
- `docs/schema.md` - "Blocked is derived, never stored" claim must be updated by this item.
- Full design: `agent://PlanBlockedState` sections A-E, D1-D2, G. Reviewed strings: `agent://ProseBlockStrings` #23, #24, #29-31 (use reviewed wording for any user-visible text).

## Files

- Modify `pkg/item/item.go` and `pkg/item/item_test.go`.
- Modify `pkg/graph/rank.go` and `pkg/graph/rank_test.go`.
- Update `docs/schema.md` (Required keys / Optional known keys / new "Manual blocks" section / Quarantine note), `plan/implementation-guide.md` §§2, 4.3, 4.6, `plan/awit-implementation-plan.md` (Decisions, Data model, Quarantine), `README.md` lifecycle note.
- Leave `Status`, `Reason`, dep IDs, external metadata, resolver APIs unchanged.

## Interfaces

In `pkg/item/item.go`:

```go
func (it *Item) SetBlockedReason(reason string) error
```

Exact behavior:

1. `reason == ""` removes the `blocked_reason` key from the YAML mapping.
2. Non-empty input is trimmed of outer whitespace, then validated: single-line (no `\n`, `\r`, or other control characters), non-empty after trim. Invalid → error, no mutation.
3. The scalar is written through the existing YAML-node helpers, preserving unknown fields, body bytes, and `Store.Save` reference normalization.
4. Parsing accepts only a YAML string scalar. Wrong type, empty/whitespace-only content, control characters, or duplicate `blocked_reason` keys are parse errors (fail closed - never selectable when the declaration is unreadable). Exact messages (prose-reviewed):
   - `item: blocked_reason must be a non-empty, single-line string without control characters`
   - `item: duplicate key blocked_reason`
5. Keep the validator private to `pkg/item`, shared by parsing and the setter. No general metadata/schema framework.
6. `SetStatus` stays a plain field setter; lifecycle commands own transitions (MB2).

In `pkg/graph/rank.go`, `Graph.classify` (precedence order):

1. Quarantined or closed nodes: `Ready` and `Blocked` both false (unchanged).
2. Healthy non-closed node: ready requires **both** absent manual block **and** all existing dependency checks passing.
3. `Blocked` = inverse of readiness for healthy non-closed nodes.

Thus Ready = healthy, non-closed, no `blocked_reason`, all deps satisfied. Blocked = healthy, non-closed, with reason and/or unsatisfied deps. Keep edges, `depSatisfied`, `OpenDepIDs`, cycle detection, archive logic, `UnblockCount`, `CriticalPath` unchanged (structural metrics may include held nodes, as with dep-blocked nodes today).

## Steps

- [ ] Record the exact API/docs contract above; write failing persistence/readiness regressions first: zero-dep item with reason is valid + blocked + not ready + not quarantined; label-only `blocked` zero-dep item stays ready; removing the reason restores readiness only when deps permit; malformed declarations (wrong type, whitespace-only, control chars, duplicate key) fail closed; `SetBlockedReason("")` removes the key; unrelated fields/body survive set/remove.
- [ ] Run `go test ./pkg/item -run BlockedReason -count=1` and `go test ./pkg/graph -run ManualBlock -count=1`; record the intended failures.
- [ ] Implement `Item.BlockedReason` field, `SetBlockedReason`, parse contract, and centralized Ready/Blocked classification (smallest change).
- [ ] Rerun the scoped tests to green. Exercise a held middle node through archive/critical-path paths (no algorithm change).
- [ ] Update all listed docs with the field, validation/presence rules, and the stored-lifecycle vs derived-eligibility distinction. Do not advertise `block`/`unblock` commands yet (MB2).

## Acceptance Criteria

- `go test ./pkg/item -run BlockedReason -count=1` and `go test ./pkg/graph -run ManualBlock -count=1` pass.
- A zero-dependency manual block is valid, blocked, not ready, not quarantined; a zero-dependency `blocked` label alone remains ready.
- Removing the reason restores readiness only when deps permit; malformed block declarations cannot become selectable.
- Normal YAML/body preservation survives set/remove; existing archive/critical-path behavior unchanged.
- Docs updated as listed; no new CLI command in this item.

## Out of scope

`block`/`unblock` commands, read-surface rendering, label warnings, skill updates (MB2/MB3); `blocked_by` external refs, synthetic deps, quarantine-dir moves, fourth lifecycle status, expiry, permissions; tracker label sync.

## Comments

### 2026-09-20T18:51:29Z agent/orchestrator

implemented

### 2026-09-20T18:51:29Z agent/orchestrator

MB1 DONE: blocked_reason graph representation; go test ./pkg/item/... ./pkg/graph/... pass; review SPEC+QUALITY PASS with 3 doc-line restorations applied by orchestrator pre-commit.
