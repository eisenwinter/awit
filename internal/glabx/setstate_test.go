package glabx

import (
	"context"
	"strings"
	"testing"
)

// --- SetState (stub-backed) ---

func TestGlabSetState(t *testing.T) {
	for _, tc := range []struct {
		name, from, want, event string
	}{
		{"close an open issue", "opened", "closed", "close"},
		{"reopen a closed issue", "closed", "open", "reopen"},
		{"same-state close repeats the event", "closed", "closed", "close"},
		{"same-state reopen repeats the event", "opened", "open", "reopen"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, dir := openClient(t)
			scriptGlabIssue(t, dir, `{"id":9,"iid":127,"title":"T","description":"kept","labels":["kept"],"state":"`+tc.from+`","web_url":"https://forge.example/group/project/-/issues/127"}`)
			if err := c.SetState(context.Background(), 127, tc.want); err != nil {
				t.Fatalf("SetState: %v", err)
			}
			if got := stubStored(t, dir, "projects_group%2Fproject_issues_127")["state"]; got != wireState(tc.want) {
				t.Fatalf("stored state = %v, want %q", got, wireState(tc.want))
			}
			stored := stubStored(t, dir, "projects_group%2Fproject_issues_127")
			if stored["description"] != "kept" || stored["title"] != "T" {
				t.Fatalf("state PUT must leave body/title unchanged: %v", stored)
			}
			var sawEvent bool
			for _, args := range argvLog(t, dir) {
				joined := strings.Join(args, " ")
				if !strings.Contains(joined, "--method PUT") || !strings.HasSuffix(joined, "projects/group%2Fproject/issues/127") {
					continue
				}
				sawEvent = true
				if !strings.Contains(joined, "-f state_event="+tc.event) {
					t.Fatalf("state argv %v missing state_event=%s", args, tc.event)
				}
				for _, a := range args {
					if strings.Contains(a, "description=") {
						t.Fatalf("state PUT must be state-only; argv carries %q", a)
					}
				}
			}
			if !sawEvent {
				t.Fatalf("argv.log missing state_event PUT: %v", argvJoined(argvLog(t, dir)))
			}
			iss, err := c.GetIssue(context.Background(), 127)
			if err != nil {
				t.Fatalf("GetIssue: %v", err)
			}
			if iss.State != tc.want {
				t.Fatalf("read-back state = %q, want %q", iss.State, tc.want)
			}
		})
	}
}

func wireState(want string) string {
	if want == "open" {
		return "opened"
	}
	return "closed"
}

func TestGlabSetStateRejects(t *testing.T) {
	c, dir := openClient(t)
	scriptGlabIssue(t, dir, `{"id":9,"iid":127,"title":"T","description":"d","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
	for _, state := range []string{"", "opened", "reopened", "close", "OPEN", "invalid"} {
		if err := c.SetState(context.Background(), 127, state); err == nil {
			t.Fatalf("state %q must be refused before any PUT", state)
		}
	}
	if n := countPUTs(t, dir); n != 0 {
		t.Fatalf("%d PUTs after refused states, want zero", n)
	}
}

func TestGlabSetStateHTTPFailures(t *testing.T) {
	for _, status := range []string{"401", "403", "404", "422", "500"} {
		t.Run(status, func(t *testing.T) {
			c, dir := openClient(t)
			writeFile(t, dir, "projects_group%2Fproject_issues_127.status", status)
			scriptGlabIssue(t, dir, `{"message":"no"}`)
			err := c.SetState(context.Background(), 127, "closed")
			if err == nil || !strings.Contains(err.Error(), status) {
				t.Fatalf("err = %v, want HTTP %s named", err, status)
			}
		})
	}
}

func TestGlabSetStateVerification(t *testing.T) {
	t.Run("server ignores the event", func(t *testing.T) {
		c, dir := openClient(t)
		t.Setenv("GLAB_STUB_PATCH_NO_STORE", "1")
		scriptGlabIssue(t, dir, `{"id":9,"iid":127,"title":"T","description":"d","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
		err := c.SetState(context.Background(), 127, "closed")
		if err == nil || !strings.Contains(err.Error(), "verification") {
			t.Fatalf("err = %v, want a verification failure", err)
		}
	})
	t.Run("missing state falls back to GET", func(t *testing.T) {
		c, dir := openClient(t)
		t.Setenv("GLAB_STUB_PATCH_OMIT_STATE", "1")
		scriptGlabIssue(t, dir, `{"id":9,"iid":127,"title":"T","description":"d","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
		if err := c.SetState(context.Background(), 127, "closed"); err != nil {
			t.Fatalf("SetState: %v", err)
		}
	})
	t.Run("fallback mismatch fails", func(t *testing.T) {
		c, dir := openClient(t)
		t.Setenv("GLAB_STUB_PATCH_OMIT_STATE", "1")
		t.Setenv("GLAB_STUB_PATCH_NO_STORE", "1")
		scriptGlabIssue(t, dir, `{"id":9,"iid":127,"title":"T","description":"d","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
		err := c.SetState(context.Background(), 127, "closed")
		if err == nil || !strings.Contains(err.Error(), "verification") {
			t.Fatalf("err = %v, want a verification failure", err)
		}
	})
	t.Run("wrong identity fails", func(t *testing.T) {
		c, dir := openClient(t)
		t.Setenv("GLAB_STUB_PATCH_WRONG_IID", "1")
		scriptGlabIssue(t, dir, `{"id":9,"iid":127,"title":"T","description":"d","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
		err := c.SetState(context.Background(), 127, "closed")
		if err == nil || !strings.Contains(err.Error(), "127") {
			t.Fatalf("err = %v, want the issue number named", err)
		}
	})
	t.Run("malformed response fails", func(t *testing.T) {
		c, dir := openClient(t)
		t.Setenv("GLAB_STUB_PATCH_KEEP_RESPONSE", "1")
		writeFile(t, dir, "projects_group%2Fproject_issues_127.patch-response", `not json`)
		scriptGlabIssue(t, dir, `{"id":9,"iid":127,"title":"T","description":"d","labels":[],"state":"opened","web_url":"https://forge.example/group/project/-/issues/127"}`)
		if err := c.SetState(context.Background(), 127, "closed"); err == nil {
			t.Fatal("malformed PUT response must fail, not fall back silently")
		}
	})
}
