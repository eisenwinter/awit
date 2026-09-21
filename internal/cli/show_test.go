package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/item"
)

func TestShowClean(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0001")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	golden(t, "show-clean.golden", []byte(stdout))
}

func TestShowQuarantined(t *testing.T) {
	dir := copyFixture(t, "cyclic")
	code, stdout, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0001")
	if code != 0 || stderr != quarantineWarning(4) {
		t.Fatalf("exit %d stderr %q, want the single quarantine warning", code, stderr)
	}
	lines := strings.Split(stdout, "\n")
	if lines[0] != "[AWIT-TEST0001] Implement OAuth2 bearer token extraction" {
		t.Fatalf("header = %q", lines[0])
	}
	if lines[1] != "status: open (QUARANTINED) | labels: - | unblocks: -1" {
		t.Fatalf("status line = %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "fault: [CYCLE] ") {
		t.Fatalf("fault line = %q, want fault: [CYCLE] ...", lines[2])
	}
	if lines[3] != "deps: AWIT-TEST0002" {
		t.Fatalf("deps line = %q", lines[3])
	}
}

func TestShowBrokenFile(t *testing.T) {
	dir := copyFixture(t, "parse-error")
	code, stdout, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0001")
	if code != 0 || stderr != quarantineWarning(1) {
		t.Fatalf("exit %d stderr %q, want exit 0 and the single quarantine warning", code, stderr)
	}
	lines := strings.Split(stdout, "\n")
	if lines[0] != "[AWIT-TEST0001] (unparseable)" {
		t.Fatalf("header = %q", lines[0])
	}
	if len(lines) < 2 || !strings.HasPrefix(lines[1], "fault: ["+string(item.ReasonParse)+"] ") {
		t.Fatalf("stdout = %q, want fault: [PARSE ERROR] ... on line 2", stdout)
	}
}

func TestShowUnknown(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0099")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	if stderr != "Error: unknown item AWIT-TEST0099\n" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestShowJSON(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "show", "AWIT-TEST0001")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	var got struct {
		ID       string   `json:"id"`
		Title    string   `json:"title"`
		Status   string   `json:"status"`
		State    string   `json:"state"`
		Labels   []string `json:"labels"`
		Deps     []string `json:"deps"`
		Unblocks int      `json:"unblocks"`
		Body     string   `json:"body"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("unmarshal: %v\nstdout = %q", err, stdout)
	}
	if got.ID != "AWIT-TEST0001" || got.State != "ready" || got.Unblocks != 2 {
		t.Fatalf("entry = %+v", got)
	}
	if len(got.Labels) != 2 || got.Labels[0] != "auth" || got.Labels[1] != "p1" {
		t.Fatalf("labels = %v", got.Labels)
	}
	if !strings.Contains(got.Body, "URL-safe tokens with `-` and `_` are dropped") {
		t.Fatalf("body = %q", got.Body)
	}
	if !strings.HasSuffix(stdout, "\n") {
		t.Fatal("json output must end with a single newline")
	}
}

func TestShowExternalLine(t *testing.T) {
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
	code, stdout, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0001")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	want := "external: gitea owner/repo#127 https://forge.example/owner/repo/issues/127"
	if !strings.Contains(stdout, want) {
		t.Fatalf("show stdout missing %q:\n%s", want, stdout)
	}
	code, stdout, stderr = run(t, "--repo", dir, "--format", "json", "show", "AWIT-TEST0001")
	if code != 0 {
		t.Fatalf("json show exit %d stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, `"tracker": "gitea"`) || !strings.Contains(stdout, `"id": 127`) {
		t.Fatalf("json show missing external:\n%s", stdout)
	}
}

// mkChain creates Root ← Mid ← Leaf with deterministic IDs.
func mkChain(t *testing.T) string {
	t.Helper()
	dir := initRepo(t)
	steps := [][]string{
		{"create", "--brief", "B.", "--id", "AWIT-TEST0001", "Root"},
		{"create", "--brief", "B.", "--id", "AWIT-TEST0002", "-d", "AWIT-TEST0001", "Mid"},
		{"create", "--brief", "B.", "--id", "AWIT-TEST0003", "-d", "AWIT-TEST0002", "Leaf"},
	}
	for _, s := range steps {
		if code, _, stderr := run(t, append([]string{"--repo", dir}, s...)...); code != 0 {
			t.Fatalf("%v: exit %d stderr %q", s, code, stderr)
		}
	}
	return dir
}

func TestShowUnblocksListsTheTransitiveSet(t *testing.T) {
	dir := mkChain(t)

	code, stdout, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0001", "--unblocks", "--format", "compact")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	for _, want := range []string{"AWIT-TEST0002", "AWIT-TEST0003"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout %q missing %s", stdout, want)
		}
	}
	if strings.Contains(stdout, "AWIT-TEST0001") {
		t.Errorf("stdout %q must not list the item itself", stdout)
	}
}

func TestShowUnblocksEmptySetPrintsNothing(t *testing.T) {
	dir := mkChain(t)

	// The leaf unblocks nothing.
	code, stdout, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0003", "--unblocks", "--format", "compact")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

func TestShowUnblocksRejectsCombinedViews(t *testing.T) {
	dir := mkChain(t)

	code, _, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0001", "--unblocks", "--full")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stderr %q)", code, stderr)
	}
}
