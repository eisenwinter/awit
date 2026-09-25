package ops

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/eisenwinter/awit/pkg/item"
)

// Open honours repo (Open of the absolute path), else AWIT_REPO, else
// Find(cwd). Precedence: repo argument → AWIT_REPO → walk up from cwd.
func Open(repo string) (*item.Store, error) {
	if repo != "" {
		abs, err := filepath.Abs(repo)
		if err != nil {
			return nil, err
		}
		return item.Open(abs)
	}
	if repo := os.Getenv("AWIT_REPO"); repo != "" {
		abs, err := filepath.Abs(repo)
		if err != nil {
			return nil, err
		}
		return item.Open(abs)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return item.Find(cwd)
}

// WalkedUpNote returns the safety-brake note for a mutating command that
// resolved its root by walking up: repo is empty, AWIT_REPO is not set, and
// the working directory holds no .awit/ of its own. Otherwise it returns "".
// The line carries no trailing newline. Callers print it to stderr on the
// mutating path only, after Open succeeds.
func WalkedUpNote(repo string, s *item.Store) string {
	if repo != "" {
		return ""
	}
	if os.Getenv("AWIT_REPO") != "" {
		return ""
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if fi, err := os.Stat(filepath.Join(cwd, item.DirName)); err == nil && fi.IsDir() {
		return ""
	}
	return fmt.Sprintf("Note: no .awit in the current directory; using %s. Run awit init here, or pass --repo / set AWIT_REPO.", s.Root)
}
