---
id: AWIT-0NJC0BDV
title: 'config: control automatic external-state pushes with a repository default'
brief: >-
  Allow repositories to disable automatic linked-issue state pushes without changing local status mutations. Preserve push-by-default behavior and provide explicit per-command overrides with visible configuration-based skips.
status: closed
deps: []
labels: [phase5, p1]
refs_base: repo
refs: []
---
## Summary

Add the optional top-level key `external_push` in `.awit/config.yaml`. An omitted key means `true`; `external_push: false` disables automatic external-state propagation from `close`, `release`, and explicit `update --status` equally. It is independent of `commit`, and does not govern explicitly requested `external push-body`, import reads, or `external check`.

Add `--push=true|false` to those three status commands. Keep `--no-push` supported and silent; do not deprecate it in this work item. Effective precedence is explicit `--push` or true `--no-push` > configuration > default true.

## Context (read first)

- `AGENTS.md`; guide §§1, 2 (`external state push`, claim commit policy), 4.2, 4.11 and 5.
- `.awit/archive/AWIT-0NHDBQDT.md` and `.awit/archive/AWIT-0NHDC5DZ.md`; both are closed. Their historical exclusions of configuration-driven pushing are superseded by this work item, not retroactively edited.
- `internal/cli/external_state.go`: `maybePushExternalState`, `setExternalState`.
- `internal/cli/{close,release,update}.go`: all mutation and push callsites, including close with a reason.
- `internal/cli/next.go`: `commitPolicy` and value-based flag detection.
- `pkg/config/config.go`: `Config`, `Load`, `Write`, `ShouldCommit`.
- Existing tea/glab state tests in `internal/cli/external_state_test.go`.

## Files

- Modify `pkg/config/config.go` and `pkg/config/config_test.go`.
- Modify `internal/cli/external_state.go`, `close.go`, `release.go`, `update.go`, and `external_state_test.go`.
- Update `plan/implementation-guide.md` §§2/4.2/4.11, `docs/schema.md` configuration and external-state prose, `plan/awit-implementation-plan.md` CLI command matrix, `README.md` Commands/Agent loop, and `internal/skill/assets/driving-awit.body.md` transition/offline/retry instructions.
- Regenerate `.omp/skills/driving-awit/SKILL.md` from the skill renderer; do not hand-edit generated content.

## Interfaces

Add to `config.Config`:

```go
ExternalPush *bool `yaml:"external_push,omitempty"`
```

Add the config-only method and one CLI policy helper:

```go
func (c Config) ShouldPushExternal() bool
func externalPushPolicy(cmd *cli.Command, cfg config.Config) (bool, error)
```

`ShouldPushExternal` returns `c.ExternalPush == nil || *c.ExternalPush`. `Default` need not populate the pointer. `Load`/`Write` use existing yaml.v3 and atomic-write behavior; an explicit false must survive round-trip, and omission must remain omission. Do not add a second config file, alias key, or environment variable.

Extend the existing shared push helper, updating its guide declaration and every caller:

```go
func maybePushExternalState(
    ctx context.Context, cmd *cli.Command, all []*item.Item,
    it *item.Item, remoteState string, push bool,
)
```

Policy details:

1. Use `cli.StringFlag{Name: "push"}` with empty value meaning unset, mirroring `commitPolicy`; parse nonempty values with `strconv.ParseBool`. Document `true|false`; accept its existing standard boolean spellings consistently with `--commit`. Do not use `cmd.IsSet`.
2. `--no-push=false` is neutral. `--push=` is unset, consistent with the existing empty-string policy convention.
3. A nonempty `--push` combined with true `--no-push` is usage error 2 before any mutation: `Error: --push and --no-push cannot be combined`.
4. Invalid nonempty boolean input is usage error 2 before any mutation: `Error: invalid --push value "<value>": use --push=true or --push=false`, formatting the value with `%q`.
5. Resolve policy after `openStore` and before any setter, `AddComment`, or `Save`. Parse invalid combinations even for a non-status update; a valid `--push=true` never turns such an update into a push trigger.
6. Continue to call the shared push helper only after successful local persistence, while holding the existing lock. Preserve same-status retry behavior and the existing status mapping.

Output and skip behavior:

- An item with neither `External` nor `ExternalProblem` is silent: there is no remote operation to skip.
- A linked or malformed-linked item whose push is disabled **by config**, without an explicit disabling flag, emits exactly one stderr line after successful local persistence:

```text
warning: <id> saved locally; external state push skipped by config external_push: false; push with awit update <id> --status <status> --push=true
```

- A push disabled explicitly by true `--no-push` or false `--push` is silent, preserving existing `--no-push` behavior. Explicit disabling remains silent even if the config is also false.
- Apply the skip before duplicate-link validation, tool discovery, authentication, or network access. Configuration-disabled malformed metadata must not emit a second failure warning.
- Enabled pushes retain the existing remote-failure warning verbatim, ordinary stdout confirmation, and exit 0 after successful local work. Local failures remain failures and never push. Existing successful command exit codes do not change; only misuse of the newly added flags introduces new usage-error cases.
- `close` keeps its reason/comment and claim handling; `release` keeps clearing assignee/claim; `update` keeps its mutation semantics. No new commits, retries, or network operations are introduced.

## Steps

- [ ] Add config round-trip/behavior tests and `TestExternalPushPolicy...` command tests. Exercise absent/true/false config, both explicit override directions, true `--no-push`, neutral `--no-push=false`, conflicts, invalid values, and repeated `Main` calls without leaked flags. Use existing subprocess fixtures to assert remote issue state/request logs and persisted local state, not mocked policy forwarding.
- [ ] Add skip tests for each actual trigger (`close`, `release`, status update), config-only stderr versus explicit silence, malformed/ambiguous metadata under disabled policy, same-status explicit override retry, and non-status update never pushing. Cover Gitea and GitLab at the observable transport boundary. Preserve close reason/comment behavior and no-push-on-local-save-failure regressions.

Representative failing behavior to add using existing helpers:

```go
func TestExternalPushPolicyConfigFalseKeepsCloseLocal(t *testing.T) {
    repo, stub := externalRepo(t)
    writeDefaultLabels(t, repo, []byte("prefix: AWIT\nstale_claim: 2h\nexternal_push: false\n"))
    writeExternalItem(t, repo, "AWIT-TEST0001", giteaExt("owner/repo", 127), []byte("body\n"))
    writeTeaIssue(t, stub, 127, `{"number":127,"title":"T","state":"open","body":"body\n"}`)
    code, _, stderr := run(t, "--repo", repo, "close", "AWIT-TEST0001", "--tea-login", "sandbox")
    if code != 0 || readItem(t, repo, "AWIT-TEST0001").Status != item.StatusClosed {
        t.Fatalf("local close failed: exit %d, stderr %q", code, stderr)
    }
    if len(stubArgvLog(t, stub)) != 0 {
        t.Fatal("config-disabled close executed tea")
    }
    if !strings.Contains(stderr, "skipped by config external_push: false") {
        t.Fatalf("missing config skip diagnostic: %q", stderr)
    }
}
```

- [ ] Run `go test ./pkg/config ./internal/cli -run 'ExternalPush|ExternalState|CommitPolicy|Claim' -count=1`; record the new config-disable/override failures before implementation.
- [ ] Implement `Config.ExternalPush`, `ShouldPushExternal`, and `externalPushPolicy` using the existing commit-policy pattern. Add the flags to all three commands, resolve before local mutation, pass the resolved bool to every `maybePushExternalState` call, and handle config-only skip reporting there before external validation.
- [ ] Run the same scoped command to green. Exercise the built CLI in a temporary repository with the subprocess fixtures: disable by config, change local status, inspect zero transport invocations, then explicitly enable the same-status update and inspect the remote state transition.
- [ ] Update the listed docs and shared API declarations; regenerate the skill copy and run `go test ./internal/skill -run TestDogfoodOmpCopyMatchesRenderer -count=1`. Submit scoped test/smoke evidence and the change for orchestrator review/commit. Do not run project-wide validation.

## Acceptance Criteria

- `go test ./pkg/config ./internal/cli -run 'ExternalPush|ExternalState|CommitPolicy|Claim' -count=1` passes, including repeated in-process invocation and both tracker paths.
- In a disposable linked repository with `external_push: false`, `awit update <id> --status closed` exits 0, saves local `closed`, leaves the remote unchanged, and prints the specified config skip warning. `close` and `release` follow the same policy and retain their existing local effects.
- `awit update <id> --status closed --push=true` overrides that config and delivers the remote state, including when local status was already closed. `--push=false` and `--no-push` skip silently and execute neither tea nor glab.
- Omitted or true config preserves existing automatic pushing. `--no-push=false` does not override false config. Unlinked items and non-status updates generate no skip warning or remote call.
- Invalid/conflicting new flags exit 2 before any item/comment write. Local write failures never trigger transport; remote failures retain local changes and exit 0.
- `commit: false` does not disable state pushing, `external_push: false` does not alter claim commits, and explicitly requested `external push-body` is unchanged.
- Guide, schema, spec, README and source/generated skill agree; renderer parity test passes. No production issue is changed for verification.

## Out of scope

Changing the global default to false; generic network-offline mode; suppressing explicit body pushes or remote reads; new environment controls; queues/retries/daemons; changing local transitions, exit codes of existing valid invocations, or claim commit policy; editing archived work-item history.

---



## Comments

### 2026-09-19T20:19:51Z agent/orchestrator

implemented
