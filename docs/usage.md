# Setup & Usage

## Install

Quick bootstrap (Linux x86_64; pick the matching asset for other platforms):

```bash
V=$(curl -s https://api.github.com/eisenwinter/awit/releases/latest | grep '"tag_name"' | cut -d'"' -f4)
curl -sL "https://github.com/eisenwinter/awit/releases/download/$V/awit_${V}_Linux_x86_64.tar.gz" | tar xz awit
./awit --version && ./awit init --skills
```

Assets are named `awit_<version>_<Os>_<Arch>.tar.gz` (`.zip` on Windows).

Or paste this to your agent and skip the shell entirely:

```text
Bootstrap awit in the current repository: download the release binary for
this machine from github.com/eisenwinter/awit/releases (assets are named
awit_<version>_<Os>_<Arch>.tar.gz, .zip on Windows), put it on PATH, then
run awit init --skills in the repo root and report the .awit layout and
the seeded skills it created.
```

From source (Go 1.27+):

```bash
go install github.com/eisenwinter/awit/cmd/awit@latest
```

Or download a tagged binary from GitHub Releases. Each tag `v*` publishes
linux, darwin and windows builds for amd64 and arm64, all with
`CGO_ENABLED=0`.

```bash
awit --version
```

A release build prints `awit <version>`. A `go build` without ldflags prints
`awit dev`.

## External tracker prerequisites

Linked external issues need their tracker's CLI, pre-authenticated by you:
Gitea via `tea` (`tea login add`), GitLab via `glab` 1.118.0
(`glab auth login --hostname <host>`; check with
`glab auth status --hostname <host>`). awit never logs in, manages tokens,
or writes tracker configuration.

## Your first repo

```bash
awit init --prefix AWIT
```

This creates `.awit/` (`config.yaml`, `items/`, `comments/`), and appends
`.awit/.lock` to `.gitignore`. With `--skills` (or an interactive prompt),
`init` also seeds the driving-awit skill into each detected agent directory
(`.claude`, `.omp`, `.opencode`, `.agents`, `.pi`); `--no-skills` skips that
step.

## Creating work items

```bash
awit template > body.md          # body skeleton (or your config template)
```

Fill in `body.md`, then:

```bash
awit create "<imperative title>" --brief "<1-3 sentences>" --body-file body.md -l <phase-or-area> -l p1 -d <dep-id>
```

`create` requires `--brief` (one to three sentences); correct a body later
with `awit update <id> --body-file body.md`. A repo-level template via
`.awit/config.yaml` `template:` replaces the default skeleton with that
file's exact bytes.

## The agent loop

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

## Finding work

- `awit list` — index view; filter with `-s` status, `-l` label, `--ready`,
  `--blocked`, `--quarantined`.
- `awit next` — top unblocked item; `-l` filters first. `-l` is AND across
  repeated flags, OR inside one comma list (`-l p0 -l auth` vs `-l p0,p1`).
- `awit prime` — deterministic whole-graph snapshot for prompt injection;
  `--max-tokens N` is a soft budget that never sheds warnings or the top
  ready row.

Every list-shaped output honours `--format compact|table|json` (compact when
stdout is not a TTY). `awit next -l p0` without `--claim` is the priority
check: it answers whether anything critical is ready and how much it
unblocks.

## Looking items up

Wherever a command takes an item, the argument may be the canonical ID
(`AWIT-XXXXXXXX`, exact, always wins), a case-insensitive `alias`, or an
external key `owner/repo#127` / `group/sub/project#127` / `#127` (bare
numbers must be unique). Ambiguous keys are refused with the matching
canonical IDs.

## Pausing work

`awit block <id> --reason "<obstacle and release condition>"` pauses with a
recorded reason; `awit unblock <id>` removes only the hold; `awit release
<id>` reopens to `open` (the hold stays); `awit close <id>` finishes the
item (the hold clears, `--reason` becomes a comment). A `blocked` label
alone never holds anything — the commands warn once on stderr when that
label lands on a hold-less item.

## External trackers

- `awit import <issue-url>` — one-time snapshot of a Gitea (`tea`) or
  GitLab (`glab`) issue: number/iid, exact body, labels, open/closed state.
- `awit external check [key]` — byte-exact local-vs-remote body comparison;
  exit 1 on any drift or error.
- `awit external push-body <key>` — explicit local-canonical repair.

For linked items `close` pushes `closed` (and `release` pushes `open`) to
the linked issue after the local save; an explicit `update --status` pushes
the mapped state the same way, and a non-status update never pushes.
Automatic pushes follow `external_push` in `.awit/config.yaml` (omitted
means true); `--push=true|false` overrides one invocation. Local state stays
canonical: a remote failure keeps the local mutation, prints a stderr retry
warning, and still exits 0. Pass `--no-push` or `--push=false` for an
explicit silent offline path, or `--tea-login` to choose the Gitea login
(ignored for GitLab).

## Archiving

```bash
awit archive --dry-run   # preview the fixed-point set
awit archive             # move it to .awit/archive/
```

Only closed items with no dependant outside the set move; the rest stay
closed and archivable later. Comments collapse into the archived file.
There is no `unarchive`; `git revert` is the way back.

## Command reference

| Command | Purpose |
| --- | --- |
| `awit init` | Create `.awit/`, `config.yaml`, gitignore the lock; offer skills |
| `awit create <title>` | Mint an ID, write a lean item |
| `awit template` | Print the body template `create` would use |
| `awit import <issue-url>` | Snapshot a Gitea or GitLab issue |
| `awit external check [key]` | Byte-exact body drift check |
| `awit external push-body <key>` | Push local body to the linked issue |
| `awit list [key]` | Index view |
| `awit label` | Label vocabulary with usage counts |
| `awit show <id>` | Core item, `--full` ref tree, `--refs-only`, `--unblocks` |
| `awit comment <id> [text]` | Timestamped comment or `--file` attachment |
| `awit update <id>` | Mutate frontmatter with a minimal diff |
| `awit close <id>` | Set `closed`, clear claim and hold, comment the reason |
| `awit release <id>` | Reopen to `open`, clear assignee and claim |
| `awit block <id>` | Pause with a recorded reason |
| `awit unblock <id>` | Remove only the manual block |
| `awit dep add\|rm <id> <dep>` | Edit `deps` with cycle pre-check |
| `awit ref add\|rm <id> <path>` | Repo-root-relative file references |
| `awit validate` | Integrity report; non-zero exit on `FAIL` |
| `awit archive` | Move finished work out of the hot path |
| `awit prime` | Deterministic state graph for prompt injection |
| `awit next` | Top unblocked item; optional `--claim` |

Global flags: `--format compact|table|json`, `--repo <path>`, `--no-color`
(accepted, no-op). Exit codes: `0` success, `1` expected non-success, `2`
usage error. Graph-reading commands print one stderr quarantine warning
when the loaded graph holds quarantined items or broken files.

## Configuration

`.awit/config.yaml` keys, one line each: `prefix` (required), advisory
`labels`, `default_labels`, `stale_claim` (default `2h`), `agent_id`,
`commit` (claim-commit default, true), `external_push` (state-push default,
true), `template` (body file for `create`). Full rules are in the
[On-disk Schema](schema.md).

## Pre-commit hook

`awit validate` is the intended pre-commit hook. A copy lives at
[`hooks/pre-commit`](hooks/pre-commit):

```sh
#!/bin/sh
exec awit validate
```

```bash
cp docs/hooks/pre-commit .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```
