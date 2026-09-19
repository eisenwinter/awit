package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/item"
)

// TestReleaseReopensClosedItem covers the source state next.go points at:
// a closed item goes back to open and the run confirms it by name.
func TestReleaseReopensClosedItem(t *testing.T) {
	dir := copyFixture(t, "clean")
	const id = "AWIT-TEST0005"
	code, stdout, stderr := run(t, "--repo", dir, "release", id)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if want := "reopened " + id + "\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	got := readItem(t, dir, id)
	if got.Status != item.StatusOpen || got.Assignee != "" || got.ClaimedAt != nil {
		t.Fatalf("status=%q assignee=%q claimed=%v", got.Status, got.Assignee, got.ClaimedAt)
	}
}

// TestReleaseClearsClaimAndConfirms covers the claimed in-progress state:
// both claim fields are deleted from the file and the confirmation names
// the reopened item.
func TestReleaseClearsClaimAndConfirms(t *testing.T) {
	dir := copyFixture(t, "clean")
	const id = "AWIT-TEST0006"
	code, stdout, stderr := run(t, "--repo", dir, "release", id)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if want := "reopened " + id + "\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	got := readItem(t, dir, id)
	if got.Status != item.StatusOpen || got.Assignee != "" || got.ClaimedAt != nil {
		t.Fatalf("status=%q assignee=%q claimed=%v", got.Status, got.Assignee, got.ClaimedAt)
	}
	raw, err := os.ReadFile(filepath.Join(dir, ".awit", "items", id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "assignee:") || strings.Contains(string(raw), "claimed_at:") {
		t.Fatalf("keys not deleted:\n%s", raw)
	}
}

// TestReleaseAlreadyOpenItemIsIdempotent: every source state, including
// already-open, ends open and unclaimed with the same confirmation.
func TestReleaseAlreadyOpenItemIsIdempotent(t *testing.T) {
	dir := copyFixture(t, "clean")
	const id = "AWIT-TEST0002"
	code, stdout, stderr := run(t, "--repo", dir, "release", id)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if want := "reopened " + id + "\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	got := readItem(t, dir, id)
	if got.Status != item.StatusOpen || got.Assignee != "" || got.ClaimedAt != nil {
		t.Fatalf("status=%q assignee=%q claimed=%v", got.Status, got.Assignee, got.ClaimedAt)
	}
}

// TestReleaseIgnoresGlobalFormat: like close and archive, the mutation
// confirmation stays a plain line under --format json.
func TestReleaseIgnoresGlobalFormat(t *testing.T) {
	dir := copyFixture(t, "clean")
	const id = "AWIT-TEST0005"
	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "release", id)
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if want := "reopened " + id + "\n"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

// TestReleaseFailedSavePrintsNoConfirmation: the success line is printed
// only after Store.Save succeeds; a write failure must not claim otherwise.
func TestReleaseFailedSavePrintsNoConfirmation(t *testing.T) {
	dir := copyFixture(t, "clean")
	itemsDir := filepath.Join(dir, ".awit", "items")
	if err := os.Chmod(itemsDir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(itemsDir, 0o755) })
	// Windows directory permissions and root both ignore the read-only bit;
	// probe instead of assuming the save will fail.
	probe, err := os.CreateTemp(itemsDir, ".probe-*")
	if err == nil {
		_ = probe.Close()
		_ = os.Remove(probe.Name())
		t.Skip("filesystem does not honour read-only directory")
	}
	const id = "AWIT-TEST0005"
	code, stdout, _ := run(t, "--repo", dir, "release", id)
	if code == 0 {
		t.Fatalf("exit = 0, want non-zero (stdout %q)", stdout)
	}
	if strings.Contains(stdout, "reopened") {
		t.Fatalf("stdout claims success despite failed save: %q", stdout)
	}
}
