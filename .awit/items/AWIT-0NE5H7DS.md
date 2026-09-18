---
id: AWIT-0NE5H7DS
title: Reconcile stale ticket text (yaml pin, golden paths)
brief: >-
  Fix two known-wrong lines in closed tickets: the nonexistent yaml v3.0.4 pin and the repo-root golden path.
status: in_progress
deps: []
labels: [phase5, p2]
refs: []
assignee: agent/orchestrator
claimed_at: "2026-09-18T06:15:43Z"
---

## Summary
Two closed-ticket texts contradict reality (both adjudicated during review; builds green regardless). (1) AWIT-0ND56D3G orders `go get gopkg.in/yaml.v3@v3.0.4`, which does not exist upstream (`go list -m -versions` shows only v3.0.0/v3.0.1) — change to `@v3.0.1`. (2) AWIT-0ND56F3G Files/Steps/Acceptance point at repo-root `testdata/golden/`; goldens live in `pkg/format/testdata/golden/` with package-scoped `goldenPath` — update paths. Ticket text only, no code.
## Acceptance Criteria
- [ ] Only the two ticket files changed
- [ ] `go build ./...`, `go test ./pkg/item ./pkg/format` still pass
