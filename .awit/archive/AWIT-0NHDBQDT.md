---
id: AWIT-0NHDBQDT
title: 'external: push local close and reopen state to Gitea'
brief: >-
  Push linked issue state after successful local close/release and explicit status updates, with local state remaining canonical. Remote failure warns without rolling back local work; --no-push provides an explicit offline path.
status: closed
deps: [AWIT-0ND56M3G, AWIT-0NF68SDS, AWIT-0NF68SDG, AWIT-0NHDBCDN, AWIT-0NHDBJDR]
labels: [phase5, p1]
refs_base: repo
refs: []
---
## Summary

For valid linked items, propagate local state to Gitea after successful local mutation. No remote state is used to choose or change local state.

## Context (read first)

- Guide §2 decision 3: only `next --claim` can commit.
- `closeAction`, `releaseAction`, `updateAction`; `AddComment` already saves the item when close has a reason.
- Draft 2’s tea wrapper and authentication/HTTP status contract.

## Files

- Modify `internal/cli/{close,release,update}.go` and tests.
- Extend `internal/teax/teax.go` and tests.
- Add a small shared CLI state-push helper in `internal/cli/external_state.go`.
- Update README Commands/Agent loop, guide resolved decisions, schema external behavior, spec, and skill transition/offline guidance.

## Interfaces

```go
func (c *Client) SetState(ctx context.Context, number int64, state string) error
```

Exact commands:

```text
tea api --login <login> --repo <owner/repo> --include -X PATCH -f state=closed repos/<owner>/<repo>/issues/127
tea api --login <login> --repo <owner/repo> --include -X PATCH -f state=open repos/<owner>/<repo>/issues/127
```

- Triggers: `close`→closed; `release`→open; `update --status closed`→closed; `update --status open|in_progress`→open. A non-status update does not push. `next --claim`, create, import, comment, ref, and archive do not push.
- Add `--no-push` and `--tea-login` to those three commands. `--no-push` performs no tea discovery/auth/network operation, even if linked metadata is malformed. No repository-wide push configuration or overloaded `commit` setting.
- Write the local item successfully first. Preserve close’s reason/comment and claim-clearing behavior. Then push while retaining the existing store lock, with the bounded subprocess deadline, so same-checkout close/reopen commands cannot reorder their remote writes.
- Remote failure, missing tea, invalid metadata, or ambiguous linked identity keeps the local mutation and ordinary confirmation. Print one stderr warning: `warning: <id> saved locally; external state push failed: <reason>; retry with awit update <id> --status <status>`. Exit remains 0 for successful local mutation. Local write failure exits 1 and never attempts remote push.
- Repeating an explicit status action pushes even if local status is already equal; this is the recovery path, without a queue or daemon. Recommend retry through update rather than duplicating a close reason comment.
- Validate response identity/state and HTTP status; do not GET remote issue state before deciding what to write. A PATCH response is delivery verification, not a source of local truth.
- No automatic retries. Different clones remain coordinated only by Git, as today; the same-checkout lock does not claim distributed consistency.

## Steps

- [ ] Add failing subprocess-backed tests for each trigger, no-push, non-status updates, same-status retry, HTTP failure with exit zero, local-save failure preventing a push, and remote failure preserving local state.
- [ ] Run `go test ./internal/cli ./internal/teax -run 'ExternalState|Close|Release|UpdateStatus' -count=1`; record failures.
- [ ] Implement `SetState` and one shared helper; call it only after successful persistence from the named actions. Keep existing confirmation/format conventions and no-commit behavior.
- [ ] Prove local close remains closed when remote state was open or remote access fails, and release remains open when remote state was closed. Prove `--no-push` never executes tea.
- [ ] Rerun the scoped command to green, update docs, and provide orchestrator commit evidence.

## Acceptance Criteria

- Scoped tests above pass.
- On a disposable linked issue, `awit close <id>` sets both local and remote closed; `awit release <id>` sets both open and clears the local claim.
- With broken authentication, close/release still persist locally, exit 0, and emit the specified warning. `awit update <id> --status <current-status>` successfully retries after credentials are repaired.
- `awit close <id> --no-push` works without tea installed and makes no remote request.
- No new commit occurs for close, release, update, or state retry.

## Out of scope

Remote-driven reopen/close, remote content import on transition, implicit pushing from claim, body/title/label writes, queues, retry policies, distributed locking.


## Comments

### 2026-09-19T16:55:43Z agent/orchestrator

implemented
