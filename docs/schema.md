# awit on-disk schema

Everything awit stores is a UTF-8 file under `.awit/` plus a single
gitignored lock file. There is no database. Commands rebuild the graph
from `items/*.md` on every invocation. This document is the v1 contract.

## Layout

```text
<repo>/
├── .gitignore                 # contains the line .awit/.lock
└── .awit/
    ├── .lock                  # advisory lock; not committed
    ├── config.yaml
    ├── items/
    │   └── PREFIX-XXXXXXXX.md # stem == frontmatter id
    └── comments/
        └── PREFIX-XXXXXXXX/
            ├── 20260917T143205Z-claude.md
            └── 20260917T151047Z-jan.md
```

Paths inside frontmatter `refs` always use forward slashes. New items
set `refs_base: repo` and store paths relative to the repository root
(the directory that contains `.awit/`), so `docs/notes/x.md` is
`<repo>/docs/notes/x.md` and `.awit/comments/<id>/file.md` is a comment.
Items that omit `refs_base` keep the historical `.awit/items/` base until
their first successful mutation, which rewrites every ref with
`filepath.Rel` (no existence check) and writes the marker in the same
atomic save. Resolution never probes both bases. On-disk paths use
`filepath` (OS separators).

## `config.yaml`

```yaml
prefix: AWIT
default_labels: [p1]
labels: [phase0, phase1, phase2, phase3, phase4, phase5, p0, p1, p2]
stale_claim: 2h
agent_id: claude
commit: false
template: plan/workitem-template.md
```

| Key | Required | Rules |
| --- | --- | --- |
| `prefix` | yes | `[A-Z][A-Z0-9]{1,7}`; missing → load error `config: prefix is required` |
| `default_labels` | no | strings; merged first-wins into `awit create -l` |
| `labels` | no | advisory vocabulary. Missing or empty disables warnings. Entries must be nonempty, with no leading/trailing whitespace or control characters; duplicates are deduplicated in memory; matching is case-sensitive. `create`/`update` warn on unknown names they introduce but still store them (exit 0). Not an allowlist |
| `stale_claim` | no | Go duration (`2h`, `90m`). Missing/zero → `2h` |
| `agent_id` | no | raw identity; `AWIT_AGENT` overrides; `--author` overrides both |
| `commit` | no | bool; repository default for `next --claim` git commits. Absent → `true`. `next --commit=true\|false` overrides per invocation; `--no-commit` (deprecated) equals `--commit=false`. Only `next --claim` reads it — never pushing, never another command |
| `template` | no | repo-root-relative forward-slash path to a body-only Markdown file. Only `create` reads the file. Absolute paths, backslashes, and lexical escape above the repo root fail `Load`. Missing/unreadable/directory/non-UTF-8/conflict-marker files fail `create` (exit 1, no item). Empty file → empty body. Absent/empty keeps the default skeleton. `import` ignores it |

Unknown keys in `config.yaml` are not part of v1; `Load` decodes into a
struct and extra keys are dropped on the next `Write`. Do not put
`external:` here.

## Item files

Filename: `.awit/items/<id>.md`. The stem **must** equal the `id` key
(case-sensitive match after parse; a case-insensitive stem collision with
another file is `DUPLICATE ID`). The file is YAML frontmatter fenced by
`---` lines, then a Markdown body. `Parse` then `Bytes()` with no setter is
byte-identical, including CRLF. After a setter, `Bytes()` writes `\n` fences
and the exact body. `Split` also accepts `\r\n`.

```markdown
---
id: AWIT-0K7M2QX9
title: Implement OAuth2 bearer token extraction
brief: >-
  One to three sentences of prose summarizing the item.
status: open
deps: []
labels: [auth, p1]
refs_base: repo
refs: []
---

## Summary

## Acceptance Criteria
```

### Required keys

| Key | Type | Rules |
| --- | --- | --- |
| `id` | string | `PREFIX-` plus 8 Crockford chars. Must equal the filename stem |
| `title` | string | Non-empty |
| `status` | enum | `open`, `in_progress`, `closed`. Blocked is derived, never stored |

Missing required keys or an unknown status → parse error → quarantine
`PARSE ERROR`.

### Optional known keys

| Key | Type | Rules |
| --- | --- | --- |
| `brief` | string | One to three sentences. `create` requires `--brief`. `validate` warns when missing or longer |
| `deps` | list of ids | Unknown id → `DANGLING DEP` on this item. Written flow style `[a, b]` |
| `labels` | list of strings | Free-form. `p0`–`p4` recommended for priority. Flow style. Optional `config.yaml` `labels` is advisory only — unknown names warn on `create`/`update` and still store |
| `assignee` | string | `human/<name>` or `agent/<id>`. Omitted when empty. Deleted by `release`; kept by `close` as the audit trail |
| `claimed_at` | RFC3339 UTC | Seconds precision. Set by `--claim`; deleted by `release` and `close` |
| `refs_base` | string | `repo` or omitted. Omitted means historical `.awit/items/`-relative refs. Invalid types/values are parse errors |
| `refs` | list of paths | Forward slashes. Block style. Always present, `[]` when empty. Relative to the repo root when `refs_base: repo`, else `.awit/items/` |
| `external` | mapping | Optional Gitea or GitLab issue link (see below). Missing is valid |
| `alias` | string | Optional human alias (see below). Missing is valid |

New items written by `awit create` use key order
`id, title, brief, status, deps, labels, refs_base, refs` and omit empty
`assignee` / `claimed_at`. `external` is appended when `--external-*` flags are set.

### Optional key: `alias`

A short human handle set by `create`/`update --alias` or `import --alias`
(`update --clear-alias` removes it). Grammar:
`[A-Za-z][A-Za-z0-9._-]{0,127}` — no whitespace, slash or `#`, and never
anything shaped like a canonical ID. Uniqueness is case-insensitive across
active parseable items but is not enforced at write time: duplicate
hand-edited aliases make alias lookup fail with the matching canonical IDs
listed, and `validate` warns about invalid or duplicate aliases (exit stays
0 unless graph faults exist). Aliases are lookup-only: filenames, `deps`
edges and commit messages always use the canonical ID.

### Item lookup keys

Every command that takes an item argument (`show`, `list`, `next`,
`update`, `close`, `release`, `comment`, `dep`, `ref`, `external check`,
`external push-body`) accepts, in precedence order: the canonical ID
(exact, case-sensitive, always wins), an alias (case-insensitive), or an
external key `owner/repo#<n>` or bare `#<n>` matched against valid
`external:` metadata (the bare form must be unique). Ambiguity is an error
listing the matching canonical IDs; unknown keys error as `unknown item <key>`
and never become filesystem paths.

### Optional key: `external`

```yaml
external:
  tracker: gitea
  repo: owner/repo
  id: 127
  url: https://forge.example/owner/repo/issues/127
```

```yaml
external:
  tracker: gitlab
  repo: group/sub/project
  id: 127
  url: https://forge.example/apps/gitlab/group/sub/project/-/work_items/127
```

`id` is the repository issue **number** (Gitea `number` or GitLab **`iid`**),
never either product's database-wide issue ID. All four subkeys are required
for a valid mapping. Extra nested keys are kept. `tracker` must be exactly
`gitea` or `gitlab`. This is additive: existing Gitea files stay valid, no
item is rewritten, and Gitea grammar is unchanged.

Gitea `repo` is `owner/name` (exactly two nonempty segments; no whitespace,
control characters, `.`/`..`, or URL delimiters). Gitea `url` is absolute
HTTP(S) with a host, no userinfo, query, or fragment; its decoded path must
end in `/<owner>/<repo>/issues/<id>` (an installation prefix before that
suffix is allowed).

GitLab `repo` has at least two slash-separated segments (subgroups allowed).
Reject leading/trailing slash, empty segments, `.` and `..`, Unicode
whitespace/control characters, and any segment containing `\/:?#@[]%`.
Ordinary dots inside a name are allowed; case and spelling are preserved.
GitLab `url` is absolute HTTP(S) with a host, no userinfo, query (including
empty `?`), or fragment (including empty `#`). Its decoded path must end
exactly in `/<complete repo>/-/issues/<iid>` or
`/<complete repo>/-/work_items/<iid>`. A prefix before that suffix is the
installation path. `work_items` is an issue-link spelling, not a promise to
import every GitLab work-item type.

Local statuses remain `open|in_progress|closed`. GitLab wire `opened` maps to
local `open` and wire `closed` to `closed`; that conversion is remote
integration, not YAML parsing. Import and remote writes for GitLab are not
part of this schema change.

`awit create` and `awit update` take `--external-tracker`, `--external-repo`,
`--external-id`, and `--external-url` together; a partial set is a usage
error (exit 2) and writes nothing. `awit update --clear-external` removes
the field and cannot be combined with those flags. An unchanged identical
mapping is a no-op.

A missing `external` key is valid. An invalid mapping, unsupported tracker,
or the old reserved scalar form (`external: gitlab#42`) does **not**
quarantine the item: Parse keeps the YAML, `External` is nil, and
`validate` prints `WARN  <id>: invalid external: <reason>` (exit 0 unless
there are graph faults). JSON `validate` writes that advisory on stderr
without changing the fault-array schema. Show, list, and compact output
display only a valid link (`gitea owner/repo#127` or
`gitlab group/sub/project#127`).

`awit import <issue-url> --brief <summary> [--alias X] [--tea-login name]`
creates an item from an existing Gitea issue through the `tea` CLI: the
item is a one-time snapshot with the issue number as `external.id`, the
decoded body byte-exact, the remote labels first-seen deduplicated (local
default labels are not merged), and `open`/`closed` mapped to the same
local status (other states are refused). Import never writes remote state
and never commits. Re-importing the same installation base + repo + issue
number — whether the earlier import is active or archived — is refused
with the existing item or archive path named.

`awit external check [key] [--tea-login name]` compares the raw local
body bytes (`Item.Body()`: every byte after the frontmatter closing
fence) against the linked Gitea issue body and nothing else — frontmatter,
title, labels, comments, and remote state are ignored, so any whitespace,
line-ending, final-newline, or leading-blank-line difference is drift. The
command is read-only: no local bytes and no remote fields change. Plain
output prints one `MATCH`, `DRIFT`, or `ERROR` line per linked item (each
naming the item and its URL) plus deterministic totals; `--format json`
prints an array of `{id, url, result, detail}` with no human lines on
stdout. Exit 1 when any row is drift or error, otherwise 0. An
authentication or read failure is an error row, never drift. The all-items
form walks linked items in canonical-ID order and skips genuinely
unlinked ones (reporting the number checked, including zero); naming an
explicitly unlinked item is an error.

`awit external push-body <key> [--tea-login name]` is the explicit
local-canonical repair: it pushes the local body bytes to the linked
issue and nothing else (no title, label, state, local-content, or history
change). The push holds the store lock until the bounded request
completes, refuses ambiguous duplicate external links, requires a 2xx
status (tea exits zero on HTTP errors, so the `--include` status line is
authoritative), and verifies the remote took the exact bytes — against the
PATCH response, or a GET of the same issue when the response omits the
body. A mismatch is an error, never success.

`awit close <id> [--tea-login name] [--no-push]`, `awit release <id>` with
the same flags, and `awit update <id> --status <s>` with the same flags
propagate local state one way to the linked issue: `close` pushes `closed`,
`release` pushes `open`, and an explicit `--status` pushes `closed` for
`closed` and `open` for `open`/`in_progress` (a non-status update never
pushes, and neither do `next --claim`, create, import, comment, ref, or
archive). The local item is saved first — keeping close's reason comment
and claim-clearing — then the push runs under the held store lock with the
bounded subprocess deadline (`tea api --login <login> --repo <owner/repo>
--include -X PATCH -f state=<open|closed>
repos/<owner>/<repo>/issues/<n>`), requiring a 2xx status and verifying
the response confirms the issue number, installation, and state (GET
fallback when the response omits it; the remote is never read to decide).
Local state stays canonical: remote failure, missing tea, invalid
metadata, or ambiguous duplicate links keep the local mutation and its
ordinary confirmation, print one stderr
`warning: <id> saved locally; external state push failed: <reason>; retry with awit update <id> --status <status>`
(exit 0), while a local write failure exits 1 and never pushes.
Repeating `update <id> --status <current>` re-pushes without a queue or
daemon. `--no-push` performs no tea discovery, auth, or network operation,
even with malformed metadata.

Byte-exactness relies on a tested `tea` behavior: `tea`'s `-F body=@file`
reader strips exactly one terminal LF, so awit writes the transport file
as the body plus one extra LF (private temp-then-rename file, mode 0600,
deleted on every exit). Supported: `tea` 0.16.0, verified by
`TestTeaBodyRoundTrip` (empty, no-LF, multi-LF, CRLF, leading blanks,
Unicode, backticks, and literal `null` round-trip byte-exact). A `tea`
build that fails that compatibility test must not be advertised as
supported. Ordinary `awit validate` stays offline: it works with `tea`
absent and networking disabled.

### Unknown keys

Any other frontmatter key is preserved on the YAML node. Teams may add
their own fields without a schema change. `validate` does not FAIL on
unknown keys. A setter that does not own the key must not delete it.

### Body

Everything after the closing `---` fence is raw Markdown, stored as
bytes. `create` seeds:

```text

## Summary

## Acceptance Criteria

```

(leading newline after the fence) when `template` is unset. When
`template:` names a file, `create` copies those bytes exactly as the
body — no extra leading newline — and does not parse them as frontmatter.
An empty template file is an empty body. Conflict marker lines
(`<<<<<<< `, `=======`, `>>>>>>> `) anywhere in the file quarantine
the item as `CONFLICT MARKERS`; a template containing them is refused
before mint so the new item is never written.

Example `plan/workitem-template.md` (body-only, no frontmatter):

```markdown
## Summary

## Context (read first)

## Acceptance Criteria
```

## Comment files

Path: `.awit/comments/<id>/<filename>`. Inline comments (not `--file`)
are Markdown with their own frontmatter:

```markdown
---
author: agent/claude
created: 2026-09-17T14:32:05Z
---

Research notes go here.
```

| Key | Rules |
| --- | --- |
| `author` | As resolved: `--author` verbatim, or `AWIT_AGENT` / `agent_id` with `agent/` prefix, or git `user.name` |
| `created` | RFC3339 UTC, seconds |

`--file` copies the source bytes verbatim — no frontmatter is added.

After a comment or attachment is written, existing refs are normalized to
the repo-root base and the item's `refs` gains a forward-slash entry
`.awit/comments/<id>/<filename>`.

## Filenames

| Kind | Pattern |
| --- | --- |
| Item | `<id>.md` in `items/` |
| Comment | `<YYYYMMDDTHHMMSSZ>-<sanitised-author>.md` |
| Attachment | `<YYYYMMDDTHHMMSSZ>-<sanitised-author><ext>` keeping the original extension |
| Collision | `-2`, `-3`, … immediately before the extension |

Author sanitising: strip one leading `agent/`, lowercase, map runes
outside `[a-z0-9._-]` to `-`, collapse `--`, trim `-`, empty → `anon`.

IDs: 8 Crockford characters (`0-9 A-H J-K M-N P-T V-Z`, no `I L O U`),
prefixed by `config.prefix` and a hyphen. Epoch `2026-01-01T00:00:00Z`.

## Lock file

`.awit/.lock` is an exclusive advisory lock (flock / LockFileEx) used by
mutating commands. `awit init` appends `.awit/.lock` to the repo
`.gitignore` and does not create the file. The file is created on first
`Store.Lock`. It is never committed.

## Seeded agent skills

`awit init` also looks for agent directories in the repo root — `.claude`,
`.omp`, `.opencode`, `.agents`, `.pi`, in that order — and offers to write
`<dir>/skills/driving-awit/SKILL.md` into each one it finds. It never
creates an agent directory; their presence is the signal that the tool is
in use. `--skills` seeds every detected directory without asking,
`--no-skills` skips the whole step, `--force` overwrites an existing skill
file instead of keeping it.

These files live **outside** `.awit/` and are not part of the item schema.
They are project config and are meant to be committed; unlike `.awit/.lock`,
`init` does not gitignore them.

## Quarantine (not schema, but load-time)

These are not extra keys; they are reasons a file fails to become a
healthy item: `PARSE ERROR`, `CONFLICT MARKERS`, `ID MISMATCH`,
`DUPLICATE ID`, `DANGLING DEP`, `CYCLE`. Invalid `external:` metadata is
none of these — it is a `validate` WARN only.
