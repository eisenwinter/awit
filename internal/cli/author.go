package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/eisenwinter/awit/internal/gitx"
	"github.com/eisenwinter/awit/pkg/config"
	"github.com/eisenwinter/awit/pkg/item"
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

func loadItem(s *item.Store, id string) (*item.Item, error) {
	it, err := s.Load(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("unknown item %s", id)
		}
		var br *item.BrokenError
		if errors.As(err, &br) {
			return nil, fmt.Errorf("%s: %s: %s (fix the file, then retry)", br.Broken.ID, br.Broken.Reason, br.Broken.Detail)
		}
		return nil, err
	}
	return it, nil
}
