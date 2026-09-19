---
id: AWIT-0ND5733G
title: 'validate --stale-claims'
brief: >-
  Add `--stale-claims` to `awit validate`: `in_progress` items whose `claimed_at` is older than `config.stale_claim` (or missing) print a `WARN [STALE CLAIM]` line with an `awit release` fix hint. Warnings never change the exit code. Includes `humanDuration` and an overridable `now` for tests.
status: closed
deps: [AWIT-0ND56S3G]
labels: [phase5, p2]
refs_base: repo
refs:
  - plan/implementation-guide.md
  - plan/awit-implementation-plan.md
---

## Summary

After this ticket `awit validate --stale-claims` additionally scans
non-quarantined `in_progress` nodes: a `claimed_at` older than the
configured `stale_claim` prints

```text
WARN  [STALE CLAIM] AWIT-TEST0006 claimed by agent/claude 3h12m ago (limit 2h)
  fix: awit release AWIT-TEST0006
```

and an `in_progress` node with no `claimed_at` prints

```text
WARN  [STALE CLAIM] AWIT-TEST0006 in progress with no claimed_at (limit 2h)
  fix: awit release AWIT-TEST0006
```

Stale lines go to the same writer as the base report, in ID order, after
all other report lines. They never change the exit code: FAILs alone
decide non-zero. Durations render via `humanDuration` (`45m`, `3h12m`,
`2d3h` for ≥48h). Production time comes from package-level
`var now = time.Now()`, which tests overwrite with a fixed value. Five
tests run green, including a golden for the warn case.

## Context (read first)

- **AWIT-0ND56S3G** — `internal/cli/validate.go` as built there: the
  `validate` command, report writer, FAIL/WARN vocabulary, and exit-code
  logic. Must be `status: closed` before you start. Read it first and
  match its conventions exactly:
  - Send stale lines to the SAME writer the base report uses (stdout
    via `cmd.Writer`), with the same `WARN  ` prefix style the base
    report uses for warnings.
  - Append the stale section AFTER all base report lines. Do not
    interleave, do not re-sort the base report, do not touch the exit
    code path.
  - Reuse the base flag set; only ADD `--stale-claims` (bool).
- **AWIT-0ND56S3G is being implemented concurrently.** Do not
  reimplement `validate` — wait for it to land if needed. If S3G
  already declares a package-level `now`, reuse it instead of declaring
  a second one. If S3G's fix-hint line for WARNs differs from
  `  fix: <cmd>` (two spaces), follow S3G and update the golden and the
  assertions below to match — but the `WARN  [STALE CLAIM] ...` first
  line is normative regardless.
- The clock is `var now = time.Now()` — an evaluated `time.Time`
  value, NOT a func. Tests assign a plain value (`now = fixed`) and
  restore it with `t.Cleanup`. Do not "improve" it into
  `var now = time.Now` (func value): the test helper below would not
  compile against that shape.
- Guide §4.2 — `Config.StaleClaim` is `config.Duration`
  (`time.Duration` underneath); default `2h`. Render the limit with
  `%s` over `time.Duration(cfg.StaleClaim)` (`2h`, `30m`).
- Fixture fact (AWIT-0ND56N3G): `clean` `AWIT-TEST0006` is
  `in_progress`, assignee `agent/claude`,
  `claimed_at: 2026-09-17T14:32:05Z`. Tests pin
  `now = 2026-09-17T17:44:05Z`, so its age is exactly 3h12m and the
  limit renders `(limit 2h)`.
- Helpers (do not redeclare): `openStore`, `loadGraph`, `run`,
  `copyFixture`, `golden`, `readItem`. `Store.Config` holds the loaded
  config; `Config.Write(awitDir)` rewrites `config.yaml`
  (`Store.Dir` is the `.awit/` dir).

## Files

- Modify: `internal/cli/validate.go` — `--stale-claims` flag, stale
  scan, `humanDuration`, `var now`.
- Create: `internal/cli/validate_stale_test.go`
- Create: `testdata/golden/validate-stale-claims.golden`

## Interfaces

- Consumes (do not reimplement):

  ```go
  func openStore(cmd *cli.Command) (*item.Store, error)
  func loadGraph(s *item.Store) (*graph.Graph, error)
  ```

- Produces (this ticket, in `internal/cli/validate.go`):

  ```go
  var now = time.Now() // overridable clock; tests overwrite it
  func humanDuration(d time.Duration) string
  func staleClaimLines(g *graph.Graph, limit time.Duration, ref time.Time) []string
  ```

  `staleClaimLines` is pure (no I/O, no globals): it returns the report
  lines WITHOUT trailing newlines and the command prints each with
  `Fprintln`. Order and wording are covered through the CLI tests.

## Steps

- [ ] **Step 1: Write the failing tests plus the golden placeholder.**

  Create `internal/cli/validate_stale_test.go` (imports `strings`,
  `testing`, `time`, `github.com/eisenwinter/awit/pkg/config`,
  `github.com/eisenwinter/awit/pkg/item`):

  ```go
  package cli

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
          0:                           "0m",
          45 * time.Minute:            "45m",
          59 * time.Minute:            "59m",
          time.Hour:                   "1h0m",
          3*time.Hour + 12*time.Minute:  "3h12m",
          47*time.Hour + 5*time.Minute:  "47h5m",
          48 * time.Hour:              "2d0h",
          51*time.Hour + 30*time.Minute: "2d3h",
      }
      for d, want := range cases {
          if got := humanDuration(d); got != want {
              t.Fatalf("humanDuration(%v) = %q, want %q", d, got, want)
          }
      }
  }
  ```

  Create `testdata/golden/validate-stale-claims.golden` as an empty
  file for now — Step 4 regenerates it and you verify the content
  before committing. The committed golden MUST contain the exact
  `WARN  [STALE CLAIM] AWIT-TEST0006 claimed by agent/claude 3h12m ago
  (limit 2h)` + fix lines.

- [ ] **Step 2: Run them, see them fail.**

  ```bash
  go test ./internal/cli -run 'TestValidateStale|TestHumanDuration' -v
  ```

  Expected failure:

  ```text
  # github.com/eisenwinter/awit/internal/cli [github.com/eisenwinter/awit/internal/cli.test]
  internal/cli/validate_stale_test.go: undefined: now
  FAIL	github.com/eisenwinter/awit/internal/cli [build failed]
  ```

  (If `validate --stale-claims` already parses but does nothing, the
  red is instead `stdout missing stale warning` — either counts; do
  not skip it.)

- [ ] **Step 3: Implement the flag, scan, and duration.**

  In `internal/cli/validate.go`:

  1. Add the package-level clock (only if S3G did not already declare
     `now` — reuse theirs if present):

     ```go
     // now is the clock for stale-claim ages; tests overwrite it.
     var now = time.Now()
     ```

  2. Register the flag on the validate command alongside S3G's flags:

     ```go
     &cli.BoolFlag{Name: "stale-claims", Usage: "warn on in_progress claims older than stale_claim"},
     ```

  3. After the base report is printed (same function, same writer),
     append:

     ```go
     if cmd.Bool("stale-claims") {
         for _, line := range staleClaimLines(g, time.Duration(s.Config.StaleClaim), now) {
             fmt.Fprintln(cmd.Writer, line)
         }
     }
     ```

     `g` is the already-loaded graph, `s` the already-open store —
     introduce no new opens. The exit-code logic below stays untouched.

  4. Add the pure helpers at the bottom of the same file:

     ```go
     // staleClaimLines reports in_progress nodes whose claim is older
     // than limit, or which carry no claimed_at at all. Quarantined
     // nodes are skipped (validate already FAILs them). Output order is
     // graph order (ID ascending). Each warning is two lines: the WARN
     // line and its fix hint.
     func staleClaimLines(g *graph.Graph, limit time.Duration, ref time.Time) []string {
         var out []string
         for _, n := range g.Order {
             if n.Quarantined() {
                 continue
             }
             if n.Item.Status != item.StatusInProgress {
                 continue
             }
             if n.Item.ClaimedAt == nil {
                 out = append(out,
                     fmt.Sprintf("WARN  [STALE CLAIM] %s in progress with no claimed_at (limit %s)", n.Item.ID, limit),
                     fmt.Sprintf("  fix: awit release %s", n.Item.ID))
                 continue
             }
             if age := ref.Sub(*n.Item.ClaimedAt); age > limit {
                 out = append(out,
                     fmt.Sprintf("WARN  [STALE CLAIM] %s claimed by %s %s ago (limit %s)", n.Item.ID, n.Item.Assignee, humanDuration(age), limit),
                     fmt.Sprintf("  fix: awit release %s", n.Item.ID))
             }
         }
         return out
     }

     // humanDuration renders 45m, 3h12m, and 2d3h once beyond 48h.
     func humanDuration(d time.Duration) string {
         if d < 0 {
             d = 0
         }
         m := int(d.Minutes())
         h := m / 60
         if h < 1 {
             return fmt.Sprintf("%dm", m)
         }
         if h < 48 {
             return fmt.Sprintf("%dh%dm", h, m%60)
         }
         return fmt.Sprintf("%dd%dh", h/24, h%24)
     }
     ```

     An empty assignee prints as empty (`claimed by  3h12m ago`) —
     that is honest (the file really has no assignee) and the fixture
     always carries `agent/claude`, so the golden pins the readable
     form. Do not invent a fallback word.

  Details that matter:

  - Strictly `age > limit`: exactly-at-limit is fresh
    (`TestValidateStaleWithinLimit` pins 1h59m < 2h; equality stays
    quiet by the same rule).
  - `limit` renders via `time.Duration.String` through `%s`
    (`2h`, `30m`) — never a custom format.
  - `g.Order` is ID-ascending (AWIT-0ND56N3G), so multi-stale output is
    deterministic with no extra sort.
  - WARN lines never touch the exit code: the command returns nil
    after printing them unless the base report already failed.
  - `--stale-claims` with no stale nodes prints nothing extra (exit
    and bytes identical to plain `validate`).

- [ ] **Step 4: Run green and mint the golden.**

  ```bash
  go test ./internal/cli -run 'TestValidateStale|TestHumanDuration' -v
  ```

  `TestValidateStaleClaimsWarn` fails on the empty golden first
  (`got` vs `want` empty). Regenerate it the same way every golden in
  this repo is made — set the package `update` flag (helpers_test.go,
  AWIT-0ND56G3G) and re-run only this test:

  ```bash
  go test ./internal/cli -run 'TestValidateStaleClaimsWarn' -update
  git diff testdata/golden/validate-stale-claims.golden
  ```

  Inspect the diff: the base report bytes plus exactly the two stale
  lines (`claimed by agent/claude 3h12m ago (limit 2h)` + fix). Then:

  ```bash
  go test ./internal/cli -count=1
  ```

  Expected: all five stale tests PASS and the package stays green.

- [ ] **Step 5: Commit.**

  ```bash
  gofmt -l internal/cli
  go vet ./internal/cli
  git add internal/cli/validate.go internal/cli/validate_stale_test.go testdata/golden/validate-stale-claims.golden
  git commit -m "cli/validate: stale-claim warnings"
  ```

- [ ] **Step 6: Close this ticket.**

  Set `status: closed` in this file's frontmatter, then:

  ```bash
  git add .awit/items/AWIT-0ND5733G.md
  git commit -m "tickets: close AWIT-0ND5733G"
  ```

## Acceptance Criteria

- `go test ./internal/cli -run 'TestValidateStale|TestHumanDuration' -count=1 -v`
  — all five tests PASS; `go test ./internal/cli -count=1` stays green.
- `awit validate --stale-claims` on `clean` with the clock pinned to
  `2026-09-17T17:44:05Z` prints
  `WARN  [STALE CLAIM] AWIT-TEST0006 claimed by agent/claude 3h12m ago (limit 2h)`
  plus `  fix: awit release AWIT-TEST0006`, exit 0, and the committed
  golden matches byte-for-byte.
- Same command with the clock at `2026-09-17T16:31:05Z` prints no
  `[STALE CLAIM]` line.
- With `stale_claim: 30m` in config and age 45m, the line reads
  `45m ago (limit 30m)`.
- `in_progress` with no `claimed_at` warns
  `in progress with no claimed_at (limit 2h)` with the release fix,
  exit 0.
- `humanDuration` pins `0m 45m 59m 1h0m 3h12m 47h5m 2d0h 2d3h`.
- Plain `awit validate` (no flag) output is byte-identical to S3G's —
  this ticket adds lines only under `--stale-claims`.
- `gofmt -l internal/cli` prints nothing.

## Out of scope

- Changing S3G's base report, FAIL/WARN wording, or exit-code logic.
- JSON rendering of stale claims (text report only).
- Auto-releasing stale claims (`awit release` stays manual).
- Locking, hooks, goreleaser (sibling phase-5 tickets).
- Redeclaring `run`, `copyFixture`, `golden`, `openStore`, `loadGraph`,
  or `now` if S3G already owns one of them.
