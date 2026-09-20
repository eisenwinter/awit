package prime

import (
	"bytes"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

func manualBlockGraph(t *testing.T) *graph.Graph {
	t.Helper()
	a := item.New("AWIT-TEST0001", "A", "A.", nil, nil)
	b := item.New("AWIT-TEST0002", "B", "B.", []string{"AWIT-TEST0001"}, nil)
	if err := a.SetBlockedReason("waiting on vendor"); err != nil {
		t.Fatal(err)
	}
	if err := b.SetBlockedReason("paused"); err != nil {
		t.Fatal(err)
	}
	return graph.Build([]*item.Item{a, b}, nil)
}

func TestManualBlockRows(t *testing.T) {
	var buf bytes.Buffer
	if err := Render(&buf, manualBlockGraph(t), Options{}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "[AWIT-TEST0001] A | Blocked reason: waiting on vendor (awit unblock AWIT-TEST0001)") {
		t.Fatalf("manual-only row missing:\n%s", out)
	}
	if !strings.Contains(out, "[AWIT-TEST0002] B <- AWIT-TEST0001 | Blocked reason: paused (awit unblock AWIT-TEST0002)") {
		t.Fatalf("combined row missing:\n%s", out)
	}
	if strings.Contains(out, "=== READY (1)") {
		t.Fatalf("blocked items must not be ready:\n%s", out)
	}
}

func TestManualBlockDepOnlyRowUnchanged(t *testing.T) {
	a := item.New("AWIT-TEST0001", "A", "A.", nil, nil)
	b := item.New("AWIT-TEST0002", "B", "B.", []string{"AWIT-TEST0001"}, nil)
	var buf bytes.Buffer
	if err := Render(&buf, graph.Build([]*item.Item{a, b}, nil), Options{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "[AWIT-TEST0002] B <- AWIT-TEST0001\n") {
		t.Fatalf("dep-only row changed:\n%s", buf.String())
	}
}
