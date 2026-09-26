---
id: AWIT-0NHDC5DZ
title: "config: control claim commits with commit and explicit overrides"
brief: >-
  Support commit:false as the repository default for claim commits while keeping explicit per-invocation overrides. Retain --no-commit compatibility but recommend config or --commit=false for new workflows.
status: closed
deps: [AWIT-0ND56A3G, AWIT-0ND56X3G, AWIT-0NFAW5DT]
labels: [phase5, p1]
refs_base: repo
refs: []
---

## Summary

Add optional `.awit/config.yaml` `commit: false`. Default remains true. Only `next --claim` consumes this setting.

## Context (read first)

- `config.Config`, `Default`, `Load`, `Write`.
- `nextAction` currently checks only `!cmd.Bool("no-commit")`.
- Guide §2 decision 3 and §5 flag-reuse warning.

## Files

- Modify `pkg/config/config.go`, config tests; `internal/cli/next.go`, next tests.
- Update schema config, README Agent loop/Commands, guide config/API/claim contracts, spec Claims/next, skill claim guidance.

## Interfaces

```go
// Config.Commit *bool `yaml:"commit,omitempty"` - nil means true.
func (c Config) ShouldCommit() bool
```

- Add `next --commit=true|false`, parsed with a value-based tri-state (empty/unset versus literal true/false) or fresh per-invocation flag object. No `IsSet` on reused flags.
- Precedence: explicit `--commit` or true `--no-commit` > config `commit` > default true.
- Supplying both `--commit=...` and true `--no-commit` is usage error 2 before mutation. Invalid bool values are usage errors. `--no-commit=false` is neutral and does not override config.
- Keep existing `--no-commit` accepted; mark it deprecated in help/docs in favor of config or `--commit=false`, but do not add a runtime deprecation warning. Existing scripts remain quiet and compatible.
- With no `--claim`, policy flags do not cause any write or commit. Config false skips staging as well as committing. Explicit true overrides config false.
- Commit policy never controls external pushing, and does not turn create/update/close/release into committing commands.

## Steps

- [ ] Write failing behavior tests for absent/false/true config, explicit override in both directions, conflicting flags, and repeated Main calls where a previous explicit flag must not leak.
- [ ] Run `go test ./pkg/config ./internal/cli -run 'CommitPolicy|Claim' -count=1`; observe failures.
- [ ] Implement `ShouldCommit` and value-based override resolution; call it only at the existing `gitx.Commit` boundary.
- [ ] Verify actual temporary Git repository history and staged paths, not only calls to a mocked commit helper; config false leaves the claim written but no new commit.
- [ ] Rerun targeted tests; update docs and deliver evidence for orchestrator commit.

## Acceptance Criteria

- Scoped tests above pass.
- In a temporary Git repo with `commit: false`, `awit next --claim --agent test <id>` claims without adding a commit; `awit next --claim --commit=true --agent test <another-ready-id>` commits only that item.
- Default config still commits claims; `--no-commit` and `--commit=false` skip it.
- `awit close`, `awit release`, and remote-push behavior are unaffected by `commit`.

## Out of scope

A global auto-commit mode, changing Git commit messages, config controlling network access, removing --no-commit in this release.

## Comments

### 2026-09-19T15:25:55Z agent/orchestrator

implemented
