package item

import "fmt"

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
		return "", fmt.Errorf("item: unknown status %q (open|in_progress|closed)", s)
	}
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
