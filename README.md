# awit

Agent work item tool: a zero-daemon Go CLI that turns Markdown files
under `.awit/` into a dependency graph for humans and agents. Offline,
versioned in Git, no database, no daemon. Every command rebuilds the
graph from `.awit/items/*.md`; a clone is the whole state.

## Install

From source (Go 1.27+):

```bash
go install github.com/eisenwinter/awit/cmd/awit@latest
```

Or download a tagged binary from GitHub Releases. Each tag `v*` publishes
linux, darwin and windows builds for amd64 and arm64, all with
`CGO_ENABLED=0`.

Verify the embed:

```bash
awit --version
```

A release build prints `awit <version>`. A `go build` without ldflags
prints `awit dev`.

## Commands

| Command | Flags | Purpose |
| --- | --- | --- |
| `awit init` | `--prefix`, `--skills`, `--no-skills`, `--force` | Create `.awit/`, `config.yaml`, gitignore `.awit/.lock`; offer to seed the driving-awit skill |
| `awit create <title>` | `--brief`, `-d` deps, `-l` labels, `--assign`, `--alias`, `--id`, `--external-tracker`, `--external-repo`, `--external-id`, `--external-url` | Mint a snowflake ID, write a lean item; optional Gitea or GitLab mapping; optional `config.template` body |
| `awit import <issue-url>` | `--brief`, `--alias`, `--tea-login` | One-time snapshot of an existing Gitea issue via `tea`: keeps the issue number, exact body, labels and open/closed state; refuses duplicates |
| `awit external check [key]` | `--tea-login` | Read-only byte-exact body comparison for linked items; `MATCH`/`DRIFT`/`ERROR` rows plus totals; exit 1 on any drift or error |
| `awit external push-body <key>` | `--tea-login` | Explicit repair: push local body bytes to the linked Gitea issue through `tea`; refuses ambiguous links; verifies the remote took the exact bytes |
| `awit list [key]` | `-s` status, `-l` label, `--ready`, `--blocked`, `--quarantined`, `--format` | Index view; `[key]` selects exactly one item |
| `awit label` | `--state open\|closed\|all`, `--format` | Observed label usage counts (not the config vocabulary) |
| `awit show <id>` | `--full`, `--refs-only` | Core item or full resolved ref tree |
| `awit comment <id> [text]` | `--file <path>`, `--author` | Timestamped comment or attached file; append to `refs` |
| `awit update <id>` | `--status`, `--brief`, `--assign`, `--label`, `--unlabel`, `--title`, `--alias`, `--clear-alias`, `--external-tracker`, `--external-repo`, `--external-id`, `--external-url`, `--clear-external`, `--no-push`, `--tea-login` | Mutate frontmatter with a minimal diff; an explicit `--status` also pushes the mapped state (`closed`→closed, `open`/`in_progress`→open) to the linked Gitea issue, local-first with a stderr retry warning on remote failure |
| `awit close <id>` | `--reason`, `--author`, `--no-push`, `--tea-login` | Set `closed`, clear `claimed_at`; does not git-commit; pushes `closed` to the linked Gitea issue unless `--no-push` |
| `awit release <id>` | `--no-push`, `--tea-login` | Reopen an in-progress or closed item as `open`, clear `assignee` and `claimed_at`; prints `reopened <id>` (plain line, ignores `--format`); pushes `open` to the linked Gitea issue unless `--no-push` |
| `awit dep add\|rm <id> <dep>` | — | Edit `deps` with cycle pre-check |
| `awit ref add\|rm <id> <path>` | — | Add or remove a repo-root-relative file reference; does not copy, delete, or commit |
| `awit validate` | `--stale-claims` | Integrity report; non-zero exit on `FAIL`; invalid `external` is a WARN |
| `awit prime` | `--max-tokens`, `-l` label | Deterministic state graph for prompt injection; `--max-tokens` is a soft budget that never sheds warnings or the top ready row |
| `awit next` | `-l` label, `--claim`, `--commit=true\|false`, `--no-commit` (deprecated), `--seed`, `--why` | Top unblocked item; optional claim; `--why` explains the pick on stderr |

Global flags: `--format compact|table|json`, `--repo <path>` (directory that
contains `.awit/`; `$AWIT_REPO` when the flag is unset, else walk up from the
working directory), `--no-color` (accepted, no-op). A mutating command run
from a subdirectory prints a one-line note on stderr naming the root it
walked up to. Exit codes: `0` success, `1` expected non-success (`next`
with no candidates, `validate` with FAIL, `external check` with any drift
or error), `2` usage error.

When a graph-reading command (`list`, `next`, `prime`, `show`, `validate`,
`dep`, `archive`) loads quarantined items or broken files, it prints one
stderr line first: `warning: N items quarantined, run awit validate`.
Stdout (including `--format json`) and exit codes are unchanged — run
`awit validate` for the fault details and their `fix:` commands.

## Agent loop

Each step is one process. State between steps lives only in files and Git.

```text
awit prime                 # token-light snapshot of the graph
awit next --claim          # claim the top unblocked item
awit show <id> --full      # item + resolved refs
awit comment <id> "..."    # research notes
awit close <id>            # unblocks downstream
```

Then `prime` again. `--claim` writes `status: in_progress`, `assignee`,
`claimed_at`, and commits `awit: claim <id>` following the commit policy:
`--commit=true|false` overrides one invocation (default `true`), a
`commit: false` line in `.awit/config.yaml` sets the repository default,
and `--no-commit` is the deprecated spelling of `--commit=false`.
`close` is a plain file write; the human or agent commits when done.
For linked items `close` pushes `closed` (and `release` pushes `open`) to
the Gitea issue through `tea` after the local save, under the store lock;
an explicit `update --status` pushes the mapped state the same way, and a
non-status update never pushes. Local state stays canonical: a remote
failure keeps the local mutation, prints
`warning: <id> saved locally; external state push failed: <reason>; retry with awit update <id> --status <status>`
on stderr, and still exits 0. Pass `--no-push` for an explicit offline path
(no tea discovery, auth, or network, even with malformed metadata), or
`--tea-login` to choose the tea login.
Filter with `-l p0` (AND across repeated flags, OR inside one comma list).

## Status

v1 targets tagged binaries for linux, darwin and windows (amd64 and
arm64) via GoReleaser. CI runs `go vet`, `staticcheck` and `go test ./...`
on ubuntu-latest and windows-latest. This repository dogfoods itself:
implementation work items live in [`.awit/items/`](.awit/items/).

## Pre-commit

`awit validate` is the intended pre-commit hook. A copy lives at
[`docs/hooks/pre-commit`](docs/hooks/pre-commit):

```sh
#!/bin/sh
exec awit validate
```

Install it with:

```bash
cp docs/hooks/pre-commit .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

The hook exits non-zero on any `FAIL` (cycles, dangling deps, conflict
markers, parse errors, duplicate ids, id mismatches), which blocks the
commit.

## Layout

```text
.awit/
├── config.yaml            # prefix, default_labels, labels, stale_claim, agent_id, optional template
├── items/PREFIX-XXXXXXXX.md
└── comments/PREFIX-XXXXXXXX/<UTC seconds>-<author>.md
```

An item is YAML frontmatter (`id`, `title`, `brief`, `status`, `deps`,
`labels`, `assignee`, `claimed_at`, `refs_base`, `refs`, optional `alias`,
optional `external`) followed by a Markdown body.
Priority is a label by convention (`p0`…`p4`). Optional `config.yaml`
`labels` is an advisory vocabulary: `create`/`update` warn on unknown
names they introduce but still store them. Blocked is derived, never
stored. The on-disk schema, including the optional `alias` and the Gitea or
GitLab `external:` mapping, is documented in [docs/schema.md](docs/schema.md).

Wherever a command takes an item — `show`, `list`, `next`, `update`,
`close`, `release`, `comment`, `dep`, `ref`, `external check`,
`external push-body` — the argument may be the
canonical ID (`AWIT-XXXXXXXX`, exact, always wins), a case-insensitive
`alias`, or an external key `owner/repo#127` / `#127` (bare numbers must be
unique). Ambiguous keys are refused with the matching canonical IDs.

## Contributing

Read [plan/implementation-guide.md](plan/implementation-guide.md) first.
It holds the resolved design decisions, the package layout, every shared
Go interface, and the work item index with dependency order. Then pick a
work item from `.awit/items/` whose `deps` are all closed.

## License

BSD 2-Clause. See [LICENSE](LICENSE).
