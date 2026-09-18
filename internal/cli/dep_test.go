package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readRaw(t *testing.T, repo, id string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, ".awit", "items", id+".md"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func changedLines(t *testing.T, before, after []byte) int {
	t.Helper()
	bl := strings.Split(string(before), "\n")
	al := strings.Split(string(after), "\n")
	if len(bl) != len(al) {
		// Report the shape so the failure is actionable.
		t.Fatalf("line count changed: %d -> %d", len(bl), len(al))
	}
	n := 0
	for i := range bl {
		if bl[i] != al[i] {
			n++
		}
	}
	return n
}

func TestDepAddWritesOneLine(t *testing.T) {
	dir := copyFixture(t, "clean")
	before := readRaw(t, dir, "AWIT-TEST0002")
	code, stdout, stderr := run(t, "--repo", dir, "dep", "add", "AWIT-TEST0002", "AWIT-TEST0001")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if !strings.HasPrefix(stdout, "[AWIT-TEST0002] open Update database migration scripts | ") {
		t.Fatalf("stdout = %q, want compact line for AWIT-TEST0002", stdout)
	}
	if !strings.HasSuffix(stdout, "\n") || strings.Count(stdout, "\n") != 1 {
		t.Fatalf("stdout = %q, want exactly one line", stdout)
	}
	got := readItem(t, dir, "AWIT-TEST0002")
	if len(got.Deps) != 1 || got.Deps[0] != "AWIT-TEST0001" {
		t.Fatalf("deps = %v, want [AWIT-TEST0001]", got.Deps)
	}
	after := readRaw(t, dir, "AWIT-TEST0002")
	if n := changedLines(t, before, after); n != 1 {
		t.Fatalf("changed lines = %d, want 1", n)
	}
}

func TestDepAddRefusesCycle(t *testing.T) {
	dir := copyFixture(t, "clean")
	before := readRaw(t, dir, "AWIT-TEST0001")
	code, stdout, stderr := run(t, "--repo", dir, "dep", "add", "AWIT-TEST0001", "AWIT-TEST0004")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	want := "Error: cannot add dependency AWIT-TEST0004 to AWIT-TEST0001.\n" +
		"Cycle: AWIT-TEST0001 -> AWIT-TEST0004 -> AWIT-TEST0001\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if after := readRaw(t, dir, "AWIT-TEST0001"); !bytes.Equal(before, after) {
		t.Fatal("file was written despite refused cycle")
	}
}

func TestDepAddSelf(t *testing.T) {
	dir := copyFixture(t, "clean")
	before := readRaw(t, dir, "AWIT-TEST0002")
	code, _, stderr := run(t, "--repo", dir, "dep", "add", "AWIT-TEST0002", "AWIT-TEST0002")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	want := "Error: cannot add dependency AWIT-TEST0002 to AWIT-TEST0002.\n" +
		"Cycle: AWIT-TEST0002 -> AWIT-TEST0002\n"
	if stderr != want {
		t.Fatalf("stderr = %q, want %q", stderr, want)
	}
	if after := readRaw(t, dir, "AWIT-TEST0002"); !bytes.Equal(before, after) {
		t.Fatal("file was written despite self-cycle")
	}
}

func TestDepAddUnknown(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, _, stderr := run(t, "--repo", dir, "dep", "add", "AWIT-TEST0099", "AWIT-TEST0001")
	if code != 1 || stderr != "Error: unknown item AWIT-TEST0099\n" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	code, _, stderr = run(t, "--repo", dir, "dep", "add", "AWIT-TEST0001", "AWIT-TEST0099")
	if code != 1 || stderr != "Error: unknown item AWIT-TEST0099\n" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
}

func TestDepAddAlreadyPresent(t *testing.T) {
	dir := copyFixture(t, "clean")
	before := readRaw(t, dir, "AWIT-TEST0003")
	code, stdout, stderr := run(t, "--repo", dir, "dep", "add", "AWIT-TEST0003", "AWIT-TEST0001")
	if code != 0 {
		t.Fatalf("exit %d stderr %q, want 0", code, stderr)
	}
	if stdout != "dependency already present\n" {
		t.Fatalf("stdout = %q, want %q", stdout, "dependency already present\n")
	}
	if after := readRaw(t, dir, "AWIT-TEST0003"); !bytes.Equal(before, after) {
		t.Fatal("file was rewritten for an already-present edge")
	}
}

func TestDepRm(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "dep", "rm", "AWIT-TEST0004", "AWIT-TEST0003")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if !strings.HasPrefix(stdout, "[AWIT-TEST0004] open Rotate API tokens | ") {
		t.Fatalf("stdout = %q, want compact line for AWIT-TEST0004", stdout)
	}
	got := readItem(t, dir, "AWIT-TEST0004")
	if len(got.Deps) != 1 || got.Deps[0] != "AWIT-TEST0001" {
		t.Fatalf("deps = %v, want [AWIT-TEST0001]", got.Deps)
	}
}

func TestDepRmAbsent(t *testing.T) {
	dir := copyFixture(t, "clean")
	before := readRaw(t, dir, "AWIT-TEST0002")
	code, _, stderr := run(t, "--repo", dir, "dep", "rm", "AWIT-TEST0002", "AWIT-TEST0001")
	if code != 1 || stderr != "Error: AWIT-TEST0002 does not depend on AWIT-TEST0001\n" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if after := readRaw(t, dir, "AWIT-TEST0002"); !bytes.Equal(before, after) {
		t.Fatal("file was written despite absent edge")
	}
	code, _, stderr = run(t, "--repo", dir, "dep", "rm", "AWIT-TEST0099", "AWIT-TEST0001")
	if code != 1 || stderr != "Error: unknown item AWIT-TEST0099\n" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
}
