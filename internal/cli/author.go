package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/eisenwinter/awit/internal/gitx"
	"github.com/eisenwinter/awit/pkg/config"
)

func withAgentPrefix(v string) string {
	if strings.Contains(v, "/") {
		return v
	}
	return "agent/" + v
}

func resolveAuthor(flagAuthor, repoRoot string, cfg config.Config) (string, error) {
	if flagAuthor != "" {
		return flagAuthor, nil
	}
	if v := os.Getenv("AWIT_AGENT"); v != "" {
		return withAgentPrefix(v), nil
	}
	if cfg.AgentID != "" {
		return withAgentPrefix(cfg.AgentID), nil
	}
	if name := gitx.UserName(repoRoot); name != "" {
		return strings.ReplaceAll(strings.ToLower(name), " ", "-"), nil
	}
	return "", fmt.Errorf("no author; pass --author or set AWIT_AGENT")
}
