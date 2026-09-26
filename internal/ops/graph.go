package ops

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/id"
	"github.com/eisenwinter/awit/pkg/item"
)

// ErrUnknownItem marks the "no such key" outcome of ResolveItemID so
// callers can distinguish it from an ambiguity error.
var ErrUnknownItem = errors.New("unknown item")

func LoadGraph(s *item.Store) (*graph.Graph, error) {
	items, broken, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	return graph.Build(items, broken), nil
}

// LoadItem resolves a user-supplied key (canonical ID, alias, or external
// key) and loads the canonical item. Store.Load stays canonical-ID-only;
// resolution happens here, under the caller's mutation lock.
func LoadItem(s *item.Store, key string) (*item.Item, error) {
	items, _, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	idStr, err := ResolveItemID(items, key)
	if err != nil {
		if errors.Is(err, ErrUnknownItem) && id.Valid(s.Config.Prefix, key) {
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

// ResolveItemID maps a user-supplied key to a canonical item ID. Canonical
// IDs match exactly (case-sensitive) and take precedence; then aliases
// match case-insensitively; then external keys owner/repo#<n> or bare
// #<n> match valid external metadata. GitLab subgroup paths such as
// group/sub/project#127 are already accepted (the key parser splits on the
// last '#'); there is no new lookup syntax. Ambiguity across trackers or
// hosts lists the matching canonical IDs, sorted. There is no prefix,
// fuzzy, or title matching and no bare integer shorthand. A key never
// becomes a filesystem path.
func ResolveItemID(items []*item.Item, key string) (string, error) {
	for _, it := range items {
		if it.ID == key {
			return it.ID, nil
		}
	}
	var aliasHits []string
	for _, it := range items {
		if it.Alias != "" && strings.EqualFold(it.Alias, key) {
			aliasHits = append(aliasHits, it.ID)
		}
	}
	switch {
	case len(aliasHits) == 1:
		return aliasHits[0], nil
	case len(aliasHits) > 1:
		sort.Strings(aliasHits)
		return "", fmt.Errorf("ambiguous alias %q matches %s", key, strings.Join(aliasHits, ", "))
	}
	if repo, num, ok := parseExternalKey(key); ok {
		var hits []string
		for _, it := range items {
			if it.External == nil || it.External.ID != num {
				continue
			}
			if repo != "" && it.External.Repo != repo {
				continue
			}
			hits = append(hits, it.ID)
		}
		switch {
		case len(hits) == 1:
			return hits[0], nil
		case len(hits) > 1:
			sort.Strings(hits)
			return "", fmt.Errorf("ambiguous external key %q matches %s", key, strings.Join(hits, ", "))
		}
	}
	return "", fmt.Errorf("%w %s", ErrUnknownItem, key)
}

// parseExternalKey splits owner/repo#<n> or #<n> lookup keys. Repo may
// contain extra slashes (GitLab subgroups); only the last '#' is the
// number separator.
func parseExternalKey(key string) (repo string, num int64, ok bool) {
	rest, found := strings.CutPrefix(key, "#")
	if !found {
		i := strings.LastIndex(key, "#")
		if i <= 0 || !strings.Contains(key[:i], "/") {
			return "", 0, false
		}
		repo, rest = key[:i], key[i+1:]
	}
	n, err := strconv.ParseInt(rest, 10, 64)
	if err != nil || n <= 0 {
		return "", 0, false
	}
	return repo, n, true
}
