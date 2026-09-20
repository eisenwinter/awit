package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/format"
)

func blockedLabelWarning(id string) string {
	return "warning: " + id + " has label \"blocked\", which does not pause work; use awit block " + id + " --reason \"...\"\n"
}

func TestBlockedLabelCreateWarnsOnce(t *testing.T) {
	dir := initRepo(t)
	code, stdout, stderr := run(t, "--repo", dir, "create", "--brief", "A test item.", "-l", "blocked", "Labeled")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	id := itemIDFromCompact(t, stdout)
	if stderr != blockedLabelWarning(id) {
		t.Fatalf("stderr = %q, want %q", stderr, blockedLabelWarning(id))
	}
	it := readItem(t, dir, id)
	if strings.Join(it.Labels, ",") != "blocked" {
		t.Fatalf("labels = %v, want [blocked] stored", it.Labels)
	}
	if it.BlockedReason != "" {
		t.Fatalf("BlockedReason = %q, label must not hold", it.BlockedReason)
	}
}

func TestBlockedLabelCreateJSONStdoutParseable(t *testing.T) {
	dir := initRepo(t)
	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "create", "--brief", "A test item.", "-l", "blocked", "Labeled")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	var e format.Entry
	if err := json.Unmarshal([]byte(stdout), &e); err != nil {
		t.Fatalf("json stdout corrupted: %v (stdout %q)", err, stdout)
	}
	if stderr != blockedLabelWarning(e.ID) {
		t.Fatalf("stderr = %q, want %q", stderr, blockedLabelWarning(e.ID))
	}
}

func TestBlockedLabelWarnExactCase(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "A test item.", "-l", "Blocked", "Cased")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if strings.Contains(stderr, "does not pause work") {
		t.Fatalf("capitalized label warned: %q", stderr)
	}
}

func TestBlockedLabelUpdateWarnsOnIntroduction(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", []string{"p1"})
	code, _, stderr := run(t, "--repo", dir, "update", id, "-l", "blocked")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stderr != blockedLabelWarning(id) {
		t.Fatalf("stderr = %q, want %q", stderr, blockedLabelWarning(id))
	}
	if got := readItem(t, dir, id); got.BlockedReason != "" {
		t.Fatalf("BlockedReason = %q, label must not hold", got.BlockedReason)
	}
	// An unrelated update does not re-warn: the label is no longer newly introduced.
	code, _, stderr = run(t, "--repo", dir, "update", id, "--title", "New")
	if code != 0 {
		t.Fatalf("title exit %d stderr %q", code, stderr)
	}
	if strings.Contains(stderr, "does not pause work") {
		t.Fatalf("repeat update warned: %q", stderr)
	}
}

func TestBlockedLabelClosedItemNoWarning(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	if code, _, stderr := run(t, "--repo", dir, "update", id, "--status", "closed"); code != 0 {
		t.Fatalf("close exit %d stderr %q", code, stderr)
	}
	code, _, stderr := run(t, "--repo", dir, "update", id, "-l", "blocked")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if strings.Contains(stderr, "does not pause work") {
		t.Fatalf("closed item warned: %q", stderr)
	}
}

func TestBlockedLabelImportCopiesWithoutHold(t *testing.T) {
	repo, stub := importRepo(t)
	writeTeaIssue(t, stub, 127, `{"id": 987654, "number": 127, "title": "Fix header parsing",
"body": "Line one.\n", "state": "open",
"labels": [{"name": "blocked"}, {"name": "p1"}],
"html_url": "https://forge.example/owner/repo/issues/127"}`)
	code, stdout, stderr := run(t, "--repo", repo, "--format", "json", "import",
		"https://forge.example/owner/repo/issues/127",
		"--brief", "Imported issue.", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	var e format.Entry
	if err := json.Unmarshal([]byte(stdout), &e); err != nil {
		t.Fatalf("json stdout corrupted: %v (stdout %q)", err, stdout)
	}
	if stderr != blockedLabelWarning(e.ID) {
		t.Fatalf("stderr = %q, want %q", stderr, blockedLabelWarning(e.ID))
	}
	it := readItem(t, repo, e.ID)
	if strings.Join(it.Labels, ",") != "blocked,p1" {
		t.Fatalf("labels = %v, want remote snapshot", it.Labels)
	}
	if it.BlockedReason != "" {
		t.Fatalf("BlockedReason = %q, import must not auto-hold", it.BlockedReason)
	}
}
