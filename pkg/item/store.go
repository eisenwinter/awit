package item

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/eisenwinter/awit/internal/gitx"
	"github.com/eisenwinter/awit/pkg/config"
	"github.com/eisenwinter/awit/pkg/id"
	"github.com/eisenwinter/awit/pkg/lock"
)

const DirName = ".awit"

type Store struct {
	Root   string
	Dir    string
	Config config.Config
}

var ErrNotFound = errors.New("no .awit directory found (run awit init)")
var ErrExists = errors.New(".awit already exists")

func Find(start string) (*Store, error) {
	abs, err := filepath.Abs(start)
	if err != nil {
		return nil, err
	}
	cur := abs
	for {
		if fi, err := os.Stat(filepath.Join(cur, DirName)); err == nil && fi.IsDir() {
			return Open(cur)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return nil, ErrNotFound
		}
		cur = parent
	}
}

func Open(repoRoot string) (*Store, error) {
	dir := filepath.Join(repoRoot, DirName)
	fi, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("item: open %s: %w", dir, err)
	}
	if !fi.IsDir() {
		return nil, fmt.Errorf("item: open %s: not a directory", dir)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return nil, err
	}
	return &Store{Root: repoRoot, Dir: dir, Config: cfg}, nil
}

func Init(repoRoot, prefix string) (*Store, error) {
	if _, err := os.Stat(filepath.Join(repoRoot, DirName)); err == nil {
		return nil, ErrExists
	}
	dir := filepath.Join(repoRoot, DirName)
	if err := os.MkdirAll(filepath.Join(dir, "items"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "comments"), 0o755); err != nil {
		return nil, err
	}
	if err := config.Default(prefix).Write(dir); err != nil {
		return nil, err
	}
	if err := ensureGitignore(repoRoot); err != nil {
		return nil, err
	}
	return Open(repoRoot)
}

func ensureGitignore(repoRoot string) error {
	p := filepath.Join(repoRoot, ".gitignore")
	data, err := os.ReadFile(p)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	content := string(data)
	dirty := false
	if content != "" && !strings.HasSuffix(content, "\n") {
		content += "\n"
		dirty = true
	}
	for _, line := range strings.Split(strings.TrimSuffix(content, "\n"), "\n") {
		if line == ".awit/.lock" {
			if dirty {
				return config.WriteAtomic(p, []byte(content))
			}
			return nil
		}
	}
	if content == "" {
		content = ".awit/.lock\n"
	} else {
		content += ".awit/.lock\n"
	}
	return config.WriteAtomic(p, []byte(content))
}

func (s *Store) ItemsDir() string {
	return filepath.Join(s.Dir, "items")
}

func (s *Store) CommentsDir(id string) string {
	return filepath.Join(s.Dir, "comments", id)
}

func (s *Store) ItemPath(id string) string {
	return filepath.Join(s.ItemsDir(), id+".md")
}

func (s *Store) Exists(id string) bool {
	_, err := os.Stat(s.ItemPath(id))
	return err == nil
}

type BrokenError struct {
	Broken Broken
}

func (e *BrokenError) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.Broken.Reason, e.Broken.ID, e.Broken.Detail)
}

func (s *Store) Load(id string) (*Item, error) {
	path := s.ItemPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if HasConflictMarkers(data) {
		return nil, &BrokenError{Broken: Broken{ID: id, Path: path, Reason: ReasonConflict, Detail: "conflict markers in file"}}
	}
	it, err := Parse(path, data)
	if err != nil {
		return nil, &BrokenError{Broken: Broken{ID: id, Path: path, Reason: ReasonParse, Detail: err.Error()}}
	}
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if it.ID != stem {
		return nil, &BrokenError{Broken: Broken{
			ID: stem, Path: path, Reason: ReasonIDMismatch,
			Detail: fmt.Sprintf("id %q != filename stem %q", it.ID, stem),
		}}
	}
	return it, nil
}

func (s *Store) LoadAll() ([]*Item, []Broken, error) {
	return loadDir(s.ItemsDir())
}

// LoadArchive reads .awit/archive/<id>.md files with the same parsing and
// duplicate rules as LoadAll. A missing archive directory yields nil, nil, nil.
func (s *Store) LoadArchive() ([]*Item, []Broken, error) {
	if _, err := os.Stat(s.ArchiveDir()); errors.Is(err, os.ErrNotExist) {
		return nil, nil, nil
	}
	return loadDir(s.ArchiveDir())
}

func loadDir(dir string) ([]*Item, []Broken, error) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(ents, func(i, j int) bool { return ents[i].Name() < ents[j].Name() })

	type rec struct {
		stem   string
		path   string
		item   *Item
		broken *Broken
	}
	var files []rec
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		stem := strings.TrimSuffix(e.Name(), ".md")
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		r := rec{stem: stem, path: path}
		switch {
		case HasConflictMarkers(data):
			r.broken = &Broken{ID: stem, Path: path, Reason: ReasonConflict, Detail: "conflict markers in file"}
		default:
			it, err := Parse(path, data)
			switch {
			case err != nil:
				r.broken = &Broken{ID: stem, Path: path, Reason: ReasonParse, Detail: err.Error()}
			case it.ID != stem:
				r.broken = &Broken{ID: stem, Path: path, Reason: ReasonIDMismatch,
					Detail: fmt.Sprintf("id %q != filename stem %q", it.ID, stem)}
			default:
				r.item = it
			}
		}
		files = append(files, r)
	}

	groups := map[string][]int{}
	for i, f := range files {
		key := strings.ToUpper(f.stem)
		groups[key] = append(groups[key], i)
	}
	for _, idxs := range groups {
		if len(idxs) < 2 {
			continue
		}
		for _, i := range idxs {
			files[i].item = nil
			files[i].broken = &Broken{
				ID: files[i].stem, Path: files[i].path,
				Reason: ReasonDuplicate, Detail: "duplicate id (case-insensitive)",
			}
		}
	}

	var items []*Item
	var broken []Broken
	for _, f := range files {
		if f.broken != nil {
			broken = append(broken, *f.broken)
			continue
		}
		items = append(items, f.item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	sort.Slice(broken, func(i, j int) bool { return broken[i].ID < broken[j].ID })
	return items, broken, nil
}

func (s *Store) Save(it *Item) error {
	if err := s.NormalizeRefs(it); err != nil {
		return err
	}
	data, err := it.Bytes()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.ItemsDir(), 0o755); err != nil {
		return err
	}
	p := s.ItemPath(it.ID)
	if err := config.WriteAtomic(p, data); err != nil {
		return err
	}
	it.Path = p
	return nil
}

// NormalizeRefs rewrites historical items-relative refs to repo-root
// relative paths and sets refs_base: repo. Already-marked items are
// left untouched. Conversion is mathematical (filepath.Rel) with no
// existence check.
func (s *Store) NormalizeRefs(it *Item) error {
	if it.RefsBase == "repo" {
		return nil
	}
	newRefs := make([]string, len(it.Refs))
	for i, ref := range it.Refs {
		joined := filepath.Join(s.ItemsDir(), filepath.FromSlash(ref))
		rel, err := filepath.Rel(s.Root, joined)
		if err != nil {
			return fmt.Errorf("item: cannot represent ref %q relative to repo root: %w", ref, err)
		}
		newRefs[i] = filepath.ToSlash(rel)
	}
	it.SetRefs(newRefs)
	return it.SetRefsBase("repo")
}

func (s *Store) Mint(now time.Time) (string, error) {
	return id.Mint(s.Config.Prefix, now, id.Worker(s.Root, gitx.Branch(s.Root)), s.Exists)
}

// Lock takes an exclusive advisory lock on Dir/.lock, creating the file.
// On lock.ErrTimeout the returned error is
// "another awit process holds .awit/.lock (waited <timeout>)".
func (s *Store) Lock(timeout time.Duration) (func() error, error) {
	rel, err := lock.Acquire(filepath.Join(s.Dir, ".lock"), timeout)
	if err == nil {
		return rel, nil
	}
	if errors.Is(err, lock.ErrTimeout) {
		return nil, fmt.Errorf("another awit process holds .awit/.lock (waited %s)", timeout)
	}
	return nil, err
}
