---
id: AWIT-0P1JVJSS
title: 'ops: external — ExternalCheckRow, CheckOne, CheckOneLine, GetExternalIssue, ExternalIssue, ExternalBase'
brief: >-
  Moves the read-only external check and the tracker dispatch helpers into internal/ops, and turns lazyOps.ExternalCheck's MATCH/DRIFT/ERROR formatting into CheckOneLine so the TUI's P key and the CLI's external check share one implementation.
status: closed
deps: [AWIT-0P1JVJSY]
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---
## Summary

`ExternalCheckRow`, `checkOne`, `externalIssue`, `getExternalIssue`, `externalBase` (external.go) move verbatim into `internal/ops/external.go`. The `switch r.Result` block of `lazyOps.ExternalCheck` (lazy.go:188-208) becomes `ops.CheckOneLine(ctx, it, login) string` — the one-line text form the TUI shows. `externalCheckAction`, `externalPushBodyAction`, `setExternalBody`, `setExternalState`, `maybePushExternalState`, `externalPushPolicy`, `duplicateExternalLinks`, `joinIDs`, `failedRows` stay in cli (they write, print, or parse flags). Nothing in `internal/ops` ever pushes.

## Context (read first)

- `docs/superpowers/specs/2026-09-25-two-binary-plan.md` §D.2 external row; spec Interfaces `CheckOneLine`.
- `internal/cli/external.go:26-34` `ExternalCheckRow`; `138-166` `checkOne`; `276-330` `externalIssue`, `externalBase`, `getExternalIssue`. Callers: `external.go:94-96` (`externalCheckAction`), `:251` (`duplicateExternalLinks` → `externalBase`), `import.go:106` (`getExternalIssue`), `:343,399` (`externalBase`).
- `internal/cli/external_test.go:127,503` — `var rows []ExternalCheckRow` → `[]ops.ExternalCheckRow`.
- `internal/cli/lazy.go:188-208` — `ExternalCheck`; after this item it is `it, err := ops.LoadItem(o.s, id); if err != nil { return err.Error() }; return ops.CheckOneLine(ctx, it, "")`.
- `internal/cli/lazy_test.go:205` pins `ERROR AWIT-TEST0001: invalid external: no external link` for an unlinked item.
- Stub trackers: `internal/cli/external_test.go:38` `externalRepo` puts a fake `tea` on PATH; the ops test avoids subprocess stubs by using `t.Setenv("PATH", t.TempDir())` so `teax.Open` fails deterministically ("tea" not found) and the line starts with `ERROR <id> <url>: `.

## Files

- `internal/ops/external.go` — new: `ExternalCheckRow`, `CheckOne`, `CheckOneLine`, `ExternalIssue`, `GetExternalIssue`, `ExternalBase`.
- `internal/ops/external_test.go` (`package ops_test`) — new.
- `internal/cli/external.go`, `import.go`, `lazy.go`, `external_test.go` — callers rewritten; movers deleted.

## Interfaces

```go
// internal/ops/external.go — doc comments verbatim (ExternalCheckRow, CheckOne, ExternalIssue, ExternalBase, GetExternalIssue).
type ExternalCheckRow struct {
	ID     string `json:"id"`
	URL    string `json:"url,omitempty"`
	Result string `json:"result"` // match | drift | error
	Detail string `json:"detail,omitempty"`
}
func CheckOne(ctx context.Context, it *item.Item, login string) ExternalCheckRow
// CheckOneLine is CheckOne rendered as the single text line awit external
// check prints for that row: "MATCH <id> <url>", "DRIFT <id> <url>[: detail]",
// "ERROR <id> <url>: detail" or "ERROR <id>: detail" when there is no URL.
func CheckOneLine(ctx context.Context, it *item.Item, login string) string
type ExternalIssue struct {
	Number int64
	Title  string
	Body   []byte
	Labels []string
	State  string
	URL    string
}
func ExternalBase(ext item.External) (string, error)
func GetExternalIssue(ctx context.Context, ext item.External, teaLogin string) (ExternalIssue, error)

## Comments

### 2026-09-25T15:01:06Z jan

implemented
