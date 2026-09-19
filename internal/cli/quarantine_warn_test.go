package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// The one-line stderr summary every graph-reading command prints when its
// initial graph load holds quarantined items or broken item files. Same
// wording for N=1; stdout, goldens and exit codes stay untouched.
func quarantineWarning(n int) string {
	return fmt.Sprintf("warning: %d items quarantined, run awit validate\n", n)
}

// TestQuarantineWarningParseErrorJSONList covers a parse-error-only repo:
// stdout stays parseable JSON, exit stays 0, stderr holds exactly one
// warning counting the broken file.
func TestQuarantineWarningParseErrorJSONList(t *testing.T) {
	dir := copyFixture(t, "parse-error")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "list")
	if code != 0 {
		t.Fatalf("exit %d stderr %q stdout %q", code, stderr, stdout)
	}
	var rows []json.RawMessage
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("stdout is not parseable JSON: %v\n%s", err, stdout)
	}
	if want := quarantineWarning(1); stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

// TestQuarantineWarningCyclicCountsNodesNotFaults: the cyclic fixture has
// four quarantined nodes but only two CYCLE fault records (triangle +
// self-loop). The warning must count nodes, and prime keeps its GRAPH
// WARNINGS section while adding exactly one stderr summary.
func TestQuarantineWarningCyclicCountsNodesNotFaults(t *testing.T) {
	dir := copyFixture(t, "cyclic")
	code, stdout, stderr := run(t, "--repo", dir, "prime")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "=== GRAPH WARNINGS ===") {
		t.Fatalf("prime lost its GRAPH WARNINGS section:\n%s", stdout)
	}
	if want := quarantineWarning(4); stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

// TestQuarantineWarningFilteredOutStillCounts warns before list filters:
// --ready hides every quarantined row yet the count stays 4, and the list
// footer stays its own separate line after the normal output.
func TestQuarantineWarningFilteredOutStillCounts(t *testing.T) {
	dir := copyFixture(t, "cyclic")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "list", "--ready")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if strings.Contains(stdout, "QUARANTINED") {
		t.Fatalf("ready listing must not show quarantined rows:\n%s", stdout)
	}
	want := quarantineWarning(4) +
		"Note: 1 open. Claim a ready item with awit next --claim; close it with awit close <id> when the work is done.\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

// TestQuarantineWarningNoReadyNext: next still exits 1 with its bare
// "No ready items" message after the single warning; N=1 keeps the same
// wording as the plural.
func TestQuarantineWarningNoReadyNext(t *testing.T) {
	dir := copyFixture(t, "dangling")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	want := quarantineWarning(1) + "No ready items\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

// TestQuarantineWarningNextExactID covers the positional next form.
func TestQuarantineWarningNextExactID(t *testing.T) {
	dir := copyFixture(t, "cyclic")
	code, _, stderr := run(t, "--repo", dir, "--format", "compact", "next", "AWIT-TEST0005")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if want := quarantineWarning(4); stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

// TestQuarantineWarningCleanRepoStaysSilent: warning-free repos keep their
// current stderr (footers aside, which never mention quarantine).
func TestQuarantineWarningCleanRepoStaysSilent(t *testing.T) {
	dir := copyFixture(t, "clean")
	for _, args := range [][]string{
		{"prime"},
		{"--format", "compact", "list"},
		{"--format", "compact", "next", "--seed", "1"},
		{"validate"},
		{"--format", "compact", "show", "AWIT-TEST0001"},
		{"--format", "compact", "dep", "add", "AWIT-TEST0002", "AWIT-TEST0001"},
		{"archive", "--dry-run"},
	} {
		code, _, stderr := run(t, append([]string{"--repo", dir}, args...)...)
		if code != 0 {
			t.Fatalf("%v: exit %d stderr %q", args, code, stderr)
		}
		if strings.Contains(stderr, "quarantined, run awit validate") {
			t.Fatalf("%v: clean repo must not warn, stderr %q", args, stderr)
		}
	}
}

// TestQuarantineWarningDepDoubleLoadWarnsOnce: dep add/rm load the graph
// twice (pre-check plus the post-write printCompact reload). The warning
// fires once, at the initial-load boundary only — even when the reload
// would compute a different count.
func TestQuarantineWarningDepDoubleLoadWarnsOnce(t *testing.T) {
	t.Run("add", func(t *testing.T) {
		dir := copyFixture(t, "cyclic")
		code, _, stderr := run(t, "--repo", dir, "--format", "compact",
			"dep", "add", "AWIT-TEST0005", "AWIT-TEST0001")
		if code != 0 {
			t.Fatalf("exit %d stderr %q", code, stderr)
		}
		if want := quarantineWarning(4); stderr != want {
			t.Fatalf("stderr = %q, want exactly one warning %q", stderr, want)
		}
	})
	t.Run("rm", func(t *testing.T) {
		// Removing 0001's dep on 0002 breaks the triangle, so the
		// printCompact reload would count 1, not 4 — it must stay silent.
		dir := copyFixture(t, "cyclic")
		code, _, stderr := run(t, "--repo", dir, "--format", "compact",
			"dep", "rm", "AWIT-TEST0001", "AWIT-TEST0002")
		if code != 0 {
			t.Fatalf("exit %d stderr %q", code, stderr)
		}
		if want := quarantineWarning(4); stderr != want {
			t.Fatalf("stderr = %q, want exactly one warning %q", stderr, want)
		}
	})
}

// TestQuarantineWarningValidateKeepsFailExit: validate still FAILs with
// exit 1 and unchanged stdout, adding only the stderr summary.
func TestQuarantineWarningValidateKeepsFailExit(t *testing.T) {
	dir := copyFixture(t, "parse-error")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "validate")
	if code != 1 {
		t.Fatalf("exit %d, want 1 (FAIL)", code)
	}
	if !strings.HasPrefix(stdout, "FAIL") {
		t.Fatalf("stdout = %q, want FAIL header", stdout)
	}
	if want := quarantineWarning(1); stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

// TestQuarantineWarningShowForms: every show form goes through one graph
// load — a clean item inside a quarantined repo and a broken file both get
// exactly one warning.
func TestQuarantineWarningShowForms(t *testing.T) {
	t.Run("clean item in quarantined repo", func(t *testing.T) {
		dir := copyFixture(t, "cyclic")
		code, _, stderr := run(t, "--repo", dir, "--format", "compact", "show", "AWIT-TEST0005")
		if code != 0 {
			t.Fatalf("exit %d stderr %q", code, stderr)
		}
		if want := quarantineWarning(4); stderr != want {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	})
	t.Run("broken file", func(t *testing.T) {
		dir := copyFixture(t, "parse-error")
		code, _, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0001")
		if code != 0 {
			t.Fatalf("exit %d stderr %q", code, stderr)
		}
		if want := quarantineWarning(1); stderr != want {
			t.Fatalf("stderr = %q, want %q", stderr, want)
		}
	})
}

// TestQuarantineWarningArchive: archive and archive --dry-run read the
// graph and warn once even when nothing is archivable.
func TestQuarantineWarningArchive(t *testing.T) {
	dir := copyFixture(t, "cyclic")
	code, stdout, stderr := run(t, "--repo", dir, "archive", "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "Would archive 0 items\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	if want := quarantineWarning(4); stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}
