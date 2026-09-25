---
id: AWIT-0P1JVJSQ
title: 'lazyawit: cmd/lazyawit main with --repo/--agent, headless tests, Taskfile and goreleaser second binary'
brief: >-
  Adds the human binary: a urfave root with only --repo/--agent that opens the store through internal/ops, shows the fatal screen on failure, and runs the alt-screen Bubble Tea program; Taskfile builds/installs/smokes both binaries and goreleaser ships lazyawit as its own archive per platform.
status: in_progress
deps: [AWIT-0P1JVJSR]
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
claimed_at: "2026-09-25T15:07:33Z"
---
## Summary

`cmd/lazyawit/main.go` runs the TUI directly: no subcommand, flags `--repo` and `--agent` (`AWIT_AGENT` source) plus urfave's `--help`/`--version`. Store-open failures render `lazy.Options{Fatal}` (q exits 0) exactly like `lazyHumanAction`; the walked-up note goes to stderr as one line; positional arguments and bad flags exit 2 with the awit-style "Incorrect usage" message. `--version` prints `lazyawit <ops.Version>`. Release and dev builds gain the second binary with its own ldflag and its own archive (`lazyawit_<version>_<os>_<arch>`). `awit lazy-human` is untouched here (TB-7).

## Context (read first)

- `docs/superpowers/specs/2026-09-25-two-binary-design.md` — Wiring, Release.
- `docs/superpowers/specs/2026-09-25-two-binary-plan.md` §D.3 (full `main.go` sketch — implement it as written), §D.5, §D.6, §D.7, §I (flag parity, VersionPrinter global, separate ldflag vars).
- `internal/cli/lazy.go` (post TB-5) — `lazyHumanAction`: the `tea.NewProgram(m, tea.WithAltScreen(), tea.WithInput(reader), tea.WithOutput(writer), tea.WithContext(ctx)).Run()` line and the `lazy.Options{Fatal: err.Error()}` branch are copied.
- `internal/cli/app.go:37-50` `report` (error → exit code mapping to mirror), `:59-61` `printVersion`, `:110-117` the `--repo` flag definition (copy the usage string verbatim), `:213-215` `usageError` message shape, `:127` `ExitErrHandler` no-op rationale.
- `internal/cli/lazy.go` `lazyHumanCmd` — the `--agent` flag definition (copy usage + `Sources: cli.EnvVars("AWIT_AGENT")` verbatim).
- `internal/cli/lazy_test.go` `TestLazyHumanRegisteredAndQuits` — the goroutine + 10s `select` shape for the headless quit test.
- `internal/lazy/lazy_test.go:20-21` — `TestMain` sets `NO_COLOR=1`; do the same in `cmd/lazyawit/main_test.go`.
- `.goreleaser.yaml` — current single build/archive; `.github/workflows/release.yml` pins goreleaser `~> v2`, so `archives[].ids` (v2.8+) is the right key (`builds:` under archives is deprecated).
- `Taskfile.yml` — `build`, `install`, `smoke`, header comment.
- `go.mod` — charm modules stay direct requirements; `go mod tidy` must be a no-op.

## Files

- `cmd/lazyawit/main.go` — new.
- `cmd/lazyawit/main_test.go` — new (`package main`).
- `.goreleaser.yaml` — second build id + two archives.
- `Taskfile.yml` — both binaries.

## Interfaces

```go
// cmd/lazyawit/main.go
// Command lazyawit is the human TUI over .awit — the former "awit lazy-human"
// as its own binary, so agents holding only awit have no TUI code path.
package main

var _ lazy.Ops = (*ops.Lazy)(nil)

func main() // os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))

// run is main minus os.Exit so tests drive it in-process. It builds a fresh
// root per call (no shared command tree, no mutex), parses --repo/--agent,
// and maps errors like internal/cli.report: an ExitCoder prints its message
// as-is with its code, anything else prints "Error: <err>" and exits 1.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int

// tui is the root Action: refuses positional arguments (exit 2), opens the
// store via ops.Open, prints ops.WalkedUpNote to stderr when non-empty,
// and runs the alt-screen program; an Open error renders the fatal screen.
func tui(ctx context.Context, cmd *cli.Command) error
