package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/eisenwinter/awit/internal/ops"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

func openTestStore(t *testing.T, dir string) *item.Store {
	t.Helper()
	s, err := item.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestClaimItemFields(t *testing.T) {
	dir := initRepo(t)
	if code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", "AWIT-TEST0001", "T"); code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}
	s := openTestStore(t, dir)
	it, err := s.Load("AWIT-TEST0001")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 24, 10, 0, 0, 500, time.UTC)
	if err := claimItem(s, it, "claude", now); err != nil {
		t.Fatal(err)
	}
	got := readItem(t, dir, "AWIT-TEST0001")
	if got.Status != item.StatusInProgress || got.Assignee != "agent/claude" {
		t.Fatalf("status/assignee = %s/%s", got.Status, got.Assignee)
	}
	if got.ClaimedAt == nil || !got.ClaimedAt.Equal(now.Truncate(time.Second)) {
		t.Fatalf("claimed_at = %v, want %v", got.ClaimedAt, now.Truncate(time.Second))
	}
}

func TestCloseItemWithAndWithoutReason(t *testing.T) {
	dir := initRepo(t)
	for _, id := range []string{"AWIT-TEST0001", "AWIT-TEST0002"} {
		if code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", id, "T"); code != 0 {
			t.Fatalf("create: exit %d stderr %q", code, stderr)
		}
		if code, _, stderr := run(t, "--repo", dir, "block", id, "--reason", "hold"); code != 0 {
			t.Fatalf("block: exit %d stderr %q", code, stderr)
		}
	}
	s := openTestStore(t, dir)
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)

	it1, _ := s.Load("AWIT-TEST0001")
	if err := closeItem(s, it1, "", "", now); err != nil {
		t.Fatal(err)
	}
	got := readItem(t, dir, "AWIT-TEST0001")
	if got.Status != item.StatusClosed || got.ClaimedAt != nil || got.BlockedReason != "" {
		t.Fatalf("no-reason close: %+v", got)
	}
	if len(got.Refs) != 0 {
		t.Fatalf("no-reason close added refs %v", got.Refs)
	}

	it2, _ := s.Load("AWIT-TEST0002")
	if err := closeItem(s, it2, "done", "jan", now); err != nil {
		t.Fatal(err)
	}
	got = readItem(t, dir, "AWIT-TEST0002")
	if got.Status != item.StatusClosed || len(got.Refs) != 1 || !strings.Contains(got.Refs[0], "comments/AWIT-TEST0002/") {
		t.Fatalf("reason close: %+v", got)
	}
	cs, err := s.Comments("AWIT-TEST0002")
	if err != nil || len(cs) != 1 || cs[0].Text != "done" || cs[0].Author != "jan" {
		t.Fatalf("comments = %+v, %v", cs, err)
	}
}

func TestBlockItemRefusesClosed(t *testing.T) {
	dir := initRepo(t)
	if code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", "AWIT-TEST0001", "T"); code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}
	if code, _, _ := run(t, "--repo", dir, "close", "AWIT-TEST0001"); code != 0 {
		t.Fatal("close failed")
	}
	s := openTestStore(t, dir)
	it, _ := s.Load("AWIT-TEST0001")
	err := blockItem(s, it, "x")
	if err == nil || !strings.Contains(err.Error(), "is closed; awit release AWIT-TEST0001") {
		t.Fatalf("err = %v", err)
	}
	if readItem(t, dir, "AWIT-TEST0001").BlockedReason != "" {
		t.Fatal("blocked_reason written despite refusal")
	}
}

func TestShowFullAndValidateTextMatchCLI(t *testing.T) {
	dir := copyFixture(t, "clean")
	s := openTestStore(t, dir)
	g, err := ops.LoadGraph(s)
	if err != nil {
		t.Fatal(err)
	}
	for id := range g.Nodes {
		_, stdout, _ := run(t, "--repo", dir, "show", id, "--full")
		if got := showFull(s, g, g.Nodes[id]); got != stdout {
			t.Fatalf("showFull(%s) =\n%s\nwant\n%s", id, got, stdout)
		}
	}
	_, stdout, _ := run(t, "--repo", dir, "validate")
	if got := validateText(g); got != stdout {
		t.Fatalf("validateText =\n%s\nwant\n%s", got, stdout)
	}
	if !strings.HasPrefix(validateText(graph.Build(nil, nil)), "PASS  0 items, 0 quarantined\n") {
		t.Fatal("validateText on empty graph")
	}
}
