---
id: AWIT-0ND5723G
title: "pkg/lock and store locking"
brief: >-
  Add pkg/lock.Acquire (50ms poll via tryLock; unix Flock, Windows LockFileEx) and Store.Lock on .awit/.lock. Mutating commands lock for 5s after openStore. Timeout prints Error: another awit process holds .awit/.lock (waited 5s).
status: closed
deps: [AWIT-0ND56E3G]
labels: [phase5, p2]
refs_base: repo
refs:
  - plan/implementation-guide.md
  - plan/awit-implementation-plan.md
---

## Summary

After this ticket `pkg/lock` exposes `Acquire(path, timeout)` which creates the file, takes an exclusive advisory lock, and polls `tryLock` every 50ms until success or timeout. Unix uses `syscall.Flock(LOCK_EX|LOCK_NB)`; Windows uses `golang.org/x/sys/windows.LockFileEx`. `Store.Lock` locks `filepath.Join(s.Dir, ".lock")` and, on timeout, returns `another awit process holds .awit/.lock (waited <duration>)`. Every mutating CLI command (`create`, `update`, `close`, `release`, `dep add`, `dep rm`, `comment`, `next --claim`) calls `s.Lock(5 * time.Second)` immediately after a successful `openStore` and `defer release()`. Read-only commands do not lock. Same-checkout concurrent `create` from ten goroutines all succeed with unique IDs.

## Context (read first)

- Guide §4.10 `pkg/lock` - the only exported signature: `func Acquire(path string, timeout time.Duration) (release func() error, err error)`. Copy it. Do not rename. `release` unlocks and closes the file; leave the `.lock` file on disk (it is gitignored).
- Guide §1 - `golang.org/x/sys` is allowed **only** in `pkg/lock` for Windows `LockFileEx`. Unix must use stdlib `syscall.Flock`, not `x/sys/unix`. Never hardcode `/`; `filepath.Join(s.Dir, ".lock")`. Linux **and** Windows.
- Guide §1 errors - CLI `Action` errors print as `Error: <msg>` via `report`. The timeout string users see is exactly `Error: another awit process holds .awit/.lock (waited 5s)` because commands pass `5 * time.Second` and `time.Duration.String()` for that value is `5s`. `Store.Lock` produces the inner message; `Main` adds the `Error: ` prefix.
- Guide §3 layout - `pkg/lock/lock.go lock_unix.go lock_windows.go`.
- Guide §4.4 `Store` - `Init` already appends `.awit/.lock` to `.gitignore` (ticket `AWIT-0ND56E3G`). Do not change `Init`. `Store.Lock` is **new**; it is not in §4.4; add it with the signature in Interfaces.
- Guide §4.11 - `openStore` already exists (`AWIT-0ND56G3G`). Insert the lock **after** it returns, never inside `openStore` (read-only commands also call `openStore`).
- Spec Phase 5 - "Optional `.awit/.lock` (flock / LockFileEx) for same-checkout concurrency". Same worktree, two processes. Cross-worktree exclusion is Git, not this lock.
- Spec Data model - the only non-committed file under `.awit/` is the lock.
- Dep `AWIT-0ND56E3G` must be `status: closed` before you start. Phase-5 means `create` / `update` / `close` / `release` / `dep` / `comment` / `next` already exist; this ticket only inserts the lock call.
- `internal/cli` tests never call `t.Parallel()` (`helpers_test.go` comment). Lock tests that share a path also must not.

## Files

- Create: `pkg/lock/lock.go`
- Create: `pkg/lock/lock_unix.go`
- Create: `pkg/lock/lock_windows.go`
- Create: `pkg/lock/lock_test.go`
- Create: `pkg/item/lock_test.go`
- Create: `internal/cli/lock_test.go`
- Modify: `pkg/item/store.go` - add `Store.Lock`; add imports `"fmt"`, `"time"`, `"github.com/eisenwinter/awit/pkg/lock"` if missing.
- Modify: `internal/cli/create.go` - lock after `openStore`.
- Modify: `internal/cli/update.go` - lock after `openStore`.
- Modify: `internal/cli/close.go` - lock after `openStore`.
- Modify: `internal/cli/release.go` - lock after `openStore`.
- Modify: `internal/cli/dep.go` - lock after `openStore` in both add and rm.
- Modify: `internal/cli/comment.go` - lock after `openStore`.
- Modify: `internal/cli/next.go` - lock after `openStore` **only when** `--claim` is set.
- Modify: `go.mod` / `go.sum` via `go get golang.org/x/sys` only.

## Interfaces

- Consumes (already implemented; do not reimplement):
  ```go
  func openStore(cmd *cli.Command) (*item.Store, error)
  func Init(repoRoot, prefix string) (*Store, error)
  func Open(repoRoot string) (*Store, error)
  func (s *Store) Dir string // field; .awit directory
  func run(t *testing.T, args ...string) (code int, stdout, stderr string)
  func initRepo(t *testing.T) string
  ```
- Produces (verbatim from guide §4.10):

  ```go
  package lock

  func Acquire(path string, timeout time.Duration) (release func() error, err error)
  ```

- Produces (this ticket, not in the guide):

  ```go
  package lock
  var ErrTimeout = errors.New("lock: timeout")
  func tryLock(f *os.File) error    // lock_unix.go / lock_windows.go
  func tryUnlock(f *os.File) error  // lock_unix.go / lock_windows.go
  func isBusy(err error) bool       // lock_unix.go / lock_windows.go

  package item
  // Lock takes an exclusive advisory lock on Dir/.lock, creating the file.
  // Blocks up to timeout. On lock.ErrTimeout the error is:
  // "another awit process holds .awit/.lock (waited <timeout.String()>)".
  // Other Acquire errors are returned unchanged.
  func (s *Store) Lock(timeout time.Duration) (release func() error, err error)
  ```

## Steps

- [ ] **Step 1: Write the failing `pkg/lock` tests.**

  Create `pkg/lock/lock_test.go`:

  ```go
  package lock

  import (
  	"errors"
  	"os"
  	"path/filepath"
  	"testing"
  	"time"
  )

  func TestAcquireRelease(t *testing.T) {
  	path := filepath.Join(t.TempDir(), "lock")
  	rel, err := Acquire(path, time.Second)
  	if err != nil {
  		t.Fatal(err)
  	}
  	if _, err := os.Stat(path); err != nil {
  		t.Fatalf("lock file missing after Acquire: %v", err)
  	}
  	if err := rel(); err != nil {
  		t.Fatal(err)
  	}
  	rel2, err := Acquire(path, time.Second)
  	if err != nil {
  		t.Fatalf("re-acquire after release: %v", err)
  	}
  	if err := rel2(); err != nil {
  		t.Fatal(err)
  	}
  }

  func TestSecondAcquireBlocksUntilRelease(t *testing.T) {
  	path := filepath.Join(t.TempDir(), "lock")
  	rel, err := Acquire(path, time.Second)
  	if err != nil {
  		t.Fatal(err)
  	}
  	started := make(chan struct{})
  	done := make(chan error, 1)
  	go func() {
  		close(started)
  		rel2, err := Acquire(path, 3*time.Second)
  		if err != nil {
  			done <- err
  			return
  		}
  		done <- rel2()
  	}()
  	<-started
  	select {
  	case err := <-done:
  		t.Fatalf("second Acquire returned before release: %v", err)
  	case <-time.After(150 * time.Millisecond):
  	}
  	if err := rel(); err != nil {
  		t.Fatal(err)
  	}
  	select {
  	case err := <-done:
  		if err != nil {
  			t.Fatalf("second Acquire: %v", err)
  		}
  	case <-time.After(3 * time.Second):
  		t.Fatal("second Acquire did not unblock after release")
  	}
  }

  func TestAcquireTimeout(t *testing.T) {
  	path := filepath.Join(t.TempDir(), "lock")
  	rel, err := Acquire(path, time.Second)
  	if err != nil {
  		t.Fatal(err)
  	}
  	defer rel()
  	start := time.Now()
  	_, err = Acquire(path, 200*time.Millisecond)
  	elapsed := time.Since(start)
  	if !errors.Is(err, ErrTimeout) {
  		t.Fatalf("err = %v, want ErrTimeout", err)
  	}
  	if elapsed < 150*time.Millisecond || elapsed > 2*time.Second {
  		t.Fatalf("timeout elapsed %v, want ~200ms", elapsed)
  	}
  }
  ```

- [ ] **Step 2: Run it, see it fail.**

  ```bash
  go test ./pkg/lock -run 'TestAcquireRelease|TestSecondAcquireBlocksUntilRelease|TestAcquireTimeout' -v
  ```

  Expected failure:

  ```text
  pkg/lock/lock_test.go: undefined: Acquire
  FAIL	github.com/eisenwinter/awit/pkg/lock [build failed]
  ```

- [ ] **Step 3: Implement `pkg/lock`.**

  Create `pkg/lock/lock.go`:

  ```go
  package lock

  import (
  	"errors"
  	"os"
  	"time"
  )

  // ErrTimeout is returned by Acquire when the exclusive lock cannot be taken
  // before the deadline.
  var ErrTimeout = errors.New("lock: timeout")

  const pollInterval = 50 * time.Millisecond

  // Acquire takes an exclusive advisory lock on path (creating the file).
  // Blocks up to timeout, polling tryLock every 50ms. The returned release
  // function unlocks and closes the file but does not remove it.
  func Acquire(path string, timeout time.Duration) (release func() error, err error) {
  	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
  	if err != nil {
  		return nil, err
  	}
  	deadline := time.Now().Add(timeout)
  	for {
  		err := tryLock(f)
  		if err == nil {
  			return func() error {
  				uerr := tryUnlock(f)
  				cerr := f.Close()
  				if uerr != nil {
  					return uerr
  				}
  				return cerr
  			}, nil
  		}
  		if !isBusy(err) {
  			f.Close()
  			return nil, err
  		}
  		if timeout == 0 || !time.Now().Before(deadline) {
  			f.Close()
  			return nil, ErrTimeout
  		}
  		sleep := pollInterval
  		if rem := time.Until(deadline); rem < sleep {
  			sleep = rem
  		}
  		if sleep <= 0 {
  			f.Close()
  			return nil, ErrTimeout
  		}
  		time.Sleep(sleep)
  	}
  }
  ```

  Create `pkg/lock/lock_unix.go`:

  ```go
  //go:build unix

  package lock

  import (
  	"errors"
  	"os"
  	"syscall"
  )

  func tryLock(f *os.File) error {
  	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
  }

  func tryUnlock(f *os.File) error {
  	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
  }

  func isBusy(err error) bool {
  	return errors.Is(err, syscall.EAGAIN) || errors.Is(err, syscall.EWOULDBLOCK)
  }
  ```

  Create `pkg/lock/lock_windows.go`:

  ```go
  //go:build windows

  package lock

  import (
  	"errors"
  	"os"

  	"golang.org/x/sys/windows"
  )

  func tryLock(f *os.File) error {
  	var ol windows.Overlapped
  	return windows.LockFileEx(
  		windows.Handle(f.Fd()),
  		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
  		0,
  		1,
  		0,
  		&ol,
  	)
  }

  func tryUnlock(f *os.File) error {
  	var ol windows.Overlapped
  	return windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
  }

  func isBusy(err error) bool {
  	return errors.Is(err, windows.ERROR_LOCK_VIOLATION)
  }
  ```

  Then (this ticket is the only one allowed to add `x/sys`):

  ```bash
  go get golang.org/x/sys
  ```

  Do not import `x/sys` from any file except `lock_windows.go`. Unix stays on `syscall`.

- [ ] **Step 4: Run the lock tests, see them pass, commit.**

  ```bash
  go test ./pkg/lock -run 'TestAcquireRelease|TestSecondAcquireBlocksUntilRelease|TestAcquireTimeout' -v
  ```

  Expected:

  ```text
  === RUN   TestAcquireRelease
  --- PASS: TestAcquireRelease
  === RUN   TestSecondAcquireBlocksUntilRelease
  --- PASS: TestSecondAcquireBlocksUntilRelease
  === RUN   TestAcquireTimeout
  --- PASS: TestAcquireTimeout
  PASS
  ok  	github.com/eisenwinter/awit/pkg/lock
  ```

  ```bash
  gofmt -w pkg/lock/lock.go pkg/lock/lock_unix.go pkg/lock/lock_windows.go pkg/lock/lock_test.go
  git add pkg/lock go.mod go.sum
  git commit -m "lock: advisory flock and LockFileEx with 50ms poll"
  ```

- [ ] **Step 5: Write the failing `Store.Lock` tests.**

  Create `pkg/item/lock_test.go`:

  ```go
  package item

  import (
  	"os"
  	"path/filepath"
  	"strings"
  	"testing"
  	"time"
  )

  func TestLockIsGitignored(t *testing.T) {
  	root := t.TempDir()
  	s, err := Init(root, "AWIT")
  	if err != nil {
  		t.Fatal(err)
  	}
  	rel, err := s.Lock(time.Second)
  	if err != nil {
  		t.Fatal(err)
  	}
  	defer rel()
  	gi, err := os.ReadFile(filepath.Join(root, ".gitignore"))
  	if err != nil {
  		t.Fatal(err)
  	}
  	found := 0
  	for _, line := range strings.Split(strings.TrimSuffix(string(gi), "\n"), "\n") {
  		if line == ".awit/.lock" {
  			found++
  		}
  	}
  	if found != 1 {
  		t.Fatalf(".gitignore .awit/.lock count = %d, want 1\n%s", found, gi)
  	}
  	if _, err := os.Stat(filepath.Join(s.Dir, ".lock")); err != nil {
  		t.Fatalf("lock file not created: %v", err)
  	}
  }

  func TestStoreLockTimeoutMessage(t *testing.T) {
  	root := t.TempDir()
  	s, err := Init(root, "AWIT")
  	if err != nil {
  		t.Fatal(err)
  	}
  	rel, err := s.Lock(time.Minute)
  	if err != nil {
  		t.Fatal(err)
  	}
  	defer rel()
  	_, err = s.Lock(50 * time.Millisecond)
  	if err == nil {
  		t.Fatal("second Lock: want timeout")
  	}
  	want := "another awit process holds .awit/.lock (waited 50ms)"
  	if err.Error() != want {
  		t.Fatalf("err = %q, want %q", err.Error(), want)
  	}
  }
  ```

- [ ] **Step 6: Run it, see it fail.**

  ```bash
  go test ./pkg/item -run 'TestLockIsGitignored|TestStoreLockTimeoutMessage' -v
  ```

  Expected: `undefined: (*Store).Lock`, `[build failed]`.

- [ ] **Step 7: Implement `Store.Lock`.**

  Add this method to `pkg/item/store.go` (do not put it in a new file). Imports to add if missing: `"fmt"`, `"time"`, `"github.com/eisenwinter/awit/pkg/lock"`. Keep existing imports.

  ```go
  // Lock takes an exclusive advisory lock on Dir/.lock, creating the file.
  // On lock.ErrTimeout the returned error is
  // "another awit process holds .awit/.lock (waited <timeout>)".
  func (s *Store) Lock(timeout time.Duration) (func() error, error) {
  	rel, err := lock.Acquire(filepath.Join(s.Dir, ".lock"), timeout)
  	if err == nil {
  		return rel, nil
  	}
  	if errors.Is(err, lock.ErrTimeout) {
  		return nil, fmt.Errorf("another awit process holds .awit/.lock (waited %s)", timeout)
  	}
  	return nil, err
  }
  ```

  `errors` is already imported in `store.go`. Path is `filepath.Join(s.Dir, ".lock")` - never `s.Dir + "/.lock"`.

- [ ] **Step 8: Run Store.Lock tests, see them pass, commit.**

  ```bash
  go test ./pkg/item -run 'TestLockIsGitignored|TestStoreLockTimeoutMessage' -v
  ```

  Expected: both `--- PASS`, `ok github.com/eisenwinter/awit/pkg/item`.

  ```bash
  gofmt -w pkg/item/store.go pkg/item/lock_test.go
  git add pkg/item/store.go pkg/item/lock_test.go
  git commit -m "item: Store.Lock on .awit/.lock"
  ```

- [ ] **Step 9: Write the failing CLI lock tests.**

  Create `internal/cli/lock_test.go`:

  ```go
  package cli

  import (
  	"fmt"
  	"os"
  	"path/filepath"
  	"sync"
  	"testing"
  	"time"

  	"github.com/eisenwinter/awit/pkg/item"
  )

  func TestCreateLockTimeout(t *testing.T) {
  	repo := initRepo(t)
  	s, err := item.Open(repo)
  	if err != nil {
  		t.Fatal(err)
  	}
  	rel, err := s.Lock(time.Minute)
  	if err != nil {
  		t.Fatal(err)
  	}
  	defer rel()
  	code, stdout, stderr := run(t, "--repo", repo, "create", "--brief", "blocked by lock.", "Blocked")
  	if code != 1 {
  		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
  	}
  	want := "Error: another awit process holds .awit/.lock (waited 5s)\n"
  	if stderr != want {
  		t.Fatalf("stderr = %q, want %q", stderr, want)
  	}
  }

  func TestConcurrentCreates(t *testing.T) {
  	repo := initRepo(t)
  	const n = 10
  	type result struct {
  		i      int
  		code   int
  		stderr string
  	}
  	ch := make(chan result, n)
  	var wg sync.WaitGroup
  	wg.Add(n)
  	for i := 0; i < n; i++ {
  		i := i
  		go func() {
  			defer wg.Done()
  			code, _, stderr := run(t, "--repo", repo, "create", "--brief", "concurrent item.", fmt.Sprintf("Item %d", i))
  			ch <- result{i: i, code: code, stderr: stderr}
  		}()
  	}
  	wg.Wait()
  	close(ch)
  	for r := range ch {
  		if r.code != 0 {
  			t.Errorf("create %d: exit %d stderr %q", r.i, r.code, r.stderr)
  		}
  	}
  	entries, err := os.ReadDir(filepath.Join(repo, item.DirName, "items"))
  	if err != nil {
  		t.Fatal(err)
  	}
  	var md []string
  	seen := map[string]bool{}
  	for _, e := range entries {
  		name := e.Name()
  		if filepath.Ext(name) != ".md" {
  			continue
  		}
  		if seen[name] {
  			t.Errorf("duplicate filename %s", name)
  		}
  		seen[name] = true
  		md = append(md, name)
  	}
  	if len(md) != n {
  		t.Fatalf("item files = %d, want %d (%v)", len(md), n, md)
  	}
  }
  ```

  `TestCreateLockTimeout` holds the lock for a full 5s. That is required to pin the user-facing string. Do not shorten the command timeout to make the test faster.

- [ ] **Step 10: Run them, see timeout/create fail (no lock in create yet).**

  ```bash
  go test ./internal/cli -run 'TestCreateLockTimeout|TestConcurrentCreates' -v
  ```

  Expected: `TestCreateLockTimeout` FAIL - create succeeds (exit 0) while another holder has the lock, because `create.go` does not call `s.Lock` yet. `TestConcurrentCreates` may PASS by luck (atomic rename) or FAIL with a mint collision; either way Step 11 still adds the lock. The red you need is `TestCreateLockTimeout` not seeing `Error: another awit process holds .awit/.lock (waited 5s)`.

- [ ] **Step 11: Insert the lock into every mutating command.**

  The pattern is exactly these three statements, immediately after a successful `openStore` (after the `if err != nil { return err }` that follows it). Add `"time"` to the file's import block if it is not already there.

  ```go
  release, err := s.Lock(5 * time.Second)
  if err != nil {
  	return err
  }
  defer release()
  ```

  **`internal/cli/create.go`** - `createAction` already has:

  ```go
  s, err := openStore(cmd)
  if err != nil {
  	return err
  }
  ```

  Insert the three statements immediately after that `if` block, before `itemID := cmd.String("id")`.

  **`internal/cli/update.go`** - same insertion in the update `Action` after `openStore`.

  **`internal/cli/close.go`** - same insertion in the close `Action` after `openStore`.

  **`internal/cli/release.go`** - same insertion in the release `Action` after `openStore`.

  **`internal/cli/comment.go`** - same insertion in `commentAction` after `openStore`, before `loadItem`.

  **`internal/cli/dep.go`** - both `dep add` and `dep rm` actions (whatever they are named: `depAddAction` / `depRmAction`, or a shared helper that is the only place `openStore` is called). Insert once per `openStore` success, not once per file if add and rm each call `openStore`. If they share a helper that already has `openStore`, insert there once.

  **`internal/cli/next.go`** - lock **only** when claiming. After `openStore` succeeds:

  ```go
  s, err := openStore(cmd)
  if err != nil {
  	return err
  }
  if cmd.Bool("claim") {
  	release, err := s.Lock(5 * time.Second)
  	if err != nil {
  		return err
  	}
  	defer release()
  }
  ```

  Do **not** lock `next` without `--claim`. Do **not** lock `list`, `show`, `validate`, `prime`, `label`, `init`. Do **not** move the lock into `openStore`.

- [ ] **Step 12: Run CLI tests, see them pass, commit.**

  ```bash
  go test ./internal/cli -run 'TestCreateLockTimeout|TestConcurrentCreates' -v
  ```

  Expected:

  ```text
  === RUN   TestCreateLockTimeout
  --- PASS: TestCreateLockTimeout
  === RUN   TestConcurrentCreates
  --- PASS: TestConcurrentCreates
  PASS
  ok  	github.com/eisenwinter/awit/internal/cli
  ```

  `TestCreateLockTimeout` takes ~5s. Then:

  ```bash
  go test ./pkg/lock ./pkg/item ./internal/cli -count=1
  gofmt -w internal/cli/create.go internal/cli/update.go internal/cli/close.go internal/cli/release.go internal/cli/dep.go internal/cli/comment.go internal/cli/next.go internal/cli/lock_test.go
  git add pkg/lock pkg/item/store.go pkg/item/lock_test.go internal/cli go.mod go.sum
  git commit -m "cli: lock mutating commands on .awit/.lock"
  ```

- [ ] **Step 13: Close ticket.**
  - Set `status: closed` in the frontmatter of `.awit/items/AWIT-0ND5723G.md`.
  - Create `.awit/comments/AWIT-0ND5723G/<YYYYMMDDTHHMMSSZ>-<author>.md` with the captured `go test` output from Steps 4, 8 and 12.
  - Append the ref `../comments/AWIT-0ND5723G/<file>.md` to this ticket's `refs` (forward slashes, block style).
  - Commit:

    ```bash
    git add .awit/items/AWIT-0ND5723G.md .awit/comments/AWIT-0ND5723G
    git commit -m "tickets: close AWIT-0ND5723G"
    ```

## Acceptance Criteria

- `go test ./pkg/lock -count=1` passes. `TestAcquireRelease`, `TestSecondAcquireBlocksUntilRelease`, `TestAcquireTimeout` exist and PASS.
- `go test ./pkg/item -run 'TestLockIsGitignored|TestStoreLockTimeoutMessage' -v` - both PASS. After `Lock`, `.awit/.lock` exists and `.gitignore` contains exactly one `.awit/.lock` line. Timeout error string is `another awit process holds .awit/.lock (waited 50ms)`.
- `go test ./internal/cli -run TestCreateLockTimeout -v` - PASS. stderr is exactly `Error: another awit process holds .awit/.lock (waited 5s)\n`, exit 1.
- `go test ./internal/cli -run TestConcurrentCreates -v` - PASS. Ten goroutines each `awit create`; ten unique `items/*.md` files.
- `create.go`, `update.go`, `close.go`, `release.go`, `dep.go`, `comment.go` each contain `s.Lock(5 * time.Second)` after `openStore`. `next.go` contains it inside `if cmd.Bool("claim")`. `list` / `show` / `validate` / `prime` / `label` / `init` do not call `Lock`.
- `pkg/lock/lock_unix.go` starts with `//go:build unix` and uses `syscall.Flock(..., syscall.LOCK_EX|syscall.LOCK_NB)`. `pkg/lock/lock_windows.go` starts with `//go:build windows` and uses `windows.LockFileEx` with `LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY`. `go.mod` requires `golang.org/x/sys`. No other package imports `x/sys`.
- `gofmt -l pkg/lock pkg/item/store.go pkg/item/lock_test.go internal/cli` prints nothing.

## Out of scope

- Changing `Init` gitignore behaviour, `openStore`, or any command's business logic beyond inserting the lock.
- Cross-host or cross-worktree locking; NFS flock semantics; lock-file removal on release.
- Locking `validate --stale-claims` (`AWIT-0ND5733G`), `list`, `show`, `prime`, `label`.
- `goreleaser`, README, `external:` (`AWIT-0ND5743G`, `AWIT-0ND5753G`).
