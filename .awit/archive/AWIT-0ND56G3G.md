---
id: AWIT-0ND56G3G
title: awit init
brief: >-
  Add `awit init` (prefix validation, `item.Init`, the success line) plus `openStore` on the root command, and grow `internal/cli/helpers_test.go` into the shared command-test harness later tickets call.
status: closed
deps: [AWIT-0ND5683G, AWIT-0ND56E3G]
labels: [phase1, p0]
refs_base: repo
refs:
  - plan/implementation-guide.md
  - plan/awit-implementation-plan.md
---

## Summary

After this ticket `awit init` creates `.awit/` in `--repo` (absolute) or the
working directory, `openStore` is the lookup every later command uses, and
`internal/cli/helpers_test.go` exposes `run`, `runStdin`, `copyFixture`,
`repoRoot`, `golden`, `readItem` and `initRepo`. No other subcommand is added.

## Context (read first)

- **guide §1** — module `github.com/eisenwinter/awit`, urfave/cli v3, yaml.v3,
  `filepath` not `/`, errors to stderr as `Error: ` via `report` (do not print
  the prefix yourself), exit 0/1/2, `t.TempDir()`, commit per green step.
- **guide §2 `--repo`** — directory that *contains* `.awit/`. Without it, walk
  up from cwd; stop at the filesystem root with `item.ErrNotFound`
  (`no .awit directory found (run awit init)`).
- **guide §4.4** — `item.Init(repoRoot, prefix)`, `Open`, `Find`, `ErrExists`
  (`.awit already exists`), `ErrNotFound`, `DirName = ".awit"`. Init creates
  `.awit/`, `items/`, `comments/`, `config.yaml`, and appends `.awit/.lock` to
  `repoRoot/.gitignore` (create the file if missing; skip if the line exists).
- **guide §4.2** — `config.Default(prefix)` has `StaleClaim = 2h`; Init writes
  that config. Exact file bytes after default init:
  `prefix: AWIT\nstale_claim: 2h\n`.
- **guide §4.11** — `openStore(cmd *cli.Command) (*item.Store, error)` honours
  `--repo` (Open of the absolute path) else `Find(cwd)`. Used by every command
  **except** init.
- **guide §5** — command tests call `Main`; helpers live in
  `internal/cli/helpers_test.go`. Fixtures land in AWIT-0ND56N3G; this ticket
  still adds `copyFixture` (`os.CopyFS` from `testdata/fixtures/<name>`) but
  its own tests only use `t.TempDir()`.
- **AWIT-0ND5683G** already created `helpers_test.go` with `runMain`. Do **not**
  redeclare `runMain`; rewrite its body to call `run`. `newRoot` currently has
  `Commands: []*cli.Command{}` — put `initCmd` in that slice; do not invent a
  register/`init()` side channel.
- **spec CLI matrix** — `awit init` flag `--prefix`. Print exactly
  `Initialized .awit in <root> (prefix <PREFIX>)` plus a newline.
- Prefix flag default `AWIT`. Reject unless the whole value matches
  `^[A-Z][A-Z0-9]{1,7}$` (2–8 uppercase alphanumerics, first a letter). Error
  text (no `Error:` prefix in the returned error):
  `prefix must be 2-8 uppercase alphanumerics starting with a letter`.

## Files

- Create: `internal/cli/init.go` — `prefixRE`, `initCmd`, `initAction`.
- Create: `internal/cli/init_test.go`.
- Modify: `internal/cli/app.go` — add `openStore`; put `initCmd` in
  `newRoot`'s `Commands` slice.
- Modify: `internal/cli/helpers_test.go` — keep `runMain`; add `run`,
  `runStdin`, `copyFixture`, `repoRoot`, `golden`, `readItem`, `initRepo`,
  `var update`.

## Interfaces

Consumes (already in the tree when this ticket's deps are closed):

```go
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int
func newRoot(stdin io.Reader, stdout, stderr io.Writer) *cli.Command
func Init(repoRoot, prefix string) (*Store, error)          // package item
func Open(repoRoot string) (*Store, error)
func Find(start string) (*Store, error)
func Parse(path string, data []byte) (*Item, error)
var ErrExists error // ".awit already exists"
var ErrNotFound error // "no .awit directory found (run awit init)"
const DirName = ".awit"
```

Produces (guide §4.11, verbatim):

```go
func openStore(cmd *cli.Command) (*item.Store, error)
```

Produces (package-private):

```go
var prefixRE = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,7}$`)
var initCmd *cli.Command
func initAction(ctx context.Context, cmd *cli.Command) error
```

Produces (test helpers in `helpers_test.go`):

```go
var update = flag.Bool("update", false, "rewrite golden files")
func run(t *testing.T, args ...string) (code int, stdout, stderr string)
func runStdin(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string)
func runMain(t *testing.T, args ...string) (code int, stdout, stderr string) // keep; body calls run
func repoRoot(t *testing.T) string // runtime.Caller(0) on this file, then ../..
func copyFixture(t *testing.T, name string) string // os.CopyFS into t.TempDir(); return dest
func golden(t *testing.T, name string, got []byte) // testdata/golden/<name> under repoRoot
func readItem(t *testing.T, repo, id string) *item.Item
func initRepo(t *testing.T) string // t.TempDir + awit init --repo; return the dir
```

`loadGraph` / `toEntry` stay out. Do not wrap `item.Init`/`Open`/`Find` errors.

## Steps

- [ ] **Step 1: Expand the test harness.**

  Replace `internal/cli/helpers_test.go` with:

```go
package cli

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/item"
)

var update = flag.Bool("update", false, "rewrite golden files")

func runStdin(t *testing.T, stdin string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = Main(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	return runStdin(t, "", args...)
}

func runMain(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	return run(t, args...)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root, err := filepath.Abs(filepath.Join(filepath.Dir(file), "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func copyFixture(t *testing.T, name string) string {
	t.Helper()
	dst := t.TempDir()
	src := filepath.Join(repoRoot(t), "testdata", "fixtures", name)
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatalf("copyFixture %s: %v", name, err)
	}
	return dst
}

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join(repoRoot(t), "testdata", "golden", name)
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run: go test ./... -update)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("golden %s mismatch\ngot:\n%s\nwant:\n%s\nrun: go test ./... -update", name, got, want)
	}
}

func readItem(t *testing.T, repo, id string) *item.Item {
	t.Helper()
	path := filepath.Join(repo, item.DirName, "items", id+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	it, err := item.Parse(path, data)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return it
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	code, _, stderr := run(t, "--repo", dir, "init")
	if code != 0 {
		t.Fatalf("init: exit %d stderr %q", code, stderr)
	}
	return dir
}
```

  `copyFixture` is unused in this ticket on purpose (no fixtures yet). Do not
  delete it. `go test ./internal/cli -count=0` must still compile after this
  step; existing `runMain` callers keep working.

- [ ] **Step 2: Failing tests for `init` and `openStore`.**

  Create `internal/cli/init_test.go`:

```go
package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

func TestInitCreatesRepo(t *testing.T) {
	dir := t.TempDir()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "--repo", dir, "init")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	wantOut := "Initialized .awit in " + abs + " (prefix AWIT)\n"
	if stdout != wantOut {
		t.Fatalf("stdout = %q, want %q", stdout, wantOut)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, ".awit", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(cfg) != "prefix: AWIT\nstale_claim: 2h\n" {
		t.Fatalf("config.yaml = %q", cfg)
	}
	for _, name := range []string{"items", "comments"} {
		st, err := os.Stat(filepath.Join(dir, ".awit", name))
		if err != nil || !st.IsDir() {
			t.Fatalf("%s: %v", name, err)
		}
	}
	gi, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gi), ".awit/.lock") {
		t.Fatalf(".gitignore = %q, want a line containing .awit/.lock", gi)
	}
}

func TestInitCustomPrefix(t *testing.T) {
	dir := t.TempDir()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "--repo", dir, "init", "--prefix", "PROJ")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "Initialized .awit in "+abs+" (prefix PROJ)\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, ".awit", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(cfg), "prefix: PROJ\n") {
		t.Fatalf("config.yaml = %q", cfg)
	}
}

func TestInitBadPrefix(t *testing.T) {
	const msg = "prefix must be 2-8 uppercase alphanumerics starting with a letter"
	for _, prefix := range []string{"A", "ABCDEFGHI", "awit", "1AB", "AB-C", "Ab", ""} {
		t.Run(prefix, func(t *testing.T) {
			code, _, stderr := run(t, "--repo", t.TempDir(), "init", "--prefix", prefix)
			if code != 1 {
				t.Fatalf("exit %d, want 1 (stderr %q)", code, stderr)
			}
			if stderr != "Error: "+msg+"\n" {
				t.Fatalf("stderr = %q", stderr)
			}
		})
	}
}

func TestInitTwiceFails(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := run(t, "--repo", dir, "init")
	if code != 0 {
		t.Fatalf("first init: exit %d stderr %q", code, stderr)
	}
	code, _, stderr = run(t, "--repo", dir, "init")
	if code != 1 {
		t.Fatalf("second init: exit %d, want 1", code)
	}
	if stderr != "Error: .awit already exists\n" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestOpenStoreWithoutRepo(t *testing.T) {
	t.Chdir(t.TempDir())
	var storeErr error
	cmd := &cli.Command{
		Name: "awit",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "repo"},
		},
		Action: func(context.Context, *cli.Command) error { return nil },
	}
	cmd.Action = func(_ context.Context, c *cli.Command) error {
		_, storeErr = openStore(c)
		return nil
	}
	if err := cmd.Run(context.Background(), []string{"awit"}); err != nil {
		t.Fatal(err)
	}
	if storeErr == nil || !errors.Is(storeErr, item.ErrNotFound) {
		t.Fatalf("openStore error = %v, want item.ErrNotFound", storeErr)
	}
}
```

- [ ] **Step 3: Run it, see it fail.**

  ```bash
  go test ./internal/cli -run 'TestInit|TestOpenStoreWithoutRepo' -v
  ```

  Expected: `TestInitCreatesRepo` (and the other `TestInit*` tests) fail because
  `init` is not a command — exit `2`, stderr
  `unknown command "init" (run "awit --help")`. `TestOpenStoreWithoutRepo`
  fails to compile: `undefined: openStore`. Overall
  `FAIL github.com/eisenwinter/awit/internal/cli [build failed]` **or** the
  init tests fail at runtime if you comment `TestOpenStoreWithoutRepo` out
  while iterating; leave it in. The build-failed form is the one you want
  before Step 4.

- [ ] **Step 4: Implement `init` and `openStore`.**

  Create `internal/cli/init.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var prefixRE = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,7}$`)

var initCmd = &cli.Command{
	Name:  "init",
	Usage: "Create a .awit directory",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "prefix",
			Value: "AWIT",
			Usage: "item id prefix (2-8 uppercase alphanumerics starting with a letter)",
		},
	},
	Action: initAction,
}

func initAction(_ context.Context, cmd *cli.Command) error {
	prefix := cmd.String("prefix")
	if !prefixRE.MatchString(prefix) {
		return fmt.Errorf("prefix must be 2-8 uppercase alphanumerics starting with a letter")
	}
	root := cmd.Root().String("repo")
	var err error
	if root == "" {
		root, err = os.Getwd()
	} else {
		root, err = filepath.Abs(root)
	}
	if err != nil {
		return err
	}
	if _, err := item.Init(root, prefix); err != nil {
		return err
	}
	fmt.Fprintf(cmd.Writer, "Initialized .awit in %s (prefix %s)\n", root, prefix)
	return nil
}
```

  Add `openStore` to `internal/cli/app.go` (same file that already has
  `newRoot` / `Main`). Imports to add: `"os"`, `"path/filepath"`,
  `"github.com/eisenwinter/awit/pkg/item"`.

```go
// openStore honours --repo (Open of the absolute path) else Find(cwd).
// Used by every command except init.
func openStore(cmd *cli.Command) (*item.Store, error) {
	if repo := cmd.Root().String("repo"); repo != "" {
		abs, err := filepath.Abs(repo)
		if err != nil {
			return nil, err
		}
		return item.Open(abs)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return item.Find(cwd)
}
```

  In `newRoot`, change the empty Commands slice to:

```go
		Commands: []*cli.Command{
			initCmd,
		},
```

  Read `--repo` with `cmd.Root().String("repo")` because the flag is declared
  on the root (AWIT-0ND5683G). Init must not call `openStore`.

- [ ] **Step 5: Run it, see it pass, commit.**

  ```bash
  go test ./internal/cli -run 'TestInit|TestOpenStoreWithoutRepo' -v
  ```

  Expected: `--- PASS: TestInitCreatesRepo`, `TestInitCustomPrefix`,
  `TestInitBadPrefix` (every subtest), `TestInitTwiceFails`,
  `TestOpenStoreWithoutRepo`, then `ok`. Existing `TestMain*` still pass:

  ```bash
  go test ./internal/cli -count=1
  gofmt -w internal/cli
  git add internal/cli/init.go internal/cli/init_test.go internal/cli/app.go internal/cli/helpers_test.go
  git commit -m "cli/init: add init, openStore and test helpers"
  ```

- [ ] **Step 6: Full check and close.**

  ```bash
  go build ./... && go vet ./... && go test ./internal/cli -count=1
  ```

  Set `status: closed` on this file, write
  `.awit/comments/AWIT-0ND56G3G/<YYYYMMDDTHHMMSSZ>-<author>.md` with the
  acceptance output, append that ref, commit `tickets: close AWIT-0ND56G3G`.

## Acceptance Criteria

- `go test ./internal/cli -run 'TestInit|TestOpenStoreWithoutRepo' -v` — all
  five tests PASS, including every `TestInitBadPrefix` subtest.
- `go test ./internal/cli -count=1` — existing skeleton tests still PASS.
- `go build -o /tmp/awit ./cmd/awit && /tmp/awit init --repo "$TMP"` in an
  empty temp dir prints `Initialized .awit in <abs> (prefix AWIT)` and writes
  `config.yaml` whose bytes are exactly `prefix: AWIT\nstale_claim: 2h\n`,
  directories `.awit/items` and `.awit/comments`, and a `.gitignore` containing
  `.awit/.lock`.
- `/tmp/awit init --repo "$TMP" --prefix a1` (same dir, new prefix attempt
  after a successful init, or a fresh dir) with prefix `a1` exits 1,
  stderr `Error: prefix must be 2-8 uppercase alphanumerics starting with a letter`.
- Second `init --repo "$TMP"` on an already-initialised dir exits 1,
  stderr `Error: .awit already exists`.
- `gofmt -l internal/cli` prints nothing.

## Out of scope

- `create`, `update`, `close`, `release`, `list`, `loadGraph`, `toEntry`.
- Calling `copyFixture` or writing anything under `testdata/`.
- Git commits, locking, colour, wrapping `item.Init` errors with extra text.
- Changing `item.Init` itself — if config bytes differ, the bug is in
  AWIT-0ND56E3G / AWIT-0ND56A3G, not here.
