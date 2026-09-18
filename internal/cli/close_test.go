package cli

import "testing"

func TestClosePrintsClosedID(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	code, stdout, stderr := run(t, "--repo", dir, "close", id)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	want := "closed " + id + "\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestCloseWithReasonPrintsClosedID(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	code, stdout, stderr := run(t, "--repo", dir, "close", id, "--reason", "done", "--author", "jane")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	want := "closed " + id + "\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}
