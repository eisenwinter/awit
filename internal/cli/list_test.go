package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestListAllCompact(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "list")
	if code != 0 {
		t.Fatalf("exit %d stderr %q stdout %q", code, stderr, stdout)
	}
	golden(t, "list_clean_compact.golden", []byte(stdout))
	wantFooter := "Note: 4 open, 1 in_progress. Claim a ready item with awit next --claim; close it with awit close <id> when the work is done.\n"
	if stderr != wantFooter {
		t.Fatalf("footer = %q, want %q", stderr, wantFooter)
	}
}

func TestListReadyOrder(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "list", "--ready")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	wantFooter := "Note: 2 open, 1 in_progress. Claim a ready item with awit next --claim; close it with awit close <id> when the work is done.\n"
	if stderr != wantFooter {
		t.Fatalf("footer = %q, want %q", stderr, wantFooter)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("ready count = %d, want 3\n%s", len(lines), stdout)
	}
	if !strings.HasPrefix(lines[0], "[AWIT-TEST0001]") {
		t.Fatalf("first ready = %q, want AWIT-TEST0001", lines[0])
	}
	if !strings.HasPrefix(lines[1], "[AWIT-TEST0002]") {
		t.Fatalf("second ready = %q, want AWIT-TEST0002", lines[1])
	}
	if !strings.HasPrefix(lines[2], "[AWIT-TEST0006]") {
		t.Fatalf("third ready = %q, want AWIT-TEST0006", lines[2])
	}
}

func TestListStatusFilter(t *testing.T) {
	dir := copyFixture(t, "clean")

	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "list", "-s", "closed")
	if code != 0 || stderr != "" {
		t.Fatalf("closed: exit %d stderr %q", code, stderr)
	}
	if stdout != "[AWIT-TEST0005] closed Write auth middleware spec | - | Unblocks: 1\n" {
		t.Fatalf("closed stdout = %q", stdout)
	}

	code, stdout, stderr = run(t, "--repo", dir, "--format", "compact", "list", "-s", "open")
	if code != 0 {
		t.Fatalf("open: exit %d stderr %q", code, stderr)
	}
	if stderr != "Note: 4 open. Claim a ready item with awit next --claim; close it with awit close <id> when the work is done.\n" {
		t.Fatalf("open footer = %q", stderr)
	}
	if strings.Contains(stdout, "AWIT-TEST0005") || strings.Contains(stdout, "AWIT-TEST0006") {
		t.Fatalf("open filter leaked closed/in_progress:\n%s", stdout)
	}
	for _, id := range []string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0003", "AWIT-TEST0004"} {
		if !strings.Contains(stdout, id) {
			t.Fatalf("open filter missing %s\n%s", id, stdout)
		}
	}

	code, stdout, stderr = run(t, "--repo", dir, "--format", "compact", "list", "-s", "in_progress")
	if code != 0 {
		t.Fatalf("in_progress: exit %d stderr %q", code, stderr)
	}
	if stderr != "Note: 1 in_progress. Claim a ready item with awit next --claim; close it with awit close <id> when the work is done.\n" {
		t.Fatalf("in_progress footer = %q", stderr)
	}
	if !strings.HasPrefix(stdout, "[AWIT-TEST0006]") || strings.Count(stdout, "\n") != 1 {
		t.Fatalf("in_progress stdout = %q", stdout)
	}

	code, _, stderr = run(t, "--repo", dir, "--format", "compact", "list", "-s", "banana")
	if code != 1 || !strings.Contains(stderr, "Error:") {
		t.Fatalf("bad status exit %d stderr %q", code, stderr)
	}
}

func TestListLabelAnd(t *testing.T) {
	dir := copyFixture(t, "clean")

	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "list", "-l", "p0", "-l", "auth")
	if code != 0 || stderr != "" {
		t.Fatalf("AND: exit %d stderr %q", code, stderr)
	}
	if stdout != "" {
		t.Fatalf("AND p0 AND auth = %q, want empty", stdout)
	}

	code, stdout, stderr = run(t, "--repo", dir, "--format", "compact", "list", "-l", "auth,db")
	if code != 0 {
		t.Fatalf("OR: exit %d stderr %q", code, stderr)
	}
	if stderr != "Note: 2 open. Claim a ready item with awit next --claim; close it with awit close <id> when the work is done.\n" {
		t.Fatalf("OR footer = %q", stderr)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("OR count = %d, want 2\n%s", len(lines), stdout)
	}
	if !strings.HasPrefix(lines[0], "[AWIT-TEST0001]") {
		t.Fatalf("OR first = %q, want 0001", lines[0])
	}
	if !strings.HasPrefix(lines[1], "[AWIT-TEST0002]") {
		t.Fatalf("OR second = %q, want 0002", lines[1])
	}
}

func TestListJSON(t *testing.T) {
	dir := copyFixture(t, "clean")

	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "list")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if !strings.HasPrefix(stderr, "Note: 4 open, 1 in_progress.") {
		t.Fatalf("json list footer = %q, want Note on stderr (stdout must stay parseable)", stderr)
	}
	var rows []struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		State  string `json:"state"`
	}
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	if len(rows) != 6 {
		t.Fatalf("len = %d, want 6", len(rows))
	}
	var found bool
	for _, r := range rows {
		if r.ID == "AWIT-TEST0005" {
			found = true
			if r.Status != "closed" || r.State != "closed" {
				t.Fatalf("0005 status=%q state=%q, want closed/closed", r.Status, r.State)
			}
		}
	}
	if !found {
		t.Fatal("0005 missing from json list")
	}
}

func TestListQuarantined(t *testing.T) {
	dir := copyFixture(t, "cyclic")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "list", "--quarantined")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stderr != quarantineWarning(4)+"Note: 4 open. Claim a ready item with awit next --claim; close it with awit close <id> when the work is done.\n" {
		t.Fatalf("stderr = %q, want warning then footer", stderr)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("quarantined count = %d, want 4 (triangle + self-loop)\n%s", len(lines), stdout)
	}
	want := []string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0003", "AWIT-TEST0004"}
	for i, id := range want {
		if !strings.HasPrefix(lines[i], "["+id+"]") {
			t.Fatalf("line %d = %q, want %s", i, lines[i], id)
		}
		if !strings.HasSuffix(lines[i], " | QUARANTINED") {
			t.Fatalf("line %d missing | QUARANTINED: %q", i, lines[i])
		}
	}
	if strings.Contains(stdout, "AWIT-TEST0005") {
		t.Fatalf("0005 is not quarantined:\n%s", stdout)
	}
}
