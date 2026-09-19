package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/eisenwinter/awit/internal/gitx"
	"github.com/eisenwinter/awit/pkg/config"
	"github.com/eisenwinter/awit/pkg/id"
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

// loadItem resolves a user-supplied key (canonical ID, alias, or external
// key) and loads the canonical item. Store.Load stays canonical-ID-only;
// resolution happens here, under the caller's mutation lock.
func loadItem(s *item.Store, key string) (*item.Item, error) {
	items, _, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	idStr, err := resolveItemID(items, key)
	if err != nil {
		if errors.Is(err, errUnknownItem) && id.Valid(s.Config.Prefix, key) {
			// Preserve the broken-file diagnostics for canonical IDs.
			if _, lerr := s.Load(key); lerr != nil {
				var br *item.BrokenError
				if errors.As(lerr, &br) {
					return nil, fmt.Errorf("%s: %s: %s (fix the file, then retry)", br.Broken.ID, br.Broken.Reason, br.Broken.Detail)
				}
			}
		}
		return nil, err
	}
	it, err := s.Load(idStr)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("unknown item %s", key)
		}
		var br *item.BrokenError
		if errors.As(err, &br) {
			return nil, fmt.Errorf("%s: %s: %s (fix the file, then retry)", br.Broken.ID, br.Broken.Reason, br.Broken.Detail)
		}
		return nil, err
	}
	return it, nil
}
