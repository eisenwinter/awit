---
id: AWIT-0ND5713G
title: End-to-end agent loop test on the loop fixture
brief: >-
  Add internal/cli/e2e_test.go that drives the five-step agent loop against the loop fixture inside a real git repo: prime, next --claim, show --full, comment, close, then drain 0002 and 0003 until next exits 1.
status: closed
deps:
  [AWIT-0ND56W3G, AWIT-0ND56X3G, AWIT-0ND56Y3G, AWIT-0ND5703G, AWIT-0ND56M3G]
labels: [phase4, p0]
refs_base: repo
refs:
  - plan/implementation-guide.md
  - plan/awit-implementation-plan.md
---

## Summary

After this ticket `internal/cli/e2e_test.go` exists and nothing else is added. The test copies `testdata/fixtures/loop`, turns it into a git repository (skip when `git` is not installed), and runs the agent loop: `prime` (golden start), `next --claim --agent claude --seed 7` (0001), `show --full` (spec text), `comment` then `show --refs-only` (2 refs), `close`, `prime` after close, `validate` PASS, claim/close 0002 and 0003, final `next` exit 1, `prime` with READY (0) BLOCKED (0) and no CRITICAL PATH, `validate` PASS, two `prime` runs byte-identical. No production files change.

## Context (read first)

- Guide §5 - command tests call `Main` through `run` / `copyFixture`. Always pass `--repo`. This ticket only creates `e2e_test.go` plus two goldens.
- Guide §8 `loop` fixture (AWIT-0ND56N3G, already on disk):
  - `AWIT-TEST0001` open, no deps, labels `auth,p1`, title `Implement OAuth2 bearer token extraction`, refs `[../../docs/spec.md]`, UnblockCount 2, Ready.
  - `AWIT-TEST0002` open, deps `[0001]`, labels `auth`, title `Add E2E auth tests`, UnblockCount 1, Blocked.
  - `AWIT-TEST0003` open, deps `[0002]`, labels `p0`, title `Rotate API tokens`, UnblockCount 0, Blocked.
  - `docs/spec.md` at the fixture root (copied by `copyFixture` because it copies the whole `testdata/fixtures/loop` tree). Body contains `URL-safe base64 alphabet` and `invalid_token`.
  - Critical path: `0001 -> 0002 -> 0003`.
- Spec §Agent surface `awit prime` (AWIT-0ND56W3G implements this exact layout). Warnings omitted when empty. Ready compact lines. Blocked lines are `[ID] Title <- depIDs`. Critical path is IDs joined by `->`. Empty CRITICAL PATH section is omitted; READY/BLOCKED headers still print with count 0.
- `next --claim` (AWIT-0ND56X3G): `--agent claude` → assignee `agent/claude`; `--seed 7`; commits `awit: claim <id>` because the e2e dir is a git repo. Do **not** pass `--no-commit`.
- `show --full` / `--refs-only` (AWIT-0ND5703G): flags **before** the id (`show --full AWIT-TEST0001`) because urfave/cli v3 stops flag parsing at the first positional. `--full` stdout contains the spec file bytes. `--refs-only` after one comment lists two refs: `../../docs/spec.md` and a `../comments/AWIT-TEST0001/...` path.
- `comment` (AWIT-0ND56Y3G): `comment --author claude AWIT-TEST0001 ...`. `close` (AWIT-0ND56M3G) does **not** git-commit.
- `validate` (AWIT-0ND56S3G): loop fixture is clean → `PASS  3 items, 0 quarantined\n` at start and again after every item is closed (still 3 files, all closed, 0 quarantined).
- Helpers already in the package: `run`, `copyFixture`, `golden`, `readItem`. Do not redeclare them. AWIT-0ND56X3G's `next_test.go` already defines `gitLookPath` and `gitRun` in package `cli`; this file must **not** redeclare those names. The git helper below is named `loopGitAvailable` / `loopGit` so the package compiles.
- Git identity must be configured or `next --claim` cannot commit. Set `user.name` and `user.email`.
- Two `prime` runs on identical state must be `bytes.Equal` (guide §5 determinism test, applied to the drained graph).

## Files

- Create: `internal/cli/e2e_test.go`
- Create: `testdata/golden/e2e_prime_start.golden`
- Create: `testdata/golden/e2e_prime_after_0001.golden`
- Modify: none. Do not add a command. Do not edit `app.go`.

## Interfaces

- Consumes (all already implemented by deps; this ticket only calls them through `Main`):
  ```go
  func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int
  func run(t *testing.T, args ...string) (code int, stdout, stderr string)
  func copyFixture(t *testing.T, name string) string
  func golden(t *testing.T, name string, got []byte)
  func readItem(t *testing.T, repo, id string) *item.Item
  ```
- Produces: nothing in the production API. Test-only helpers in `e2e_test.go` (names chosen so they do not collide with `next_test.go`):
  ```go
  func loopGitAvailable(t *testing.T)
  func loopGit(t *testing.T, dir string, args ...string) string
  func initLoopRepo(t *testing.T) string
  func loopMustRun(t *testing.T, args ...string) (stdout string)
  ```

## Steps

- [ ] **Step 1: Write goldens and the failing test.**

  `testdata/golden/e2e_prime_start.golden` - spec §Agent surface applied to the N3G loop fixture (trailing newline, no GRAPH WARNINGS section):

  ```text
  === READY (1) ===
  [AWIT-TEST0001] Implement OAuth2 bearer token extraction | auth,p1 | Unblocks: 2

  === BLOCKED (2) ===
  [AWIT-TEST0002] Add E2E auth tests <- AWIT-TEST0001
  [AWIT-TEST0003] Rotate API tokens <- AWIT-TEST0002

  === CRITICAL PATH (3) ===
  AWIT-TEST0001 -> AWIT-TEST0002 -> AWIT-TEST0003
  ```

  `testdata/golden/e2e_prime_after_0001.golden` - after 0001 is closed, 0002 is Ready (UnblockCount 1), 0003 still blocked on 0002:

  ```text
  === READY (1) ===
  [AWIT-TEST0002] Add E2E auth tests | auth | Unblocks: 1

  === BLOCKED (1) ===
  [AWIT-TEST0003] Rotate API tokens <- AWIT-TEST0002

  === CRITICAL PATH (2) ===
  AWIT-TEST0002 -> AWIT-TEST0003
  ```

  If AWIT-0ND56W3G's renderer emits these bytes with a different blank-line convention, the golden comparison will fail - fix the renderer or this golden so they agree with the spec block above. Do not weaken the test to `strings.Contains` for the start/after goldens.

  Create `internal/cli/e2e_test.go`:

  ```go
  package cli

  import (
  	"os/exec"
  	"strings"
  	"testing"

  	"github.com/eisenwinter/awit/pkg/item"
  )

  func loopGitAvailable(t *testing.T) {
  	t.Helper()
  	if _, err := exec.LookPath("git"); err != nil {
  		t.Skip("git not installed")
  	}
  }

  func loopGit(t *testing.T, dir string, args ...string) string {
  	t.Helper()
  	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
  	out, err := cmd.CombinedOutput()
  	if err != nil {
  		t.Fatalf("git %s: %v: %s", args, err, out)
  	}
  	return strings.TrimSpace(string(out))
  }

  func initLoopRepo(t *testing.T) string {
  	t.Helper()
  	loopGitAvailable(t)
  	dir := copyFixture(t, "loop")
  	loopGit(t, dir, "init", "-q")
  	loopGit(t, dir, "config", "user.name", "tester")
  	loopGit(t, dir, "config", "user.email", "tester@example.com")
  	loopGit(t, dir, "add", ".")
  	loopGit(t, dir, "commit", "-q", "-m", "init")
  	return dir
  }

  func loopMustRun(t *testing.T, args ...string) string {
  	t.Helper()
  	code, stdout, stderr := run(t, args...)
  	if code != 0 {
  		t.Fatalf("awit %s: exit %d stderr %q stdout %q", args, code, stderr, stdout)
  	}
  	if stderr != "" {
  		t.Fatalf("awit %s: stderr %q", args, stderr)
  	}
  	return stdout
  }

  func TestAgentLoop(t *testing.T) {
  	dir := initLoopRepo(t)
  	repo := []string{"--repo", dir}

  	start := loopMustRun(t, append(repo, "prime")...)
  	golden(t, "e2e_prime_start.golden", []byte(start))

  	nextOut := loopMustRun(t, append(repo, "--format", "compact", "next", "--claim", "--agent", "claude", "--seed", "7")...)
  	if !strings.HasPrefix(nextOut, "[AWIT-TEST0001]") {
  		t.Fatalf("next --claim = %q, want 0001", nextOut)
  	}
  	claimed := readItem(t, dir, "AWIT-TEST0001")
  	if claimed.Status != item.StatusInProgress || claimed.Assignee != "agent/claude" {
  		t.Fatalf("claimed status=%q assignee=%q", claimed.Status, claimed.Assignee)
  	}
  	if got := loopGit(t, dir, "log", "-1", "--pretty=%s"); got != "awit: claim AWIT-TEST0001" {
  		t.Fatalf("claim commit = %q", got)
  	}

  	full := loopMustRun(t, append(repo, "show", "--full", "AWIT-TEST0001")...)
  	if !strings.Contains(full, "URL-safe base64 alphabet") {
  		t.Fatalf("show --full missing spec token grammar:\n%s", full)
  	}
  	if !strings.Contains(full, "invalid_token") {
  		t.Fatalf("show --full missing spec 401 body:\n%s", full)
  	}

  	_ = loopMustRun(t, append(repo, "comment", "--author", "claude", "AWIT-TEST0001", "loop", "research")...)
  	refsOnly := loopMustRun(t, append(repo, "show", "--refs-only", "AWIT-TEST0001")...)
  	if !strings.Contains(refsOnly, "../../docs/spec.md") {
  		t.Fatalf("refs-only missing spec ref:\n%s", refsOnly)
  	}
  	if !strings.Contains(refsOnly, "../comments/AWIT-TEST0001/") {
  		t.Fatalf("refs-only missing comment ref:\n%s", refsOnly)
  	}
  	if strings.Contains(refsOnly, "\\") {
  		t.Fatalf("refs-only contains a backslash:\n%s", refsOnly)
  	}
  	nRefs := 0
  	if strings.Contains(refsOnly, "../../docs/spec.md") {
  		nRefs++
  	}
  	if strings.Contains(refsOnly, "../comments/AWIT-TEST0001/") {
  		nRefs++
  	}
  	if nRefs != 2 {
  		t.Fatalf("refs-only should mention 2 refs, got %d\n%s", nRefs, refsOnly)
  	}
  	afterComment := readItem(t, dir, "AWIT-TEST0001")
  	if len(afterComment.Refs) != 2 {
  		t.Fatalf("item refs = %v, want 2", afterComment.Refs)
  	}

  	_ = loopMustRun(t, append(repo, "close", "AWIT-TEST0001")...)
  	closed := readItem(t, dir, "AWIT-TEST0001")
  	if closed.Status != item.StatusClosed {
  		t.Fatalf("status after close = %q", closed.Status)
  	}

  	after := loopMustRun(t, append(repo, "prime")...)
  	golden(t, "e2e_prime_after_0001.golden", []byte(after))

  	val := loopMustRun(t, append(repo, "--format", "compact", "validate")...)
  	if val != "PASS  3 items, 0 quarantined\n" {
  		t.Fatalf("validate after 0001 close = %q", val)
  	}

  	next2 := loopMustRun(t, append(repo, "--format", "compact", "next", "--claim", "--agent", "claude", "--seed", "7")...)
  	if !strings.HasPrefix(next2, "[AWIT-TEST0002]") {
  		t.Fatalf("next 2 = %q, want 0002", next2)
  	}
  	_ = loopMustRun(t, append(repo, "close", "AWIT-TEST0002")...)

  	next3 := loopMustRun(t, append(repo, "--format", "compact", "next", "--claim", "--agent", "claude", "--seed", "7")...)
  	if !strings.HasPrefix(next3, "[AWIT-TEST0003]") {
  		t.Fatalf("next 3 = %q, want 0003", next3)
  	}
  	_ = loopMustRun(t, append(repo, "close", "AWIT-TEST0003")...)

  	code, stdout, stderr := run(t, append(repo, "--format", "compact", "next", "--seed", "7")...)
  	if code != 1 || stdout != "" || stderr != "No ready items\n" {
  		t.Fatalf("final next: exit %d stdout %q stderr %q", code, stdout, stderr)
  	}

  	drained := loopMustRun(t, append(repo, "prime")...)
  	if !strings.Contains(drained, "=== READY (0) ===") {
  		t.Fatalf("drained prime missing READY (0):\n%s", drained)
  	}
  	if !strings.Contains(drained, "=== BLOCKED (0) ===") {
  		t.Fatalf("drained prime missing BLOCKED (0):\n%s", drained)
  	}
  	if strings.Contains(drained, "CRITICAL PATH") {
  		t.Fatalf("drained prime still has CRITICAL PATH:\n%s", drained)
  	}

  	val2 := loopMustRun(t, append(repo, "--format", "compact", "validate")...)
  	if val2 != "PASS  3 items, 0 quarantined\n" {
  		t.Fatalf("validate drained = %q", val2)
  	}

  	again := loopMustRun(t, append(repo, "prime")...)
  	if drained != again {
  		t.Fatalf("prime not deterministic\nfirst:\n%s\nsecond:\n%s", drained, again)
  	}
  }
  ```

- [ ] **Step 2: Run it, see it fail.**

  ```bash
  go test ./internal/cli -run TestAgentLoop -v
  ```

  Expected: the test is compiled (deps are closed so `prime` / `next` / `show` / `comment` / `close` / `validate` exist). It fails because this file was just added and the first assertion that does not match current behaviour is a red, **or** - if every command already works - this is the first time the loop is wired together and a missing `docs/spec.md` copy / claim commit / prime blank line will fail. If `git` is missing the test SKIPs; that is not a pass of the loop. On a machine with git, a typical first red is a golden mismatch or `unknown command` if a dep was not actually closed.

  Do not skip Step 2. If the test SKIPs, install git or run on the Linux CI image; do not delete the skip.

- [ ] **Step 3: No production code.** This ticket has no implementation step. If the test fails, the bug is in a dep (prime renderer, next claim, show --full, comment refs, close, validate). Fix that ticket's code, do not special-case the loop fixture in `e2e_test.go`.

- [ ] **Step 4: Run it, see it pass, commit.**

  ```bash
  go test ./internal/cli -run TestAgentLoop -v
  ```

  Expected:

  ```text
  === RUN   TestAgentLoop
  --- PASS: TestAgentLoop
  PASS
  ```

  (or `SKIP` only when `git` is not on PATH).

  ```bash
  go test ./internal/cli -count=1
  gofmt -w internal/cli/e2e_test.go
  git add internal/cli/e2e_test.go testdata/golden/e2e_prime_start.golden testdata/golden/e2e_prime_after_0001.golden
  git commit -m "cli: end-to-end agent loop against the loop fixture"
  ```

- [ ] **Step 5: Full check and close.**

  ```bash
  go build ./... && go vet ./... && go test ./internal/cli -count=1
  ```

  Set `status: closed` on this file, write `.awit/comments/AWIT-0ND5713G/<YYYYMMDDTHHMMSSZ>-<author>.md` with the acceptance output, append that ref, commit `tickets: close AWIT-0ND5713G`.

## Acceptance Criteria

- `go test ./internal/cli -run TestAgentLoop -v` PASS (SKIP only when git is absent).
- Start `prime` bytes equal `e2e_prime_start.golden`.
- `next --claim --agent claude --seed 7` claims `AWIT-TEST0001`, git subject `awit: claim AWIT-TEST0001`.
- `show --full AWIT-TEST0001` contains `URL-safe base64 alphabet` and `invalid_token`.
- After `comment --author claude`, `show --refs-only` mentions both `../../docs/spec.md` and `../comments/AWIT-TEST0001/`; the item has 2 refs, forward slashes only.
- `prime` after closing 0001 equals `e2e_prime_after_0001.golden`.
- `validate` is `PASS  3 items, 0 quarantined\n` after 0001 closes and again after 0003 closes.
- Claim/close 0002 then 0003; final `next` exits 1 with stderr `No ready items\n`.
- Drained `prime` contains `=== READY (0) ===` and `=== BLOCKED (0) ===` and does not contain `CRITICAL PATH`.
- Two drained `prime` runs are byte-identical.
- No file under `internal/cli/` other than `e2e_test.go` is created or modified. `gofmt -l internal/cli/e2e_test.go` prints nothing.

## Out of scope

- Implementing `prime`, `next`, `show --full`, `comment`, `close`, or `validate`. Changing the loop fixture. Adding a lock around the loop. Windows-specific git config beyond `user.name` / `user.email`.
