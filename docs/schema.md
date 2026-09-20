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
atomic save. `ref add` stats the resolved repo-root-absolute target before
any mutation and refuses a missing target (exit 1, item bytes unchanged)
unless `--allow-missing` plans it ahead; other commands never check, and
read-time `[missing]` reporting is unchanged. Resolution never probes both
bases. On-disk paths use `filepath` (OS separators).

## `config.yaml`

```yaml
prefix: AWIT
default_labels: [p1]
labels: [phase0, phase1, phase2, phase3, phase4, phase5, p0, p1, p2]
stale_claim: 2h
agent_id: claude
commit: false
external_push: false
template: plan/workitem-template.md
```

| Key | Required | Rules |
| --- | --- | --- |
| `prefix` | yes | `[A-Z][A-Z0-9]{1,7}`; missing → load error `config: prefix is required` |
| `default_labels` | no | strings; merged first-wins into `awit create -l` |
| `labels` | no | advisory vocabulary. Missing or empty disables warnings. Entries must be nonempty, with no leading/trailing whitespace or control characters; duplicates are deduplicated in memory; matching is case-sensitive. `create`/`update` warn on unknown names they introduce but still store them (exit 0). Not an allowlist |
| `stale_claim` | no | Go duration (`2h`, `90m`). Missing/zero → `2h` |
| `agent_id` | no | raw identity; `AWIT_AGENT` overrides; `--author` overrides both |
| `commit` | no | bool; repository default for `next --claim` git commits. Absent → `true`. `next --commit=true\|false` overrides per invocation; `--no-commit` (deprecated) equals `--commit=false`. Only `next --claim` reads it — never pushing, never another command. Independent of `external_push` |
| `external_push` | no | bool; repository default for automatic linked-issue state pushes from `close`, `release`, and explicit `update --status`. Absent → `true`. `--push=true\|false` overrides per invocation; true `--no-push` equals `--push=false`; `--no-push=false` is neutral. Does not govern `external push-body`, import, or `external check`. Independent of `commit` |
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
| `status` | enum | `open`, `in_progress`, `closed`. Stored lifecycle; ready/blocked eligibility is derived, with an optional stored manual hold (see Manual blocks) |

Missing required keys or an unknown status → parse error → quarantine
`PARSE ERROR`.

### Optional known keys

| Key | Type | Rules |
| --- | --- | --- |
| `brief` | string | One to three sentences. `create` requires `--brief`; `import` derives it from the remote title (else the body's first sentence, capped at 240 code points) unless given explicitly. `validate` warns when missing or longer |
| `deps` | list of ids | Unknown id → `DANGLING DEP` on this item. Written flow style `[a, b]` |
| `labels` | list of strings | Free-form. `p0`–`p4` recommended for priority. Flow style. Optional `config.yaml` `labels` is advisory only — unknown names warn on `create`/`update` and still store |
| `assignee` | string | `human/<name>` or `agent/<id>`. Omitted when empty. Deleted by `release`; kept by `close` as the audit trail |
| `claimed_at` | RFC3339 UTC | Seconds precision. Set by `--claim`; deleted by `release` and `close` |
| `refs_base` | string | `repo` or omitted. Omitted means historical `.awit/items/`-relative refs. Invalid types/values are parse errors |
| `refs` | list of paths | Forward slashes. Block style. Always present, `[]` when empty. Relative to the repo root when `refs_base: repo`, else `.awit/items/` |
| `external` | mapping | Optional Gitea or GitLab issue link (see below). Missing is valid |
| `alias` | string | Optional human alias (see below). Missing is valid |
| `blocked_reason` | string | Optional manual hold (see Manual blocks). Missing or removed means unblocked |

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

### Manual blocks

A `blocked_reason` is a durable local hold: a healthy non-closed item that
carries one is `Blocked` regardless of its dependencies, so a `blocked`
tracker label with zero open deps is representable without a new status,
a new quarantine category, or graph edges. Labels alone never hold an
item — a zero-dependency item with only a `blocked` label stays ready.

Presence and validation: the key is optional; missing or removed means
unblocked. When present it must be a YAML string scalar holding a
non-empty single line without control characters (outer whitespace is
trimmed). A wrong type, empty or whitespace-only content, embedded
control characters, or a duplicate `blocked_reason` key is a parse error
→ quarantine `PARSE ERROR`: the hold fails closed and the item is never
selectable while its declaration is unreadable.

Eligibility vs structure: readiness requires both no manual block and all
deps satisfied; clearing the reason restores readiness only when the deps
permit. Edges, cycle detection, unblock counts, the critical path, and
archive eligibility are unchanged — held nodes stay structural, exactly
like dep-blocked nodes today.

### Item lookup keys

Every command that takes an item argument (`show`, `list`, `next`,
`update`, `close`, `release`, `comment`, `dep`, `ref`, `external check`,
`external push-body`) accepts, in precedence order: the canonical ID
(exact, case-sensitive, always wins), an alias (case-insensitive), or an
external key `owner/repo#<n>` (GitLab subgroups: `group/sub/project#127`)
or bare `#<n>` matched against valid `external:` metadata (the bare form
must be unique). Ambiguity — including the same repo and number on two
trackers or hosts — is an error listing the matching canonical IDs;
unknown keys error as `unknown item <key>` and never become filesystem
paths. There is no new lookup syntax and no tracker prefix on the key;
aliases and canonical IDs disambiguate.

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
integration, not YAML parsing.

GitLab transport and auth (wrapper contract, `internal/glabx`, qualified
against glab 1.118.0): glab is pre-authenticated by the operator — awit
never logs in, selects logins, reads tokens, or writes configuration, and
only the named non-secret settings (`subfolder`, `api_host`,
`api_protocol`) are ever read. The effective installation subfolder for
the link host decides the repo segments (`GITLAB_SUBFOLDER` wins inside
glab); contradictory `api_host`/`api_protocol`/subfolder overrides fail
instead of targeting another instance. Every request is host-scoped
(`--hostname` carries the bare link host — glab rejects host:port there —
with one `%2F`-encoded project segment). Description writes travel
byte-exact: the transport file holds exactly the body, no line-ending is
added or stripped, and delivery needs 2xx, verified identity, and
`bytes.Equal` on the returned/read-back description. A body with any
column-zero `/lowercase` line is refused before any mutation — GitLab
would execute it as a quick action instead of storing it, even inside
fenced code blocks, which awit does not exempt and never rewrites around.

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

`awit import <issue-url> [--brief <summary>] [--alias X] [--tea-login name]`
creates an item from an existing Gitea issue through `tea` or GitLab issue
through `glab`. GitLab is recognized only by the `/-/issues/` or
`/-/work_items/` URL shape — never by host — and both spellings of the
same issue are one identity. The item is a one-time snapshot: Gitea
`number` or GitLab `iid` as `external.id`, the decoded body/description
byte-exact, remote labels first-seen exact-name deduplicated (local
default labels are not merged), and `open`/`closed` (GitLab wire
`opened`→`open`) mapped to the same local status (other states are
refused). The stored URL is the validated input URL. `--tea-login` is
Gitea-only and ignored for GitLab. Import never writes remote state and
never commits. Duplicate identity is `(tracker, normalized installation
base, exact repo, iid)` across active items and archived item files;
Gitea and GitLab with the same host/repo/number do not collide. Re-import
is refused with the existing item or archive path named.

An omitted (or empty) `--brief` is derived after fetch validation and before
the mutation lock: the normalized remote title when it holds any
non-whitespace rune, else the body's first sentence (`.`/`!`/`?` followed by
whitespace or end-of-source, the `sentenceCount` boundary; newlines alone
never split). Normalization trims outer Unicode whitespace and collapses
each inner run to one ASCII space; derived values are capped at 240 Unicode
code points (`…` ellipsis, no space before it). An explicit `--brief` is
stored verbatim and uncapped; a blank remote title is stored unchanged and
never replaced by the fallback. Blank title plus empty body with no explicit
override exits 1 (`cannot derive import brief: remote title and body are
empty; pass --brief`) before minting.

`awit external check [key] [--tea-login name]` compares the raw local
body bytes (`Item.Body()`: every byte after the frontmatter closing
fence) against the linked issue body — Gitea through `tea`, GitLab
through `glab` — and nothing else: frontmatter, title, labels, comments,
and remote state are ignored, so any whitespace, line-ending,
final-newline, or leading-blank-line difference is drift. `--tea-login`
is Gitea-only and ignored for GitLab. The command is read-only: no local
bytes and no remote fields change. Plain output prints one `MATCH`,
`DRIFT`, or `ERROR` line per linked item (each naming the item and its
URL) plus deterministic totals; `--format json` prints an array of
`{id, url, result, detail}` with no human lines on stdout. Exit 1 when
any row is drift or error, otherwise 0. An authentication or read failure
is an error row, never drift. The all-items form walks linked items in
canonical-ID order and skips genuinely unlinked ones (reporting the
number checked, including zero); naming an explicitly unlinked item is an
error.

`awit external push-body <key> [--tea-login name]` is the explicit
local-canonical repair: it pushes the local body bytes to the linked
issue and nothing else (no title, label, state, local-content, or history
change). The push holds the store lock until the bounded request
completes, refuses ambiguous duplicate external links, and verifies the
remote took the exact bytes — against the write response, or a GET of the
same issue when the response omits the body. A mismatch is an error,
never success. Gitea requires a 2xx status (tea exits zero on HTTP
errors, so the `--include` status line is authoritative); GitLab requires
a 2xx status the same way (glab exits nonzero on HTTP errors) and
refuses column-zero `/command` bodies before any mutation, since GitLab
would execute them as quick actions instead of storing them.

`awit close <id> [--tea-login name] [--push=true|false] [--no-push]`, `awit release <id>` with
the same flags, and `awit update <id> --status <s>` with the same flags
propagate local state one way to the linked issue: `close` pushes `closed`,
`release` pushes `open`, and an explicit `--status` pushes `closed` for
`closed` and `open` for `open`/`in_progress` (a non-status update never
pushes, and neither do `next --claim`, create, import, comment, ref, or
archive). `--tea-login` is Gitea-only and ignored for GitLab. Automatic
pushes follow `.awit/config.yaml` `external_push:` (omitted → true);
`--push=true|false` overrides per invocation (true `--no-push` equals
`--push=false`; `--no-push=false` is neutral). The local
item is saved first — keeping close's reason comment and claim-clearing —
then the push runs under the held store lock with the bounded subprocess
deadline (Gitea: `tea api --login <login> --repo <owner/repo> --include
-X PATCH -f state=<open|closed> repos/<owner>/<repo>/issues/<n>`; GitLab:
`glab api` PUT of `state_event=<close|reopen>` on the single-segment
`%2F`-encoded project endpoint), requiring a 2xx status and verifying
the response confirms the issue number, installation, and state (GET
fallback when the response omits it; the remote is never read to decide).
Local state stays canonical: remote failure, a missing tool, invalid
metadata, or ambiguous duplicate links keep the local mutation and its
ordinary confirmation, print one stderr
`warning: <id> saved locally; external state push failed: <reason>; retry with awit update <id> --status <status>`
(exit 0), while a local write failure exits 1 and never pushes.
Repeating `update <id> --status <current>` re-pushes without a queue or
daemon. A config-disabled skip of a linked item prints
`warning: <id> saved locally; external state push skipped by config external_push: false; push with awit update <id> --status <status> --push=true`
after the local save and before tool discovery; `--no-push` and `--push=false`
are silent and perform no tool discovery, auth, or network
operation, even with malformed metadata.

Byte-exactness relies on a tested `tea` behavior: `tea`'s `-F body=@file`
reader strips exactly one terminal LF, so awit writes the transport file
as the body plus one extra LF (private temp-then-rename file, mode 0600,
deleted on every exit). GitLab needs no such adaptation: `glab` sends
`-F description=@file` byte-exact, so the transport file carries exactly
the body bytes, verified by `TestGlabBodyRoundTrip` (empty, no-LF,
multi-LF, CRLF, leading blanks, Unicode, backticks, and literal `null`
round-trip byte-exact). Supported: `tea` 0.16.0, verified by
`TestTeaBodyRoundTrip` over the same edges. A `tea` or `glab` build that
fails its compatibility test must not be advertised as supported.
Ordinary `awit validate` stays offline: it works with both tools absent
and networking disabled.

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
none of these — it is a `validate` WARN only. A malformed
`blocked_reason` (wrong type, empty content, control characters,
duplicate key) is a `PARSE ERROR`, not a new category: the hold fails
closed and the item is never selectable until the declaration is readable.
