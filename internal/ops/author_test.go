package ops_test

import (
	"os/exec"
	"testing"

	"github.com/eisenwinter/awit/internal/gitx"
	"github.com/eisenwinter/awit/internal/ops"
	"github.com/eisenwinter/awit/pkg/config"
)

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
			got, err := ops.ResolveAuthor(tt.flag, dir, config.Config{AgentID: tt.agentID})
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

func TestWithAgentPrefix(t *testing.T) {
	for in, want := range map[string]string{"claude": "agent/claude", "agent/claude": "agent/claude", "human/jane": "human/jane"} {
		if got := ops.WithAgentPrefix(in); got != want {
			t.Errorf("WithAgentPrefix(%q) = %q, want %q", in, got, want)
		}
	}
}
