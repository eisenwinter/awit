// Package resolver maps the forward-slash refs stored in item frontmatter to
// files on disk. The caller chooses the base directory: Store.Root for items
// with refs_base: repo, or ItemsDir() for historical items-relative refs.
package resolver

import (
	"os"
	"path/filepath"
	"strings"
)

// Resolved is the outcome of resolving one ref. Content is nil whenever Err
// is non-nil; Path is always populated so callers can print where they looked.
type Resolved struct {
	Ref     string // as written in frontmatter
	Path    string // absolute, OS separators
	Content []byte // nil when Err != nil
	Err     error  // os.ErrNotExist etc.
}

// Resolve maps every ref relative to baseDir (FromSlash applied) and reads
// it. The result has exactly one element per ref, in input order. Resolve
// never returns an error itself; per-ref failures live in Resolved.Err.
func Resolve(baseDir string, refs []string) []Resolved {
	out := make([]Resolved, 0, len(refs))
	for _, ref := range refs {
		p := filepath.Clean(filepath.Join(baseDir, filepath.FromSlash(ref)))
		abs, err := filepath.Abs(p)
		if err != nil {
			out = append(out, Resolved{Ref: ref, Path: p, Err: err})
			continue
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			out = append(out, Resolved{Ref: ref, Path: abs, Err: err})
			continue
		}
		out = append(out, Resolved{Ref: ref, Path: abs, Content: data})
	}
	return out
}

// IsItemRef reports whether path names a ".md" file directly inside itemsDir
// and returns its stem, which is the item ID. It does not touch the disk:
// whether the file exists is the caller's concern (see Resolved.Err).
func IsItemRef(itemsDir, path string) (id string, ok bool) {
	absDir, err := filepath.Abs(itemsDir)
	if err != nil {
		return "", false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(absDir, absPath)
	if err != nil {
		return "", false
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	if strings.ContainsRune(rel, filepath.Separator) {
		return "", false
	}
	stem, isMD := strings.CutSuffix(rel, ".md")
	if !isMD || stem == "" {
		return "", false
	}
	return stem, true
}
