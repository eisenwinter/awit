package item

import (
	"bytes"
	"errors"
	"strings"
)

var (
	ErrNoFrontmatter           = errors.New("item: no frontmatter")
	ErrUnterminatedFrontmatter = errors.New("item: unterminated frontmatter")
)

// Split separates YAML frontmatter from the markdown body.
// data must start with "---\n" or "---\r\n". The closing fence is a line
// whose trimmed text is exactly "---". front is the bytes between the two
// fence lines; body is every byte after the closing fence line (including
// its terminating newline being consumed, not copied into body).
func Split(data []byte) (front, body []byte, err error) {
	var rest []byte
	switch {
	case bytes.HasPrefix(data, []byte("---\r\n")):
		rest = data[len("---\r\n"):]
	case bytes.HasPrefix(data, []byte("---\n")):
		rest = data[len("---\n"):]
	default:
		return nil, nil, ErrNoFrontmatter
	}
	lineStart := 0
	for lineStart <= len(rest) {
		lineEnd := lineStart
		for lineEnd < len(rest) && rest[lineEnd] != '\n' {
			lineEnd++
		}
		line := rest[lineStart:lineEnd]
		if bytes.Equal(bytes.TrimSpace(line), []byte("---")) {
			front = rest[:lineStart]
			after := lineEnd
			if after < len(rest) && rest[after] == '\n' {
				after++
			}
			return front, rest[after:], nil
		}
		if lineEnd == len(rest) {
			break
		}
		lineStart = lineEnd + 1
	}
	return nil, nil, ErrUnterminatedFrontmatter
}

// HasConflictMarkers reports git conflict markers. A line matches when its
// TrimSpace text has prefix "<<<<<<< " or ">>>>>>> ", or is entirely '='
// runes of length >= 7. A show delimiter "===== REF 1/1 =====" must not match
// (it is not all-equals, and it is not 7 leading equals as the whole line).
func HasConflictMarkers(data []byte) bool {
	lineStart := 0
	for lineStart <= len(data) {
		lineEnd := lineStart
		for lineEnd < len(data) && data[lineEnd] != '\n' {
			lineEnd++
		}
		trimmed := strings.TrimSpace(string(data[lineStart:lineEnd]))
		switch {
		case strings.HasPrefix(trimmed, "<<<<<<< "):
			return true
		case strings.HasPrefix(trimmed, ">>>>>>> "):
			return true
		case len(trimmed) >= 7 && strings.Trim(trimmed, "=") == "":
			return true
		}
		if lineEnd == len(data) {
			break
		}
		lineStart = lineEnd + 1
	}
	return false
}
