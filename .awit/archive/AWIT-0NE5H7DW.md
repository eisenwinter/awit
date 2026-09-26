---
id: AWIT-0NE5H7DW
title: Tag v0.1.0 and verify the stamped release build
brief: >-
  Create annotated tag v0.1.0, confirm the goreleaser ldflags stamp, then report for the human to push.
status: closed
deps: [AWIT-0ND5743G]
labels: [phase5, p2]
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

No tags exist, so `Taskfile.dev` stamps the git SHA and the `v*` release workflow never fired. Create annotated tag `v0.1.0` locally, prove `task --taskfile Taskfile.dev build && ./bin/awit --version` prints `awit v0.1.0`, `go test ./...` green. Do NOT push - `git push --tags` is the human's call.

## Acceptance Criteria

- [ ] Tag `v0.1.0` exists locally, tree was clean at tag time
- [ ] Stamped binary prints `awit v0.1.0`

## Comments

### 2026-09-18T06:27:44Z agent/orchestrator

implemented

### 2026-09-18T06:27:44Z agent/orchestrator

Tag v0.1.0 created locally (annotated, on 900cbdd), ./bin/awit --version prints 'awit v0.1.0', go test ./... 10 packages green. NOT pushed; human runs git push --tags.
