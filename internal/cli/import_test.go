package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/internal/teax/teaxtest"
	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/item"
)

func writeTeaLogins(t *testing.T, dir string, pairs ...[2]string) {
	t.Helper()
	type login struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	var rows []login
	for _, p := range pairs {
		rows = append(rows, login{Name: p[0], URL: p[1], Token: "SECRET-TOKEN"})
	}
	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "logins.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeTeaIssue scripts the stub's response for GET repos/owner/repo/issues/<n>.
func writeTeaIssue(t *testing.T, dir string, n int, bodyJSON string) {
	t.Helper()
	key := fmt.Sprintf("repos_owner_repo_issues_%d", n)
	if err := os.WriteFile(filepath.Join(dir, key+".json"), []byte(bodyJSON), 0o644); err != nil {
		t.Fatal(err)
	}
}

func importRepo(t *testing.T) (repo, stubDir string) {
	t.Helper()
	repo = initRepo(t)
	stubDir = teaxtest.Install(t)
	writeTeaLogins(t, stubDir, [2]string{"sandbox", "https://forge.example"})
	return repo, stubDir
}

func importArgs(repo, stub string) []string {
	return []string{"--repo", repo, "import", "https://forge.example/owner/repo/issues/127",
		"--brief", "Imported issue.", "--tea-login", "sandbox"}
}

const faithfulIssue = `{"id": 987654, "number": 127, "title": "Fix header parsing",
"body": "Line one.\n\nLine two.\n", "state": "open",
"labels": [{"name": "bug"}, {"name": "p1"}, {"name": "bug"}],
"html_url": "https://forge.example/owner/repo/issues/127"}`

func TestImportFaithful(t *testing.T) {
	repo, stub := importRepo(t)
	writeDefaultLabels(t, repo, []byte("prefix: AWIT\ndefault_labels: [phase1, p0]\nstale_claim: 2h\n"))
	writeTeaIssue(t, stub, 127, faithfulIssue)
	code, stdout, stderr := run(t, "--repo", repo, "import",
		"https://forge.example/owner/repo/issues/127",
		"--brief", "Imported issue.", "--alias", "DTRM-F21", "--tea-login", "sandbox")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	id := itemIDFromCompact(t, stdout)
	it := readItem(t, repo, id)
	if it.Title != "Fix header parsing" || it.Brief != "Imported issue." || it.Status != item.StatusOpen {
		t.Fatalf("item = %+v", it)
	}
	if it.External == nil || it.External.Tracker != "gitea" || it.External.Repo != "owner/repo" ||
		it.External.ID != 127 || it.External.URL != "https://forge.example/owner/repo/issues/127" {
		t.Fatalf("External = %+v", it.External)
	}
	if got := strings.Join(it.Labels, ","); got != "bug,p1" {
		t.Fatalf("labels = %q, want first-seen deduped remote labels without config defaults", got)
	}
	if it.Alias != "DTRM-F21" {
		t.Fatalf("Alias = %q", it.Alias)
	}
	if string(it.Body()) != "Line one.\n\nLine two.\n" {
		t.Fatalf("body = %q, want the exact decoded remote body", it.Body())
	}
	if it.Assignee != "" || it.ClaimedAt != nil {
		t.Fatalf("import must not infer a claim: assignee=%q claimed=%v", it.Assignee, it.ClaimedAt)
	}
}

func TestImportNumberIsNotDatabaseID(t *testing.T) {
	repo, stub := importRepo(t)
	writeTeaIssue(t, stub, 127, faithfulIssue) // database id 987654
	code, stdout, stderr := run(t, importArgs(repo, stub)...)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	it := readItem(t, repo, itemIDFromCompact(t, stdout))
	if it.External.ID != 127 {
		t.Fatalf("external.id = %d, want issue number 127, not the database id", it.External.ID)
	}
	raw, err := os.ReadFile(it.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "987654") {
		t.Fatalf("database id leaked into the item file:\n%s", raw)
	}
}

func TestImportClosedState(t *testing.T) {
	repo, stub := importRepo(t)
	writeTeaIssue(t, stub, 127, `{"number": 127, "title": "Done", "body": "b", "state": "closed"}`)
	code, stdout, stderr := run(t, importArgs(repo, stub)...)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	it := readItem(t, repo, itemIDFromCompact(t, stdout))
	if it.Status != item.StatusClosed {
		t.Fatalf("status = %q, want closed", it.Status)
	}
	if !strings.HasPrefix(stdout, "["+it.ID+"] closed ") {
		t.Fatalf("stdout = %q, want closed status in the compact line", stdout)
	}
}

func TestImportUnsupportedState(t *testing.T) {
	repo, stub := importRepo(t)
	writeTeaIssue(t, stub, 127, `{"number": 127, "title": "T", "body": "b", "state": "merged"}`)
	code, _, stderr := run(t, importArgs(repo, stub)...)
	if code != 1 {
		t.Fatalf("exit %d, want 1 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stderr, "merged") {
		t.Fatalf("stderr = %q, want the refused state named", stderr)
	}
	assertNoItems(t, repo)
}

func TestImportByteExactBody(t *testing.T) {
	repo, stub := importRepo(t)
	body := "Intro\r\n\r\nTrailing spaces   \r\nlast, no newline"
	writeTeaIssue(t, stub, 127, `{"number": 127, "title": "T", "body": `+quote(body)+`, "state": "open"}`)
	code, stdout, stderr := run(t, importArgs(repo, stub)...)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	it := readItem(t, repo, itemIDFromCompact(t, stdout))
	if string(it.Body()) != body {
		t.Fatalf("body = %q, want byte-exact %q", it.Body(), body)
	}
	raw, err := os.ReadFile(it.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(raw), body) {
		t.Fatalf("file must end with the exact body bytes:\n%q", raw)
	}
}

func TestImportNullBody(t *testing.T) {
	repo, stub := importRepo(t)
	writeTeaIssue(t, stub, 127, `{"number": 127, "title": "T", "body": null, "state": "open"}`)
	code, stdout, stderr := run(t, importArgs(repo, stub)...)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	it := readItem(t, repo, itemIDFromCompact(t, stdout))
	if len(it.Body()) != 0 {
		t.Fatalf("null body must map to empty, got %q", it.Body())
	}
}

func TestImportDuplicateActive(t *testing.T) {
	repo, stub := importRepo(t)
	writeTeaIssue(t, stub, 127, faithfulIssue)
	code, stdout, stderr := run(t, importArgs(repo, stub)...)
	if code != 0 {
		t.Fatalf("first import: exit %d stderr %q", code, stderr)
	}
	first := readItem(t, repo, itemIDFromCompact(t, stdout))
	before, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	code, _, stderr = run(t, importArgs(repo, stub)...)
	if code != 1 {
		t.Fatalf("second import: exit %d, want 1", code)
	}
	if !strings.Contains(stderr, first.ID) {
		t.Fatalf("stderr = %q, want the existing item id %s", stderr, first.ID)
	}
	after, err := os.ReadFile(first.Path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("duplicate import modified the existing item")
	}
	ents, err := os.ReadDir(filepath.Join(repo, ".awit", "items"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ents) != 1 {
		t.Fatalf("duplicate import created files: %d entries", len(ents))
	}
}

func TestImportDuplicateArchived(t *testing.T) {
	repo, stub := importRepo(t)
	writeTeaIssue(t, stub, 127, faithfulIssue)
	archDir := filepath.Join(repo, ".awit", "archive")
	if err := os.MkdirAll(archDir, 0o755); err != nil {
		t.Fatal(err)
	}
	archPath := filepath.Join(archDir, "AWIT-TEST0009.md")
	writeItemFile(t, archPath, "AWIT-TEST0009", `external:
  tracker: gitea
  repo: owner/repo
  id: 127
  url: https://forge.example/owner/repo/issues/127
`)
	code, _, stderr := run(t, importArgs(repo, stub)...)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "AWIT-TEST0009") {
		t.Fatalf("stderr = %q, want the archive path naming AWIT-TEST0009", stderr)
	}
	assertNoItems(t, repo)
}

func TestImportUninspectableArchiveRefuses(t *testing.T) {
	repo, stub := importRepo(t)
	writeTeaIssue(t, stub, 127, faithfulIssue)
	archDir := filepath.Join(repo, ".awit", "archive")
	if err := os.MkdirAll(archDir, 0o755); err != nil {
		t.Fatal(err)
	}
	badPath := filepath.Join(archDir, "AWIT-TEST0008.md")
	if err := os.WriteFile(badPath, []byte("---\ntitle: [unclosed\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := run(t, importArgs(repo, stub)...)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "AWIT-TEST0008") {
		t.Fatalf("stderr = %q, want the uninspectable path named", stderr)
	}
	assertNoItems(t, repo)
}

func TestImportMissingTea(t *testing.T) {
	repo := initRepo(t)
	teaxtest.HideTea(t)
	code, _, stderr := run(t, "--repo", repo, "import",
		"https://forge.example/owner/repo/issues/127", "--brief", "Imported issue.")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "install tea") {
		t.Fatalf("stderr = %q, want install tea hint", stderr)
	}
	assertNoItems(t, repo)
}

func TestImportNoMatchingLogin(t *testing.T) {
	repo := initRepo(t)
	stub := teaxtest.Install(t)
	writeTeaLogins(t, stub, [2]string{"other", "https://elsewhere.example"})
	code, _, stderr := run(t, "--repo", repo, "import",
		"https://forge.example/owner/repo/issues/127", "--brief", "Imported issue.")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "tea login add") {
		t.Fatalf("stderr = %q, want tea login add hint", stderr)
	}
	assertNoItems(t, repo)
}

func TestImportHTTP401ExitZero(t *testing.T) {
	repo, stub := importRepo(t)
	if err := os.WriteFile(filepath.Join(stub, "user.status"), []byte("401"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := run(t, importArgs(repo, stub)...)
	if code != 1 {
		t.Fatalf("exit %d, want 1 (tea exits 0 on HTTP 401; the status line must be checked)", code)
	}
	if !strings.Contains(stderr, "401") || !strings.Contains(stderr, "tea login add") {
		t.Fatalf("stderr = %q, want HTTP status and re-login hint", stderr)
	}
	if strings.Contains(stderr, "SECRET-TOKEN") {
		t.Fatalf("stderr leaks token material: %q", stderr)
	}
	assertNoItems(t, repo)
}

func TestImportMalformedResponse(t *testing.T) {
	repo, stub := importRepo(t)
	writeTeaIssue(t, stub, 127, `not json`)
	code, _, _ := run(t, importArgs(repo, stub)...)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	assertNoItems(t, repo)
}

func TestImportIssueNotFound(t *testing.T) {
	repo, stub := importRepo(t)
	if err := os.WriteFile(filepath.Join(stub, "repos_owner_repo_issues_127.status"), []byte("404"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTeaIssue(t, stub, 127, `{"message": "Not Found"}`)
	code, _, stderr := run(t, importArgs(repo, stub)...)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "not found") {
		t.Fatalf("stderr = %q, want not found", stderr)
	}
	assertNoItems(t, repo)
}

func TestImportConflictMarkerBodyRefused(t *testing.T) {
	repo, stub := importRepo(t)
	body := "before\n<<<<<<< line\nmiddle\n=======\nother\n>>>>>>> line\nafter\n"
	writeTeaIssue(t, stub, 127, `{"number": 127, "title": "T", "body": `+quote(body)+`, "state": "open"}`)
	code, _, stderr := run(t, importArgs(repo, stub)...)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "conflict") || !strings.Contains(stderr, "reconcil") {
		t.Fatalf("stderr = %q, want conflict-marker refusal with reconcile hint", stderr)
	}
	assertNoItems(t, repo)
}

func TestImportInvalidAlias(t *testing.T) {
	repo, stub := importRepo(t)
	writeTeaIssue(t, stub, 127, faithfulIssue)
	code, _, stderr := run(t, "--repo", repo, "import",
		"https://forge.example/owner/repo/issues/127",
		"--brief", "Imported issue.", "--tea-login", "sandbox", "--alias", "bad alias")
	if code != 2 {
		t.Fatalf("exit %d, want 2 (stderr %q)", code, stderr)
	}
	assertNoItems(t, repo)
}

func TestImportRequiresBrief(t *testing.T) {
	repo, stub := importRepo(t)
	writeTeaIssue(t, stub, 127, faithfulIssue)
	code, _, stderr := run(t, "--repo", repo, "import", "https://forge.example/owner/repo/issues/127")
	if code != 2 {
		t.Fatalf("exit %d, want 2 (stderr %q)", code, stderr)
	}
	assertNoItems(t, repo)
}

func TestImportRejectsBadURL(t *testing.T) {
	repo, stub := importRepo(t)
	writeTeaIssue(t, stub, 127, faithfulIssue)
	code, _, _ := run(t, "--repo", repo, "import", "https://forge.example/owner/repo/pulls/127", "--brief", "B.")
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
	assertNoItems(t, repo)
}

func TestCreateIDRejectsAliasShape(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", "DTRM-F21", "T")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "invalid id DTRM-F21") {
		t.Fatalf("stderr = %q", stderr)
	}
}

// --- alias lookup ---

func TestAliasCreateAndResolve(t *testing.T) {
	dir := initRepo(t)
	a := createOne(t, dir, "Alpha", "Brief A.")
	code, stdout, stderr := run(t, "--repo", dir, "create", "--brief", "Brief B.", "--alias", "DTRM-F21", "Beta")
	if code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}
	b := readItem(t, dir, itemIDFromCompact(t, stdout))
	if b.Alias != "DTRM-F21" {
		t.Fatalf("Alias = %q", b.Alias)
	}

	code, stdout, stderr = run(t, "--repo", dir, "show", "DTRM-F21")
	if code != 0 || !strings.Contains(stdout, "["+b.ID+"] Beta") {
		t.Fatalf("show by alias: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "alias: DTRM-F21") {
		t.Fatalf("show must print the alias line: %q", stdout)
	}
	code, stdout, stderr = run(t, "--repo", dir, "show", "dtrm-f21")
	if code != 0 || !strings.Contains(stdout, "["+b.ID+"] Beta") {
		t.Fatalf("alias lookup must be case-insensitive: exit %d stdout %q stderr %q", code, stdout, stderr)
	}

	code, stdout, stderr = run(t, "--repo", dir, "--format", "json", "list", "DTRM-F21")
	if code != 0 {
		t.Fatalf("list: exit %d stderr %q", code, stderr)
	}
	var entries []format.Entry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatalf("list json: %v\n%s", err, stdout)
	}
	if len(entries) != 1 || entries[0].ID != b.ID || entries[0].Alias != "DTRM-F21" {
		t.Fatalf("list json = %s", stdout)
	}

	code, stdout, stderr = run(t, "--repo", dir, "next", "DTRM-F21")
	if code != 0 || !strings.Contains(stdout, "["+b.ID+"]") {
		t.Fatalf("next by alias: exit %d stdout %q stderr %q", code, stdout, stderr)
	}

	// Mutation through the alias must hit the intended canonical file only.
	code, _, stderr = run(t, "--repo", dir, "update", "DTRM-F21", "--title", "Renamed")
	if code != 0 {
		t.Fatalf("update: exit %d stderr %q", code, stderr)
	}
	if got := readItem(t, dir, b.ID); got.Title != "Renamed" {
		t.Fatalf("canonical file not mutated: title = %q", got.Title)
	}
	if got := readItem(t, dir, a.ID); got.Title != "Alpha" {
		t.Fatalf("unrelated item mutated: title = %q", got.Title)
	}
}

func TestAliasClaimRefusalRules(t *testing.T) {
	dir := initRepo(t)
	code, stdout, stderr := run(t, "--repo", dir, "create", "--brief", "Brief B.", "--alias", "DTRM-F21", "Beta")
	if code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}
	id := itemIDFromCompact(t, stdout)
	code, _, stderr = run(t, "--repo", dir, "update", id, "--status", "closed")
	if code != 0 {
		t.Fatalf("update: exit %d stderr %q", code, stderr)
	}
	code, _, stderr = run(t, "--repo", dir, "next", "--claim", "--no-commit", "--agent", "claude", "DTRM-F21")
	if code != 1 {
		t.Fatalf("claim of closed item by alias: exit %d, want 1", code)
	}
	if !strings.Contains(stderr, id+" is closed") {
		t.Fatalf("stderr = %q, want the closed refusal naming the canonical id", stderr)
	}
}

func TestAliasAmbiguous(t *testing.T) {
	dir := initRepo(t)
	writeItemFile(t, filepath.Join(dir, ".awit", "items", "AWIT-TEST0002.md"), "AWIT-TEST0002", "alias: DUP\n")
	writeItemFile(t, filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md"), "AWIT-TEST0001", "alias: dup\n")
	code, _, stderr := run(t, "--repo", dir, "show", "dup")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	i1 := strings.Index(stderr, "AWIT-TEST0001")
	i2 := strings.Index(stderr, "AWIT-TEST0002")
	if i1 < 0 || i2 < 0 || i1 > i2 {
		t.Fatalf("stderr = %q, want both canonical ids sorted", stderr)
	}
	if !strings.Contains(strings.ToLower(stderr), "ambiguous") {
		t.Fatalf("stderr = %q, want an ambiguity error", stderr)
	}
}

func TestAliasInvalidOnCreate(t *testing.T) {
	for _, alias := range []string{"1bad", "bad alias", "AWIT-TEST0001"} {
		dir := initRepo(t)
		code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--alias", alias, "T")
		if code != 2 {
			t.Fatalf("--alias %q: exit %d, want 2 (stderr %q)", alias, code, stderr)
		}
		assertNoItems(t, dir)
	}
}

func TestAliasUpdateAndClear(t *testing.T) {
	dir := initRepo(t)
	it := createOne(t, dir, "T", "B.")
	code, _, stderr := run(t, "--repo", dir, "update", it.ID, "--alias", "DTRM-F21")
	if code != 0 {
		t.Fatalf("update --alias: exit %d stderr %q", code, stderr)
	}
	if got := readItem(t, dir, it.ID); got.Alias != "DTRM-F21" {
		t.Fatalf("Alias = %q", got.Alias)
	}
	code, _, stderr = run(t, "--repo", dir, "update", "dtrm-f21", "--clear-alias")
	if code != 0 {
		t.Fatalf("update --clear-alias: exit %d stderr %q", code, stderr)
	}
	got := readItem(t, dir, it.ID)
	if got.Alias != "" {
		t.Fatalf("Alias = %q after clear", got.Alias)
	}
	raw, err := os.ReadFile(got.Path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "alias") {
		t.Fatalf("cleared alias still in file:\n%s", raw)
	}
	code, _, _ = run(t, "--repo", dir, "update", it.ID, "--alias", "X", "--clear-alias")
	if code != 2 {
		t.Fatalf("--alias + --clear-alias: exit %d, want 2", code)
	}
}

func TestAliasDepResolution(t *testing.T) {
	dir := initRepo(t)
	a := createOne(t, dir, "Alpha", "Brief A.")
	code, stdout, stderr := run(t, "--repo", dir, "create", "--brief", "Brief B.", "--alias", "DTRM-F21", "Beta")
	if code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}
	b := readItem(t, dir, itemIDFromCompact(t, stdout))
	code, _, stderr = run(t, "--repo", dir, "dep", "add", "DTRM-F21", a.ID)
	if code != 0 {
		t.Fatalf("dep add: exit %d stderr %q", code, stderr)
	}
	got := readItem(t, dir, b.ID)
	if len(got.Deps) != 1 || got.Deps[0] != a.ID {
		t.Fatalf("deps = %v, want canonical %s", got.Deps, a.ID)
	}
	// Dependency positions accept aliases on create too.
	code, stdout, stderr = run(t, "--repo", dir, "create", "--brief", "Brief C.", "-d", "DTRM-F21", "Gamma")
	if code != 0 {
		t.Fatalf("create -d alias: exit %d stderr %q", code, stderr)
	}
	c := readItem(t, dir, itemIDFromCompact(t, stdout))
	if len(c.Deps) != 1 || c.Deps[0] != b.ID {
		t.Fatalf("deps = %v, want canonical %s", c.Deps, b.ID)
	}
}

// --- external key lookup ---

func externalItem(t *testing.T, dir, id, repo string, n int) {
	t.Helper()
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", id,
		"--external-tracker", "gitea", "--external-repo", repo,
		"--external-id", fmt.Sprint(n),
		"--external-url", fmt.Sprintf("https://forge.example/%s/issues/%d", repo, n), "Ext "+id)
	if code != 0 {
		t.Fatalf("create external: exit %d stderr %q", code, stderr)
	}
}

func TestExternalLookupShowListNext(t *testing.T) {
	dir := initRepo(t)
	externalItem(t, dir, "AWIT-TEST0001", "owner/repo", 127)
	for _, key := range []string{"owner/repo#127", "#127"} {
		code, stdout, stderr := run(t, "--repo", dir, "show", key)
		if code != 0 || !strings.Contains(stdout, "[AWIT-TEST0001]") {
			t.Fatalf("show %q: exit %d stdout %q stderr %q", key, code, stdout, stderr)
		}
	}
	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "list", "owner/repo#127")
	if code != 0 {
		t.Fatalf("list: exit %d stderr %q", code, stderr)
	}
	var entries []format.Entry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatalf("list json: %v\n%s", err, stdout)
	}
	if len(entries) != 1 || entries[0].ID != "AWIT-TEST0001" {
		t.Fatalf("list json = %s", stdout)
	}
	code, stdout, stderr = run(t, "--repo", dir, "next", "#127")
	if code != 0 || !strings.Contains(stdout, "[AWIT-TEST0001]") {
		t.Fatalf("next: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}

func TestExternalLookupAmbiguousBareNumber(t *testing.T) {
	dir := initRepo(t)
	externalItem(t, dir, "AWIT-TEST0001", "owner/repo", 127)
	externalItem(t, dir, "AWIT-TEST0002", "other/lib", 127)
	code, _, stderr := run(t, "--repo", dir, "show", "#127")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "AWIT-TEST0001") || !strings.Contains(stderr, "AWIT-TEST0002") {
		t.Fatalf("stderr = %q, want both canonical ids", stderr)
	}
	code, stdout, stderr := run(t, "--repo", dir, "show", "other/lib#127")
	if code != 0 || !strings.Contains(stdout, "[AWIT-TEST0002]") {
		t.Fatalf("repo-qualified lookup: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}

func TestExternalLookupCanonicalPrecedence(t *testing.T) {
	dir := initRepo(t)
	externalItem(t, dir, "AWIT-TEST0001", "owner/repo", 127)
	// A hand-written (invalid but parseable) alias that equals a canonical
	// id must not shadow the canonical item.
	writeItemFile(t, filepath.Join(dir, ".awit", "items", "AWIT-TEST0002.md"), "AWIT-TEST0002", "alias: AWIT-TEST0001\n")
	code, stdout, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0001")
	if code != 0 || !strings.Contains(stdout, "Ext AWIT-TEST0001") {
		t.Fatalf("canonical id must win over an alias: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}

func TestExternalLookupUnknown(t *testing.T) {
	dir := initRepo(t)
	externalItem(t, dir, "AWIT-TEST0001", "owner/repo", 127)
	code, _, stderr := run(t, "--repo", dir, "show", "#128")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "unknown item #128") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestAliasValidateWarns(t *testing.T) {
	dir := initRepo(t)
	writeItemFile(t, filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md"), "AWIT-TEST0001", "alias: bad alias\n")
	writeItemFile(t, filepath.Join(dir, ".awit", "items", "AWIT-TEST0002.md"), "AWIT-TEST0002", "alias: DUP\n")
	writeItemFile(t, filepath.Join(dir, ".awit", "items", "AWIT-TEST0003.md"), "AWIT-TEST0003", "alias: dup\n")
	code, stdout, stderr := run(t, "--repo", dir, "validate")
	if code != 0 {
		t.Fatalf("validate: exit %d (alias problems warn, not FAIL) stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "WARN  AWIT-TEST0001") || !strings.Contains(stdout, "invalid alias") {
		t.Fatalf("stdout missing invalid-alias warning:\n%s", stdout)
	}
	if !strings.Contains(stdout, "WARN  AWIT-TEST0002") || !strings.Contains(stdout, "duplicate alias") {
		t.Fatalf("stdout missing duplicate-alias warning:\n%s", stdout)
	}
}

// --- helpers ---

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func writeItemFile(t *testing.T, path, id, extraFrontmatter string) {
	t.Helper()
	content := "---\nid: " + id + "\ntitle: " + id + "\nstatus: open\n" + extraFrontmatter + "---\n\nbody\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertNoItems(t *testing.T, repo string) {
	t.Helper()
	ents, err := os.ReadDir(filepath.Join(repo, ".awit", "items"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".md") {
			t.Fatalf("refused import left item file %s", e.Name())
		}
	}
}
