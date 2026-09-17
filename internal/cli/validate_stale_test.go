package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/eisenwinter/awit/pkg/config"
	"github.com/eisenwinter/awit/pkg/item"
)

// pinNow fixes the stale-claim clock and restores it after the test.
func pinNow(t *testing.T, s string) {
	t.Helper()
	fixed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatal(err)
	}
	old := now
	now = fixed
	t.Cleanup(func() { now = old })
}

func TestValidateStaleClaimsWarn(t *testing.T) {
	dir := copyFixture(t, "clean")
	pinNow(t, "2026-09-17T17:44:05Z")
	code, stdout, stderr := run(t, "--repo", dir, "validate", "--stale-claims")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q, WARN must not change the exit", code, stderr)
	}
	want := "WARN  [STALE CLAIM] AWIT-TEST0006 claimed by agent/claude 3h12m ago (limit 2h)\n" +
		"  fix: awit release AWIT-TEST0006\n"
	if !strings.Contains(stdout, want) {
		t.Fatalf("stdout missing stale warning:\n%s", stdout)
	}
	golden(t, "validate-stale-claims.golden", []byte(stdout))
}

func TestValidateStaleWithinLimit(t *testing.T) {
	dir := copyFixture(t, "clean")
	// claimed 14:32:05Z; now is 1h59m later — inside the 2h limit.
	pinNow(t, "2026-09-17T16:31:05Z")
	code, stdout, _ := run(t, "--repo", dir, "validate", "--stale-claims")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	if strings.Contains(stdout, "[STALE CLAIM]") {
		t.Fatalf("no stale warning expected within limit:\n%s", stdout)
	}
}

func TestValidateStaleConfigOverride(t *testing.T) {
	dir := copyFixture(t, "clean")
	st, err := item.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := st.Config
	cfg.StaleClaim = config.Duration(30 * time.Minute)
	if err := cfg.Write(st.Dir); err != nil {
		t.Fatal(err)
	}
	// Age is 45m: fresh under 2h, stale under 30m.
	pinNow(t, "2026-09-17T15:17:05Z")
	code, stdout, _ := run(t, "--repo", dir, "validate", "--stale-claims")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	want := "WARN  [STALE CLAIM] AWIT-TEST0006 claimed by agent/claude 45m ago (limit 30m)\n" +
		"  fix: awit release AWIT-TEST0006\n"
	if !strings.Contains(stdout, want) {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestValidateStaleNoClaimedAt(t *testing.T) {
	dir := copyFixture(t, "clean")
	st, err := item.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	it, err := st.Load("AWIT-TEST0006")
	if err != nil {
		t.Fatal(err)
	}
	it.SetClaimedAt(nil)
	if err := st.Save(it); err != nil {
		t.Fatal(err)
	}
	pinNow(t, "2026-09-17T17:44:05Z")
	code, stdout, _ := run(t, "--repo", dir, "validate", "--stale-claims")
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	want := "WARN  [STALE CLAIM] AWIT-TEST0006 in progress with no claimed_at (limit 2h)\n" +
		"  fix: awit release AWIT-TEST0006\n"
	if !strings.Contains(stdout, want) {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestHumanDuration(t *testing.T) {
	cases := map[time.Duration]string{
		0:                             "0m",
		45 * time.Minute:              "45m",
		59 * time.Minute:              "59m",
		time.Hour:                     "1h0m",
		3*time.Hour + 12*time.Minute:  "3h12m",
		47*time.Hour + 5*time.Minute:  "47h5m",
		48 * time.Hour:                "2d0h",
		51*time.Hour + 30*time.Minute: "2d3h",
	}
	for d, want := range cases {
		if got := humanDuration(d); got != want {
			t.Fatalf("humanDuration(%v) = %q, want %q", d, got, want)
		}
	}
}
