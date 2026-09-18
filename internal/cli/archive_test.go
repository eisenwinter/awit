package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveDryRun(t *testing.T) {
	dir := copyFixture(t, "archive")
	code, stdout, stderr := run(t, "--repo", dir, "archive", "--dry-run")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	golden(t, "archive-dry-run.golden", []byte(stdout))
	if _, err := os.Stat(filepath.Join(dir, ".awit", "archive")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("dry-run must not create archive/")
	}
}

func TestArchiveMovesAndValidateStaysGreen(t *testing.T) {
	dir := copyFixture(t, "archive")
	code, stdout, stderr := run(t, "--repo", dir, "archive")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	want := "archived AWIT-TEST0004\narchived AWIT-TEST0005\nArchived 2 items\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	for _, id := range []string{"AWIT-TEST0004", "AWIT-TEST0005"} {
		if _, err := os.Stat(filepath.Join(dir, ".awit", "items", id+".md")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s still in items/", id)
		}
		if _, err := os.Stat(filepath.Join(dir, ".awit", "archive", id+".md")); err != nil {
			t.Fatalf("%s missing from archive/: %v", id, err)
		}
	}
	for _, id := range []string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0003"} {
		if _, err := os.Stat(filepath.Join(dir, ".awit", "items", id+".md")); err != nil {
			t.Fatalf("%s must stay: %v", id, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, ".awit", "archive", "AWIT-TEST0004.md"))
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "archive-TEST0004.golden", data)

	code, out, _ := run(t, "--repo", dir, "validate")
	if code != 0 || !strings.HasPrefix(out, "PASS") {
		t.Fatalf("validate after archive: exit %d\n%s", code, out)
	}
	// Second run: nothing left to do, still exit 0.
	code, out, _ = run(t, "--repo", dir, "archive")
	if code != 0 || out != "Archived 0 items\n" {
		t.Fatalf("second run: exit %d out %q", code, out)
	}
}

func TestArchiveNothingOnClean(t *testing.T) {
	// clean: TEST0005 is closed but TEST0006 (in_progress) depends on it.
	dir := copyFixture(t, "clean")
	code, stdout, _ := run(t, "--repo", dir, "archive", "--dry-run")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	want := "skip AWIT-TEST0005: dependant AWIT-TEST0006 not archivable\nWould archive 0 items\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}
