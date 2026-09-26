package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/item"
)

// writeLabelItem drops a raw item file into a copied fixture. text is the
// complete file: frontmatter fences included.
func writeLabelItem(t *testing.T, repo, name, text string) {
	t.Helper()
	path := filepath.Join(repo, item.DirName, "items", name)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// enrichLabelFixture returns a copy of the clean fixture plus
// AWIT-TEST0007 (closed, labels [auth]) and AWIT-TEST0008 (in_progress,
// labels [db]), so the three --state values have vocabularies to differ in.
func enrichLabelFixture(t *testing.T) string {
	t.Helper()
	repo := copyFixture(t, "clean")
	writeLabelItem(t, repo, "AWIT-TEST0007.md", `---
id: AWIT-TEST0007
title: Retire legacy session cookies
brief: Remove the cookie fallback once bearer auth ships.
status: closed
deps: []
labels: [auth]
refs: []
---

## Summary

Closed work that used to carry the auth label.

## Acceptance Criteria

- Cookie fallback is gone
`)
	writeLabelItem(t, repo, "AWIT-TEST0008.md", `---
id: AWIT-TEST0008
title: Harden refresh-token storage
brief: Encrypt refresh tokens at rest before rotation ships.
status: in_progress
deps: []
labels: [db]
refs: []
---

## Summary

Refresh tokens are stored in plain text today.

## Acceptance Criteria

- Refresh tokens are encrypted at rest
`)
	return repo
}

// The pristine clean fixture: open vocabulary is auth 1, db 1, p0 1, p1 1
// (0005 closed and 0006 in_progress carry no labels; 0003 has none either).
// All counts tie at 1, so the order is the label-ascending tie-break.
func TestLabelDefaultOpen(t *testing.T) {
	repo := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", repo, "label")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
	}
	want := "auth 1\ndb 1\np0 1\np1 1\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

// The only closed item in clean (0005) carries no labels: the closed
// vocabulary is empty. Compact prints nothing, json prints "[]", exit 0.
func TestLabelClosed(t *testing.T) {
	repo := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", repo, "label", "--state", "closed")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	code, stdout, stderr = run(t, "--repo", repo, "--format", "json", "label", "--state", "closed")
	if code != 0 {
		t.Fatalf("json exit %d, want 0 (stderr %q)", code, stderr)
	}
	if stdout != "[]\n" {
		t.Fatalf("json stdout = %q, want %q", stdout, "[]\n")
	}
}

// all counts every parseable item: auth on 0001+0007, db on 0002+0008,
// p0 on 0004, p1 on 0001. auth and db tie at 2 and order label-ascending.
func TestLabelAll(t *testing.T) {
	repo := enrichLabelFixture(t)
	code, stdout, stderr := run(t, "--repo", repo, "label", "--state", "all")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
	}
	want := "auth 2\ndb 2\np0 1\np1 1\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

// open means "status != closed": the closed 0007 (auth) drops out, the
// in_progress 0008 (db) stays in, and the count-desc sort puts db (2) first.
func TestLabelOpenExcludesClosedIncludesInProgress(t *testing.T) {
	repo := enrichLabelFixture(t)
	code, stdout, stderr := run(t, "--repo", repo, "label")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
	}
	want := "db 2\nauth 1\np0 1\np1 1\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	// On the same copy, closed sees only 0007's label.
	code, stdout, stderr = run(t, "--repo", repo, "label", "--state", "closed")
	if code != 0 {
		t.Fatalf("closed exit %d, want 0 (stderr %q)", code, stderr)
	}
	if stdout != "auth 1\n" {
		t.Fatalf("closed stdout = %q, want %q", stdout, "auth 1\n")
	}
}

func TestLabelBadState(t *testing.T) {
	repo := copyFixture(t, "clean")
	const msg = "Error: --state must be open, closed or all\n"
	for _, state := range []string{"bogus", "OPEN", ""} {
		t.Run(state, func(t *testing.T) {
			code, stdout, stderr := run(t, "--repo", repo, "label", "--state", state)
			if code != 1 {
				t.Fatalf("exit %d, want 1 (stderr %q)", code, stderr)
			}
			if stderr != msg {
				t.Fatalf("stderr = %q, want %q", stderr, msg)
			}
			if stdout != "" {
				t.Fatalf("stdout = %q, want empty", stdout)
			}
		})
	}
}

func TestLabelJSON(t *testing.T) {
	repo := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", repo, "--format", "json", "label")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
	}
	want := `[
  {
    "label": "auth",
    "count": 1
  },
  {
    "label": "db",
    "count": 1
  },
  {
    "label": "p0",
    "count": 1
  },
  {
    "label": "p1",
    "count": 1
  }
]
`
	if stdout != want {
		t.Fatalf("stdout =\n%s\nwant\n%s", stdout, want)
	}
}

func TestLabelTableGolden(t *testing.T) {
	repo := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", repo, "--format", "table", "label")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
	}
	golden(t, "label_clean_table.golden", []byte(stdout))
}

// 0001 in the dangling fixture has deps: [AWIT-TEST9999], so the graph
// quarantines it - but it parses, so its auth and p1 must count. The extra
// AWIT-TEST0003 is invalid YAML, so its ghost label must not count at all.
func TestLabelCountsQuarantinedSkipsBroken(t *testing.T) {
	repo := copyFixture(t, "dangling")
	writeLabelItem(t, repo, "AWIT-TEST0003.md", `---
id: AWIT-TEST0003
title: [unclosed
status: open
deps: []
labels: [ghost]
refs: []
---

Invalid YAML on purpose; this label must not count.
`)
	code, stdout, stderr := run(t, "--repo", repo, "label")
	if code != 0 {
		t.Fatalf("exit %d, want 0 (stderr %q)", code, stderr)
	}
	want := "auth 1\np1 1\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q (ghost must be absent, quarantined 0001 counted)", stdout, want)
	}
}

func TestLabelCountsIncludesUsedUndeclaredExcludesUnusedDeclared(t *testing.T) {
	dir := initRepo(t)
	writeDefaultLabels(t, dir, []byte("prefix: AWIT\nlabels: [p1]\nstale_claim: 2h\n"))
	seedItem(t, dir, "AWIT-TEST0001", "T", "A test item.", []string{"typo"})
	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "label", "--state", "all")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	var rows []format.LabelCount
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("json: %v (stdout %q)", err, stdout)
	}
	if len(rows) != 1 || rows[0].Label != "typo" || rows[0].Count != 1 {
		t.Fatalf("rows = %+v, want [{typo 1}] (used undeclared in, declared unused out)", rows)
	}
}

// A bad --format value must surface Detect's error, not be swallowed.
func TestLabelBadFormat(t *testing.T) {
	repo := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", repo, "--format", "yaml", "label")
	if code != 1 {
		t.Fatalf("exit %d, want 1 (stderr %q)", code, stderr)
	}
	want := "Error: format: unknown format \"yaml\" (compact|table|json)\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
}
