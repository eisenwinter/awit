---
id: AWIT-0NZJPBSK
title: 'item: Store.LoadArchive via shared loadDir'
brief: >-
  The archive directory has a writer but no reader; LoadArchive reuses LoadAll's parsing and duplicate rules on .awit/archive so the TUI can browse archived items without new parsing code.
status: closed
deps: []
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---
## Summary

`.awit/archive/<id>.md` files are ordinary item files with comments collapsed under `## Comments` (`pkg/item/archive.go:52-65`). `LoadAll` already knows how to read a directory of item files, detect conflict markers, ID mismatches and duplicate stems, and skip sub-directories. Factor its body into `loadDir(dir)` and expose `LoadArchive()` over `ArchiveDir()`. A missing archive directory is an empty result, not an error.

## Context (read first)

- `docs/design-spec.md` §3 (filesystem layout), §4 "Archival Engine".
- `pkg/item/store.go:159-232` — `LoadAll`; the whole body moves into `loadDir` unchanged.
- `pkg/item/archive.go:16-85` — attachments move to `ArchiveDir()/<id>/`, a directory the `e.IsDir()` skip in the loader already ignores.
- `pkg/item/archive_test.go:16` — `archiveStore(t)` writes AWIT-TEST0004 with three comment files (one `.log` attachment).

## Files

- `pkg/item/store.go` — `loadDir`, `LoadAll` one-liner, `LoadArchive`.
- `pkg/item/archive_test.go` — `TestLoadArchive*`.

## Interfaces

```go
// LoadArchive reads .awit/archive/<id>.md files with the same parsing and
// duplicate rules as LoadAll. A missing archive directory yields nil, nil, nil.
func (s *Store) LoadArchive() ([]*Item, []Broken, error)

## Comments

### 2026-09-24T21:24:34Z jan

implemented
