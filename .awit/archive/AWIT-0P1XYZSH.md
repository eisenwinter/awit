---
id: AWIT-0P1XYZSH
title: "docs+usage: Config tab in README, usage, design-spec, schema, lazyawit Usage and package docs"
brief: >-
  Updates every three-tab mention and key table for the fourth tab and the e key, adds the schema.md note that lazyawit saves are validated full rewrites, changes the lazyawit root Usage and internal/lazy package doc, and records the manual smoke; no behaviour change, no golden change.
status: closed
deps: [AWIT-0P1XYZSK]
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

Doc-drift closure for the Config tab. Every place that lists the tabs or keys gains `Config` / `4` / `e`; `docs/schema.md` states that `lazyawit` saves go through the same `Load`/`Write` rules and rewrite the file; the `lazyawit` root `Usage` and the `internal/lazy` package doc list the fourth tab. Finishes with the full-suite gate the orchestrator runs.

## Context (read first)

- `docs/superpowers/specs/2026-09-26-config-tab-plan.md` §F (table of edits, exact sentences), §G (smoke).
- `README.md:88-96` - `## lazyawit (human TUI)`.
- `docs/usage.md:162-192` - `## Browsing interactively` paragraph and key table (`1/2/3` row 176, `enter` row 179, `R` row 188).
- `docs/design-spec.md:197` - `lazyawit` paragraph; `:269` - `internal/lazy/` map row (still true; leave).
- `docs/schema.md:60-62` - `## config.yaml` closing paragraph.
- `cmd/lazyawit/main.go:31` - root `Usage`; `internal/lazy/ops.go:1-9` - package doc.
- `site/` is generated - do not touch. No CHANGELOG exists.

## Files

- `README.md`, `docs/usage.md`, `docs/design-spec.md`, `docs/schema.md` - text edits below.
- `cmd/lazyawit/main.go` - `Usage` string.
- `internal/lazy/ops.go` - package doc.

## Interfaces

Exact text:

- `README.md` lazyawit section, first sentence list: `Issues (list + detail), Graph (prime overview / focused DAG), Queue (ready order), Config (view and edit .awit/config.yaml with the same validation awit applies on load); c/b/u/m close/block/unblock/comment, space/r claim/release, e edit a config value, V validate, P external check; no mouse.` (keep the existing backtick styling per token).
- `docs/usage.md` paragraph, appended sentence: "The Config tab lists every `config.yaml` key with its rule, default and current value; `e`/`enter` edits one value, which is checked with the rules `awit` applies when loading the file (plus the `prefix` grammar) and then written atomically; `R` re-reads the file."
- `docs/usage.md` table: `| `1/2/3/4` | Switch tabs (Issues / Graph / Queue / Config) |`; after the `enter` row: `| `e`| Edit the selected config value (Config;`enter` also edits) |`; `R` row: `| `R` | Reload items and config from disk |`.
- `docs/design-spec.md` line 197: `(tabs: issues/graph/queue/config)` and append: "The Config tab edits `.awit/config.yaml` through `pkg/config` `Load`/`Write` with the same validation and the `init` prefix grammar; a save rewrites the file under the store lock."
- `docs/schema.md` after line 62 (same paragraph): "`lazyawit`'s Config tab edits these keys through the same `Load`/`Write` pair: a value is checked with the rules above plus the `prefix` grammar before the file is rewritten, so every save is a full rewrite (comments and unknown keys are dropped, as after any `Write`)."
- `cmd/lazyawit/main.go`: `Usage: "Browse and triage .awit items in a keyboard-driven TUI (tabs: issues, graph, queue, config)"`.
- `internal/lazy/ops.go` package doc: `// Bubble Tea program (tabs: issues, graph, queue, config) for browsing open items` / `// and archive, inspecting details, triaging the ready queue and editing` / `// .awit/config.yaml.`

## Steps

- [ ] Apply the six edits above verbatim.
- [ ] `go build ./... && go test ./cmd/lazyawit ./internal/lazy` - PASS (no test pins the Usage string; goldens unchanged).
- [ ] `grep -rn "issues, graph, queue)" cmd internal docs README.md` prints nothing; `grep -rn "issues/graph/queue)" docs README.md` prints nothing outside `docs/superpowers/specs/`.
- [ ] Manual smoke, only if a terminal is available (report "skipped: no TTY" otherwise): `d=$(mktemp -d) && cp -r testdata/fixtures/clean/.awit "$d"/ && go run ./cmd/lazyawit --repo "$d"`; press `4 j j j e`, backspace twice, type `90m`, `enter` (toast `saved stale_claim`, row `stale_claim    90m`), `R`, `q`; then `cat "$d/.awit/config.yaml"` shows `prefix: AWIT` and `stale_claim: 90m`. Report what was seen.
- [ ] Orchestrator gate (once, after this item): `go build ./... && go vet ./... && go test ./... -race`, `gofmt -l .` empty, `go mod tidy` no-op, `go list -deps ./cmd/awit | grep -E 'charmbracelet|internal/lazy'` empty.

## Acceptance Criteria

- README, `docs/usage.md`, `docs/design-spec.md` and `docs/schema.md` describe the fourth tab, the `e` key and the validated full-rewrite save exactly as above; `site/` untouched.
- `lazyawit --help` shows `(tabs: issues, graph, queue, config)`.
- Full suite green with `-race`; no golden change in this item; `cmd/awit` dependency graph still free of TUI code.

## Out of scope

- Any behaviour change; historical specs under `docs/superpowers/specs/` stay as written.

## Comments

### 2026-09-25T18:26:24Z jan

implemented
