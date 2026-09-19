package cli

import (
	"fmt"

	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/urfave/cli/v3"
)

// warnQuarantined prints the single stderr summary that makes graph
// exclusions visible: "warning: N items quarantined, run awit validate".
// N counts quarantined parseable nodes plus broken files — not fault
// records — so a cycle with several members counts those members and a
// node with several reasons counts once. The same wording is used for
// N=1. Call it exactly once per command, right after the command's
// initial loadGraph; warn before any filtering so hidden quarantined
// items still count, and never from a post-write reload (dep's
// printCompact). stdout, goldens and exit codes are untouched; label
// loads items without building a graph and never warns.
func warnQuarantined(cmd *cli.Command, g *graph.Graph) {
	n := len(g.Quarantined()) + len(g.Broken)
	if n == 0 {
		return
	}
	fmt.Fprintf(cmd.Root().ErrWriter, "warning: %d items quarantined, run awit validate\n", n)
}
