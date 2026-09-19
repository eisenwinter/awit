package teax

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/internal/teax/teaxtest"
)

func openStateClient(t *testing.T) (*Client, string) {
	t.Helper()
	dir := teaxtest.Install(t)
	writeLogins(t, dir, [2]string{"sandbox", "https://forge.example"})
	c, err := Open(context.Background(), testExternal(), "")
	if err != nil {
		t.Fatal(err)
	}
	return c, dir
}

func stubState(t *testing.T, dir string, n int64) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, fmt.Sprintf("repos_owner_repo_issues_%d.json", n)))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		State *string `json:"state"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	if m.State == nil {
		return ""
	}
	return *m.State
}

func TestTeaSetStateClosed(t *testing.T) {
	c, dir := openStateClient(t)
	scriptTeaIssue(t, dir, 127, `{"number":127,"title":"T","state":"open","body":"b"}`)
	if err := c.SetState(context.Background(), 127, "closed"); err != nil {
		t.Fatalf("SetState closed: %v", err)
	}
	if got := stubState(t, dir, 127); got != "closed" {
		t.Fatalf("remote state = %q, want closed", got)
	}
}

func TestTeaSetStateOpen(t *testing.T) {
	c, dir := openStateClient(t)
	scriptTeaIssue(t, dir, 127, `{"number":127,"title":"T","state":"closed","body":"b"}`)
	if err := c.SetState(context.Background(), 127, "open"); err != nil {
		t.Fatalf("SetState open: %v", err)
	}
	if got := stubState(t, dir, 127); got != "open" {
		t.Fatalf("remote state = %q, want open", got)
	}
}

func TestTeaSetStateArgv(t *testing.T) {
	c, dir := openStateClient(t)
	scriptTeaIssue(t, dir, 127, `{"number":127,"title":"T","state":"open","body":"b"}`)
	if err := c.SetState(context.Background(), 127, "closed"); err != nil {
		t.Fatalf("SetState: %v", err)
	}
	found := false
	for _, args := range argvLog(t, dir) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "PATCH") && strings.Contains(joined, "state=closed") &&
			strings.HasSuffix(joined, "repos/owner/repo/issues/127") {
			found = true
			for _, want := range []string{"--login sandbox", "--repo owner/repo", "--include", "-X PATCH", "-f state=closed"} {
				if !strings.Contains(joined, want) {
					t.Fatalf("state PATCH argv %v missing %q", args, want)
				}
			}
		}
	}
	if !found {
		t.Fatalf("argv.log missing state PATCH: %v", argvLog(t, dir))
	}
}

func TestTeaSetStateRejectsHTTPErrorStatusExitZero(t *testing.T) {
	c, dir := openStateClient(t)
	if err := os.WriteFile(filepath.Join(dir, "repos_owner_repo_issues_127.status"), []byte("403"), 0o644); err != nil {
		t.Fatal(err)
	}
	scriptTeaIssue(t, dir, 127, `{"message":"Forbidden"}`)
	err := c.SetState(context.Background(), 127, "closed")
	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err = %v, want HTTP 403 named", err)
	}
}

func TestTeaSetStateVerificationMismatch(t *testing.T) {
	c, dir := openStateClient(t)
	t.Setenv("TEA_STUB_PATCH_NO_STORE", "1")
	scriptTeaIssue(t, dir, 127, `{"number":127,"title":"T","state":"open","body":"b"}`)
	err := c.SetState(context.Background(), 127, "closed")
	if err == nil || !strings.Contains(err.Error(), "verification") {
		t.Fatalf("err = %v, want a verification failure", err)
	}
}

func TestTeaSetStateWrongNumberIdentity(t *testing.T) {
	c, dir := openStateClient(t)
	t.Setenv("TEA_STUB_PATCH_WRONG_NUMBER", "1")
	scriptTeaIssue(t, dir, 127, `{"number":127,"title":"T","state":"open","body":"b"}`)
	err := c.SetState(context.Background(), 127, "closed")
	if err == nil || !strings.Contains(err.Error(), "127") {
		t.Fatalf("err = %v, want the issue number named", err)
	}
}
