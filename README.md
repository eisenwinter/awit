# awit

Zero-daemon Go CLI that turns Markdown files under `.awit/` into a
dependency graph for humans and agents. Offline, versioned in Git, no
database, no daemon. Every command rebuilds the graph from
`.awit/items/*.md`; a clone is the whole state.

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
| `awit init` | `--prefix` | Create `.awit/`, `config.yaml`, gitignore `.awit/.lock` |
| `awit create <title>` | `--brief`, `-d` deps, `-l` labels, `--assign`, `--id` | Mint a snowflake ID, write a lean item |
| `awit list` | `-s` status, `-l` label, `--ready`, `--blocked`, `--quarantined`, `--format` | Index view |
| `awit label` | `--state open\|closed\|all`, `--format` | Label vocabulary with usage counts |
| `awit show <id>` | `--full`, `--refs-only` | Core ticket or full resolved ref tree |
| `awit comment <id> [text]` | `--file <path>`, `--author` | Timestamped comment or attached file; append to `refs` |
| `awit update <id>` | `--status`, `--brief`, `--assign`, `--label`, `--unlabel`, `--title` | Mutate frontmatter with a minimal diff |
| `awit close <id>` | `--reason`, `--author` | Set `closed`, clear `claimed_at`; does not git-commit |
| `awit release <id>` | — | Set `open`, clear `assignee` and `claimed_at` |
| `awit dep add\|rm <id> <dep>` | — | Edit `deps` with cycle pre-check |
| `awit validate` | `--stale-claims` | Integrity report; non-zero exit on `FAIL` |
| `awit prime` | `--max-tokens`, `-l` label | Deterministic state graph for prompt injection |
| `awit next` | `-l` label, `--claim`, `--no-commit`, `--seed` | Top unblocked item; optional claim |

Global flags: `--format compact|table|json`, `--repo <path>` (directory that
contains `.awit/`), `--no-color` (accepted, no-op). Exit codes: `0` success,
`1` expected non-success (`next` with no candidates, `validate` with FAIL),
`2` usage error.

## Agent loop

Each step is one process. State between steps lives only in files and Git.

```text
awit prime                 # token-light snapshot of the graph
awit next --claim          # claim the top unblocked item
awit show <id> --full      # ticket + resolved refs
awit comment <id> "..."    # research notes
awit close <id>            # unblocks downstream
```

Then `prime` again. `--claim` writes `status: in_progress`, `assignee`,
`claimed_at`, and commits `awit: claim <id>` unless `--no-commit`.
`close` is a plain file write; the human or agent commits when done.
Filter with `-l p0` (AND across repeated flags, OR inside one comma list).

## Status

v1 targets tagged binaries for linux, darwin and windows (amd64 and
arm64) via GoReleaser. CI runs `go vet`, `staticcheck` and `go test ./...`
on ubuntu-latest and windows-latest. This repository dogfoods itself:
implementation tickets live in [`.awit/items/`](.awit/items/).

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
├── config.yaml            # prefix, default_labels, stale_claim, agent_id
├── items/PREFIX-XXXXXXXX.md
└── comments/PREFIX-XXXXXXXX/<UTC seconds>-<author>.md
```

An item is YAML frontmatter (`id`, `title`, `brief`, `status`, `deps`,
`labels`, `assignee`, `claimed_at`, `refs`) followed by a Markdown body.
Priority is a label by convention (`p0`…`p4`). Blocked is derived, never
stored. The on-disk schema, including the reserved `external:` key, is
documented in [docs/schema.md](docs/schema.md).

## Contributing

Read [plan/implementation-guide.md](plan/implementation-guide.md) first.
It holds the resolved design decisions, the package layout, every shared
Go interface, and the ticket index with dependency order. Then pick a
ticket from `.awit/items/` whose `deps` are all closed.

## License

BSD 2-Clause. See [LICENSE](LICENSE).
