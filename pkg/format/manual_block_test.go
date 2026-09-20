package format

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func blockedEntry() Entry {
	return Entry{
		ID:            "AWIT-TEST0001",
		Title:         "T",
		Status:        "open",
		State:         "blocked",
		Labels:        []string{},
		Deps:          []string{},
		Unblocks:      0,
		BlockedReason: "waiting on vendor",
	}
}

func TestManualBlockCompactSuffix(t *testing.T) {
	got := Line(blockedEntry())
	want := "[AWIT-TEST0001] open T | - | Unblocks: 0 | Blocked reason: waiting on vendor"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if plain := Line(Entry{ID: "AWIT-TEST0002", Title: "U", Status: "open", State: "ready"}); strings.Contains(plain, "Blocked reason") {
		t.Fatalf("unblocked line must not mention a reason: %q", plain)
	}
}

func TestManualBlockSingleTable(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOne(&buf, Table, blockedEntry()); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	var stateIdx, reasonIdx int = -1, -1
	for i, l := range lines {
		if l == "State: blocked" {
			stateIdx = i
		}
		if l == "Blocked reason: waiting on vendor" {
			reasonIdx = i
		}
	}
	if reasonIdx < 0 {
		t.Fatalf("no Blocked reason line in:\n%s", buf.String())
	}
	if stateIdx < 0 || reasonIdx != stateIdx+1 {
		t.Fatalf("Blocked reason must follow State directly, got:\n%s", buf.String())
	}
}

func TestManualBlockListTableColumn(t *testing.T) {
	rows := []Entry{blockedEntry()}
	var buf bytes.Buffer
	if err := Write(&buf, Table, rows); err != nil {
		t.Fatal(err)
	}
	header := strings.SplitN(buf.String(), "\n", 2)[0]
	if !strings.Contains(header, "BLOCKED_REASON") {
		t.Fatalf("header = %q, want BLOCKED_REASON column", header)
	}
	if !strings.Contains(buf.String(), "waiting on vendor") {
		t.Fatalf("table missing reason:\n%s", buf.String())
	}

	var plain bytes.Buffer
	if err := Write(&plain, Table, []Entry{{ID: "AWIT-TEST0002", Title: "U", Status: "open", State: "ready"}}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.SplitN(plain.String(), "\n", 2)[0], "BLOCKED_REASON") {
		t.Fatalf("column must be absent when no row has a reason:\n%s", plain.String())
	}
}

func TestManualBlockJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, JSON, []Entry{blockedEntry()}); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["blocked_reason"] != "waiting on vendor" {
		t.Fatalf("rows = %v", rows)
	}
	if rows[0]["state"] != "blocked" {
		t.Fatalf("state = %v, want blocked", rows[0]["state"])
	}

	var one bytes.Buffer
	if err := WriteOne(&one, JSON, blockedEntry()); err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(one.Bytes(), &obj); err != nil {
		t.Fatal(err)
	}
	if obj["blocked_reason"] != "waiting on vendor" {
		t.Fatalf("obj = %v", obj)
	}
}
