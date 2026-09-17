package item

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/eisenwinter/awit/pkg/config"
)

func SanitizeAuthor(author string) string {
	s := strings.TrimPrefix(author, "agent/")
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if ok {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := b.String()
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	out = strings.Trim(out, "-")
	if out == "" {
		return "anon"
	}
	return out
}

func CommentFileName(now time.Time, author, ext string) string {
	return now.UTC().Format("20060102T150405Z") + "-" + SanitizeAuthor(author) + ext
}

func uniqueCommentFile(dir, stampAuthor, ext string) (string, error) {
	for n := 1; n < 10000; n++ {
		var name string
		if n == 1 {
			name = stampAuthor + ext
		} else {
			name = stampAuthor + "-" + strconv.Itoa(n) + ext
		}
		_, err := os.Stat(filepath.Join(dir, name))
		if errors.Is(err, os.ErrNotExist) {
			return name, nil
		}
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("item: comment filename collisions in %s", dir)
}

func (s *Store) AddComment(it *Item, author string, now time.Time, text string) (string, error) {
	dir := s.CommentsDir(it.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	base := now.UTC().Format("20060102T150405Z") + "-" + SanitizeAuthor(author)
	filename, err := uniqueCommentFile(dir, base, ".md")
	if err != nil {
		return "", err
	}
	created := now.UTC().Format(time.RFC3339)
	body := "---\nauthor: " + author + "\ncreated: " + created + "\n---\n\n" + strings.TrimSpace(text) + "\n"
	if err := config.WriteAtomic(filepath.Join(dir, filename), []byte(body)); err != nil {
		return "", err
	}
	ref := path.Join("../comments", it.ID, filename)
	refs := append(append([]string{}, it.Refs...), ref)
	it.SetRefs(refs)
	if err := s.Save(it); err != nil {
		return "", err
	}
	return ref, nil
}

func (s *Store) AttachFile(it *Item, author string, now time.Time, src string) (string, error) {
	dir := s.CommentsDir(it.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ext := filepath.Ext(src)
	base := now.UTC().Format("20060102T150405Z") + "-" + SanitizeAuthor(author)
	filename, err := uniqueCommentFile(dir, base, ext)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	if err := config.WriteAtomic(filepath.Join(dir, filename), data); err != nil {
		return "", err
	}
	ref := path.Join("../comments", it.ID, filename)
	refs := append(append([]string{}, it.Refs...), ref)
	it.SetRefs(refs)
	if err := s.Save(it); err != nil {
		return "", err
	}
	return ref, nil
}
