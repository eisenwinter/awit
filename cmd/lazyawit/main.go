// Command lazyawit is the human TUI over .awit — the former "awit lazy-human"
// as its own binary, so agents holding only awit have no TUI code path.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/eisenwinter/awit/internal/lazy"
	"github.com/eisenwinter/awit/internal/ops"
	isatty "github.com/mattn/go-isatty"
	"github.com/urfave/cli/v3"
)

var _ lazy.Ops = (*ops.Lazy)(nil)

func main() { os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }

// run is main minus os.Exit so tests drive it in-process. It builds a fresh
// root per call (no shared command tree, no mutex), parses --repo/--agent,
// and maps errors like internal/cli.report: an ExitCoder prints its message
// as-is with its code, anything else prints "Error: <err>" and exits 1.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	cli.VersionPrinter = func(cmd *cli.Command) { fmt.Fprintf(cmd.Root().Writer, "lazyawit %s\n", ops.Version) }
	root := &cli.Command{
		Name:            "lazyawit",
		Usage:           "Browse and triage .awit items in a keyboard-driven TUI (tabs: issues, graph, queue, config)",
		Version:         ops.Version,
		HideHelpCommand: true,
		Reader:          stdin,
		Writer:          stdout,
		ErrWriter:       stderr,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "repo", Usage: "`DIR` containing .awit (default: $AWIT_REPO, else walk up from the working directory)"},
			&cli.StringFlag{Name: "agent", Usage: "identity for claims (default: $AWIT_AGENT, then config agent_id)", Sources: cli.EnvVars("AWIT_AGENT")},
		},
		ExitErrHandler: func(context.Context, *cli.Command, error) {},
		OnUsageError: func(_ context.Context, _ *cli.Command, err error, _ bool) error {
			return cli.Exit(fmt.Sprintf("Incorrect usage: %v (run \"lazyawit --help\")", err), 2)
		},
		Action: tui,
	}
	err := root.Run(context.Background(), append([]string{"lazyawit"}, args...))
	if err == nil {
		return 0
	}
	var coder cli.ExitCoder
	if errors.As(err, &coder) {
		if msg := err.Error(); msg != "" {
			fmt.Fprintln(stderr, msg)
		}
		return coder.ExitCode()
	}
	fmt.Fprintf(stderr, "Error: %v\n", err)
	return 1
}

// tui is the root Action: refuses positional arguments (exit 2), refuses
// non-terminal stdio (exit 1), opens the store via ops.Open, prints
// ops.WalkedUpNote to stderr when non-empty, and runs the alt-screen
// program; an Open error renders the fatal screen.
func tui(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() > 0 {
		return cli.Exit(fmt.Sprintf("lazyawit takes no arguments, got %q (run \"lazyawit --help\")", cmd.Args().First()), 2)
	}
	if !isTerm(cmd.Root().Reader, cmd.Root().Writer) {
		return cli.Exit("lazyawit requires an interactive terminal", 1)
	}
	repo := cmd.String("repo")
	s, err := ops.Open(repo)
	var m lazy.Model
	if err != nil {
		m = lazy.New(nil, ctx, lazy.Options{Fatal: err.Error()}) // fatal screen, q exits 0
	} else {
		if n := ops.WalkedUpNote(repo, s); n != "" {
			fmt.Fprintln(cmd.Root().ErrWriter, n)
		}
		m = lazy.New(ops.NewLazy(s, cmd.String("agent"), time.Now), ctx, lazy.Options{})
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen(), tea.WithInput(cmd.Root().Reader), tea.WithOutput(cmd.Root().Writer), tea.WithContext(ctx)).Run()
	return err
}

// isTerm reports whether r and w are both character-device terminals.
// Anything else (pipes, buffers, files) refuses the TUI: Bubble Tea needs a
// real terminal for the alt screen and key input.
func isTerm(r io.Reader, w io.Writer) bool {
	rf, ok := r.(*os.File)
	if !ok || !isatty.IsTerminal(rf.Fd()) {
		return false
	}
	wf, ok := w.(*os.File)
	if !ok || !isatty.IsTerminal(wf.Fd()) {
		return false
	}
	return true
}
