---
id: AWIT-0NZJPBSG
title: 'cli: lazy-human command, lazyOps, parity tests, docs'
brief: >-
  Registers awit lazy-human, implements lazy.Ops over the store with the extracted CLI primitives so every row, detail and byte matches the CLI, proves parity against list/show/prime/next on the fixtures, and documents the command.
status: closed
deps: [AWIT-0NZJPBSK, AWIT-0NZJPBSH, AWIT-0NZJPBSN]
labels: [tui, p1]
refs_base: repo
refs: [.awit/comments/AWIT-0NZJPBSG/20260924T215213Z-jan.md]
assignee: agent/orchestrator
---
## Summary

The last piece: `internal/cli/lazy.go` defines the `lazy-human` command (`--agent` flag, `AWIT_AGENT` source), boots `tea.Program` with the root writer/reader, and implements `lazy.Ops` as `lazyOps{s, agent, now}` using `loadGraph`, `toEntry`/`format.Line`, `showFull`, `validateText`, `checkOne`, `loadItem`, `refuseClaim`, `resolveAuthor`, `s.Config.Agent` and the WI-3 primitives under `s.Lock(5s)`. Tests compare against the CLI on `testdata/fixtures/clean` and `archive`: filter parity vs `list`, detail vs `show --full`, overview vs `prime`, queue vs `prime` READY, and on-disk bytes vs `close --no-push`, `block`, `unblock`, `release --no-push`, `next --claim --commit=false`. Docs gain the command.

## Context (read first)

- Design spec "Architecture", "Error handling", "Acceptance"; plan §D.2, §D.14, §F, §G.
- `internal/cli/app.go:96-152` — register `lazyHumanCmd` after `nextCmd`; `openStore` precedence.
- `internal/cli/next.go:181-185,261-287` — identity error string `no agent identity; pass --agent or set AWIT_AGENT`; `refuseClaim`.
- `internal/cli/close.go:52-59` — author only when a reason is given.
- `internal/cli/external.go:145-166` — `checkOne(ctx, it, login) ExternalCheckRow`; the text line shapes at 109-131.
- `internal/cli/helpers_test.go` — `run`, `copyFixture`, `readItem`; `pkg/item` `Store.Comments`.
- Bubble Tea v1: `tea.NewProgram(model, tea.WithInput(io.Reader), tea.WithOutput(io.Writer), tea.WithAltScreen(), tea.WithContext(ctx)).Run()` returns `(tea.Model, error)`.

## Files

- `internal/cli/lazy.go` — command, action, `lazyOps`.
- `internal/cli/app.go` — one line in `Commands`.
- `internal/cli/lazy_test.go` — parity, bytes, refusals, registration, quit smoke.
- `docs/design-spec.md` — §5 row after `next`; §7 map line for `internal/lazy/`; §7 closing sentence.
- `README.md` — `## Commands` row after `awit next`.
- `docs/usage.md` — `## Browsing interactively` section before `## Command reference`; reference row.

## Interfaces

```go
var lazyHumanCmd = &cli.Command{
	Name:  "lazy-human",
	Usage: "Browse and triage items in a keyboard-driven TUI (tabs: issues, graph, queue)",
	Flags: []cli.Flag{&cli.StringFlag{Name: "agent", Usage: "identity for claims (default: $AWIT_AGENT, then config agent_id)", Sources: cli.EnvVars("AWIT_AGENT")}},
	Action: lazyHumanAction,
}
func lazyHumanAction(ctx context.Context, cmd *cli.Command) error

type lazyOps struct {
	s     *item.Store
	agent string
	now   func() time.Time
}
// Method set = lazy.Ops. Load = loadGraph(s). LoadArchive drops Broken.
// Line = format.Line(toEntry(n)). ArchiveLine = format.Line(archiveEntry(it)) with State "closed", Unblocks 0.
// Detail = showFull(s, g, g.Nodes[id]) ("unknown item <id>\n" when absent). ArchiveDetail = os.ReadFile(s.ArchivePath(id)).
// Mutations: rel, err := s.Lock(5*time.Second); defer rel(); it, err := loadItem(s, id); primitive; return err.
// Claim: also loadGraph + refuseClaim(g.Nodes[id]); agent := s.Config.Agent(o.agent); "" → the next error string; claimItem(s, it, agent, o.now()).
// Close: author resolved via resolveAuthor("", s.Root, s.Config) only when reason != ""; closeItem(s, it, reason, author, o.now().UTC()).
// Comment: strings.TrimSpace(text)=="" → errors.New("empty comment"); resolveAuthor; s.AddComment(it, author, o.now().UTC(), text).
// Validate = validateText(g). ExternalCheck = one line formatted exactly like externalCheckAction prints checkOne(ctx, it, "").
