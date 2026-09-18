package item

import (
	"fmt"
	"strings"
)

type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusClosed     Status = "closed"
)

func ParseStatus(s string) (Status, error) {
	switch Status(s) {
	case StatusOpen, StatusInProgress, StatusClosed:
		return Status(s), nil
	default:
		msg := fmt.Sprintf("item: unknown status %q (open|in_progress|closed)", s)
		if sug := suggestStatus(s); sug != "" {
			msg += fmt.Sprintf("; did you mean %q?", sug)
		}
		return "", fmt.Errorf("%s", msg)
	}
}

// suggestStatus returns the closest valid status to s when it is only a
// keystroke or two away (Levenshtein distance <= 3), or "" when s resembles
// nothing valid. Comparison is case-insensitive so "Open" suggests "open".
func suggestStatus(s string) string {
	s = strings.ToLower(s)
	best, bestDist := "", -1
	for _, c := range []string{string(StatusOpen), string(StatusInProgress), string(StatusClosed)} {
		if d := levenshtein(s, c); bestDist < 0 || d < bestDist {
			best, bestDist = c, d
		}
	}
	if bestDist < 0 || bestDist > 3 {
		return ""
	}
	return best
}

// levenshtein is the edit distance between two short ASCII strings.
func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

type Reason string

const (
	ReasonParse      Reason = "PARSE ERROR"
	ReasonConflict   Reason = "CONFLICT MARKERS"
	ReasonIDMismatch Reason = "ID MISMATCH"
	ReasonDuplicate  Reason = "DUPLICATE ID"
	ReasonDangling   Reason = "DANGLING DEP"
	ReasonCycle      Reason = "CYCLE"
)

type Broken struct {
	ID     string
	Path   string
	Reason Reason
	Detail string
}
