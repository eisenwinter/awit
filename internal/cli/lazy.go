package cli

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/eisenwinter/awit/internal/lazy"
	"github.com/eisenwinter/awit/internal/ops"
	"github.com/urfave/cli/v3"
)

var lazyHumanCmd = &cli.Command{
	Name:   "lazy-human",
	Usage:  "Browse and triage items in a keyboard-driven TUI (tabs: issues, graph, queue)",
	Flags:  []cli.Flag{&cli.StringFlag{Name: "agent", Usage: "identity for claims (default: $AWIT_AGENT, then config agent_id)", Sources: cli.EnvVars("AWIT_AGENT")}},
	Action: lazyHumanAction,
}

func lazyHumanAction(ctx context.Context, cmd *cli.Command) error {
	s, err := openStore(cmd)
	var m lazy.Model
	if err != nil {
		m = lazy.New(nil, ctx, lazy.Options{Fatal: err.Error()})
	} else {
		m = lazy.New(ops.NewLazy(s, cmd.String("agent"), time.Now), ctx, lazy.Options{})
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen(), tea.WithInput(cmd.Root().Reader), tea.WithOutput(cmd.Root().Writer), tea.WithContext(ctx)).Run()
	return err
}
