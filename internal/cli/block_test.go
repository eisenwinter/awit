package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/eisenwinter/awit/pkg/item"
)

func seedClaimedItem(t *testing.T, repo, id string) {
	t.Helper()
	st, err := item.Open(repo)
	if err != nil {
		t.Fatal(err)
	}
	it, err := st.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	it.SetStatus(item.StatusInProgress)
	it.SetAssignee("agent/claude")
	it.SetClaimedAt(&now)
	if err := st.Save(it); err != nil {
		t.Fatal(err)
	}
}

func TestBlockCommandSavesReasonAndClearsClaim(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	seedClaimedItem(t, dir, id)
	code, stdout, stderr := run(t, "--repo", dir, "block", id, "--reason", "waiting on vendor API")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if stdout != "blocked "+id+": waiting on vendor API\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	got := readItem(t, dir, id)
	if got.BlockedReason != "waiting on vendor API" {
		t.Fatalf("reason = %q", got.BlockedReason)
	}
	if got.Status != item.StatusOpen {
		t.Fatalf("status = %q, want open", got.Status)
	}
	if got.Assignee != "" || got.ClaimedAt != nil {
		t.Fatalf("claim not cleared: assignee=%q claimed=%v", got.Assignee, got.ClaimedAt)
	}
}

func TestBlockCommandRequiresReason(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	for _, args := range [][]string{
		{"--repo", dir, "block", id},
		{"--repo", dir, "block", id, "--reason", ""},
		{"--repo", dir, "block", id, "--reason", "  "},
		{"--repo", dir, "block", id, "--reason", "line one\nline two"},
	} {
		code, _, stderr := run(t, args...)
		if code != 2 {
			t.Fatalf("args %q: exit %d, want 2 stderr %q", args, code, stderr)
		}
		if stderr != "block requires --reason with a non-empty, single-line explanation\n" {
			t.Fatalf("args %q: stderr = %q", args, stderr)
		}
	}
	if got := readItem(t, dir, id); got.BlockedReason != "" {
		t.Fatalf("failed block mutated reason to %q", got.BlockedReason)
	}
}

func TestBlockCommandArity(t *testing.T) {
	dir := initRepo(t)
	for _, args := range [][]string{
		{"--repo", dir, "block"},
		{"--repo", dir, "block", "AWIT-TEST0001", "AWIT-TEST0002", "--reason", "x"},
	} {
		code, _, stderr := run(t, args...)
		if code != 2 {
			t.Fatalf("args %q: exit %d, want 2", args, code)
		}
		if stderr != "block takes exactly one item id\n" {
			t.Fatalf("args %q: stderr = %q", args, stderr)
		}
	}
}

func TestBlockCommandRefusesClosed(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	if code, _, stderr := run(t, "--repo", dir, "close", id); code != 0 {
		t.Fatalf("close: exit %d stderr %q", code, stderr)
	}
	code, _, stderr := run(t, "--repo", dir, "block", id, "--reason", "too late")
	if code != 1 {
		t.Fatalf("exit %d, want 1 stderr %q", code, stderr)
	}
	want := id + " is closed; awit release " + id + " to reopen it before blocking\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
}

func TestBlockCommandReplacesReason(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	if code, _, _ := run(t, "--repo", dir, "block", id, "--reason", "first"); code != 0 {
		t.Fatalf("first block failed")
	}
	code, stdout, _ := run(t, "--repo", dir, "block", id, "--reason", "second")
	if code != 0 || stdout != "blocked "+id+": second\n" {
		t.Fatalf("exit %d stdout %q", code, stdout)
	}
	if got := readItem(t, dir, id); got.BlockedReason != "second" {
		t.Fatalf("reason = %q, want second", got.BlockedReason)
	}
}

func TestUnblockCommandRemovesReasonPreservesDeps(t *testing.T) {
	dir := initRepo(t)
	st, err := item.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Save(item.New("AWIT-TEST0001", "A", "A.", nil, nil)); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(item.New("AWIT-TEST0002", "B", "B.", []string{"AWIT-TEST0001"}, nil)); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := run(t, "--repo", dir, "block", "AWIT-TEST0002", "--reason", "waiting"); code != 0 {
		t.Fatalf("block: exit %d stderr %q", code, stderr)
	}
	code, stdout, stderr := run(t, "--repo", dir, "unblock", "AWIT-TEST0002")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if stdout != "unblocked AWIT-TEST0002\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	got := readItem(t, dir, "AWIT-TEST0002")
	if got.BlockedReason != "" {
		t.Fatalf("reason = %q, want empty", got.BlockedReason)
	}
	if len(got.Deps) != 1 || got.Deps[0] != "AWIT-TEST0001" {
		t.Fatalf("deps = %q, want [AWIT-TEST0001]", got.Deps)
	}
	if got.Status != item.StatusOpen {
		t.Fatalf("status = %q, want open", got.Status)
	}
}

func TestUnblockCommandIdempotent(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	code, stdout, stderr := run(t, "--repo", dir, "unblock", id)
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if stdout != "unblocked "+id+"\n" {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestUnblockCommandArity(t *testing.T) {
	dir := initRepo(t)
	for _, args := range [][]string{
		{"--repo", dir, "unblock"},
		{"--repo", dir, "unblock", "AWIT-TEST0001", "AWIT-TEST0002"},
	} {
		code, _, stderr := run(t, args...)
		if code != 2 {
			t.Fatalf("args %q: exit %d, want 2", args, code)
		}
		if stderr != "unblock takes exactly one item id\n" {
			t.Fatalf("args %q: stderr = %q", args, stderr)
		}
	}
}

func TestManualBlockClaimRefused(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	if code, _, stderr := run(t, "--repo", dir, "block", id, "--reason", "waiting on vendor"); code != 0 {
		t.Fatalf("block: exit %d stderr %q", code, stderr)
	}
	code, _, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--no-commit", "--agent", "claude", id)
	if code != 1 {
		t.Fatalf("claim: exit %d, want 1 stderr %q", code, stderr)
	}
	want := id + " is manually blocked (waiting on vendor); awit unblock " + id + " once resolved\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if got := readItem(t, dir, id); got.Status != item.StatusOpen || got.Assignee != "" {
		t.Fatalf("refused claim mutated item: status=%q assignee=%q", got.Status, got.Assignee)
	}
}

func TestManualBlockRankedNextSkips(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	if code, _, stderr := run(t, "--repo", dir, "block", id, "--reason", "waiting"); code != 0 {
		t.Fatalf("block: exit %d stderr %q", code, stderr)
	}
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--seed", "1")
	if code != 1 || stdout != "" || stderr != "No ready items\n" {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}

func TestManualBlockLookupStillDisplays(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	if code, _, stderr := run(t, "--repo", dir, "block", id, "--reason", "waiting on vendor"); code != 0 {
		t.Fatalf("block: exit %d stderr %q", code, stderr)
	}
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", id)
	if code != 0 {
		t.Fatalf("lookup: exit %d stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "Blocked reason: waiting on vendor") {
		t.Fatalf("lookup stdout = %q, want blocked reason", stdout)
	}
}

func TestManualBlockReleasePreserves(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	seedClaimedItem(t, dir, id)
	if code, _, stderr := run(t, "--repo", dir, "block", id, "--reason", "waiting"); code != 0 {
		t.Fatalf("block: exit %d stderr %q", code, stderr)
	}
	code, stdout, stderr := run(t, "--repo", dir, "release", id)
	if code != 0 || stdout != "reopened "+id+"\n" {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if got := readItem(t, dir, id); got.BlockedReason != "waiting" {
		t.Fatalf("release cleared reason: %q", got.BlockedReason)
	}
}

func TestManualBlockCloseClears(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	if code, _, stderr := run(t, "--repo", dir, "block", id, "--reason", "waiting"); code != 0 {
		t.Fatalf("block: exit %d stderr %q", code, stderr)
	}
	code, stdout, stderr := run(t, "--repo", dir, "close", id)
	if code != 0 || stdout != "closed "+id+"\n" {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	if got := readItem(t, dir, id); got.BlockedReason != "" {
		t.Fatalf("close kept reason: %q", got.BlockedReason)
	}
}

func TestManualBlockStatusClosedClears(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	if code, _, stderr := run(t, "--repo", dir, "block", id, "--reason", "waiting"); code != 0 {
		t.Fatalf("block: exit %d stderr %q", code, stderr)
	}
	if code, _, stderr := run(t, "--repo", dir, "update", id, "--status", "closed"); code != 0 {
		t.Fatalf("update: exit %d stderr %q", code, stderr)
	}
	if got := readItem(t, dir, id); got.BlockedReason != "" {
		t.Fatalf("status-closed kept reason: %q", got.BlockedReason)
	}
}

func TestManualBlockStatusOpenPreserves(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	if code, _, stderr := run(t, "--repo", dir, "block", id, "--reason", "waiting"); code != 0 {
		t.Fatalf("block: exit %d stderr %q", code, stderr)
	}
	if code, _, stderr := run(t, "--repo", dir, "update", id, "--status", "in_progress"); code != 0 {
		t.Fatalf("update: exit %d stderr %q", code, stderr)
	}
	if got := readItem(t, dir, id); got.BlockedReason != "waiting" {
		t.Fatalf("status update cleared reason: %q", got.BlockedReason)
	}
}

func TestManualBlockListShowPrimeValidate(t *testing.T) {
	dir := initRepo(t)
	st, err := item.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Save(item.New("AWIT-TEST0001", "A", "A.", nil, nil)); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(item.New("AWIT-TEST0002", "B", "B.", []string{"AWIT-TEST0001"}, nil)); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := run(t, "--repo", dir, "block", "AWIT-TEST0001", "--reason", "waiting on vendor"); code != 0 {
		t.Fatalf("block: exit %d stderr %q", code, stderr)
	}

	code, stdout, _ := run(t, "--repo", dir, "--format", "compact", "list", "--blocked")
	if code != 0 || !strings.Contains(stdout, "Blocked reason: waiting on vendor") {
		t.Fatalf("list --blocked: exit %d stdout %q", code, stdout)
	}

	code, stdout, _ = run(t, "--repo", dir, "--format", "table", "list")
	if code != 0 || !strings.Contains(stdout, "BLOCKED_REASON") || !strings.Contains(stdout, "waiting on vendor") {
		t.Fatalf("list table: exit %d stdout %q", code, stdout)
	}

	code, stdout, _ = run(t, "--repo", dir, "--format", "json", "list")
	if code != 0 || !strings.Contains(stdout, `"blocked_reason": "waiting on vendor"`) {
		t.Fatalf("list json: exit %d stdout %q", code, stdout)
	}

	code, stdout, _ = run(t, "--repo", dir, "show", "AWIT-TEST0001")
	if code != 0 || !strings.Contains(stdout, "waiting on vendor") {
		t.Fatalf("show: exit %d stdout %q", code, stdout)
	}

	code, stdout, _ = run(t, "--repo", dir, "--format", "json", "show", "AWIT-TEST0001")
	if code != 0 || !strings.Contains(stdout, `"blocked_reason": "waiting on vendor"`) {
		t.Fatalf("show json: exit %d stdout %q", code, stdout)
	}

	code, stdout, _ = run(t, "--repo", dir, "prime")
	if code != 0 {
		t.Fatalf("prime: exit %d", code)
	}
	if !strings.Contains(stdout, "[AWIT-TEST0001] A | Blocked reason: waiting on vendor (awit unblock AWIT-TEST0001)") {
		t.Fatalf("prime manual row: stdout %q", stdout)
	}
	if !strings.Contains(stdout, "[AWIT-TEST0002] B <- AWIT-TEST0001") {
		t.Fatalf("prime dep row: stdout %q", stdout)
	}

	code, stdout, _ = run(t, "--repo", dir, "validate")
	if code != 0 || !strings.Contains(stdout, "PASS") {
		t.Fatalf("validate: exit %d stdout %q", code, stdout)
	}
}

func TestManualBlockPrimeCombinedRow(t *testing.T) {
	dir := initRepo(t)
	st, err := item.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Save(item.New("AWIT-TEST0001", "A", "A.", nil, nil)); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(item.New("AWIT-TEST0002", "B", "B.", []string{"AWIT-TEST0001"}, nil)); err != nil {
		t.Fatal(err)
	}
	if code, _, stderr := run(t, "--repo", dir, "block", "AWIT-TEST0002", "--reason", "paused"); code != 0 {
		t.Fatalf("block: exit %d stderr %q", code, stderr)
	}
	_, stdout, _ := run(t, "--repo", dir, "prime")
	want := "[AWIT-TEST0002] B <- AWIT-TEST0001 | Blocked reason: paused (awit unblock AWIT-TEST0002)"
	if !strings.Contains(stdout, want) {
		t.Fatalf("prime combined row: stdout %q, want %q", stdout, want)
	}
}

func TestBlockCommandNoSubprocess(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	st, err := item.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	it, err := st.Load(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := it.SetExternal(&item.External{Tracker: "gitea", Repo: "owner/repo", ID: 7, URL: "https://example.com/owner/repo/issues/7"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Save(it); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", "/nonexistent-block-test")
	code, stdout, stderr := run(t, "--repo", dir, "block", id, "--reason", "waiting")
	if code != 0 || stdout != "blocked "+id+": waiting\n" {
		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	code, stdout, _ = run(t, "--repo", dir, "unblock", id)
	if code != 0 || stdout != "unblocked "+id+"\n" {
		t.Fatalf("unblock: exit %d stdout %q", code, stdout)
	}
}
