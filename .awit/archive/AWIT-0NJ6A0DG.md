---
id: AWIT-0NJ6A0DG
title: 'external: route body checks and local-first state pushes to GitLab'
brief: >-
  Extend existing external checks, explicit body repair, and local-first state delivery to linked GitLab issues. Preserve Gitea output, exit, warning, locking, and offline contracts while using glab's verified body/state operations.
status: closed
deps: [AWIT-0NJ69QDP, AWIT-0NJ69VDN]
labels: [phase5, p1]
refs_base: repo
refs: []
---
## Summary

Route existing check, push-body, and post-save state-push workflows by tracker. Keep local content/state canonical, retain exact output schemas and warnings, and prove mixed-tracker behavior without changing Gitea rows.

## Context (read first)

- Approved GL2/GL3 contracts; `glab-assessment.md` §§D.5, G, J.
- `.awit/archive/AWIT-0NHDBNDS.md`, `AWIT-0NHDBQDT.md`.
- `internal/cli/external.go`: `externalCheckAction`, `checkOne`, `externalPushBodyAction`, `duplicateExternalLinks`.
- `internal/cli/external_state.go:maybePushExternalState`; existing close/release/update trigger points and tests.
- Guide §§2, 4.11–4.13; README Agent loop; schema external body/state contracts.

## Files

- Modify `internal/cli/external.go`, `internal/cli/external_test.go`.
- Modify `internal/cli/external_state.go`, `internal/cli/external_state_test.go`.
- Update tracker-specific help/comments in `internal/cli/close.go`, `release.go`, `update.go`; preserve action ordering.
- Use GL2’s portable glab fixture alongside existing teaxtest. No production teax changes.
- Docs: README Commands/Agent loop; schema external check/body/state/transport paragraphs; guide §2 state push and §4.11 command helpers; spec external/update/close/release matrix rows; embedded skill loop, Quick reference, Common mistakes, and regenerated `.omp` skill.

## Interfaces

Consume GL3’s `getExternalIssue`, `externalBase`, tracker-aware duplicate checks, and GL2’s verified glab operations. Add only two private CLI dispatch helpers:

```go
func setExternalBody(ctx context.Context, ext item.External, teaLogin string, body []byte) error
func setExternalState(ctx context.Context, ext item.External, teaLogin, state string) error

// Preserve:
func checkOne(ctx context.Context, it *item.Item, login string) ExternalCheckRow
func maybePushExternalState(ctx context.Context, cmd *cli.Command, all []*item.Item, it *item.Item, remoteState string)

type ExternalCheckRow struct {
    ID     string `json:"id"`
    URL    string `json:"url,omitempty"`
    Result string `json:"result"`
    Detail string `json:"detail,omitempty"`
}
```

Each helper switches on tracker, calls `teax.Open(ctx, ext, teaLogin)` or `glabx.Open(ctx, ext)`, then invokes the matching operation. GitLab ignores tea-login. Do not add a dynamic registry, shared transport, retry layer, or alter teax return types.

Behavior:

- `checkOne` uses the common read helper. Compare only `Item.Body()` with fetched bytes. GitLab schema/auth/identity failures are ERROR, never DRIFT. All-items check preserves canonical-ID ordering, includes all linked/error rows, and skips only genuinely unlinked items. Keep deterministic totals, pure JSON row arrays, exit 1 on any error/drift and 0 otherwise.
- `externalPushBodyAction` keeps its local snapshot/store lock until bounded delivery verification completes. Reuse tracker-aware duplicate protection. GL2’s unsafe-body check prevents PUT; do not catch that error and report success or alter the body. Local bytes/history remain unchanged on both success and failure.
- The body push changes no title/labels/state; state push changes no body/title/labels. All subgroup encoding and state_event translation stay in glabx, not command handlers.
- Preserve triggers exactly: close→closed; release→open; explicit update closed→closed, open/in_progress→open. Same-status update still pushes. Non-status update, claim, create, import, comment, ref, and archive never push.
- Preserve local save first and held lock. Local-save failure prevents remote writes. Remote failure never rolls back local success, duplicates close reason comments, or changes ordinary confirmation/exit 0. Emit the existing single stderr warning:

  ```text
  warning: <id> saved locally; external state push failed: <reason>; retry with awit update <id> --status <status>
  ```

- `--no-push` returns before metadata validation, either executable lookup, configuration lookup, auth, or network—even for malformed metadata. No new login/config flags; `--tea-login` remains meaningful only for Gitea.
- Keep same-checkout locking guarantees; do not claim coordination across clones or make automatic retries.

## Steps

- [ ] **RED — add `TestExternalCheckGitLab`, `TestExternalCheckMixedTrackers`, and `TestExternalPushBodyGitLab`.** Use temporary repositories with both portable stubs. Cover match/drift/error aggregation, stable ordering/JSON purity/exit status, null/byte-edge bodies, invalid metadata, wrong identity, duplicate-link refusal, harmless cross-tracker equal-iid links, and local byte preservation. Record remote title/labels/state before and after body writes.
- [ ] **Run RED:** `go test ./internal/cli -run 'ExternalCheckGitLab|ExternalCheckMixedTrackers|ExternalPushBodyGitLab' -count=1 -v`; show GitLab rows currently route through tea or fail.
- [ ] **RED — add `TestExternalStateGitLab` cases and `TestExternalPushBodyGitLabQuickActionRefusal`.** Mirror observable D4 cases: all triggers, same-status retry, non-trigger mutations, missing glab/auth/HTTP/schema failures, malformed metadata, no-push with both tools hidden, local-save failure, preserved reason/claim behavior, and warning/exit 0 after remote failure. Assert `/close` body push exits 1, produces no PUT, preserves local/remote bytes and state, and emits safe-format guidance.

  ```go
  code, _, stderr := run(t, "--repo", repo, "update", "GL-STATE", "--status", "closed")
  if code != 0 {
      t.Fatalf("successful local save must remain success: %s", stderr)
  }
  ```

  In the remote-failure subtest also load the item and assert local closed plus the one retry warning; in the successful subtest read the stub’s remote issue and assert wire `closed`.
- [ ] **Run RED:** `go test ./internal/cli -run 'ExternalStateGitLab|ExternalPushBodyGitLabQuickActionRefusal' -count=1 -v`.
- [ ] **GREEN — switch reads to `getExternalIssue`, add the two write-dispatch helpers, and route the existing post-save helper.** Preserve output structs, warning text, trigger points, local-first ordering, and lock lifetime. Keep teax behavior untouched.
- [ ] **Run GREEN:** `go test ./internal/cli ./internal/glabx ./internal/teax -run 'ExternalCheck|ExternalPushBody|ExternalState|Glab|Tea' -count=1 -v`. Run the temporary-directory/disposable-issue CLI scenario below and record exact outcomes.
- [ ] **Finish paired user guidance and regenerate the embedded skill copy.** Explain GitLab auth prerequisite, both URL shapes, subgroup keys, raw file transport without LF adaptation, strict quick-action refusal—including conservative fenced matches—and unchanged no-push/retry behavior. Hand evidence to the orchestrator for commit and final project-wide validation.

## Acceptance Criteria

- Focused GREEN command passes, including existing Gitea tests. The orchestrator separately runs project-wide format/lint/build/tests once after integration; individual work-item workers do not run them.
- Build one candidate binary and use a fresh temporary `$R` and an authorized disposable `$GL_ISSUE_URL`:

  ```sh
  awit --repo "$R" init --prefix AWIT --no-skills
  awit --repo "$R" import "$GL_ISSUE_URL" --brief 'GitLab integration smoke.' --alias GL-SMOKE
  awit --repo "$R" external check GL-SMOKE
  awit --repo "$R" external push-body GL-SMOKE
  awit --repo "$R" external check GL-SMOKE
  awit --repo "$R" close GL-SMOKE
  awit --repo "$R" release GL-SMOKE
  awit --repo "$R" update GL-SMOKE --status in_progress
  awit --repo "$R" update GL-SMOKE --status in_progress
  awit --repo "$R" validate
  ```

  Initial and post-push checks report MATCH. Verify GitLab transitions closed→opened and both explicit in_progress commands deliver reopen; local state remains in_progress. No new commit is created. Inspect the remote through host-scoped glab GET, not a production HTTP client.
- In the temporary repo, make an explicit body-only local edit to create safe drift, preserving frontmatter. Check exits 1/DRIFT; push-body then check produces MATCH. Repeat representative no-LF/CRLF/multiple-LF cases and confirm title/labels/state unchanged. Use `Item.Body()`/decoded JSON for byte comparisons.
- Replace the local body with an unsafe column-zero slash-command example. `awit --repo "$R" external push-body GL-SMOKE` exits 1 with the refusal guidance; both local body and every remote field remain unchanged. Restore the temporary local body explicitly; never auto-rewrite it in production.
- Portable temporary-repo tests demonstrate: mixed tracker rows keep their exact output contracts; broken remote auth still saves close/release locally and emits exactly the retry warning; repairing the scripted auth and `update --status <current>` retries without a second close reason; `--no-push` works with neither binary available and performs no configuration/auth calls.
- `go test ./internal/skill -run 'Render|DogfoodOmpCopyMatchesRenderer' -count=1` passes. The real glab and tea compatibility tests run without skips in release qualification:

  ```sh
  AWIT_TEST_REAL_GLAB=1 go test ./internal/glabx -run '^TestGlabBodyRoundTrip$' -count=1 -v
  AWIT_TEST_REAL_TEA=1 go test ./internal/teax -run '^TestTeaBodyRoundTrip$' -count=1 -v
  ```

## Out of scope

Inbound synchronization, automatic body pushing, title/label synchronization, remote issue creation, MRs, login management, queues/retries, daemon, distributed locking, or any change to Gitea output/auth/LF behavior.



## Comments

### 2026-09-19T19:24:37Z agent/orchestrator

implemented
