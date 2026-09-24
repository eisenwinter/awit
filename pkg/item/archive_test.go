package item

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// archiveStore inits a store in a temp dir and writes AWIT-TEST0004 with
// the three comment files from the testdata/fixtures/archive fixture,
// bytes identical, so the CLI goldens in internal/cli match.
func archiveStore(t *testing.T) *Store {
	t.Helper()
	s, err := Init(t.TempDir(), "AWIT")
	if err != nil {
		t.Fatal(err)
	}
	test0004 := `---
id: AWIT-TEST0004
title: Fix URL-safe base64 token rejection
brief: The middleware rejects URL-safe bearer tokens before signature checks.
status: closed
deps: []
labels: []
refs:
  - ../comments/AWIT-TEST0004/20260916T090000Z-jan.md
  - ../comments/AWIT-TEST0004/20260916T091500Z-jan.log
  - ../comments/AWIT-TEST0004/20260916T093000Z-claude.md
---

## Summary

URL-safe base64 characters in bearer tokens must authenticate.

## Acceptance Criteria

- Tokens from the URL-safe alphabet authenticate successfully
`
	if err := os.WriteFile(s.ItemPath("AWIT-TEST0004"), []byte(test0004), 0o644); err != nil {
		t.Fatal(err)
	}
	dir := s.CommentsDir("AWIT-TEST0004")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"20260916T090000Z-jan.md": `---
author: jan
created: 2026-09-16T09:00:00Z
---

Started on the spec; grammar section drafted.
`,
		"20260916T091500Z-jan.log": `2026-09-16T09:15:00Z PASS TestHeaderGrammar
2026-09-16T09:15:01Z PASS TestUnauthorizedPayload
`,
		"20260916T093000Z-claude.md": `---
author: agent/claude
created: 2026-09-16T09:30:00Z
---

Reviewed. Two nits fixed inline:

- header grammar now cites RFC 6750 §2.1
- 401 body example uses ` + "`error_description`" + `
`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func TestCommentsSortedAndClassified(t *testing.T) {
	s := archiveStore(t)
	got, err := s.Comments("AWIT-TEST0004")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Author != "jan" || got[0].Attachment || got[0].Text != "Started on the spec; grammar section drafted." {
		t.Fatalf("first = %+v", got[0])
	}
	if !got[1].Attachment || got[1].File != "20260916T091500Z-jan.log" || got[1].Author != "" {
		t.Fatalf("second = %+v", got[1])
	}
	if got[2].Author != "agent/claude" || got[2].Created.Format(time.RFC3339) != "2026-09-16T09:30:00Z" {
		t.Fatalf("third = %+v", got[2])
	}
}

func TestCommentsMissingDir(t *testing.T) {
	s := archiveStore(t)
	got, err := s.Comments("AWIT-TEST0001")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestArchiveMovesEverything(t *testing.T) {
	s := archiveStore(t)
	it, err := s.Load("AWIT-TEST0004")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Archive(it); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.ItemPath("AWIT-TEST0004")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("item still present: %v", err)
	}
	if _, err := os.Stat(s.CommentsDir("AWIT-TEST0004")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("comments dir still present: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.ArchiveDir(), "AWIT-TEST0004", "20260916T091500Z-jan.log")); err != nil {
		t.Fatalf("attachment not moved: %v", err)
	}
	data, err := os.ReadFile(s.ArchivePath("AWIT-TEST0004"))
	if err != nil {
		t.Fatal(err)
	}
	arch, err := Parse(s.ArchivePath("AWIT-TEST0004"), data)
	if err != nil {
		t.Fatalf("archive file must still parse: %v", err)
	}
	wantRefs := []string{".awit/archive/AWIT-TEST0004/20260916T091500Z-jan.log"}
	if !slices.Equal(arch.Refs, wantRefs) {
		t.Fatalf("refs = %v, want %v", arch.Refs, wantRefs)
	}
	if !bytes.Contains(data, []byte("\n## Comments\n\n### 2026-09-16T09:00:00Z jan\n\nStarted on the spec; grammar section drafted.\n\n### 2026-09-16T09:30:00Z agent/claude\n\nReviewed.")) {
		t.Fatalf("comments not collapsed:\n%s", data)
	}
}

func TestArchiveNoCommentsNoSection(t *testing.T) {
	s := archiveStore(t)
	it := New("AWIT-TEST0001", "lone", "b", nil, nil)
	it.SetStatus(StatusClosed)
	if err := s.Save(it); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(s.ItemPath("AWIT-TEST0001"))
	if err := s.Archive(it); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(s.ArchivePath("AWIT-TEST0001"))
	if !bytes.Equal(before, after) {
		t.Fatalf("no-comment archive must be byte-identical\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestArchiveIdempotentAfterCrash(t *testing.T) {
	s := archiveStore(t)
	it, _ := s.Load("AWIT-TEST0004")
	// Simulate a crash after the archive file was written but before deletes.
	if err := os.MkdirAll(s.ArchiveDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.ArchivePath("AWIT-TEST0004"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Archive(it); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(s.ArchivePath("AWIT-TEST0004"))
	if bytes.Equal(data, []byte("stale")) {
		t.Fatal("archive file was not rebuilt")
	}
}

func TestLoadArchiveEmptyWhenMissing(t *testing.T) {
	s := archiveStore(t)
	items, broken, err := s.LoadArchive()
	if err != nil || items != nil || broken != nil {
		t.Fatalf("LoadArchive on missing dir = %v, %v, %v; want nil, nil, nil", items, broken, err)
	}
}

func TestLoadArchiveReadsArchivedItems(t *testing.T) {
	s := archiveStore(t)
	it, err := s.Load("AWIT-TEST0004")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Archive(it); err != nil {
		t.Fatal(err)
	}
	items, broken, err := s.LoadArchive()
	if err != nil {
		t.Fatal(err)
	}
	if len(broken) != 0 {
		t.Fatalf("broken = %v, want none", broken)
	}
	if len(items) != 1 || items[0].ID != "AWIT-TEST0004" || items[0].Status != StatusClosed {
		t.Fatalf("items = %+v, want one closed AWIT-TEST0004", items)
	}
	if !bytes.Contains(items[0].Body(), []byte("## Comments")) {
		t.Fatalf("archived body lacks collapsed comments:\n%s", items[0].Body())
	}
	if items[0].Path != s.ArchivePath("AWIT-TEST0004") {
		t.Fatalf("Path = %q, want %q", items[0].Path, s.ArchivePath("AWIT-TEST0004"))
	}
	// LoadAll no longer sees it.
	live, _, err := s.LoadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(live) != 0 {
		t.Fatalf("LoadAll after archive = %d items, want 0", len(live))
	}
}

func TestLoadArchiveQuarantinesBrokenFiles(t *testing.T) {
	s := archiveStore(t)
	if err := os.MkdirAll(s.ArchiveDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.ArchivePath("AWIT-TEST0009"), []byte("no frontmatter\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	items, broken, err := s.LoadArchive()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 || len(broken) != 1 || broken[0].Reason != ReasonParse {
		t.Fatalf("items=%v broken=%v", items, broken)
	}
}
