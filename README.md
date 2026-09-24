# awit

Agent work item tool: a zero-daemon Go CLI that turns Markdown files
under `.awit/` into a dependency graph for humans and agents. Offline,
versioned in Git, no database, no daemon. Every command rebuilds the
graph from `.awit/items/*.md`; a clone is the whole state.

Full documentation: **https://eisenwinter.github.io/awit/**

## Install

Quick bootstrap (detects your OS/arch; Windows needs the `.zip` asset and `tar`/`Expand-Archive` instead):

```bash
V=$(curl -s https://api.github.com/eisenwinter/awit/releases/latest | grep '"tag_name"' | cut -d'"' -f4)
OS=$(uname -s | tr '[:upper:]' '[:lower:]'); ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
curl -sL "https://github.com/eisenwinter/awit/releases/download/$V/awit_${V#v}_${OS}_${ARCH}.tar.gz" | tar xz awit
./awit --version && ./awit init --skills
```

Assets are named `awit_<version>_<os>_<arch>.tar.gz` (`.zip` on Windows, e.g. `awit_0.5.0_linux_amd64.tar.gz`).
`awit init --skills` creates `.awit/` and seeds the driving-awit skill into
every detected agent directory without asking.

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

Linked external issues need their tracker's CLI, pre-authenticated by you:
Gitea via `tea` (`tea login add`), GitLab via `glab` 1.118.0
(`glab auth login --hostname <host>`; check with
`glab auth status --hostname <host>`). awit never logs in, manages tokens,
or writes tracker configuration. GitLab description writes travel
byte-exact with no line-ending adaptation, and a body with a column-zero
`/command` line is refused before any mutation (GitLab would execute it as
a quick action instead of storing it).

## Commands

| Command | Flags | Purpose |
| --- | --- | --- |
| `awit init` | `--prefix`, `--skills`, `--no-skills`, `--force` | Create `.awit/`, `config.yaml`, gitignore `.awit/.lock`; offer to seed the driving-awit skill |
| `awit create <title>` | `--brief`, `--body`, `--body-file`, `-d` deps, `-l` labels, `--assign`, `--alias`, `--id`, `--external-tracker`, `--external-repo`, `--external-id`, `--external-url` | Mint a snowflake ID, write a lean item; optional Gitea or GitLab mapping; body from `--body`/`--body-file`, else `config.template`, else the built-in skeleton |
| `awit import <issue-url>` | `[--brief]`, `--alias`, `--tea-login` | One-time snapshot of an existing Gitea (`tea`) or GitLab (`glab`) issue: keeps number/iid, exact body, labels and open/closed state; refuses tracker-aware duplicates. Omitted `--brief` derives from the remote title (else the body's first sentence, capped at 240 code points). `--tea-login` is Gitea-only and ignored for GitLab. Labels are copied once as metadata and never synced afterwards |
| `awit template` | — | Print the body template `create` would use: the `config.yaml` `template:` file's exact bytes, else the built-in skeleton; no flags; ignores `--format`; builds no graph, so no quarantine warning |
| `awit external check [key]` | `--tea-login` | Read-only byte-exact body comparison for linked Gitea (`tea`) or GitLab (`glab`) items; `MATCH`/`DRIFT`/`ERROR` rows plus totals; exit 1 on any drift or error. `--tea-login` is Gitea-only and ignored for GitLab |
| `awit external push-body <key>` | `--tea-login` | Explicit repair: push local body bytes to the linked issue (Gitea via `tea`, GitLab via `glab`); refuses ambiguous links and column-zero `/command` bodies GitLab would execute as quick actions; verifies the remote took the exact bytes |
| `awit list [key]` | `-s` status, `-l` label, `--ready`, `--blocked`, `--quarantined`, `--format` | Index view; `[key]` selects exactly one item |
| `awit label` | `--state open\|closed\|all`, `--format` | Observed label usage counts (not the config vocabulary) |
| `awit show <id>` | `--full`, `--refs-only`, `--unblocks` | Core item, full resolved ref tree, or the open items this one transitively unblocks |
| `awit comment <id> [text]` | `--file <path>`, `--author` | Timestamped comment or attached file; append to `refs` |
| `awit update <id>` | `--status`, `--brief`, `--body`, `--body-file`, `--assign`, `--label`, `--unlabel`, `--title`, `--alias`, `--clear-alias`, `--external-tracker`, `--external-repo`, `--external-id`, `--external-url`, `--clear-external`, `--push=true\|false`, `--no-push`, `--tea-login` | Mutate frontmatter with a minimal diff; an explicit `--status` also pushes the mapped state (`closed`→closed, `open`/`in_progress`→open) to the linked Gitea or GitLab issue unless `external_push: false` or `--push=false`/`--no-push`; local-first with a stderr retry warning on remote failure |
| `awit close <id>` | `--reason`, `--author`, `--push=true\|false`, `--no-push`, `--tea-login` | Set `closed`, clear `claimed_at` and any manual block; does not git-commit; pushes `closed` to the linked Gitea or GitLab issue unless `external_push: false` or `--push=false`/`--no-push` |
| `awit release <id>` | `--push=true\|false`, `--no-push`, `--tea-login` | Reopen an in-progress or closed item as `open`, clear `assignee` and `claimed_at`; any manual block stays; prints `reopened <id>` (plain line, ignores `--format`); pushes `open` to the linked Gitea or GitLab issue unless `external_push: false` or `--push=false`/`--no-push` |
| `awit block <id>` | `--reason` | Pause an item with a recorded reason: sets `open`, clears the claim, keeps deps/labels/refs; prints `blocked <id>: <reason>` (plain line, ignores `--format`); refuses closed items; never touches git or the tracker |
| `awit unblock <id>` | — | Remove only the manual block; never claims, reopens, or pushes; idempotent; prints `unblocked <id>` (plain line, ignores `--format`) |
| `awit dep add\|rm <id> <dep>` | — | Edit `deps` with cycle pre-check |
| `awit ref add\|rm <id> <path>` | `add --allow-missing` | Add or remove a repo-root-relative file reference; `add` refuses a missing target (exit 1, no write) unless `--allow-missing` plans it ahead; does not copy, delete, or commit |
| `awit validate` | `--stale-claims` | Integrity report; non-zero exit on `FAIL`; invalid `external` is a WARN |
| `awit prime` | `--max-tokens`, `-l` label | Deterministic state graph for prompt injection; `--max-tokens` is a soft budget that never sheds warnings or the top ready row |
| `awit next` | `-l` label, `--claim`, `--commit=true\|false`, `--no-commit` (deprecated), `--seed`, `--why` | Top unblocked item; optional claim; `--why` explains the pick on stderr |
| `awit lazy-human` | `--agent` | Keyboard-driven TUI: Issues (list + detail), Graph (prime overview / focused DAG), Queue (ready order); `c/b/u/m` close/block/unblock/comment, `space`/`r` claim/release, `V` validate, `P` external check; no mouse, never pushes or commits |

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
`awit template` builds no graph, so it never prints the warning.

Labels never pause work: `create`, `update`, and `import` print one stderr
line — `warning: <id> has label "blocked", which does not pause work;
use awit block <id> --reason "..."` — when a non-closed item without a
manual block newly receives the exact label `blocked`. Stdout and exit
codes are unchanged. To actually hold an item, run
`awit block <id> --reason "<obstacle and release condition>"`;
`awit release` never clears the hold, `awit close` does.

## Agent loop

```text
awit prime                 # token-light snapshot of the graph
awit next --claim          # claim the top unblocked item
awit show <id> --full      # item + resolved refs
awit comment <id> "..."    # research notes
awit block <id> --reason "waiting on X; unblock when Y"  # pause with a reason
awit unblock <id>          # resume once the condition is resolved
awit close <id>            # unblocks downstream
```

Then `prime` again. `--claim` writes `status: in_progress`, `assignee`,
`claimed_at`, and commits `awit: claim <id>` following the commit policy:
`--commit=true|false` overrides one invocation (default `true`), a
`commit: false` line in `.awit/config.yaml` sets the repository default,
and `--no-commit` is the deprecated spelling of `--commit=false`.
`close` is a plain file write; the human or agent commits when done.
For linked items `close` pushes `closed` (and `release` pushes `open`) to
the linked issue — Gitea through `tea`, GitLab through `glab` as a
`state_event` reopen/close — after the local save, under the store lock;
an explicit `update --status` pushes the mapped state the same way, and a
non-status update never pushes. Automatic pushes follow
`external_push` in `.awit/config.yaml` (omitted means true);
`--push=true|false` overrides one invocation (true `--no-push` equals
`--push=false`; `--no-push=false` is neutral). Local state stays canonical: a remote
failure keeps the local mutation, prints
`warning: <id> saved locally; external state push failed: <reason>; retry with awit update <id> --status <status>`
on stderr, and still exits 0. A config-disabled skip prints
`warning: <id> saved locally; external state push skipped by config external_push: false; push with awit update <id> --status <status> --push=true`.
Pass `--no-push` or `--push=false` for an explicit silent offline path
(no tool discovery, auth, or network, even with malformed metadata), or
`--tea-login` to choose the Gitea login (ignored for GitLab).
Filter with `-l p0` (AND across repeated flags, OR inside one comma list).

## Status

v1 targets tagged binaries for linux, darwin and windows (amd64 and
arm64) via GoReleaser. CI runs `go vet`, `staticcheck` and `go test ./...`
on ubuntu-latest and windows-latest. This repository dogfoods itself:
the v1 implementation work items are closed and live in [`.awit/archive/`](.awit/archive/).

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
├── templates/workitem.md  # optional body template; config.yaml template: names any repo-root-relative file
├── items/PREFIX-XXXXXXXX.md
└── comments/PREFIX-XXXXXXXX/<UTC seconds>-<author>.md
```

An item is YAML frontmatter (`id`, `title`, `brief`, `status`, `deps`,
`labels`, `assignee`, `claimed_at`, `refs_base`, `refs`, optional `alias`,
optional `external`, optional `blocked_reason`) followed by a Markdown body.
Priority is a label by convention (`p0`…`p4`). Optional `config.yaml`
`labels` is an advisory vocabulary: `create`/`update` warn on unknown
names they introduce but still store them. Lifecycle is stored
(`open`/`in_progress`/`closed`) with an optional stored manual hold
(`blocked_reason`); ready/blocked eligibility is derived. The on-disk
schema, including the optional `alias`, the Gitea or GitLab `external:`
mapping, and manual blocks, is documented in [docs/schema.md](docs/schema.md).

Wherever a command takes an item — `show`, `list`, `next`, `update`,
`close`, `release`, `comment`, `dep`, `ref`, `external check`,
`external push-body` — the argument may be the
canonical ID (`AWIT-XXXXXXXX`, exact, always wins), a case-insensitive
`alias`, or an external key `owner/repo#127` / `group/sub/project#127` /
`#127` (bare numbers must be unique). Ambiguous keys — including the same
repo and number on two trackers or hosts — are refused with the matching
canonical IDs.

GitLab issue and work_items URLs import the same way; the stored URL is
the input spelling, and `group/sub/project#127` looks up a subgroup project:

```sh
awit import https://forge.example/group/sub/project/-/work_items/127 \
  --brief "Imported GitLab issue." --alias GL-IMPORT
```

## Contributing

Read [docs/design-spec.md](docs/design-spec.md) first — published at **https://eisenwinter.github.io/awit/design-spec/**. It holds the resolved design decisions and the package layout; signatures live in code (`go doc`), the closed v1 items in `.awit/archive/`. The on-disk format is [docs/schema.md](docs/schema.md). Then open a work item under `.awit/` whose `deps` are all closed.

## License

BSD 2-Clause. See [LICENSE](LICENSE).
