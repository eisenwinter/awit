package cli

import (
	"strings"
	"testing"
)

func TestRootHelpShowsTypicalSession(t *testing.T) {
	code, stdout, stderr := run(t, "--help")
	if code != 0 {
		t.Fatalf("--help exit = %d, want 0 (stderr %q)", code, stderr)
	}
	for _, want := range []string{
		"TYPICAL SESSION (set AWIT_AGENT first):",
		`awit create "Title" --brief "..."`,
		"awit prime",
		"awit next --claim",
		"awit show <id> --full",
		`awit comment <id> "note"`,
		"awit close <id>",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("--help missing %q; got:\n%s", want, stdout)
		}
	}
}

func TestUpdateStatusHelpNamesValues(t *testing.T) {
	code, stdout, stderr := run(t, "update", "--help")
	if code != 0 {
		t.Fatalf("update --help exit = %d, want 0 (stderr %q)", code, stderr)
	}
	for _, want := range []string{"open", "in_progress", "closed"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("update --help missing %q; got:\n%s", want, stdout)
		}
	}
}

func TestNextClaimHelpNamesAgent(t *testing.T) {
	code, stdout, stderr := run(t, "next", "--help")
	if code != 0 {
		t.Fatalf("next --help exit = %d, want 0 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stdout, "AWIT_AGENT") {
		t.Errorf("next --help missing AWIT_AGENT; got:\n%s", stdout)
	}
}

func TestUpdateBadStatusSuggests(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "T", "B.", nil)

	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--status", "close")
	if code != 1 {
		t.Fatalf("close typo exit = %d, want 1 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stderr, `did you mean "closed"`) {
		t.Errorf("close typo stderr missing suggestion; got %q", stderr)
	}

	code, _, stderr = run(t, "--repo", dir, "update", "AWIT-TEST0001", "--status", "bogus")
	if code != 1 {
		t.Fatalf("bogus exit = %d, want 1 (stderr %q)", code, stderr)
	}
	for _, want := range []string{"open", "in_progress", "closed"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("bogus stderr missing %q; got %q", want, stderr)
		}
	}
}

func TestListFooterWithOpenItems(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "list")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout == "" {
		t.Fatal("list printed no rows")
	}
	want := "Note: 4 open, 1 in_progress. Claim a ready item with awit next --claim; close it with awit close <id> when the work is done.\n"
	if stderr != want {
		t.Errorf("footer = %q, want %q", stderr, want)
	}
}

func TestListFooterSuppressed(t *testing.T) {
	dir := copyFixture(t, "clean")

	code, _, stderr := run(t, "--repo", dir, "--format", "compact", "list", "-s", "closed")
	if code != 0 {
		t.Fatalf("closed filter exit %d stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Errorf("closed filter stderr = %q, want empty (no footer)", stderr)
	}

	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "list", "-l", "p0", "-l", "auth")
	if code != 0 {
		t.Fatalf("empty filter exit %d stderr %q", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("empty filter stdout = %q, want empty", stdout)
	}
	if stderr != "" {
		t.Errorf("empty filter stderr = %q, want empty (no footer)", stderr)
	}
}
