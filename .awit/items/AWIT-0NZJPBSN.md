---
id: AWIT-0NZJPBSN
title: 'lazy: mutations — c/b/m prompts, u, toast, reload, quarantine footer, V, P (async)'
brief: >-
  Adds the close/block/comment prompts, immediate unblock, read-only archive refusal, external check as an async tea.Cmd with a toast, and pins the quarantine footer and reload-resort behaviour, completing the in-TUI mutation surface over Ops.
status: closed
deps: [AWIT-0NZJPBSP, AWIT-0NZJPBSZ, AWIT-0NZJPBSQ]
labels: [tui, p1]
refs_base: repo
refs: [.awit/comments/AWIT-0NZJPBSN/20260924T213300Z-jan.md]
assignee: agent/orchestrator
---
## Summary

Wire the remaining keys on every tab that has a `selectedID()`: `c` (close, optional reason), `b` (block, reason required), `m` (comment, text required) open a one-line `textinput` prompt in `modeInput`; `enter` submits through `Ops` via `act`, `esc` cancels. `u` unblocks immediately. `P` runs `Ops.ExternalCheck` inside a `tea.Cmd` and the resulting `externalMsg` becomes a toast. Archive rows and no-selection cases refuse with toasts. Reload after each mutation re-sorts the Queue; the quarantine footer reflects the snapshot.

## Context (read first)

- Design spec "Keymap", "Error handling"; plan §D.12, §D.15.
- `internal/cli/block.go:44-46` — empty reason is a usage error (`block requires --reason with a non-empty, single-line explanation`); the TUI refuses before calling `Ops` with the shorter toast `block requires a non-empty reason`. Multi-line/control-character reasons cannot be typed in a single-line input; `SetBlockedReason` errors still surface as toasts.
- `internal/cli/comment.go` — empty text is `empty comment`.
- `internal/cli/external.go:109-131` — the `MATCH`/`DRIFT`/`ERROR` line shapes `Ops.ExternalCheck` returns.
- WI-4 `act`, `toast`, `reload`; WI-5 `selectedID` rules (archive → ""); WI-6/7 tabs.

## Files

- `internal/lazy/mutations.go` — `openInput`, `submitInput`, `externalMsg`, `externalCmd`, key handling for `c/b/u/m/P`.
- `internal/lazy/model.go`, `view.go` — `modeInput` dispatch, prompt labels, `externalMsg` handling, archive-read-only toast.
- `internal/lazy/mutations_test.go`; goldens `prompt_close`, `prompt_block`, `prompt_comment`.

## Interfaces

```go
type externalMsg struct{ line string }
func (m *Model) openInput(kind inputKind, id string)   // mode=modeInput, inputTarget=id, input reset+focused; labels: "close reason (optional):", "block reason:", "comment:"
func (m *Model) submitInput()                          // inputClose → act(Close(id, v), "closed "+id); inputBlock → v=="" ? toast : act(Block, "blocked <id>: <v>"); inputComment → v=="" ? toast "empty comment" : act(Comment, "commented "+id)
func (m *Model) externalCmd(id string) tea.Cmd         // func() tea.Msg { return externalMsg{m.ops.ExternalCheck(m.ctx, id)} }
