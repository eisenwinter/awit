package cli

import (
	"strings"
	"testing"
)

func TestPrimeCLI(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "prime")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	golden(t, "prime-clean.golden", []byte(stdout))
	if strings.Contains(stdout, "\r") {
		t.Fatal("output contains CR bytes")
	}
}

func TestPrimeLabelFilter(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "prime", "-l", "p0")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "=== READY (0) ===\n") {
		t.Fatalf("ready must be empty after -l p0:\n%s", stdout)
	}
	if !strings.Contains(stdout, "=== BLOCKED (1) ===\n[AWIT-TEST0004] Rotate API tokens <- AWIT-TEST0001, AWIT-TEST0003\n") {
		t.Fatalf("blocked must hold only 0004:\n%s", stdout)
	}
	if !strings.Contains(stdout, "=== CRITICAL PATH (3) ===\nAWIT-TEST0001 -> AWIT-TEST0003 -> AWIT-TEST0004\n") {
		t.Fatalf("critical path is unfiltered:\n%s", stdout)
	}
}

func TestPrimeMaxTokensCLI(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "prime", "--max-tokens", "40")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	top := "[AWIT-TEST0001] open Implement OAuth2 bearer token extraction | auth,p1 | Unblocks: 2\n"
	if !strings.Contains(stdout, top) {
		t.Fatalf("top ready row must survive truncation:\n%s", stdout)
	}
	if strings.Contains(stdout, "CRITICAL PATH") {
		t.Fatalf("critical path is shed before the top ready row:\n%s", stdout)
	}
	if !strings.Contains(stdout, "(+4 more)\n") {
		t.Fatalf("want (+4 more) suffix:\n%s", stdout)
	}
	// The global --format flag is ignored by prime, even when bogus.
	code, _, stderr = run(t, "--repo", dir, "--format", "bogus", "prime")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q, want format ignored", code, stderr)
	}
}

func TestPrimeTinyBudgetCLI(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "prime", "--max-tokens", "1")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	top := "[AWIT-TEST0001] open Implement OAuth2 bearer token extraction | auth,p1 | Unblocks: 2\n"
	if !strings.Contains(stdout, top) {
		t.Fatalf("MaxTokens=1 must keep the complete top ready row, not just its ID:\n%s", stdout)
	}
	again := func() string {
		_, out, _ := run(t, "--repo", dir, "prime", "--max-tokens", "1")
		return out
	}()
	if stdout != again {
		t.Fatalf("budgeted prime must be byte-identical across runs:\n%q\n---\n%q", stdout, again)
	}
}

func TestPrimeNegativeBudgetCLI(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, _, stderr := run(t, "--repo", dir, "prime", "--max-tokens", "-1")
	if code != 2 {
		t.Fatalf("exit %d, want 2 for a negative token budget", code)
	}
	if !strings.Contains(stderr, "--max-tokens") {
		t.Fatalf("stderr must name the offending flag, got %q", stderr)
	}
}
