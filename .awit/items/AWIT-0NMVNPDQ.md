---
id: AWIT-0NMVNPDQ
title: 'cli: add block and unblock commands with truthful read surfaces'
brief: >-
  Add awit block and unblock commands with the specified transition table, refuse or skip held items in every claim path, and render reasons in next, prime, list and show.
status: closed
deps: [AWIT-0NMVMSDQ]
labels: [phase6, p0]
refs_base: repo
refs: [.awit/comments/AWIT-0NMVNPDQ/20260920T190521Z-orchestrator.md, .awit/comments/AWIT-0NMVNPDQ/20260920T190521Z-orchestrator-2.md]
assignee: agent/orchestrator
---
## Summary

Expose the MB1 manual block to agents and operators: `awit block <id> --reason` and `awit unblock <id>` in a new `internal/cli/block.go`, the full lifecycle transition table, claim refusal in `next`, and reason rendering in `format.Entry`, `prime`, `list`, and `show`. No tracker or git invocations from these commands.

## Context (read first)

- `AGENTS.md`; guide §§1, 2, 4.7, 4.8, 4.11 and 5.
- MB1 (`AWIT-0NMVMSDQ`): `Item.BlockedReason`, `SetBlockedReason`, Ready/Blocked classification. Read it via `awit show AWIT-0NMVMSDQ`.
- `internal/cli/next.go:139-175,258-280` — `nextAction` ranked vs exact lookup; `refuseClaim` rejects quarantined/closed/dep-blocked/claimed.
- `internal/cli/release.go:45-56` — `releaseAction` sets open, clears assignee/claim, optional remote-open push.
- `internal/cli/app.go:127-145` — `newRoot` command registration; `toEntry` conversion.
- `pkg/format/format.go` — `Entry`, compact/table/JSON rendering.
- `pkg/prime/prime.go:59-120` — `blockedLine`, `Render`, `entryOf`.
- `internal/cli/list.go:43-114`, `internal/cli/close.go`, `internal/cli/update.go` — presentation and status transitions.
- Full design: `agent://PlanBlockedState` sections D3-D4, G. Reviewed strings: `agent://ProseBlockStrings` (use the reviewed finals verbatim).

## Files

- New `internal/cli/block.go` (`blockCmd`, `unblockCmd`, `blockAction`, `unblockAction`) + behavior tests; register both in `newRoot`.
- Modify `internal/cli/next.go` (claim refusal), `close.go` + `update.go` (close/status-closed clears reason; release/status-open preserves).
- Modify `pkg/format/format.go` (`Entry.BlockedReason` + rendering), `internal/cli/app.go` (`toEntry`), `pkg/prime/prime.go` (`entryOf`, `blockedLine`).
- Update `docs/schema.md`, `plan/implementation-guide.md` §§2, 4.7, 4.8, 4.11, `plan/awit-implementation-plan.md`, `README.md` Commands/Agent loop.
- No change to resolver, SCC, ranking, critical path, archive, tracker transport.

## Interfaces

Commands (prose-reviewed help; wrappings ≤75 cols):

- `awit block <id> --reason "<obstacle and release condition>"`
  - Usage: `Pause an item with a recorded reason until it is unblocked`; ArgsUsage `<id>`; `--reason` Usage: `why work stopped and what will unblock it`.
  - Description:
    ```text
    Record a local manual block; dependency edges and tracker labels are
    unchanged. The reason must be non-empty and single-line. The item's status
    becomes open and its claim is cleared. Run awit unblock <id> once the
    condition is resolved.
    ```
- `awit unblock <id>`
  - Usage: `Remove an item's manual block`; ArgsUsage `<id>`.
  - Description:
    ```text
    Remove only the manual block. Open dependencies can still keep the item
    blocked. Unblocking does not claim or reopen the item and does not change
    tracker labels.
    ```
- Use existing `openStore`, `noteWalkedUp`, `loadItem` lookup, `Store.Lock(5s)`, setters, one `Store.Save`. Validate arity/reason before mutation; usage errors exit 2; no success line before persistence. Neither command commits, discovers tracker tools, reads remote state, pushes, or auto-comments.

Transition table:

| Action | Result |
| --- | --- |
| `block` on open/in-progress | store/replace reason, status open, clear assignee + `claimed_at`, preserve deps/labels/body/refs/external link — one save, no intermediate selectable state |
| repeated `block` | replaces reason, same logical state |
| `block` on closed | refuse exit 1: `%s is closed; awit release %s to reopen it before blocking` |
| `unblock` | remove only `blocked_reason`; preserve status, deps, claim fields; never claims/reopens; idempotent |
| `release` | preserves the manual block (existing open/claim-clear + optional remote-open push) |
| `update --status open\|in_progress` | preserves the manual block |
| `close` / `update --status closed` | removes the manual block (alongside existing claim handling) |
| dep add/remove/close | recompute dep readiness normally; never clears the manual block |
| label add/remove | never sets or clears the manual block |

Changed help (prose-reviewed):

- `release`: `Return an item to open and clear its claim; any manual block stays`
- `close`: `Mark an item closed, clearing the claim timestamp and any manual block`
- `update --status`: ``set status: `open`, in_progress or closed; closed clears any manual block`` (backticks required — urfave placeholder)
- `list --blocked`: `include blocked items (open deps or a manual block)`
- `next` Description:
  ```text
  Ranked selection skips manually blocked items. An explicit [id] without
  --claim is a lookup, so it can print a blocked item. To claim one, resolve
  the recorded condition and run awit unblock <id> first.
  ```
- `prime` Description:
  ```text
  Manually blocked items appear under BLOCKED with their reason, never
  under READY. A tight --max-tokens budget can shed blocked rows; run
  awit list --blocked for the full queue.
  ```

Diagnostics/confirmations (exact):

- arity exit 2: `block takes exactly one item id` / `unblock takes exactly one item id`
- bad reason exit 2: `block requires --reason with a non-empty, single-line explanation`
- claim refusal exit 1: `%s is manually blocked (%s); awit unblock %s once resolved` (check `refuseClaim` after quarantine/closed, before dep-blocked message; keep dep-only message when no reason)
- success stdout: `blocked %s: %s` / `unblocked %s` (plain lines even under `--format json`, like close/release)

Presentation:

- `format.Entry.BlockedReason string` with `json:"blocked_reason,omitempty"`; populate in both `toEntry` and `entryOf` (no package-boundary change).
- JSON keeps `state: "blocked"`; reason distinguishes the cause.
- Compact suffix: ` | Blocked reason: %s`. Single-item table: `Blocked reason: %s` right after State. List table: conditional `BLOCKED_REASON` column when any displayed row has a reason.
- Prime rows: manual-only `[<id>] <title> | Blocked reason: %s (awit unblock %s)`; combined `[<id>] <title> <- <deps> | Blocked reason: %s (awit unblock %s)`; dep-only rows unchanged. Reuse BLOCKED section, label filtering, ordering, token shedding. No HELD section, no new warnings. `list --blocked` includes dep- and manually blocked rows via the existing graph predicate.

## Steps

- [ ] Write failing CLI transition regressions (block saves reason + clears claim atomically; all claim forms refuse/skip; read-only `next <id>` still displays; unblock preserves deps; release preserves block; close/status-closed clears; no git/tea/glab invocation) and projection regressions (prime manual-only/combined rows; list/show human+JSON; valid block is not a graph fault).
- [ ] Run scoped tests; record intended failures.
- [ ] Implement `internal/cli/block.go`, registration, `next.go` refusal, `close.go`/`update.go` transitions, both `Entry` conversions, prime rendering.
- [ ] Rerun scoped tests to green. Smoke the real CLI in a temp checkout per plan §G scenario (create A, B-on-A; claim A; block A; ranked `next` → No ready items; `next --claim A` refused; `next A` displays; prime/list/show/validate; release preserves; unblock; close A; B ready; dep-still-open unblock case; tracker tools unavailable with external mapping still succeeds locally).
- [ ] Update all listed docs. Submit scoped proof for orchestrator review/commit, not project-wide validation.

## Acceptance Criteria

- Block saves reason + claim release atomically; every claim/selection form obeys the hold; exact read-only lookup remains usable.
- Unblock preserves deps; release cannot resume a held item; close and status-closed clear the reason.
- Read surfaces expose manual and dependency causes; valid blocks produce no graph faults; `validate` stays PASS.
- `block`/`unblock` never invoke git, tea, or glab; no tracker push from these commands.
- Targeted tests `ManualBlock`, `BlockCommand`, `UnblockCommand` (`./internal/cli`) and `ManualBlock` (`./pkg/format`, `./pkg/prime`) pass.

## Out of scope

Label warning, skill/agent-doc updates (MB3); label sync or `blocked`→hold mapping; new HELD section; expiry/permissions; live-case migration (MB4).
