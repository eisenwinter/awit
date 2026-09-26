---
id: AWIT-0ND5753G
title: "Reserve `external:` key and document the schema"
brief: >-
  Keep external: off the Item struct, prove Parse then SetStatus then Bytes preserves external: gitlab#42, confirm validate ignores unknown keys, and land docs/schema.md covering config.yaml, item frontmatter, comments, filenames, and the reserved external: <provider>#<number> key.
status: closed
deps: [AWIT-0ND56D3G]
labels: [phase5, p2]
refs_base: repo
refs:
  - plan/implementation-guide.md
  - plan/awit-implementation-plan.md
---

## Summary

After this ticket `external:` is a documented reserved frontmatter key that v1 never reads. `Item` gains no `External` field. `TestParsePreservesExternalKey` shows `external: gitlab#42` survives `Parse` → `SetStatus` → `Bytes`. `TestValidateIgnoresUnknownKeys` PASSes without changing `validate.go` - unknown keys are already kept on the YAML node and are not quarantine reasons. `docs/schema.md` is the on-disk contract (config.yaml, item frontmatter, comment format, filenames, reserved `external: <provider>#<number>`). README Layout links that file.

## Context (read first)

- Guide §4.3 `Item` - exported fields are `ID, Title, Brief, Status, Deps, Labels, Assignee, ClaimedAt, Refs, Path` plus unexported `doc *yaml.Node` and `body []byte`. Comment already says "extra keys preserved inside doc". **Do not add `External string` or any other field.** Setters never delete keys they do not own; `SetStatus` only updates `status`.
- Guide §2 key order for **new** items: `id, title, brief, status, deps, labels, assignee, claimed_at, refs`. `New` does not write `external`. Imports that already have the key keep it because Parse stores the mapping node.
- Guide §1 - unknown keys preserved; round-trip of a parsed file is byte-identical when no setter ran (`AWIT-0ND56D3G` `TestParseUnknownKeyKept` already uses `external: gitlab#42` without a setter). This ticket adds the setter case.
- Spec Phase 5 - "Reserve `external:` frontmatter key (unused) for a future GitLab/GitHub mirror". Format: `external: <provider>#<number>` (example `gitlab#42`, `github#99`). v1 never reads it, never validates the provider, never fetches.
- Spec Data model - required keys `id`, `title`, `status`; unknown keys preserved on write. `config.yaml` holds `prefix`, optional `default_labels`, `stale_claim` (default `2h`), `agent_id`.
- Guide §2 comment format - frontmatter `author`, `created` (RFC3339 UTC), blank line, text. `--file` copies bytes verbatim, no frontmatter. Filename `<YYYYMMDDTHHMMSSZ>-<sanitised author><ext>`; collision `-2`, `-3` before the extension.
- Dep `AWIT-0ND56D3G` must be `status: closed`. Phase 5 means `validate` (`AWIT-0ND56S3G`) already exists; this ticket does not change `validate.go`. `TestValidateIgnoresUnknownKeys` is expected to **PASS on the first run** after you add it.
- `AWIT-0ND5743G` writes the full README and already links `docs/schema.md`. If that ticket has landed, do **not** rewrite README; only insert the Layout sentence if the link is missing. If README has no Layout section yet, append the Layout block from Step 7 and nothing else.
- Helpers (`AWIT-0ND56G3G`): `run`, `initRepo`. Do not redeclare them. `loadGraph` / `toEntry` are `AWIT-0ND56J3G`; this ticket does not need them.

## Files

- Modify: `pkg/item/item.go` - doc comment on `type Item` only. No new fields, no new methods.
- Modify: `pkg/item/item_test.go` - append `TestParsePreservesExternalKey` and `TestItemHasNoExternalField`.
- Modify: `internal/cli/validate_test.go` - append `TestValidateIgnoresUnknownKeys`. If that file is absent (S3G not on disk yet - it will be by phase 5), create it with **only** this test; do not redeclare `run` / `initRepo`.
- Create: `docs/schema.md` - the document in Step 5.
- Modify: `README.md` - Layout link only, and only if missing (Step 7).

## Interfaces

- Consumes (verbatim; do not change signatures):
  ```go
  func Parse(path string, data []byte) (*Item, error)
  func (it *Item) SetStatus(s Status)
  func (it *Item) Bytes() ([]byte, error)
  type Item struct { /* no External field */ }
  func run(t *testing.T, args ...string) (code int, stdout, stderr string)
  func initRepo(t *testing.T) string
  ```
- Produces: no new Go API. The reserved key lives only in YAML and in `docs/schema.md`.

## Steps

- [ ] **Step 1: Write the failing item tests.**

  Append to `pkg/item/item_test.go` (same package; do not create a new file). Add `"reflect"` to the import block if it is not already there.

  ```go
  func TestParsePreservesExternalKey(t *testing.T) {
  	raw := []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nexternal: gitlab#42\n---\nbody\n")
  	it, err := Parse("x.md", raw)
  	if err != nil {
  		t.Fatal(err)
  	}
  	it.SetStatus(StatusInProgress)
  	got, err := it.Bytes()
  	if err != nil {
  		t.Fatal(err)
  	}
  	if !bytes.Contains(got, []byte("external: gitlab#42")) {
  		t.Fatalf("external key dropped after SetStatus:\n%s", got)
  	}
  	if !bytes.Contains(got, []byte("status: in_progress")) {
  		t.Fatalf("status not updated:\n%s", got)
  	}
  	if bytes.Contains(got, []byte("status: open\n")) {
  		t.Fatalf("old status still present:\n%s", got)
  	}
  }

  func TestItemHasNoExternalField(t *testing.T) {
  	st := reflect.TypeOf(Item{})
  	for i := 0; i < st.NumField(); i++ {
  		if st.Field(i).Name == "External" {
  			t.Fatal("Item must not grow an External field; keep external: on the yaml node")
  		}
  	}
  }
  ```

  `TestParseUnknownKeyKept` already exists from `AWIT-0ND56D3G` (Parse → Bytes, no setter). Leave it. This test is the setter case.

- [ ] **Step 2: Run them.**

  ```bash
  go test ./pkg/item -run 'TestParsePreservesExternalKey|TestItemHasNoExternalField' -v
  ```

  Expected: `TestItemHasNoExternalField` PASS (there is no field). `TestParsePreservesExternalKey` PASS if `SetStatus` already edits the node in place (it does, after `AWIT-0ND56D3G`). If `TestParsePreservesExternalKey` FAILs because `external:` was dropped, the bug is in `SetStatus` / `Bytes` rebuilding YAML from struct fields - fix that in `item.go` by encoding `it.doc` (the mapping node), not a fresh struct marshal. Do **not** add a field to make the test pass.

  Either a FAIL you then fix, or an immediate PASS, is acceptable for this step; do not skip running it. Record the output.

- [ ] **Step 3: Doc comment on `Item`. Re-run.**

  In `pkg/item/item.go`, replace the comment immediately above `type Item struct` with:

  ```go
  // Item is one Markdown ticket under .awit/items/.
  //
  // Known frontmatter keys map to the exported fields below. Unknown keys are
  // kept on the unexported YAML mapping node and survive Bytes() after any
  // setter. The key "external" is reserved for a future GitLab/GitHub mirror
  // (value form "external: <provider>#<number>", e.g. gitlab#42) and is never
  // read in v1 - do not add an External field.
  type Item struct {
  ```

  Do not add a field. Do not add a getter. Do not special-case the key in `Parse`.

  ```bash
  go test ./pkg/item -run 'TestParsePreservesExternalKey|TestItemHasNoExternalField|TestParseUnknownKeyKept|TestSetStatusOneLineDiff|TestRoundTripByteIdentical' -v
  ```

  Expected: every listed test `--- PASS`. Then:

  ```bash
  gofmt -w pkg/item/item.go pkg/item/item_test.go
  git add pkg/item/item.go pkg/item/item_test.go
  git commit -m "item: reserve external key on the yaml node"
  ```

- [ ] **Step 4: `TestValidateIgnoresUnknownKeys` - write it, see it PASS.**

  Append to `internal/cli/validate_test.go` (create the file with `package cli` and the imports below if it does not exist; if it exists, only append the test and add missing imports). Helpers `run` and `initRepo` already live in `helpers_test.go` - do not redeclare them.

  ```go
  package cli

  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"

  	"github.com/eisenwinter/awit/pkg/item"
  )

  func TestValidateIgnoresUnknownKeys(t *testing.T) {
  	repo := initRepo(t)
  	raw := []byte(`---
  id: AWIT-TEST0001
  title: External reserved
  brief: An item that carries the reserved external key for a future mirror.
  status: open
  deps: []
  labels: []
  refs: []
  external: gitlab#42
  ---

  ## Summary

  Reserved key only.

  ## Acceptance Criteria

  - validate does not FAIL on external
  `)
  	path := filepath.Join(repo, item.DirName, "items", "AWIT-TEST0001.md")
  	if err := os.WriteFile(path, raw, 0o644); err != nil {
  		t.Fatal(err)
  	}
  	code, stdout, stderr := run(t, "--repo", repo, "validate")
  	if code != 0 {
  		t.Fatalf("validate exit %d stderr %q stdout %q", code, stderr, stdout)
  	}
  	if strings.Contains(stdout, "FAIL") || strings.Contains(stderr, "FAIL") {
  		t.Fatalf("unknown key treated as FAIL:\nstdout=%q\nstderr=%q", stdout, stderr)
  	}
  	if !strings.Contains(stdout, "PASS") {
  		t.Fatalf("validate stdout = %q, want PASS", stdout)
  	}
  }
  ```

  If `validate_test.go` already has `package cli` and overlapping imports, append **only** the function and add `"os"`, `"path/filepath"`, `"strings"`, `"github.com/eisenwinter/awit/pkg/item"` if missing.

  Run:

  ```bash
  go test ./internal/cli -run TestValidateIgnoresUnknownKeys -v
  ```

  Expected (no `validate.go` edit):

  ```text
  === RUN   TestValidateIgnoresUnknownKeys
  --- PASS: TestValidateIgnoresUnknownKeys
  PASS
  ok  	github.com/eisenwinter/awit/internal/cli
  ```

  If it FAILs because stdout is not the word `PASS` but still exit 0 with no `FAIL` (for example a golden compact report), keep the exit-0 and no-`FAIL` assertions and drop the `Contains("PASS")` line, then record the actual stdout in the closing comment. Do **not** teach `validate` about `external`. Do **not** add `external` to any allow-list; the point is that unknown keys need no allow-list.

  ```bash
  gofmt -w internal/cli/validate_test.go
  git add internal/cli/validate_test.go
  git commit -m "cli: validate ignores reserved external key"
  ```

- [ ] **Step 5: Write `docs/schema.md`.**

  Create `docs/schema.md` with exactly this document:

  ````markdown
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

  | Key              | Required | Rules                                                                   |
  | ---------------- | -------- | ----------------------------------------------------------------------- |
  | `prefix`         | yes      | `[A-Z][A-Z0-9]{1,7}`; missing → load error `config: prefix is required` |
  | `default_labels` | no       | strings; merged first-wins into `awit create -l`                        |
  | `stale_claim`    | no       | Go duration (`2h`, `90m`). Missing/zero → `2h`                          |
  | `agent_id`       | no       | raw identity; `AWIT_AGENT` overrides; `--author` overrides both         |

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

  | Key      | Type   | Rules                                                             |
  | -------- | ------ | ----------------------------------------------------------------- |
  | `id`     | string | `PREFIX-` plus 8 Crockford chars. Must equal the filename stem    |
  | `title`  | string | Non-empty                                                         |
  | `status` | enum   | `open`, `in_progress`, `closed`. Blocked is derived, never stored |

  Missing required keys or an unknown status → parse error → quarantine
  `PARSE ERROR`.

  ### Optional known keys

  | Key          | Type            | Rules                                                                                        |
  | ------------ | --------------- | -------------------------------------------------------------------------------------------- |
  | `brief`      | string          | One to three sentences. `create` requires `--brief`. `validate` warns when missing or longer |
  | `deps`       | list of ids     | Unknown id → `DANGLING DEP` on this item. Written flow style `[a, b]`                        |
  | `labels`     | list of strings | Free-form. `p0`–`p4` recommended for priority. Flow style                                    |
  | `assignee`   | string          | `human/<name>` or `agent/<id>`. Omitted when empty; `SetAssignee("")` deletes the key        |
  | `claimed_at` | RFC3339 UTC     | Seconds precision. Set by `--claim`; deleted by `release` and `close`                        |
  | `refs`       | list of paths   | Relative to `.awit/items/`, forward slashes. Block style. Always present, `[]` when empty    |

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

  | Key       | Rules                                                                                                   |
  | --------- | ------------------------------------------------------------------------------------------------------- |
  | `author`  | As resolved: `--author` verbatim, or `AWIT_AGENT` / `agent_id` with `agent/` prefix, or git `user.name` |
  | `created` | RFC3339 UTC, seconds                                                                                    |

  `--file` copies the source bytes verbatim - no frontmatter is added.

  After a comment or attachment is written, the item's `refs` gains a
  forward-slash entry `../comments/<id>/<filename>` and the item is saved.

  ## Filenames

  | Kind       | Pattern                                                                     |
  | ---------- | --------------------------------------------------------------------------- |
  | Item       | `<id>.md` in `items/`                                                       |
  | Comment    | `<YYYYMMDDTHHMMSSZ>-<sanitised-author>.md`                                  |
  | Attachment | `<YYYYMMDDTHHMMSSZ>-<sanitised-author><ext>` keeping the original extension |
  | Collision  | `-2`, `-3`, … immediately before the extension                              |

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
  ````

  Trailing newline on the file. Then:

  ```bash
  git add docs/schema.md
  git commit -m "docs: on-disk schema and reserved external key"
  ```

- [ ] **Step 6: Point README Layout at `docs/schema.md`.**

  If `README.md` already contains the substring `docs/schema.md` (ticket `AWIT-0ND5743G` landed first), do nothing to README.

  If it does not, open the `## Layout` section. After the layout tree (or after the paragraph that lists frontmatter keys), insert this sentence and no other README edits:

  ```markdown
  The on-disk schema, including the reserved `external:` key, is
  documented in [docs/schema.md](docs/schema.md).
  ```

  If there is no `## Layout` section, append it:

  ````markdown
  ## Layout

  ```text
  .awit/
  ├── config.yaml
  ├── items/PREFIX-XXXXXXXX.md
  └── comments/PREFIX-XXXXXXXX/<UTC seconds>-<author>.md
  ```
  ````

  The on-disk schema, including the reserved `external:` key, is
  documented in [docs/schema.md](docs/schema.md).

  ````

  Confirm:

  ```bash
  python3 - <<'PY'
  t = open('README.md', encoding='utf-8').read()
  assert 'docs/schema.md' in t
  s = open('docs/schema.md', encoding='utf-8').read()
  assert 'external: <provider>#<number>' in s
  assert 'never read' in s.lower() or 'never\nread' in s.lower() or 'never reads' in s
  assert 'config.yaml' in s
  assert 'YYYYMMDDTHHMMSSZ' in s
  print('schema docs ok')
  PY
  ````

  Expected: `schema docs ok`.

  If README changed:

  ```bash
  git add README.md
  git commit -m "docs: link schema from README Layout"
  ```

- [ ] **Step 7: Close ticket.**

  ```bash
  go test ./pkg/item -run 'TestParsePreservesExternalKey|TestItemHasNoExternalField' -v
  go test ./internal/cli -run TestValidateIgnoresUnknownKeys -v
  ```

  - Set `status: closed` in `.awit/items/AWIT-0ND5753G.md`.
  - Create `.awit/comments/AWIT-0ND5753G/<YYYYMMDDTHHMMSSZ>-<author>.md` with those two test outputs and `schema docs ok`.
  - Append the ref `../comments/AWIT-0ND5753G/<file>.md` to this ticket's `refs` (forward slashes, block style).
  - Commit:

    ```bash
    git add .awit/items/AWIT-0ND5753G.md .awit/comments/AWIT-0ND5753G
    git commit -m "tickets: close AWIT-0ND5753G"
    ```

## Acceptance Criteria

- `go test ./pkg/item -run TestParsePreservesExternalKey -v` PASS: after `Parse` → `SetStatus(in_progress)` → `Bytes()`, the bytes contain `external: gitlab#42` and `status: in_progress`.
- `go test ./pkg/item -run TestItemHasNoExternalField -v` PASS. `Item` has no exported or unexported field named `External`.
- The doc comment on `type Item` states that `external` is reserved, never read in v1, and must not become a struct field.
- `go test ./internal/cli -run TestValidateIgnoresUnknownKeys -v` PASS without edits to `validate.go`. Exit 0, no `FAIL` in stdout/stderr.
- `docs/schema.md` documents `config.yaml`, item frontmatter (required, optional, reserved `external: <provider>#<number>` never read in v1), comment format, filenames, and the lock file.
- `README.md` contains `docs/schema.md`.
- `gofmt -l pkg/item internal/cli/validate_test.go` prints nothing. No new Go package.

## Out of scope

- Implementing a GitLab/GitHub mirror, fetching issues, writing `external:` from any command.
- Adding `external` to `item.New` key order or to `awit create` flags.
- Changing `validate` rules, quarantine reasons, or `SetStatus` beyond keeping unknown keys.
- goreleaser, pre-commit hook file, version ldflags - `AWIT-0ND5743G`.
- `pkg/lock` - `AWIT-0ND5723G`.
- Rewriting the rest of README (Install, Commands table, Agent loop) - `AWIT-0ND5743G`.
