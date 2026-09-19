---
id: AWIT-0NHDC2DN
title: 'ref: add and remove repo-root references with explicit legacy migration'
brief: >-
  Add ref add/rm and make new references repo-root-relative. Preserve old items-relative refs through an explicit per-item base marker and deterministic rewrite-on-touch, not filesystem-existence guessing.
status: closed
deps: [AWIT-0ND56Z3G, AWIT-0ND5703G, AWIT-0ND56Y3G, AWIT-0NE610DS]
labels: [phase4, p1]
refs_base: repo
refs: [.awit/comments/AWIT-0NHDC2DN/20260919T152555Z-orchestrator.md]
---
## Summary

Add `awit ref add <key> <path>` and `awit ref rm <key> <path>`. New refs use `docs/notes/x.md` and `.awit/comments/<id>/file.md`, relative to the root containing `.awit`.

## Context (read first)

- Guide §§2, 4.3, 4.4, 4.9, archive layout/comment collapse.
- `resolver.Resolve`, `IsItemRef`; all show helper callsites.
- `Store.AddComment`, `AttachFile`, `Archive`, `Save` currently assume items-relative strings.
- Existing files may contain bare `OTHER.md`, `../comments/...`, and `../../docs/...`; existence-based fallback can select the wrong file when both bases contain a path.

## Files

- Add `internal/cli/ref.go`, `internal/cli/ref_test.go`; register in app.go.
- Modify `pkg/item/{item,store,comment,archive}.go`, `pkg/resolver/resolver.go`, `internal/cli/show.go`, relevant tests/fixtures/goldens.
- Update schema Layout/Refs/Comments, guide §§2/4/6/8, spec Data model/Archive/Phase 4, README, embedded skill and generated copy.

## Interfaces

```go
// Optional serialized field; empty means historical items base.
// Item.RefsBase string
func (it *Item) SetRefsBase(base string) error // only "repo" or empty
func (s *Store) NormalizeRefs(it *Item) error
```

- New items include `refs_base: repo` immediately before `refs`. Absence means the original `.awit/items/` base, including archived legacy items. Invalid marker types/values are normal parse errors.
- No root-first/existence-based probing. A marker determines the base even if a file is missing. `resolver.Resolve(baseDir, refs)` keeps the same Go types/signature but its documented parameter becomes a general base directory; show chooses `Store.Root` for marked items, `ItemsDir()` otherwise. `IsItemRef` still receives ItemsDir.
- Normalize every old ref on first successful item mutation using `filepath.Rel(Store.Root, filepath.Join(ItemsDir(), oldRef))`, then ToSlash, and set the marker atomically in the same item write. No existence check and no all-item migration command are required. Preserve references to outside-root files as root-relative `../...` when mathematically representable; do not silently retarget or delete them. Cross-volume paths that cannot be represented error before the write.
- `Store.Save` calls NormalizeRefs before serialization. `Store.Archive` also normalizes before its direct Bytes/archive write. Comment/attachment mutation normalizes **before** appending new root-relative refs, avoiding a mixed-base list.
- Ref command path arguments are root-relative regardless of cwd. Normalize separators/dot segments; reject absolute paths and empty paths. Missing targets are allowed so planning refs can precede documents; existing show missing reporting remains. This change is not a new filesystem sandbox.
- Add deduplicates normalized paths, preserves other ref order, and reports `ref added <id>: <path>` or `ref already present <id>: <path>`. Remove compares normalized root-relative paths, removes the entry, and reports `ref removed <id>: <path>`; absent refs error without saving. Both hold Store.Lock, respect walked-up notes, never copy/delete the target and never commit.
- Archive comment recognition uses normalized `.awit/comments/<id>/` paths; attachment relocation writes `.awit/archive/<id>/<file>`. Non-comment refs remain valid independent of the archive directory’s depth.
- Legacy compatibility is intentional and limited to the absent marker. Once a file is touched, it has the new root-relative representation; it never writes a legacy representation again.

## Steps

- [ ] Write failing tests with the same lexical ref existing at both possible bases, missing refs, old direct item refs, nested cwd, comments/attachments, archive after migration, and interruption/no-partial-write behavior. Assert resolved target identity, not merely changed strings.
- [ ] Run `go test ./pkg/item ./pkg/resolver ./internal/cli -run 'Ref|Archive|Comment|Attach|ShowFull' -count=1`; capture failures for the new contract.
- [ ] Implement the marker/setter, NormalizeRefs, save/archive/comment integration, and explicit base selection throughout show. Implement ref add/rm using normal atomic save and lock conventions.
- [ ] Update fixture/golden expectations only where new writes now contain root-relative refs/marker; keep legacy fixtures to prove compatibility and no read-time mutation. Do not mass-edit closed work-item bodies or audit comments.
- [ ] Rerun targeted tests, update all documented reference examples and guide signatures, regenerate skill output from its asset, and hand evidence to the orchestrator.

## Acceptance Criteria

- The scoped test command above passes on Linux and Windows.
- `awit ref add <id> docs/notes/x.md` stores that path, not `../../docs/notes/x.md`; running from a nested cwd resolves to the same root target.
- A legacy item remains byte-identical after `show --full`. After `awit update <id> --brief 'Updated summary.'`, refs migrate with `refs_base: repo` and resolve to the exact former targets.
- `awit ref rm <id> docs/notes/x.md` removes only the reference; the target file remains.
- Comment attachment creation followed by archive retains attachment bytes and resolvability; graph validation remains unchanged.

## Out of scope

Guessing bases from file existence, rewriting Markdown links inside bodies, recursive traversal changes, a bulk migration command, restricting all existing refs to an in-repo sandbox.

