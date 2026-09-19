---
id: AWIT-0NJC0BDQ
title: 'ref: reject missing add targets unless allow-missing is explicit'
brief: >-
  Reject mistyped reference paths when adding them, naming the resolved absolute target without saving the item. Preserve future-document references with --allow-missing and leave historical reference normalization non-validating.
status: open
deps: []
labels: [phase4, p2]
refs_base: repo
refs: []
---
## Summary

Make `awit ref add <id> <path>` check the proposed target's existence before modifying or saving the item. Add `--allow-missing` to `ref add` only for intentional planning references. Reference storage remains normalized, forward-slash and repository-root-relative regardless of cwd.

## Context (read first)

- `AGENTS.md`; guide §§1, 2, 4.4, 4.9 and 5.
- `.awit/archive/AWIT-0NHDC2DN.md`, closed. Its unconditional missing-target allowance becomes explicit opt-in; its non-validating migration contract remains intact.
- `internal/cli/ref.go`: `refCmd`, `normalizeRefArg`, `refAdd`, `refRm`.
- `pkg/item/store.go`: `Store.NormalizeRefs` and `Save`.
- `internal/cli/ref_test.go`, especially `TestRefAddMissingTargetAllowed`, nested-cwd and legacy-migration tests.
- `pkg/item/refs_test.go`: `TestNormalizeRefsMissingStillRewrites`, outside-root and cross-volume behavior.

## Files

- Modify `internal/cli/ref.go` and `internal/cli/ref_test.go`.
- Keep `pkg/item/store.go` and `pkg/resolver` behavior unchanged; retain their migration/resolution regressions.
- Update `docs/schema.md` reference rules under Item files, `plan/implementation-guide.md` §§2/4.4, `plan/awit-implementation-plan.md` Data model/CLI command matrix/Phase 4, `README.md` Commands, and `internal/skill/assets/driving-awit.body.md` Vocabulary/Quick reference.
- Regenerate `.omp/skills/driving-awit/SKILL.md` from the source asset.

## Interfaces

Existing signatures remain unchanged:

```go
func normalizeRefArg(p string) (string, error)
func refAdd(cmd *cli.Command, id, raw string) error
func (s *Store) NormalizeRefs(it *Item) error
```

Add a boolean flag named exactly `allow-missing` to the `ref add` subcommand, default false, read via `cmd.Bool("allow-missing")`.

Processing order and errors:

1. Normalize the user argument with existing `normalizeRefArg`; retain its rejection of absolute/empty paths and its separator/dot-segment handling.
2. Open the store, preserve the walked-up note, acquire the existing store lock, and load the target item with `loadItem` as today.
3. Resolve the proposed path using `filepath.Abs(filepath.Join(s.Root, filepath.FromSlash(p)))`. This is a cleaned lexical absolute path, not a symlink-canonicalized path; do not use cwd as its base and do not introduce `EvalSymlinks`.
4. Call `os.Stat` on that path **before** `NormalizeRefs`, deduplication, setters, or `Save`.
5. If the error satisfies `errors.Is(err, os.ErrNotExist)` and `--allow-missing` is false, return an ordinary error, producing exit 1 and exactly:

```text
Error: ref target does not exist: "<resolved-absolute-path>"; use --allow-missing to add it anyway
```

Format the path with `%q`; on Windows this displays an escaped native-separator path. A walked-up note may precede the error, as before.

6. `--allow-missing` waives only not-exist errors. Other stat errors still exit 1 without saving, with `Error: cannot stat ref target "<resolved-absolute-path>": <underlying-error>`. Propagate absolute-path resolution errors without writing.
7. After a successful check or permitted absence, use existing `NormalizeRefs`, deduplication, append and `Save` behavior. Success text stays `ref added ...` or `ref already present ...`.

Decisions and boundaries:

- Check even a duplicate add: repeating an add of a now-missing target errors unless `--allow-missing` is supplied; with the flag it returns the existing idempotent confirmation without rewriting.
- `os.Stat` follows symlinks. A dangling symlink is a missing target; permission failures are not excused by the flag.
- This is **existence-only**, not a new readability/type/security validator. Existing directories remain admissible; read-time resolution may still report its existing error. Outside-root `../` references remain allowed. Neither referenced content nor files are created, copied, removed or committed.
- Use existing native Windows absolute-path conventions, `filepath` for filesystem paths, and existing backslash normalization for ref input. Include native drive/UNC absolute cases in Windows tests; do not invent a new cross-platform Windows path parser.
- `Store.NormalizeRefs`, `Store.Save`, `update`, import, archive, and ref removal do not gain existence checks. Otherwise a deleted historical document could block unrelated status changes or archive, and migration would become filesystem-dependent. Import still copies issue bodies rather than validating Markdown links.
- The check is a point-in-time guard against typos, not a guarantee against another process deleting the target afterward. Keep read-time missing reporting.

## Steps

- [ ] Replace `TestRefAddMissingTargetAllowed` with a default-rejection regression plus an explicit `--allow-missing` preservation test. Add before/after item-byte assertions on rejection, including a legacy item whose marker must not be persisted. Cover nested cwd, normalized backslashes, a duplicate missing ref, existing files/directories, and non-not-exist errors where portable.

Representative default-rejection test:

```go
func TestRefAddMissingTargetDoesNotSave(t *testing.T) {
    dir := dualBaseRepo(t)
    itemPath := filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md")
    before, err := os.ReadFile(itemPath)
    if err != nil { t.Fatal(err) }
    code, stdout, stderr := run(t, "--repo", dir, "ref", "add", "AWIT-TEST0001", "docs/planned.md")
    want := fmt.Sprintf("Error: ref target does not exist: %q; use --allow-missing to add it anyway\n", filepath.Join(dir, "docs", "planned.md"))
    if code != 1 || stdout != "" || stderr != want {
        t.Fatalf("exit %d, stdout %q, stderr %q; want %q", code, stdout, stderr, want)
    }
    after, err := os.ReadFile(itemPath)
    if err != nil { t.Fatal(err) }
    if !bytes.Equal(before, after) { t.Fatal("rejected add saved the item") }
}
```

- [ ] Run `go test ./internal/cli ./pkg/item -run 'Ref|UpdateMigratesLegacy' -count=1`; record failure because the current command accepts the missing target and does not recognize the escape hatch.
- [ ] Add the flag and the small existence check directly to `refAdd`, before normalization/deduplication. Use `os.Stat` and `errors.Is`; do not add a store-wide validator or change public interfaces.
- [ ] Preserve and run `TestNormalizeRefsMissingStillRewrites`; add a command-level missing-legacy-ref update check if not already covered. Ensure planned refs still render `[missing]` through the existing show path. Run the scoped command to green, including platform-native cases on Windows.
- [ ] Smoke the built CLI in a temporary repository: missing add exits 1 with no file diff; adding the same path with `--allow-missing` succeeds; `show --full` reports missing; creating the document makes the same ref resolve without editing the item.
- [ ] Update the listed docs and regenerate the skill copy. Run `go test ./internal/skill -run TestDogfoodOmpCopyMatchesRenderer -count=1`; hand scoped evidence to the orchestrator for review/commit, without project-wide validation.

## Acceptance Criteria

- `go test ./internal/cli ./pkg/item -run 'Ref|UpdateMigratesLegacy' -count=1` passes on Linux and Windows.
- In a temporary repository, `awit ref add <id> docs/typo.md` exits 1, names the resolved absolute path in the specified error, prints no success line, and leaves item bytes unchanged.
- `awit ref add <id> docs/typo.md --allow-missing` exits 0 and stores `docs/typo.md`. `show --full` retains its existing `[missing]` reporting; creating that target later makes the reference resolve.
- Running from nested cwd produces the same resolved target. Ref storage uses forward slashes; error paths use `%q` on the native absolute path. Existing absolute/empty-path rejection remains in effect even with the escape hatch.
- Rejected legacy-item adds do not persist migration. An unrelated `awit update <id> --brief 'Updated summary.'` still migrates missing historical refs and succeeds; removal still works after a target disappears.
- No target is copied/deleted, no commit is introduced, no existence check is added to import/save/migration, and docs/source/generated skill match.

## Out of scope

Checking Markdown links; global reference validity enforcement; changing reference bases or migration; requiring regular/readable files; prohibiting outside-root refs; resolving symlinks to new stored paths; creating planned documents; changing read-time missing reporting; editing archived work-item history.

---


