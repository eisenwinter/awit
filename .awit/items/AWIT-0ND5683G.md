---
id: AWIT-0ND5683G
title: CLI skeleton with urfave/cli v3 and global flags
brief: >-
  Stand up the awit binary: module dependencies, a three-line main, and
  internal/cli.Main with the root urfave/cli v3 command, global flags,
  version/help output, error-to-exit-code mapping and SplitLabels.
status: open
deps: []
labels: [phase0, p0]
refs:
  - ../../plan/implementation-guide.md
  - ../../plan/awit-implementation-plan.md
---

## Summary

After this ticket `go build ./...` produces a working `awit` binary that answers
`--version`, `--help`, and accepts the three global flags `--format`, `--repo`,
`--no-color`. `internal/cli.Main` is the single entry point used by both
`cmd/awit/main.go` and every future command test: it builds the root
`*cli.Command`, runs it, and turns the returned error into an exit code
(`0` success, `1` expected non-success, `2` usage error). `SplitLabels` is
implemented because later commands parse repeated `-l` flags with it.

No subcommands exist yet — `awit init`, `awit list`, … are added by later
tickets, which only append to the root command's `Commands` slice. No store,
no graph, no YAML reading happens here.

## Context (read first)

Read these before writing code:

- **guide §1 Global constraints** — module path `github.com/eisenwinter/awit`,
  Go 1.27.1, only `github.com/urfave/cli/v3` and `gopkg.in/yaml.v3` as
  dependencies, `go vet` clean on Linux and Windows, commit after every green
  step with `<scope>: <imperative summary>`.
- **guide §1 exit codes** — `0` success, `1` expected non-success, `2` usage
  error. Errors go to stderr prefixed `Error: `.
- **guide §2, row "CLI package layout"** — commands live in `internal/cli/`,
  one file per command; `cmd/awit/main.go` is three lines.
- **guide §2, row 1 "Multiple `-l` flags"** — AND across flags, OR within a
  flag; parsed into `[][]string` by `cli.SplitLabels`.
- **guide §3 Repository layout** — `internal/cli/app.go` holds the root
  command, global flags, `Main()` and the helpers.
- **guide §4.11 `internal/cli`** — the exact signatures of `Main`, `Version`
  and `SplitLabels`. Copy them verbatim; do not rename.
- **guide §5 Testing conventions** — stdlib `testing` only, table-driven,
  command tests call `Main([]string{...}, strings.NewReader(""), &out, &errb)`,
  helpers live in `internal/cli/helpers_test.go`.
- **spec "CLI command matrix"** (`plan/awit-implementation-plan.md`) — the
  thirteen commands that will hang off this root, and the line
  "Global flags: `--format`, `--repo <path>`, `--no-color`".
- **guide §2, row "`--repo` semantics"** — `--repo` is the directory that
  *contains* `.awit/`. This ticket only declares the flag; resolution belongs
  to `openStore` in AWIT-0ND56G3G.
- **guide §1, `--no-color`** — accepted and a deliberate no-op so scripts
  written today keep working. Do not implement colour.

Facts about urfave/cli v3 that this ticket depends on (verified against the
library source — do not re-derive them, just use them):

1. `func (cmd *Command) Run(ctx context.Context, osArgs []string) error`.
   `osArgs[0]` is the program name, so pass
   `append([]string{"awit"}, args...)`.
2. `Command.Writer`, `Command.ErrWriter` and `Command.Reader` are inherited by
   subcommands when the subcommand leaves them nil. Setting them on the root
   is therefore enough for every future command, and `cmd.Reader` is how
   `awit comment` will read stdin (AWIT-0ND56Y3G).
3. Setting `Command.Version` on the root makes urfave add the built-in
   `--version` / `-v` flag. When it is set, `Run` prints the version through
   the package-level hook `cli.VersionPrinter` and returns `nil` (exit 0).
   The default printer writes `"<name> version <version>"`; awit overrides it
   to print `awit <version>`.
4. Help (`--help` / `-h`) is handled by urfave before the action runs and also
   returns `nil`. The help text is written to `cmd.Root().Writer` and contains
   the headings `USAGE:`, `COMMANDS:` and `GLOBAL OPTIONS:`.
5. Flags declared on the root are **persistent by default** in v3 (the opt-out
   is `Local: true`), so `awit list --repo /x` parses even though `--repo` is
   declared on the root. Read them with `cmd.Root().String("repo")`.
6. If `Command.ExitErrHandler` is nil, urfave calls `cli.HandleExitCoder`,
   which prints to the *package-level* `cli.ErrWriter` (`os.Stderr`, not our
   stream) and then calls `os.Exit`. That would kill the test process, so the
   root **must** set `ExitErrHandler` to a no-op and let `Main` do the
   reporting. `handleExitCoder` always delegates to the root command, so one
   no-op on the root covers every future subcommand.
7. `cli.ExitCoder` is `interface { error; ExitCode() int }`; `cli.Exit(msg, n)`
   returns one, and its `Error()` is exactly `msg`.
8. `Command.OnUsageError` is consulted on the command whose flags failed to
   parse and is *not* inherited, so `Main` installs the same handler on the
   root and, recursively, on every registered subcommand.

**Naming note, read twice:** the package is `package cli` *and* it imports
`github.com/urfave/cli/v3` as `cli`. This is legal Go — a package never refers
to itself by name, so the identifier `cli` is free inside `internal/cli`.
Do **not** alias the urfave import, because later tickets copy signatures such
as `func openStore(cmd *cli.Command) (*item.Store, error)` from guide §4.11
verbatim. `cmd/awit/main.go` imports `github.com/eisenwinter/awit/internal/cli`
with no alias either; it does not import urfave at all.

## Files

- Create: `cmd/awit/main.go` — three-line entry point.
- Create: `internal/cli/app.go` — `Version`, `Main`, `newRoot`, `rootAction`,
  `report`, `setUsageHandler`, `SplitLabels`.
- Test: `internal/cli/app_test.go` — `TestReport`, `TestMainVersion`,
  `TestMainHelp`, `TestMainUnknownCommand`, `TestMainUsageError`,
  `TestMainNoArgsPrintsHelp`, `TestSplitLabels`.
- Test: `internal/cli/helpers_test.go` — `runMain` helper only. Later tickets
  add `copyFixture` to this same file; they must not redeclare `runMain`.
- Modify: `go.mod` / `go.sum` — add the two dependencies.

No fixtures and no golden files in this ticket.

## Interfaces

Consumes: nothing. This is the first ticket.

Produces (from guide §4.11, verbatim):

```go
// Main runs the CLI with the given args (excluding program name) and streams; returns the exit code.
// Tests call Main directly; cmd/awit/main.go calls os.Exit(cli.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)).
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int

// Version is set via -ldflags "-X github.com/eisenwinter/awit/internal/cli.Version=v1.2.3"; default "dev".
var Version = "dev"

// SplitLabels turns repeated -l values into groups: ["p0,p1","auth"] → [["p0","p1"],["auth"]]. Trims spaces, drops empties.
func SplitLabels(flags []string) [][]string
```

Package-private helpers introduced here (later tickets may use them):

```go
// newRoot builds the root command with the global flags and the given streams.
func newRoot(stdin io.Reader, stdout, stderr io.Writer) *cli.Command

// rootAction runs when no subcommand matched: bare "awit" prints help,
// "awit nope" is a usage error with exit code 2.
func rootAction(ctx context.Context, cmd *cli.Command) error

// report writes err to w using the awit convention and returns the exit code.
// nil → 0 and no output; cli.ExitCoder → its message (no prefix) and its code;
// anything else → "Error: <err>" and 1.
func report(w io.Writer, err error) int

// setUsageHandler installs h on cmd and, recursively, on every subcommand.
func setUsageHandler(cmd *cli.Command, h cli.OnUsageErrorFunc)

// printVersion is the cli.VersionPrinter override: "awit <Version>".
func printVersion(cmd *cli.Command)
```

Not in this ticket: `openStore`, `loadGraph` and `toEntry` from guide §4.11.
`openStore` and `loadGraph` arrive with `awit init` (AWIT-0ND56G3G) and
`awit list` (AWIT-0ND56J3G) because they need `pkg/item` and `pkg/graph`, which
do not exist yet. Do **not** write placeholder versions of them.

## Steps

- [ ] **Step 1: Add the two module dependencies.**

  ```bash
  go get github.com/urfave/cli/v3@latest
  go get gopkg.in/yaml.v3@latest
  ```

  Confirm the result:

  ```bash
  cat go.mod
  ```

  Expected: the `module github.com/eisenwinter/awit` and `go 1.27.1` lines are
  unchanged and a `require` block now names `github.com/urfave/cli/v3` and
  `gopkg.in/yaml.v3` with concrete versions. `gopkg.in/yaml.v3` is not imported
  by any file until `pkg/config` (AWIT-0ND56A3G) lands, so **do not run
  `go mod tidy` in this ticket** — it would drop the requirement again.

  ```bash
  git add go.mod go.sum
  git commit -m "cli: add urfave/cli v3 and yaml.v3 dependencies"
  ```

- [ ] **Step 2: Failing test for the error-to-exit-code mapping.**

  Create `internal/cli/app_test.go`:

  ```go
  package cli

  import (
  	"bytes"
  	"errors"
  	"reflect"
  	"strings"
  	"testing"

  	"github.com/urfave/cli/v3"
  )

  func TestReport(t *testing.T) {
  	tests := []struct {
  		name     string
  		err      error
  		wantCode int
  		wantOut  string
  	}{
  		{name: "nil is success", err: nil, wantCode: 0, wantOut: ""},
  		{
  			name:     "plain error gets the Error prefix",
  			err:      errors.New("boom"),
  			wantCode: 1,
  			wantOut:  "Error: boom\n",
  		},
  		{
  			name:     "wrapped error is unwrapped by the formatter",
  			err:      fmt.Errorf("load: %w", errors.New("boom")),
  			wantCode: 1,
  			wantOut:  "Error: load: boom\n",
  		},
  		{
  			name:     "exit coder keeps its code and prints bare",
  			err:      cli.Exit("No ready items", 1),
  			wantCode: 1,
  			wantOut:  "No ready items\n",
  		},
  		{
  			name:     "usage exit coder returns 2",
  			err:      cli.Exit(`unknown command "nope"`, 2),
  			wantCode: 2,
  			wantOut:  "unknown command \"nope\"\n",
  		},
  		{
  			name:     "empty exit coder message prints nothing",
  			err:      cli.Exit("", 3),
  			wantCode: 3,
  			wantOut:  "",
  		},
  	}

  	for _, tt := range tests {
  		t.Run(tt.name, func(t *testing.T) {
  			var buf bytes.Buffer
  			got := report(&buf, tt.err)
  			if got != tt.wantCode {
  				t.Errorf("report code = %d, want %d", got, tt.wantCode)
  			}
  			if buf.String() != tt.wantOut {
  				t.Errorf("report output = %q, want %q", buf.String(), tt.wantOut)
  			}
  		})
  	}
  }
  ```

  Add `"fmt"` to the import block (the wrapped-error row uses it).

- [ ] **Step 3: Run it, see it fail.**

  ```bash
  go test ./internal/cli -run TestReport -v
  ```

  Expected failure: the package has no non-test file yet, so the build fails
  with `internal/cli/app_test.go:NN:11: undefined: report` and
  `FAIL github.com/eisenwinter/awit/internal/cli [build failed]`.

- [ ] **Step 4: Implement `Version` and `report`.**

  Create `internal/cli/app.go`:

  ```go
  // Package cli implements the awit command line interface. Every command lives
  // in its own file in this package; cmd/awit/main.go only calls Main.
  package cli

  import (
  	"context"
  	"errors"
  	"fmt"
  	"io"
  	"strings"

  	"github.com/urfave/cli/v3"
  )

  // Version is set via -ldflags "-X github.com/eisenwinter/awit/internal/cli.Version=v1.2.3".
  var Version = "dev"

  // report writes err to w using the awit convention and returns the process
  // exit code. A cli.ExitCoder carries its own code and its message is printed
  // as-is (so "awit next" can exit 1 with a bare "No ready items"); every other
  // error is an unexpected failure and gets the "Error: " prefix and code 1.
  func report(w io.Writer, err error) int {
  	if err == nil {
  		return 0
  	}
  	var coder cli.ExitCoder
  	if errors.As(err, &coder) {
  		if msg := err.Error(); msg != "" {
  			fmt.Fprintln(w, msg)
  		}
  		return coder.ExitCode()
  	}
  	fmt.Fprintf(w, "Error: %v\n", err)
  	return 1
  }
  ```

- [ ] **Step 5: Run it, see it pass, commit.**

  ```bash
  go test ./internal/cli -run TestReport -v
  ```

  Expected: six `--- PASS` lines and `ok github.com/eisenwinter/awit/internal/cli`.

  ```bash
  gofmt -w internal/cli/app.go internal/cli/app_test.go
  git add internal/cli/app.go internal/cli/app_test.go
  git commit -m "cli: map errors and exit coders to exit codes"
  ```

- [ ] **Step 6: Failing tests for the root command and `Main`.**

  Create `internal/cli/helpers_test.go` — this file is shared with later
  tickets, so it contains only the helper:

  ```go
  package cli

  import (
  	"bytes"
  	"strings"
  	"testing"
  )

  // runMain drives the CLI exactly like cmd/awit/main.go does, with empty stdin
  // and buffered streams. Tests in this package never call t.Parallel(): Main
  // installs the package-level cli.VersionPrinter hook.
  func runMain(t *testing.T, args ...string) (code int, stdout, stderr string) {
  	t.Helper()
  	var out, errb bytes.Buffer
  	code = Main(args, strings.NewReader(""), &out, &errb)
  	return code, out.String(), errb.String()
  }
  ```

  Append to `internal/cli/app_test.go`:

  ```go
  func TestMainVersion(t *testing.T) {
  	for _, arg := range []string{"--version", "-v"} {
  		code, stdout, stderr := runMain(t, arg)
  		if code != 0 {
  			t.Fatalf("%s exit = %d, want 0 (stderr %q)", arg, code, stderr)
  		}
  		if !strings.Contains(stdout, "awit dev") {
  			t.Errorf("%s stdout = %q, want it to contain %q", arg, stdout, "awit dev")
  		}
  		if stderr != "" {
  			t.Errorf("%s stderr = %q, want empty", arg, stderr)
  		}
  	}
  }

  func TestMainHelp(t *testing.T) {
  	code, stdout, stderr := runMain(t, "--help")
  	if code != 0 {
  		t.Fatalf("--help exit = %d, want 0 (stderr %q)", code, stderr)
  	}
  	for _, want := range []string{"USAGE", "awit", "--format", "--repo", "--no-color"} {
  		if !strings.Contains(stdout, want) {
  			t.Errorf("--help stdout is missing %q; got:\n%s", want, stdout)
  		}
  	}
  }

  func TestMainNoArgsPrintsHelp(t *testing.T) {
  	code, stdout, stderr := runMain(t)
  	if code != 0 {
  		t.Fatalf("bare awit exit = %d, want 0 (stderr %q)", code, stderr)
  	}
  	if !strings.Contains(stdout, "USAGE") {
  		t.Errorf("bare awit stdout = %q, want it to contain USAGE", stdout)
  	}
  }

  func TestMainUnknownCommand(t *testing.T) {
  	code, _, stderr := runMain(t, "nope")
  	if code != 2 {
  		t.Fatalf("unknown command exit = %d, want 2 (stderr %q)", code, stderr)
  	}
  	if len(stderr) == 0 {
  		t.Fatal("unknown command wrote nothing to stderr")
  	}
  	if !strings.Contains(stderr, "nope") {
  		t.Errorf("stderr = %q, want it to name the unknown command", stderr)
  	}
  }

  func TestMainUsageError(t *testing.T) {
  	code, _, stderr := runMain(t, "--bogus")
  	if code != 2 {
  		t.Fatalf("bad flag exit = %d, want 2 (stderr %q)", code, stderr)
  	}
  	if !strings.Contains(stderr, "bogus") {
  		t.Errorf("stderr = %q, want it to name the offending flag", stderr)
  	}
  }

  func TestMainGlobalFlagsAreAccepted(t *testing.T) {
  	code, stdout, stderr := runMain(t, "--no-color", "--format", "json", "--repo", ".", "--help")
  	if code != 0 {
  		t.Fatalf("global flags exit = %d, want 0 (stderr %q)", code, stderr)
  	}
  	if !strings.Contains(stdout, "USAGE") {
  		t.Errorf("stdout = %q, want help output", stdout)
  	}
  }
  ```

- [ ] **Step 7: Run them, see them fail.**

  ```bash
  go test ./internal/cli -run 'TestMain' -v
  ```

  Expected failure: `internal/cli/helpers_test.go:NN:9: undefined: Main` and
  `FAIL github.com/eisenwinter/awit/internal/cli [build failed]`.

- [ ] **Step 8: Implement the root command and `Main`.**

  Add to `internal/cli/app.go`:

  ```go
  func init() {
  	// urfave's default printer writes "awit version dev"; awit writes "awit dev".
  	cli.VersionPrinter = printVersion
  }

  // printVersion is the cli.VersionPrinter override. It reads Version at call
  // time so an -ldflags override is honoured.
  func printVersion(cmd *cli.Command) {
  	fmt.Fprintf(cmd.Root().Writer, "awit %s\n", Version)
  }

  // Main runs the CLI with the given args (program name excluded) and streams,
  // and returns the exit code: 0 success, 1 expected non-success, 2 usage error.
  func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
  	root := newRoot(stdin, stdout, stderr)
  	setUsageHandler(root, usageError)
  	return report(stderr, root.Run(context.Background(), append([]string{"awit"}, args...)))
  }

  // newRoot builds the root command. Later tickets register their commands by
  // appending to the Commands slice below; nothing else in this function moves.
  func newRoot(stdin io.Reader, stdout, stderr io.Writer) *cli.Command {
  	return &cli.Command{
  		Name:      "awit",
  		Usage:     "Turn Markdown files under .awit/ into a dependency graph",
  		ArgsUsage: "<command> [arguments]",
  		Version:   Version,
  		// Subcommands inherit these three when they leave them nil, so every
  		// command writes to the streams the caller passed in. Reader is what
  		// "awit comment" reads its body from.
  		Reader:    stdin,
  		Writer:    stdout,
  		ErrWriter: stderr,
  		Flags: []cli.Flag{
  			&cli.StringFlag{
  				Name:  "format",
  				Usage: "output format: `compact`, table or json (default: table on a terminal)",
  			},
  			&cli.StringFlag{
  				Name:  "repo",
  				Usage: "`DIR` containing .awit (default: walk up from the working directory)",
  			},
  			&cli.BoolFlag{
  				Name:  "no-color",
  				Usage: "accepted and ignored; awit output is never coloured",
  			},
  		},
  		// Without this, urfave calls cli.HandleExitCoder, which writes to the
  		// package-level cli.ErrWriter and calls os.Exit - fatal in tests.
  		// handleExitCoder always delegates to the root, so this one no-op
  		// covers every subcommand too. Main does the reporting instead.
  		ExitErrHandler: func(context.Context, *cli.Command, error) {},
  		Action:         rootAction,
  		Commands:       []*cli.Command{},
  	}
  }

  // rootAction runs when the first argument did not name a command.
  func rootAction(ctx context.Context, cmd *cli.Command) error {
  	if name := cmd.Args().First(); name != "" {
  		return cli.Exit(fmt.Sprintf("unknown command %q (run \"awit --help\")", name), 2)
  	}
  	return cli.ShowRootCommandHelp(cmd)
  }

  // usageError turns a flag-parsing failure into exit code 2. urfave only
  // consults OnUsageError on the command that failed to parse and never
  // inherits it, which is why setUsageHandler walks the tree.
  func usageError(_ context.Context, _ *cli.Command, err error, _ bool) error {
  	return cli.Exit(fmt.Sprintf("Incorrect usage: %v (run \"awit --help\")", err), 2)
  }

  // setUsageHandler installs h on cmd and on every subcommand, recursively.
  func setUsageHandler(cmd *cli.Command, h cli.OnUsageErrorFunc) {
  	cmd.OnUsageError = h
  	for _, sub := range cmd.Commands {
  		setUsageHandler(sub, h)
  	}
  }
  ```

- [ ] **Step 9: Run them, see them pass, commit.**

  ```bash
  go test ./internal/cli -run 'TestMain' -v
  ```

  Expected: `--- PASS` for `TestMainVersion`, `TestMainHelp`,
  `TestMainNoArgsPrintsHelp`, `TestMainUnknownCommand`, `TestMainUsageError`,
  `TestMainGlobalFlagsAreAccepted`, then `ok`.

  If `TestMainVersion` reports `stdout = "awit version dev\n"`, the `init()`
  hook is missing — `cli.VersionPrinter` must be assigned.

  ```bash
  gofmt -w internal/cli/app.go internal/cli/app_test.go internal/cli/helpers_test.go
  git add internal/cli/app.go internal/cli/app_test.go internal/cli/helpers_test.go
  git commit -m "cli: add root command, global flags and Main"
  ```

- [ ] **Step 10: Failing test for `SplitLabels`.**

  Append to `internal/cli/app_test.go`:

  ```go
  func TestSplitLabels(t *testing.T) {
  	tests := []struct {
  		name  string
  		flags []string
  		want  [][]string
  	}{
  		{
  			name:  "one group per flag, comma splits within a flag",
  			flags: []string{"p0,p1", "auth"},
  			want:  [][]string{{"p0", "p1"}, {"auth"}},
  		},
  		{
  			name:  "spaces trimmed and trailing empty dropped",
  			flags: []string{" p0 , "},
  			want:  [][]string{{"p0"}},
  		},
  		{
  			name:  "empty flag produces no group",
  			flags: []string{""},
  			want:  nil,
  		},
  		{
  			name:  "no flags",
  			flags: nil,
  			want:  nil,
  		},
  		{
  			name:  "double comma collapses",
  			flags: []string{"a,,b"},
  			want:  [][]string{{"a", "b"}},
  		},
  		{
  			name:  "three flags stay three groups",
  			flags: []string{"p0", "auth,api", "db"},
  			want:  [][]string{{"p0"}, {"auth", "api"}, {"db"}},
  		},
  	}

  	for _, tt := range tests {
  		t.Run(tt.name, func(t *testing.T) {
  			got := SplitLabels(tt.flags)
  			if !reflect.DeepEqual(got, tt.want) {
  				t.Errorf("SplitLabels(%q) = %#v, want %#v", tt.flags, got, tt.want)
  			}
  		})
  	}
  }
  ```

- [ ] **Step 11: Run it, see it fail.**

  ```bash
  go test ./internal/cli -run TestSplitLabels -v
  ```

  Expected failure: `internal/cli/app_test.go:NN:10: undefined: SplitLabels`
  and `FAIL github.com/eisenwinter/awit/internal/cli [build failed]`.

- [ ] **Step 12: Implement `SplitLabels`.**

  Append to `internal/cli/app.go`:

  ```go
  // SplitLabels turns repeated -l values into label groups. Groups are ANDed and
  // the labels inside a group are ORed (guide §2, decision 1), so
  // ["p0,p1", "auth"] means (p0 OR p1) AND auth. Whitespace is trimmed and empty
  // entries are dropped; a flag that contributes no label contributes no group,
  // and no input at all returns nil so graph.FilterLabels treats it as "no filter".
  func SplitLabels(flags []string) [][]string {
  	var groups [][]string
  	for _, flag := range flags {
  		var group []string
  		for _, part := range strings.Split(flag, ",") {
  			if label := strings.TrimSpace(part); label != "" {
  				group = append(group, label)
  			}
  		}
  		if len(group) > 0 {
  			groups = append(groups, group)
  		}
  	}
  	return groups
  }
  ```

- [ ] **Step 13: Run it, see it pass, commit.**

  ```bash
  go test ./internal/cli -run TestSplitLabels -v
  ```

  Expected: six `--- PASS` subtests, then `ok`.

  ```bash
  gofmt -w internal/cli/app.go internal/cli/app_test.go
  git add internal/cli/app.go internal/cli/app_test.go
  git commit -m "cli: parse repeated label flags into AND/OR groups"
  ```

- [ ] **Step 14: Wire up the binary.**

  Create `cmd/awit/main.go`:

  ```go
  // Command awit turns Markdown files under .awit/ into a dependency graph.
  package main

  import (
  	"os"

  	"github.com/eisenwinter/awit/internal/cli"
  )

  func main() {
  	os.Exit(cli.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
  }
  ```

  Nothing else belongs in this file — no flag parsing, no error handling.

- [ ] **Step 15: Smoke-test the real binary.**

  ```bash
  go build -o awit-smoke ./cmd/awit
  ./awit-smoke --version
  ./awit-smoke nope; echo "exit=$?"
  ./awit-smoke --version -X ldflags check
  ```

  Expected for the first two:

  ```text
  awit dev
  unknown command "nope" (run "awit --help")
  exit=2
  ```

  Then check the version can be stamped at link time:

  ```bash
  go build -ldflags "-X github.com/eisenwinter/awit/internal/cli.Version=v9.9.9" -o awit-smoke ./cmd/awit
  ./awit-smoke --version
  ```

  Expected: `awit v9.9.9`.

  Remove the throwaway binary and commit:

  ```bash
  rm -f awit-smoke
  gofmt -w cmd/awit/main.go
  git add cmd/awit/main.go
  git commit -m "cli: add cmd/awit entry point"
  ```

  If `awit-smoke` was created inside the repo, make sure it is gone before
  committing; do not add it to `.gitignore` in this ticket.

- [ ] **Step 16: Full check for the packages touched.**

  ```bash
  go build ./...
  go vet ./...
  go test ./internal/cli/... -v
  ```

  Expected: no output from `build` and `vet`; every test above passes.

- [ ] **Step 17: Close ticket.**

  1. Create the comment directory and file
     `.awit/comments/AWIT-0ND5683G/<YYYYMMDDTHHMMSSZ>-<author>.md` (UTC stamp,
     e.g. `20260918T090000Z-claude.md`) with this shape:

     ```markdown
     ---
     author: <author>
     created: <YYYY-MM-DDTHH:MM:SSZ>
     ---

     Acceptance output:

     ```text
     $ ./awit --version
     awit dev
     $ ./awit nope; echo "exit=$?"
     unknown command "nope" (run "awit --help")
     exit=2
     $ go test ./internal/cli/...
     ok  	github.com/eisenwinter/awit/internal/cli
     ```
     ```

     Paste the real output you observed, not this sample.
  2. In `.awit/items/AWIT-0ND5683G.md` set `status: closed` and append
     `- ../comments/AWIT-0ND5683G/<file>.md` to `refs`.
  3. Commit:

     ```bash
     git add .awit/items/AWIT-0ND5683G.md .awit/comments/AWIT-0ND5683G
     git commit -m "tickets: close AWIT-0ND5683G"
     ```

## Acceptance Criteria

With `go build -o awit ./cmd/awit` in the repository root:

- `go build ./...` — no output, exit 0.
- `go vet ./...` — no output, exit 0.
- `./awit --version` — prints exactly `awit dev`, exit 0.
- `./awit -v` — same output as `--version`, exit 0.
- `./awit --help` — exit 0; output contains `USAGE`, `--format`, `--repo`,
  `--no-color`.
- `./awit` with no arguments — exit 0, prints the same help text.
- `./awit nope` — stderr is exactly
  `unknown command "nope" (run "awit --help")`, exit 2.
- `./awit --bogus` — stderr starts with `Incorrect usage:` and names `bogus`,
  exit 2.
- `go build -ldflags "-X github.com/eisenwinter/awit/internal/cli.Version=v9.9.9" -o awit ./cmd/awit && ./awit --version`
  — prints `awit v9.9.9`.
- `go test ./internal/cli/... -v` — `TestReport`, `TestMainVersion`,
  `TestMainHelp`, `TestMainNoArgsPrintsHelp`, `TestMainUnknownCommand`,
  `TestMainUsageError`, `TestMainGlobalFlagsAreAccepted` and `TestSplitLabels`
  all PASS.
- `git status --short` — clean; no stray `awit` or `awit-smoke` binary.

## Out of scope

- Any subcommand. `init` is AWIT-0ND56G3G, `create` is AWIT-0ND56H3G, `list` is
  AWIT-0ND56J3G, and so on. Leave `Commands: []*cli.Command{}` empty.
- `openStore`, `loadGraph` and `toEntry` (guide §4.11). They need `pkg/item`
  and `pkg/graph`; adding stubs now would have to be deleted later.
- Resolving `--repo` or walking up to find `.awit/` — AWIT-0ND56G3G.
- Interpreting `--format` or TTY detection — `pkg/format` in AWIT-0ND56F3G.
- Any colour output at all; `--no-color` stays a no-op (guide §1).
- `pkg/id` (AWIT-0ND5693G) and `pkg/config` (AWIT-0ND56A3G) — this ticket only
  adds their shared dependencies to `go.mod`.
- The CI workflow — AWIT-0ND56B3G.
- `go mod tidy`: it would drop the still-unused `gopkg.in/yaml.v3` requirement.
