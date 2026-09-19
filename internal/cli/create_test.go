package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/id"
	"github.com/eisenwinter/awit/pkg/item"
)

func TestCreateMintsAndWrites(t *testing.T) {
	dir := initRepo(t)
	code, stdout, stderr := run(t, "--repo", dir, "create", "--brief", "Fix header parsing.", "Implement", "OAuth2", "token", "extraction")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	it := onlyItem(t, dir)
	if it.Title != "Implement OAuth2 token extraction" {
		t.Fatalf("title = %q", it.Title)
	}
	if it.Brief != "Fix header parsing." || it.Status != item.StatusOpen {
		t.Fatalf("brief/status = %q %q", it.Brief, it.Status)
	}
	if it.Assignee != "" || it.ClaimedAt != nil {
		t.Fatalf("assignee/claimed_at set: %q %v", it.Assignee, it.ClaimedAt)
	}
	if string(it.Body()) != "\n## Summary\n\n## Acceptance Criteria\n\n" {
		t.Fatalf("body = %q", it.Body())
	}
	if !strings.Contains(string(it.Body()), "## Acceptance Criteria") {
		t.Fatal("body missing ## Acceptance Criteria")
	}
	if !id.Valid("AWIT", it.ID) {
		t.Fatalf("minted id %q is not valid", it.ID)
	}
	wantLine := "[" + it.ID + "] open Implement OAuth2 token extraction | - | Unblocks: 0\n"
	if stdout != wantLine {
		t.Fatalf("stdout = %q, want %q", stdout, wantLine)
	}
}

func TestCreateRequiresBrief(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "a title")
	if code != 2 {
		t.Fatalf("exit %d, want 2 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stderr, "brief") {
		t.Fatalf("stderr = %q, want it to mention brief", stderr)
	}
}

func TestCreateNeedsTitle(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "A brief.")
	if code != 1 {
		t.Fatalf("exit %d, want 1 (stderr %q)", code, stderr)
	}
	if stderr != "Error: create needs a title\n" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestCreateWithDeps(t *testing.T) {
	dir := initRepo(t)
	a := createOne(t, dir, "A", "Brief A.")
	code, stdout, stderr := run(t, "--repo", dir, "create", "--brief", "Brief B.", "-d", a.ID, "B")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	b := readItem(t, dir, itemIDFromCompact(t, stdout))
	if len(b.Deps) != 1 || b.Deps[0] != a.ID {
		t.Fatalf("deps = %v", b.Deps)
	}
}

func TestCreateUnknownDep(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "-d", "AWIT-TEST0001", "X")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if stderr != "Error: unknown dep AWIT-TEST0001\n" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestCreateInvalidDepID(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "-d", "not-an-id", "X")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if stderr != "Error: invalid id not-an-id\n" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestCreateIDOverride(t *testing.T) {
	dir := initRepo(t)
	code, stdout, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", "AWIT-TEST0001", "Override")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	it := readItem(t, dir, "AWIT-TEST0001")
	if it.Title != "Override" || it.ID != "AWIT-TEST0001" {
		t.Fatalf("item = %+v", it)
	}
	if !strings.HasPrefix(stdout, "[AWIT-TEST0001] ") {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestCreateIDOverrideExists(t *testing.T) {
	dir := initRepo(t)
	createOne(t, dir, "First", "B.")
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", onlyItem(t, dir).ID, "Second")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	got := onlyItem(t, dir)
	if !strings.HasPrefix(stderr, "Error: item ") || !strings.HasSuffix(stderr, " already exists\n") {
		t.Fatalf("stderr = %q", stderr)
	}
	if !strings.Contains(stderr, got.ID) {
		t.Fatalf("stderr = %q, want the existing id", stderr)
	}
}

func TestCreateDefaultLabelsMerged(t *testing.T) {
	dir := initRepo(t)
	writeDefaultLabels(t, dir, []byte("prefix: AWIT\ndefault_labels: [phase1, p0]\nstale_claim: 2h\n"))
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "-l", "extra", "Merged")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	it := onlyItem(t, dir)
	if strings.Join(it.Labels, ",") != "phase1,p0,extra" {
		t.Fatalf("labels = %v", it.Labels)
	}
}

func TestCreateLabelDedupe(t *testing.T) {
	dir := initRepo(t)
	writeDefaultLabels(t, dir, []byte("prefix: AWIT\ndefault_labels: [p0]\nstale_claim: 2h\n"))
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "-l", "p0,p1", "Dedupe")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	it := onlyItem(t, dir)
	if strings.Join(it.Labels, ",") != "p0,p1" {
		t.Fatalf("labels = %v", it.Labels)
	}
}

func TestCreateAssign(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--assign", "agent/claude", "Assigned")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	it := onlyItem(t, dir)
	if it.Assignee != "agent/claude" || it.Status != item.StatusOpen || it.ClaimedAt != nil {
		t.Fatalf("assignee=%q status=%q claimed=%v", it.Assignee, it.Status, it.ClaimedAt)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".awit", "items", it.ID+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "claimed_at") {
		t.Fatalf("file contains claimed_at:\n%s", raw)
	}
}

func TestCreateJSON(t *testing.T) {
	dir := initRepo(t)
	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "create", "--brief", "One sentence.", "JSON item")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	var e format.Entry
	if err := json.Unmarshal([]byte(stdout), &e); err != nil {
		t.Fatalf("json: %v (stdout %q)", err, stdout)
	}
	if e.Title != "JSON item" || e.Brief != "One sentence." || e.Status != "open" || e.State != "ready" || e.Unblocks != 0 {
		t.Fatalf("entry = %+v", e)
	}
	if !id.Valid("AWIT", e.ID) {
		t.Fatalf("id %q", e.ID)
	}
}

func TestCreateExternalMapping(t *testing.T) {
	dir := initRepo(t)
	code, stdout, stderr := run(t, "--repo", dir, "create",
		"--brief", "A linked issue.",
		"--id", "AWIT-TEST0001",
		"--external-tracker", "gitea",
		"--external-repo", "owner/repo",
		"--external-id", "127",
		"--external-url", "https://forge.example/owner/repo/issues/127",
		"Linked")
	if code != 0 {
		t.Fatalf("exit %d stderr %q stdout %q", code, stderr, stdout)
	}
	it := readItem(t, dir, "AWIT-TEST0001")
	if it.External == nil {
		t.Fatal("External is nil")
	}
	if it.External.Tracker != "gitea" || it.External.Repo != "owner/repo" || it.External.ID != 127 {
		t.Fatalf("External = %+v", it.External)
	}
	if it.External.URL != "https://forge.example/owner/repo/issues/127" {
		t.Fatalf("URL = %q", it.External.URL)
	}
	code, stdout, stderr = run(t, "--repo", dir, "show", "AWIT-TEST0001")
	if code != 0 {
		t.Fatalf("show exit %d stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "external: gitea owner/repo#127 https://forge.example/owner/repo/issues/127") {
		t.Fatalf("show stdout = %q", stdout)
	}
	code, stdout, stderr = run(t, "--repo", dir, "--format", "json", "list")
	if code != 0 {
		t.Fatalf("list exit %d stderr %q", code, stderr)
	}
	var entries []format.Entry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatalf("list json: %v\n%s", err, stdout)
	}
	if len(entries) != 1 || entries[0].External == nil || entries[0].External.ID != 127 {
		t.Fatalf("list json = %s", stdout)
	}
}

func TestCreatePartialExternalDoesNotWrite(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create",
		"--brief", "A linked issue.",
		"--external-tracker", "gitea",
		"--external-repo", "owner/repo",
		"Linked")
	if code != 2 {
		t.Fatalf("exit %d, want 2 stderr %q", code, stderr)
	}
	ents, err := os.ReadDir(filepath.Join(dir, ".awit", "items"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Fatalf("partial mapping wrote %d items", len(ents))
	}
}

func TestCreateInvalidExternalDoesNotWrite(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create",
		"--brief", "A linked issue.",
		"--external-tracker", "github",
		"--external-repo", "owner/repo",
		"--external-id", "127",
		"--external-url", "https://forge.example/owner/repo/issues/127",
		"Linked")
	if code != 2 {
		t.Fatalf("exit %d, want 2 stderr %q", code, stderr)
	}
	ents, err := os.ReadDir(filepath.Join(dir, ".awit", "items"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 0 {
		t.Fatalf("invalid mapping wrote %d items", len(ents))
	}
}

func createOne(t *testing.T, repo, title, brief string) *item.Item {
	t.Helper()
	code, stdout, stderr := run(t, "--repo", repo, "create", "--brief", brief, title)
	if code != 0 {
		t.Fatalf("create %q: exit %d stderr %q", title, code, stderr)
	}
	return readItem(t, repo, itemIDFromCompact(t, stdout))
}

func onlyItem(t *testing.T, repo string) *item.Item {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(repo, ".awit", "items"))
	if err != nil {
		t.Fatal(err)
	}
	var md []os.DirEntry
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".md") {
			md = append(md, e)
		}
	}
	if len(md) != 1 {
		t.Fatalf("want 1 item, got %d", len(md))
	}
	return readItem(t, repo, strings.TrimSuffix(md[0].Name(), ".md"))
}

func itemIDFromCompact(t *testing.T, stdout string) string {
	t.Helper()
	if !strings.HasPrefix(stdout, "[") {
		t.Fatalf("stdout %q", stdout)
	}
	end := strings.IndexByte(stdout, ']')
	if end < 2 {
		t.Fatalf("stdout %q", stdout)
	}
	return stdout[1:end]
}

func writeDefaultLabels(t *testing.T, repo string, yaml []byte) {
	t.Helper()
	p := filepath.Join(repo, ".awit", "config.yaml")
	if err := os.WriteFile(p, yaml, 0o644); err != nil {
		t.Fatal(err)
	}
}
