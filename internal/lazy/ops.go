// Package lazy implements the lazyawit TUI: a keyboard-driven
// Bubble Tea program (tabs: issues, graph, queue) for browsing open items
// and archive, inspecting details, and triaging the ready queue.
//
// The TUI reads through the Ops seam, implemented by internal/ops (`ops.Lazy`)
// with the same functions the CLI commands call, so every row, detail and
// byte written matches the corresponding command. The TUI never pushes
// external state and never git-commits.
package lazy

import (
	"context"

	"github.com/eisenwinter/awit/pkg/config"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

// Ops is everything the TUI needs from the store. internal/ops (`ops.Lazy`)
// implements it with the same functions the CLI commands call, so every row,
// detail and byte written matches the corresponding command. The TUI never
// pushes external state and never git-commits. The Config tab reads and
// writes .awit/config.yaml through Config and SaveConfig; nothing else in
// the TUI touches the file.
type Ops interface {
	Load() (*graph.Graph, error)
	LoadArchive() ([]*item.Item, error)
	Line(n *graph.Node) string
	ArchiveLine(it *item.Item) string
	Detail(g *graph.Graph, id string) string
	ArchiveDetail(id string) (string, error)
	Close(id, reason string) error
	Block(id, reason string) error
	Unblock(id string) error
	Comment(id, text string) error
	Claim(id string) error
	Release(id string) error
	Validate(g *graph.Graph) string
	ExternalCheck(ctx context.Context, id string) string
	Config() (config.Config, error)
	SaveConfig(c config.Config) error
}
