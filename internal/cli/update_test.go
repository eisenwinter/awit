package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eisenwinter/awit/internal/gitx"
	"github.com/eisenwinter/awit/pkg/config"
	"github.com/eisenwinter/awit/pkg/item"
)

func seedItem(t *testing.T, repo, id, title, brief string, labels []string) {
	t.Helper()
	st, err := item.Open(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Save(item.New(id, title, brief, nil, labels)); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateStatusOneLineDiff(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "One line", "Brief.", nil)
	path := filepath.Join(dir, ".awit", "items", id+".md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "--repo", dir, "update", id, "--status", "in_progress")
	if code != 0 || stdout != "updated "+id+": status=in_progress\n" {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b, a := strings.Split(string(before), "\n"), strings.Split(string(after), "\n")
	if len(b) != len(a) {
		t.Fatalf("line count %d -> %d\n%s\n%s", len(b), len(a), before, after)
	}
	var changed []string
	for i := range b {
		if b[i] != a[i] {
			changed = append(changed, a[i])
		}
	}
	if len(changed) != 1 || changed[0] != "status: in_progress" {
		t.Fatalf("changed = %q, want [status: in_progress]", changed)
	}
}

func TestUpdateTitleAndBrief(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "Old", "Old brief.", nil)
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--title", "New title", "--brief", "New brief.")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	it := readItem(t, dir, "AWIT-TEST0001")
	if it.Title != "New title" || it.Brief != "New brief." {
		t.Fatalf("title=%q brief=%q", it.Title, it.Brief)
	}
}

func TestUpdateLabelsAddRemove(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "L", "B.", []string{"keep", "drop"})
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--label", "add,keep", "--unlabel", "drop")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if g := strings.Join(readItem(t, dir, "AWIT-TEST0001").Labels, ","); g != "keep,add" {
		t.Fatalf("labels = %s", g)
	}
}

func TestUpdateNothing(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "T", "B.", nil)
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001")
	if code != 1 || stderr != "Error: nothing to update\n" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
}

func TestUpdateBadStatus(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "T", "B.", nil)
	_, perr := item.ParseStatus("banana")
	if perr == nil {
		t.Fatal("ParseStatus(banana) succeeded")
	}
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--status", "banana")
	if code != 1 || stderr != "Error: "+perr.Error()+"\n" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
}

func TestUpdateUnknownItem(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--title", "x")
	if code != 1 || stderr != "Error: unknown item AWIT-TEST0001\n" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
}

func TestUpdateBrokenItem(t *testing.T) {
	dir := initRepo(t)
	p := filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md")
	if err := os.WriteFile(p, []byte("---\nid: AWIT-TEST0001\ntitle: [unclosed\nstatus: open\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--title", "x")
	if code != 1 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if !strings.Contains(stderr, "Error: AWIT-TEST0001:") || !strings.Contains(stderr, string(item.ReasonParse)) || !strings.Contains(stderr, "(fix the file, then retry)") {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestCloseClearsClaimedAt(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	st, err := item.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	it, err := st.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Date(2026, 9, 17, 14, 32, 5, 0, time.UTC)
	it.SetAssignee("agent/claude")
	it.SetClaimedAt(&ts)
	if err := st.Save(it); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "--repo", dir, "close", id)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "closed "+id+"\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "closed "+id+"\n")
	}
	got := readItem(t, dir, id)
	if got.Status != item.StatusClosed || got.ClaimedAt != nil || got.Assignee != "agent/claude" {
		t.Fatalf("status=%q claimed=%v assignee=%q", got.Status, got.ClaimedAt, got.Assignee)
	}
}

func TestCloseWithReasonWritesComment(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	code, stdout, stderr := run(t, "--repo", dir, "close", id, "--reason", "done", "--author", "jane")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "closed "+id+"\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "closed "+id+"\n")
	}
	got := readItem(t, dir, id)
	if got.Status != item.StatusClosed || len(got.Refs) != 1 || !strings.HasPrefix(got.Refs[0], ".awit/comments/"+id+"/") || strings.Contains(got.Refs[0], "\\") {
		t.Fatalf("status=%q refs=%v", got.Status, got.Refs)
	}
	ents, err := os.ReadDir(filepath.Join(dir, ".awit", "comments", id))
	if err != nil || len(ents) != 1 {
		t.Fatalf("comments: %v %v", ents, err)
	}
	body, err := os.ReadFile(filepath.Join(dir, ".awit", "comments", id, ents[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(body, []byte("author: jane")) || !bytes.Contains(body, []byte("done")) {
		t.Fatalf("comment = %s", body)
	}
}

func TestCloseReasonNeedsAuthor(t *testing.T) {
	dir := initRepo(t)
	t.Setenv("AWIT_AGENT", "")
	if name := gitx.UserName(dir); name != "" {
		t.Skipf("git user.name is %q", name)
	}
	seedItem(t, dir, "AWIT-TEST0001", "T", "B.", nil)
	code, _, stderr := run(t, "--repo", dir, "close", "AWIT-TEST0001", "--reason", "done")
	if code != 1 || stderr != "Error: no author; pass --author or set AWIT_AGENT\n" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
}

func TestReleaseClearsAssignee(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	st, err := item.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	it, err := st.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	ts := time.Date(2026, 9, 17, 14, 32, 5, 0, time.UTC)
	it.SetStatus(item.StatusInProgress)
	it.SetAssignee("bob")
	it.SetClaimedAt(&ts)
	if err := st.Save(it); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := run(t, "--repo", dir, "release", id)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	got := readItem(t, dir, id)
	if got.Status != item.StatusOpen || got.Assignee != "" || got.ClaimedAt != nil {
		t.Fatalf("status=%q assignee=%q claimed=%v", got.Status, got.Assignee, got.ClaimedAt)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".awit", "items", id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "assignee:") || strings.Contains(string(raw), "claimed_at:") {
		t.Fatalf("keys not deleted:\n%s", raw)
	}
}

func TestResolveAuthorPrecedence(t *testing.T) {
	tests := []struct{ name, flag, env, agentID, gitName, want, wantErr string }{
		{name: "flag verbatim", flag: "Jane Doe", want: "Jane Doe"},
		{name: "flag with slash", flag: "human/jane", want: "human/jane"},
		{name: "env prefixed", env: "claude", want: "agent/claude"},
		{name: "env already prefixed", env: "agent/claude", want: "agent/claude"},
		{name: "config prefixed", agentID: "codex", want: "agent/codex"},
		{name: "config already prefixed", agentID: "agent/codex", want: "agent/codex"},
		{name: "git sanitised", gitName: "Jane Doe", want: "jane-doe"},
		{name: "none", wantErr: "no author; pass --author or set AWIT_AGENT"},
		{name: "flag beats env", flag: "x", env: "y", want: "x"},
		{name: "env beats config", env: "e", agentID: "c", want: "agent/e"},
		{name: "config beats git", agentID: "c", gitName: "G", want: "agent/c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("AWIT_AGENT", tt.env)
			if tt.gitName != "" {
				if _, err := exec.LookPath("git"); err != nil {
					t.Skip("git not installed")
				}
				git := func(args ...string) {
					t.Helper()
					cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("git %s: %v: %s", args, err, out)
					}
				}
				git("init", "-q")
				git("config", "user.name", tt.gitName)
			}
			if tt.name == "none" && gitx.UserName(dir) != "" {
				t.Skipf("git user.name is %q", gitx.UserName(dir))
			}
			got, err := resolveAuthor(tt.flag, dir, config.Config{AgentID: tt.agentID})
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q err %v, want %q", got, err, tt.want)
			}
		})
	}
}
func TestUpdateEchoesChangedFields(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "T", "B.", nil)
	code, stdout, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--status", "in_progress")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	want := "updated AWIT-TEST0001: status=in_progress\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestUpdateEchoesMultipleFieldsInOrder(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "Old", "Old brief.", nil)
	code, stdout, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001",
		"--status", "in_progress", "--title", "New", "--brief", "New brief.", "--assign", "bob", "--label", "a")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	want := "updated AWIT-TEST0001: status=in_progress\n" +
		"updated AWIT-TEST0001: title=New\n" +
		"updated AWIT-TEST0001: brief=New brief.\n" +
		"updated AWIT-TEST0001: assignee=bob\n" +
		"updated AWIT-TEST0001: labels=a\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestUpdateClearExternal(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create",
		"--brief", "A linked issue.",
		"--id", "AWIT-TEST0001",
		"--external-tracker", "gitea",
		"--external-repo", "owner/repo",
		"--external-id", "127",
		"--external-url", "https://forge.example/owner/repo/issues/127",
		"Linked")
	if code != 0 {
		t.Fatalf("create exit %d stderr %q", code, stderr)
	}
	path := filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := readItem(t, dir, "AWIT-TEST0001").Body()
	code, stdout, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--clear-external")
	if code != 0 {
		t.Fatalf("exit %d stderr %q stdout %q", code, stdout, stderr)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(after, []byte("external:")) {
		t.Fatalf("external still present:\n%s", after)
	}
	if !bytes.Contains(after, []byte("title: Linked")) {
		t.Fatalf("unrelated keys changed:\n%s", after)
	}
	got := readItem(t, dir, "AWIT-TEST0001")
	if !bytes.Equal(got.Body(), body) {
		t.Fatalf("body changed: %q -> %q", body, got.Body())
	}
	if bytes.Equal(before, after) {
		t.Fatal("file unchanged after --clear-external")
	}
}

func TestUpdatePartialExternalDoesNotWrite(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "T", "B.", nil)
	path := filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001",
		"--external-tracker", "gitea", "--external-repo", "owner/repo")
	if code != 2 {
		t.Fatalf("exit %d, want 2 stderr %q", code, stderr)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("partial mapping wrote:\n%s", after)
	}
}

func TestUpdateInvalidExternalDoesNotWrite(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "T", "B.", nil)
	path := filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001",
		"--external-tracker", "gitea",
		"--external-repo", "owner/repo",
		"--external-id", "127",
		"--external-url", "https://forge.example/owner/repo/issues/999")
	if code != 2 {
		t.Fatalf("exit %d, want 2 stderr %q", code, stderr)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("invalid mapping wrote:\n%s", after)
	}
}

func TestUpdateClearExternalMutuallyExclusive(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "T", "B.", nil)
	path := filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001",
		"--clear-external",
		"--external-tracker", "gitea",
		"--external-repo", "owner/repo",
		"--external-id", "127",
		"--external-url", "https://forge.example/owner/repo/issues/127")
	if code != 2 {
		t.Fatalf("exit %d, want 2 stderr %q", code, stderr)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("mutually exclusive flags wrote:\n%s", after)
	}
}

func TestUpdateIdenticalExternalNoop(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create",
		"--brief", "A linked issue.",
		"--id", "AWIT-TEST0001",
		"--external-tracker", "gitea",
		"--external-repo", "owner/repo",
		"--external-id", "127",
		"--external-url", "https://forge.example/owner/repo/issues/127",
		"Linked")
	if code != 0 {
		t.Fatalf("create exit %d stderr %q", code, stderr)
	}
	path := filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001",
		"--external-tracker", "gitea",
		"--external-repo", "owner/repo",
		"--external-id", "127",
		"--external-url", "https://forge.example/owner/repo/issues/127")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("identical mapping rewrote:\n%s", after)
	}
	if stdout != "" {
		t.Fatalf("noop stdout = %q, want empty", stdout)
	}
}

func TestUpdateBodyReplacesBodyOnly(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "keep me", "--id", "AWIT-TEST0001", "-l", "auth", "T")
	if code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}

	code, _, stderr = run(t, "--repo", dir, "update", "AWIT-TEST0001", "--body", "## Replaced\n")
	if code != 0 {
		t.Fatalf("update: exit %d stderr %q", code, stderr)
	}

	it := readItem(t, dir, "AWIT-TEST0001")
	if got := string(it.Body()); got != "## Replaced\n" {
		t.Errorf("body = %q, want %q", got, "## Replaced\n")
	}
	if it.Brief != "keep me" {
		t.Errorf("brief = %q, want it untouched", it.Brief)
	}
	if len(it.Labels) != 1 || it.Labels[0] != "auth" {
		t.Errorf("labels = %v, want [auth] untouched", it.Labels)
	}
}

func TestUpdateBodyFileFromStdin(t *testing.T) {
	dir := initRepo(t)
	if code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", "AWIT-TEST0001", "T"); code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}

	code, _, stderr := runStdin(t, "## Piped\n", "--repo", dir, "update", "AWIT-TEST0001", "--body-file", "-")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if got := string(readItem(t, dir, "AWIT-TEST0001").Body()); got != "## Piped\n" {
		t.Errorf("body = %q, want %q", got, "## Piped\n")
	}
}

func TestUpdateBodyRefusesConflictMarkersWithoutWriting(t *testing.T) {
	dir := initRepo(t)
	if code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", "AWIT-TEST0001", "T"); code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}
	before := string(readItem(t, dir, "AWIT-TEST0001").Body())

	marker := strings.Repeat("<", 7) + " HEAD\nx\n" + strings.Repeat("=", 7) + "\ny\n" + strings.Repeat(">", 7) + " other\n"
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--body", marker)
	if code == 0 {
		t.Fatalf("exit = 0, want non-zero")
	}
	if !strings.Contains(stderr, "conflict markers") {
		t.Errorf("stderr = %q, want it to mention conflict markers", stderr)
	}
	if got := string(readItem(t, dir, "AWIT-TEST0001").Body()); got != before {
		t.Errorf("body changed to %q, want it unwritten", got)
	}
}

func TestUpdateBodyFlagsAreMutuallyExclusive(t *testing.T) {
	dir := initRepo(t)
	if code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", "AWIT-TEST0001", "T"); code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}

	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--body", "x", "--body-file", "y.md")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stderr %q)", code, stderr)
	}
}
