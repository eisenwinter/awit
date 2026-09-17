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

Paths inside frontmatter `refs` always use forward slashes, relative to
`.awit/items/`. On-disk paths use `filepath` (OS separators).

## `config.yaml`

```yaml
prefix: AWIT
default_labels: [p1]
stale_claim: 2h
agent_id: claude
```

| Key | Required | Rules |
| --- | --- | --- |
| `prefix` | yes | `[A-Z][A-Z0-9]{1,7}`; missing → load error `config: prefix is required` |
| `default_labels` | no | strings; merged first-wins into `awit create -l` |
| `stale_claim` | no | Go duration (`2h`, `90m`). Missing/zero → `2h` |
| `agent_id` | no | raw identity; `AWIT_AGENT` overrides; `--author` overrides both |

Unknown keys in `config.yaml` are not part of v1; `Load` decodes into a
struct and extra keys are dropped on the next `Write`. Do not put
`external:` here.

## Item files

Filename: `.awit/items/<id>.md`. The stem **must** equal the `id` key
(case-sensitive match after parse; a case-insensitive stem collision with
another file is `DUPLICATE ID`). The file is YAML frontmatter fenced by
`---` lines, then a Markdown body. `Bytes()` always writes `\n`; `Split`
also accepts `\r\n`.

```markdown
---
id: AWIT-0K7M2QX9
title: Implement OAuth2 bearer token extraction
brief: >-
  One to three sentences of prose summarizing the item.
status: open
deps: []
labels: [auth, p1]
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
| `labels` | list of strings | Free-form. `p0`–`p4` recommended for priority. Flow style |
| `assignee` | string | `human/<name>` or `agent/<id>`. Omitted when empty; `SetAssignee("")` deletes the key |
| `claimed_at` | RFC3339 UTC | Seconds precision. Set by `--claim`; deleted by `release` and `close` |
| `refs` | list of paths | Relative to `.awit/items/`, forward slashes. Block style. Always present, `[]` when empty |

New items written by `awit create` use key order
`id, title, brief, status, deps, labels, refs` and omit empty
`assignee` / `claimed_at`.

### Reserved key: `external`

```yaml
external: gitlab#42
```

Form: `external: <provider>#<number>` (examples: `gitlab#42`,
`github#99`). Reserved for a future GitLab/GitHub mirror. **v1 never
reads this key**: no struct field, no getter, no validate rule, no
fetch. `Parse` keeps it on the YAML node; `SetStatus` and every other
setter leave it in place; `Bytes()` emits it. Do not add `External` to
`pkg/item.Item`.

### Unknown keys

Any other frontmatter key is preserved the same way as `external`.
Teams may add their own fields without a schema change. `validate`
does not FAIL on unknown keys. A setter that does not own the key
must not delete it.

### Body

Everything after the closing `---` fence is raw Markdown, stored as
bytes. `create` seeds:

```text

## Summary

## Acceptance Criteria

```

(leading newline after the fence). Conflict marker lines
(`<<<<<<< `, `=======`, `>>>>>>> `) anywhere in the file quarantine
the item as `CONFLICT MARKERS`.

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

After a comment or attachment is written, the item's `refs` gains a
forward-slash entry `../comments/<id>/<filename>` and the item is saved.

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

## Quarantine (not schema, but load-time)

These are not extra keys; they are reasons a file fails to become a
healthy item: `PARSE ERROR`, `CONFLICT MARKERS`, `ID MISMATCH`,
`DUPLICATE ID`, `DANGLING DEP`, `CYCLE`. `external:` is none of these.
