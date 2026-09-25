// Package cli implements the awit command line interface. Every command lives
// in its own file in this package; cmd/awit/main.go only calls Main.
// Command actions delegate store-level work to internal/ops.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/eisenwinter/awit/internal/ops"
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

// typicalSessionBlock teaches the agent loop in the top-level --help, in
// urfave's house shape: uppercase header, 3-space indent, ≤75 cols. A claim
// sets in_progress by itself, so the loop never needs update --status.
const typicalSessionBlock = `

TYPICAL SESSION (set AWIT_AGENT first):
   awit create "Title" --brief "..."   file work items before you code
   awit prime                          see ready and blocked items
   awit next --claim                   claim a ready item, sets in_progress
   awit next --claim <id>              claim that exact item when ready
   awit show <id> --full               read the item and its refs
   awit comment <id> "note"            add a progress note while you work
   awit block <id> --reason "..."      pause until a condition clears
   awit unblock <id>                   remove the manual block
   awit close <id>                     close it when the work is done
`

// rootHelpTemplate is urfave's default root template with the session block
// spliced between COMMANDS and GLOBAL OPTIONS.
var rootHelpTemplate = strings.Replace(cli.RootCommandHelpTemplate, "{{if .VisibleFlagCategories}}", typicalSessionBlock+"{{if .VisibleFlagCategories}}", 1)

// newRoot builds the root command. Later work items register their commands by
// appending to the Commands slice below; nothing else in this function moves.
func newRoot(stdin io.Reader, stdout, stderr io.Writer) *cli.Command {
	return &cli.Command{
		Name:                          "awit",
		Usage:                         "Turn Markdown files under .awit/ into a dependency graph",
		ArgsUsage:                     "<command> [arguments]",
		Version:                       Version,
		CustomRootCommandHelpTemplate: rootHelpTemplate,
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
				Usage: "`DIR` containing .awit (default: $AWIT_REPO, else walk up from the working directory)",
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
			importCmd,
			updateCmd,
			closeCmd,
			releaseCmd,
			blockCmd,
			unblockCmd,
			archiveCmd,
			validateCmd,
			listCmd,
			nextCmd,
			lazyHumanCmd,
			primeCmd,
			showCmd,
			depCmd,
			refCmd,
			commentCmd,
			labelCmd,
			templateCmd,
			externalCmd,
		},
	}
}

// openStore honours --repo (Open of the absolute path), else AWIT_REPO, else
// Find(cwd). Precedence: --repo flag → AWIT_REPO → walk up from cwd.
// Used by every command except init.
func openStore(cmd *cli.Command) (*item.Store, error) { return ops.Open(cmd.Root().String("repo")) }

// noteWalkedUp emits the safety-brake note on stderr when a mutating command
// resolved its root by walking up: no --repo was passed, AWIT_REPO was not
// set, and the working directory holds no .awit/ of its own. Call it from the
// mutating path only, after openStore succeeds; read-only commands stay
// silent so their stdout keeps its golden-file contract.
func noteWalkedUp(cmd *cli.Command, s *item.Store) {
	if n := ops.WalkedUpNote(cmd.Root().String("repo"), s); n != "" {
		fmt.Fprintln(cmd.Root().ErrWriter, n)
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

// SplitLabels turns repeated -l values into label groups. Groups are ANDed and
// the labels inside a group are ORed (spec §6, Label Filter Logic), so
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

// graphItems flattens graph order into the item slice resolveItemID scans.
func graphItems(g *graph.Graph) []*item.Item {
	items := make([]*item.Item, 0, len(g.Order))
	for _, n := range g.Order {
		items = append(items, n.Item)
	}
	return items
}
