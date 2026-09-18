---
id: AWIT-0NE5H7DZ
title: 'dev.md: ban running the binary against the repo root'
brief: >-
  Add a never-use-repo-root rule to dev.md guards after test pollution wrote junk items into .awit/items.
status: closed
deps: []
labels: [phase5, p1]
refs: [../comments/AWIT-0NE5H7DZ/20260918T061540Z-orchestrator.md]
assignee: agent/orchestrator
---

## Summary
During AWIT-0ND5723G a manual concurrent-create reproduction ran against the repo root, landing 11 `AWIT-0NDD*` junk items plus a stale `.awit/.lock` in the real queue (removed uncommitted). Add a guard to every dev agent file so it cannot recur. Rule: never run the built binary or `go run` with the repo root as `--repo`/cwd; smoke tests MUST use a temp dir; before reporting, `git status` must show no `AWIT-*` files under `.awit/items` and no `.awit/.lock`.
## Acceptance Criteria
- [ ] Rule present in `dev.md`, `dev-grok.md`, `dev-kimi.md`, `dev-muse.md`, `dev-glm.md`
- [ ] `git status` shows only the five intended files
