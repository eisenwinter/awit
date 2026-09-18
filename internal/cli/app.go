// Package cli implements the awit command line interface. Every command lives
// in its own file in this package; cmd/awit/main.go only calls Main.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

// Version is set via -ldflags "-X github.com/eisenwinter/awit/internal/cli.Version=v1.2.3".
var Version = "dev"

// mainMu serializes Main: newRoot reuses the package-level subcommand tree
// (createCmd, ...) and urfave/cli v3 Run mutates it (flag parse state,
// setupDefaults), so concurrent Main calls race. Production makes one call
// per process; the mutex only matters to in-process concurrent test drivers.
// Cross-process exclusion is the .awit/.lock file lock, not this mutex.
var mainMu sync.Mutex

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
	mainMu.Lock()
	defer mainMu.Unlock()
	root := newRoot(stdin, stdout, stderr)
	setUsageHandler(root, usageError)
	return report(stderr, root.Run(context.Background(), append([]string{"awit"}, args...)))
}

// newRoot builds the root command. Later work items register their commands by
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
		Commands: []*cli.Command{
			initCmd,
			createCmd,
			updateCmd,
			closeCmd,
			releaseCmd,
			archiveCmd,
			validateCmd,
			listCmd,
			nextCmd,
			primeCmd,
			showCmd,
			depCmd,
			commentCmd,
			labelCmd,
		},
	}
}

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

func toEntry(n *graph.Node) format.Entry {
	e := format.Entry{
		ID:       n.Item.ID,
		Title:    n.Item.Title,
		Brief:    n.Item.Brief,
		Status:   string(n.Item.Status),
		Labels:   n.Item.Labels,
		Deps:     n.Item.Deps,
		Assignee: n.Item.Assignee,
		Unblocks: n.UnblockCount,
	}
	if e.Labels == nil {
		e.Labels = []string{}
	}
	if e.Deps == nil {
		e.Deps = []string{}
	}
	switch {
	case n.Quarantined():
		e.State = "quarantined"
		for _, f := range n.Faults {
			e.Faults = append(e.Faults, "["+string(f.Reason)+"] "+f.Detail)
		}
	case n.Item.Status == item.StatusClosed:
		e.State = "closed"
	case n.Ready:
		e.State = "ready"
	default:
		e.State = "blocked"
	}
	return e
}
