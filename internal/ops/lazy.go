package ops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/eisenwinter/awit/pkg/config"
	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

// Lazy implements the TUI's Ops seam (internal/lazy.Ops) over a store with
// the same functions the CLI commands call, so every row, detail and byte
// written matches the corresponding command. Every mutation takes the
// store lock for 5s like the CLI actions do. It never pushes external state
// and never git-commits. Config and SaveConfig read and write
// .awit/config.yaml, adopting the result into the store.
type Lazy struct {
	s     *item.Store
	agent string
	now   func() time.Time
}

// NewLazy binds s, the --agent value (resolved through s.Config.Agent at
// claim time) and a clock (time.Now in production, fixed in tests).
func NewLazy(s *item.Store, agent string, now func() time.Time) *Lazy {
	return &Lazy{s: s, agent: agent, now: now}
}

func (o *Lazy) Load() (*graph.Graph, error) {
	return LoadGraph(o.s)
}

// Config re-reads .awit/config.yaml with config.Load and adopts the result
// as the store's config, so a later Claim or comment author resolution sees
// the file's current agent_id. On error the store's config is left as is.
func (o *Lazy) Config() (config.Config, error) {
	c, err := config.Load(o.s.Dir)
	if err != nil {
		return config.Config{}, err
	}
	o.s.Config = c
	return c, nil
}

// SaveConfig takes the store lock, writes c with Config.Write
// (temp-then-rename) and adopts c as the store's config. c is expected to
// be Normalize output; the TUI validates before calling.
func (o *Lazy) SaveConfig(c config.Config) error {
	rel, err := o.s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer rel()
	if err := c.Write(o.s.Dir); err != nil {
		return err
	}
	o.s.Config = c
	return nil
}

func (o *Lazy) LoadArchive() ([]*item.Item, error) {
	items, _, err := o.s.LoadArchive()
	return items, err
}

func (o *Lazy) Line(n *graph.Node) string {
	return format.Line(ToEntry(n))
}

func (o *Lazy) ArchiveLine(it *item.Item) string {
	return format.Line(ArchiveEntry(it))
}

func (o *Lazy) Detail(g *graph.Graph, id string) string {
	n := g.Nodes[id]
	if n == nil {
		return fmt.Sprintf("unknown item %s\n", id)
	}
	return ShowFull(o.s, g, n)
}

func (o *Lazy) ArchiveDetail(id string) (string, error) {
	b, err := os.ReadFile(o.s.ArchivePath(id))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (o *Lazy) Close(id, reason string) error {
	rel, err := o.s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer rel()
	it, err := LoadItem(o.s, id)
	if err != nil {
		return err
	}
	author := ""
	if reason != "" {
		if author, err = ResolveAuthor("", o.s.Root, o.s.Config); err != nil {
			return err
		}
	}
	return CloseItem(o.s, it, reason, author, o.now().UTC())
}

func (o *Lazy) Block(id, reason string) error {
	rel, err := o.s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer rel()
	it, err := LoadItem(o.s, id)
	if err != nil {
		return err
	}
	return BlockItem(o.s, it, reason)
}

func (o *Lazy) Unblock(id string) error {
	rel, err := o.s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer rel()
	it, err := LoadItem(o.s, id)
	if err != nil {
		return err
	}
	return UnblockItem(o.s, it)
}

func (o *Lazy) Comment(id, text string) error {
	rel, err := o.s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer rel()
	it, err := LoadItem(o.s, id)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return errors.New("empty comment")
	}
	author, err := ResolveAuthor("", o.s.Root, o.s.Config)
	if err != nil {
		return err
	}
	_, err = o.s.AddComment(it, author, o.now().UTC(), text)
	return err
}

func (o *Lazy) Claim(id string) error {
	rel, err := o.s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer rel()
	it, err := LoadItem(o.s, id)
	if err != nil {
		return err
	}
	g, err := LoadGraph(o.s)
	if err != nil {
		return err
	}
	n := g.Nodes[it.ID]
	if n == nil {
		return fmt.Errorf("unknown item %s", id)
	}
	if err := RefuseClaim(n); err != nil {
		return err
	}
	agent := o.s.Config.Agent(o.agent)
	if agent == "" {
		return fmt.Errorf("no agent identity; pass --agent or set AWIT_AGENT")
	}
	return ClaimItem(o.s, it, agent, o.now())
}

func (o *Lazy) Release(id string) error {
	rel, err := o.s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer rel()
	it, err := LoadItem(o.s, id)
	if err != nil {
		return err
	}
	return ReleaseItem(o.s, it)
}

func (o *Lazy) Validate(g *graph.Graph) string {
	return ValidateText(g)
}

func (o *Lazy) ExternalCheck(ctx context.Context, id string) string {
	it, err := LoadItem(o.s, id)
	if err != nil {
		return err.Error()
	}
	return CheckOneLine(ctx, it, "")
}
