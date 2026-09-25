package ops

import (
	"fmt"
	"os"
	"strings"

	"github.com/eisenwinter/awit/internal/gitx"
	"github.com/eisenwinter/awit/pkg/config"
)

// WithAgentPrefix prefixes v with "agent/" unless it already carries a "/".
func WithAgentPrefix(v string) string {
	if strings.Contains(v, "/") {
		return v
	}
	return "agent/" + v
}

// ResolveAuthor picks the comment author: flagAuthor verbatim, else
// AWIT_AGENT (agent/ prefixed), else config agent_id (agent/ prefixed), else
// the git user.name lower-cased with spaces as dashes; otherwise
// "no author; pass --author or set AWIT_AGENT".
func ResolveAuthor(flagAuthor, repoRoot string, cfg config.Config) (string, error) {
	if flagAuthor != "" {
		return flagAuthor, nil
	}
	if v := os.Getenv("AWIT_AGENT"); v != "" {
		return WithAgentPrefix(v), nil
	}
	if cfg.AgentID != "" {
		return WithAgentPrefix(cfg.AgentID), nil
	}
	if name := gitx.UserName(repoRoot); name != "" {
		return strings.ReplaceAll(strings.ToLower(name), " ", "-"), nil
	}
	return "", fmt.Errorf("no author; pass --author or set AWIT_AGENT")
}
