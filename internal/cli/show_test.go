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
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
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
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q, want exit 0", code, stderr)
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
