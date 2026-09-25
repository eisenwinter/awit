package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/eisenwinter/awit/internal/lazy"
	"github.com/eisenwinter/awit/internal/ops"
	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
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
		m = lazy.New(&lazyOps{s: s, agent: cmd.String("agent"), now: time.Now}, ctx, lazy.Options{})
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen(), tea.WithInput(cmd.Root().Reader), tea.WithOutput(cmd.Root().Writer), tea.WithContext(ctx)).Run()
	return err
}

type lazyOps struct {
	s     *item.Store
	agent string
	now   func() time.Time
}

func (o *lazyOps) Load() (*graph.Graph, error) {
	return ops.LoadGraph(o.s)
}

func (o *lazyOps) LoadArchive() ([]*item.Item, error) {
	items, _, err := o.s.LoadArchive()
	return items, err
}

func (o *lazyOps) Line(n *graph.Node) string {
	return format.Line(ops.ToEntry(n))
}

func (o *lazyOps) ArchiveLine(it *item.Item) string {
	return format.Line(ops.ArchiveEntry(it))
}

func (o *lazyOps) Detail(g *graph.Graph, id string) string {
	n := g.Nodes[id]
	if n == nil {
		return fmt.Sprintf("unknown item %s\n", id)
	}
	return showFull(o.s, g, n)
}

func (o *lazyOps) ArchiveDetail(id string) (string, error) {
	b, err := os.ReadFile(o.s.ArchivePath(id))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (o *lazyOps) Close(id, reason string) error {
	rel, err := o.s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer rel()
	it, err := ops.LoadItem(o.s, id)
	if err != nil {
		return err
	}
	author := ""
	if reason != "" {
		if author, err = ops.ResolveAuthor("", o.s.Root, o.s.Config); err != nil {
			return err
		}
	}
	return ops.CloseItem(o.s, it, reason, author, o.now().UTC())
}

func (o *lazyOps) Block(id, reason string) error {
	rel, err := o.s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer rel()
	it, err := ops.LoadItem(o.s, id)
	if err != nil {
		return err
	}
	return ops.BlockItem(o.s, it, reason)
}

func (o *lazyOps) Unblock(id string) error {
	rel, err := o.s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer rel()
	it, err := ops.LoadItem(o.s, id)
	if err != nil {
		return err
	}
	return ops.UnblockItem(o.s, it)
}

func (o *lazyOps) Comment(id, text string) error {
	rel, err := o.s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer rel()
	it, err := ops.LoadItem(o.s, id)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("empty comment")
	}
	author, err := ops.ResolveAuthor("", o.s.Root, o.s.Config)
	if err != nil {
		return err
	}
	_, err = o.s.AddComment(it, author, o.now().UTC(), text)
	return err
}

func (o *lazyOps) Claim(id string) error {
	rel, err := o.s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer rel()
	it, err := ops.LoadItem(o.s, id)
	if err != nil {
		return err
	}
	g, err := ops.LoadGraph(o.s)
	if err != nil {
		return err
	}
	n := g.Nodes[it.ID]
	if n == nil {
		return fmt.Errorf("unknown item %s", id)
	}
	if err := ops.RefuseClaim(n); err != nil {
		return err
	}
	agent := o.s.Config.Agent(o.agent)
	if agent == "" {
		return fmt.Errorf("no agent identity; pass --agent or set AWIT_AGENT")
	}
	return ops.ClaimItem(o.s, it, agent, o.now())
}

func (o *lazyOps) Release(id string) error {
	rel, err := o.s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer rel()
	it, err := ops.LoadItem(o.s, id)
	if err != nil {
		return err
	}
	return ops.ReleaseItem(o.s, it)
}

func (o *lazyOps) Validate(g *graph.Graph) string {
	return validateText(g)
}

func (o *lazyOps) ExternalCheck(ctx context.Context, id string) string {
	it, err := ops.LoadItem(o.s, id)
	if err != nil {
		return err.Error()
	}
	r := checkOne(ctx, it, "")
	switch r.Result {
	case "match":
		return fmt.Sprintf("MATCH %s %s", r.ID, r.URL)
	case "drift":
		if r.Detail != "" {
			return fmt.Sprintf("DRIFT %s %s: %s", r.ID, r.URL, r.Detail)
		}
		return fmt.Sprintf("DRIFT %s %s", r.ID, r.URL)
	default:
		if r.URL != "" {
			return fmt.Sprintf("ERROR %s %s: %s", r.ID, r.URL, r.Detail)
		}
		return fmt.Sprintf("ERROR %s: %s", r.ID, r.Detail)
	}
}
