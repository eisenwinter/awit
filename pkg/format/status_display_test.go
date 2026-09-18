package format

import (
	"bytes"
	"strings"
	"testing"
)

// RED test for AWIT-0NF68SDR: compact and table list views must carry each
// item's status distinctly (open vs in_progress vs closed) without json.
func TestStatusVisibleWithoutJSON(t *testing.T) {
	open := Entry{ID: "AWIT-TEST0001", Title: "T", Status: "open", State: "ready", Labels: []string{}, Unblocks: 0}
	inProgress := Entry{ID: "AWIT-TEST0002", Title: "T", Status: "in_progress", State: "ready", Labels: []string{}, Unblocks: 0}
	closed := Entry{ID: "AWIT-TEST0003", Title: "T", Status: "closed", State: "closed", Labels: []string{}, Unblocks: 0}

	for _, e := range []Entry{open, inProgress, closed} {
		if !strings.Contains(Line(e), e.Status) {
			t.Fatalf("Line(%q) = %q, want status %q visible", e.ID, Line(e), e.Status)
		}
	}

	var buf bytes.Buffer
	if err := Write(&buf, Table, []Entry{open, inProgress, closed}); err != nil {
		t.Fatalf("Write table: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "STATUS") {
		t.Fatalf("table output missing STATUS column:\n%s", out)
	}
	for _, want := range []string{"open", "in_progress", "closed"} {
		if !strings.Contains(out, want) {
			t.Fatalf("table output missing status %q:\n%s", want, out)
		}
	}
}
