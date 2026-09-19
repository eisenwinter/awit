---
id: AWIT-0NHDBSDQ
title: 'cli: warn once about quarantined items on graph-reading commands'
brief: >-
  Make graph exclusions visible with one stable stderr warning per command without changing results, exit codes, or machine-readable stdout.
status: closed
deps: [AWIT-0ND56S3G, AWIT-0ND56J3G, AWIT-0ND56W3G, AWIT-0ND56X3G, AWIT-0ND56R3G, AWIT-0NE610DS]
labels: [phase5, p1]
refs_base: repo
refs: []
---
## Summary

Whenever a command successfully loads a graph containing quarantined items or broken item files, print exactly:

```text
warning: N items quarantined, run awit validate
```

Use the same text for N=1. Warn before filtering; hidden quarantined items still count.

## Context (read first)

- `loadGraph` in `validate.go`; `Graph.Quarantined`, `Graph.Broken`.
- `depAdd`, `depRm`, `printCompact`: two loads must not mean two warnings.
- `list` already has a separate workflow footer on stderr.

## Files

- Modify `internal/cli/{app,list,next,prime,show,validate,dep,archive}.go` and focused tests.
- Update README quarantine/exit-code guidance, guide CLI contracts, spec Quarantine, and embedded skill warnings.

## Interfaces

```go
func warnQuarantined(cmd *cli.Command, g *graph.Graph)
```

Count `len(g.Quarantined()) + len(g.Broken)`, i.e. quarantined parseable items plus broken files, **not number of fault records**. A cycle with several members counts those members; multiple reasons on one node count it once.

Call once at the command boundary after its initial load: `list`, both `next` forms, `prime`, all `show` forms, `validate`, `dep add`, `dep rm`, `archive` and `archive --dry-run`. Do not emit from `printCompact`’s post-write reload. New commands that read a graph must follow this same boundary rule.

`label` currently loads items without building a graph; leave its behavior unchanged. Create/update/close/release/comment do not gain graph scans merely to warn. Help/init never warn.

## Steps

- [ ] Add failing command tests covering parse-error-only repositories, cyclic nodes with multiple faults, filtered-out quarantines, no-ready `next`, JSON output, clean repositories, and dep’s double load.
- [ ] Run `go test ./internal/cli -run 'QuarantineWarning' -count=1`; see the missing warnings fail.
- [ ] Implement the helper and explicit initial-load callsites without changing the guide’s `loadGraph` signature or introducing global warning state.
- [ ] Verify stdout/golden content remains unchanged; adjust only expected stderr where needed. Keep the list footer as a separate existing line after its normal output.
- [ ] Rerun targeted tests and relevant existing command tests; update docs and deliver evidence for orchestrator commit.

## Acceptance Criteria

- `go test ./internal/cli -run 'QuarantineWarning|Validate|List|Next|Prime|Show|Dep|Archive' -count=1` passes.
- `awit --repo testdata/fixtures/parse-error list --format json` has parseable unchanged JSON stdout and exactly one quarantine warning on stderr; exit remains 0.
- `awit --repo testdata/fixtures/cyclic prime` retains its GRAPH WARNINGS section and additionally emits one stderr summary.
- `validate` keeps its current FAIL exit; warning-free clean commands keep their current stdout/stderr.

## Out of scope

Changing quarantine classification, exposing broken files as synthetic list rows, altering exit codes, adding graph work to commands that do not read one.


## Comments

### 2026-09-19T15:38:16Z agent/orchestrator

implemented
