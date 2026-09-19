package item

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/eisenwinter/awit/pkg/config"
)

func (s *Store) ArchiveDir() string           { return filepath.Join(s.Dir, "archive") }
func (s *Store) ArchivePath(id string) string { return filepath.Join(s.ArchiveDir(), id+".md") }

// Archive collapses it and its comments into ArchivePath(it.ID),
// moves attachments to ArchiveDir()/<id>/, then removes the item file
// and its comments directory. Eligibility is the caller's job.
func (s *Store) Archive(it *Item) error {
	comments, err := s.Comments(it.ID)
	if err != nil {
		return err
	}
	if err := s.NormalizeRefs(it); err != nil {
		return err
	}
	// 1. rewrite refs: drop comment refs, redirect attachment refs.
	prefix := path.Join(".awit/comments", it.ID) + "/"
	byFile := make(map[string]Comment, len(comments))
	for _, c := range comments {
		byFile[c.File] = c
	}
	refs := make([]string, 0, len(it.Refs))
	for _, r := range it.Refs {
		if !strings.HasPrefix(r, prefix) {
			refs = append(refs, r)
			continue
		}
		if c, ok := byFile[strings.TrimPrefix(r, prefix)]; ok && c.Attachment {
			refs = append(refs, path.Join(".awit/archive", it.ID, c.File))
		}
		// comment refs and refs to missing files are dropped
	}
	it.SetRefs(refs)
	data, err := it.Bytes()
	if err != nil {
		return err
	}
	// 2. append the collapsed comments.
	var b bytes.Buffer
	b.Write(data)
	first := true
	for _, c := range comments {
		if c.Attachment {
			continue
		}
		if first {
			b.WriteString("\n## Comments\n")
			first = false
		}
		fmt.Fprintf(&b, "\n### %s %s\n\n%s\n", c.Created.Format(time.RFC3339), c.Author, c.Text)
	}
	// 3. write archive file, move attachments, delete originals.
	if err := os.MkdirAll(s.ArchiveDir(), 0o755); err != nil {
		return err
	}
	if err := config.WriteAtomic(s.ArchivePath(it.ID), b.Bytes()); err != nil {
		return err
	}
	for _, c := range comments {
		if !c.Attachment {
			continue
		}
		if err := moveFile(filepath.Join(s.CommentsDir(it.ID), c.File), filepath.Join(s.ArchiveDir(), it.ID, c.File)); err != nil {
			return err
		}
	}
	if err := os.Remove(s.ItemPath(it.ID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.RemoveAll(s.CommentsDir(it.ID))
}

// moveFile renames src to dst, creating dst's directory; on rename
// failure (cross-device) it copies atomically and removes src.
func moveFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := config.WriteAtomic(dst, data); err != nil {
		return err
	}
	return os.Remove(src)
}
