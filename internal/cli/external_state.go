package cli

import (
	"context"
	"fmt"

	"github.com/eisenwinter/awit/internal/teax"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

// maybePushExternalState propagates a local status change to the linked
// Gitea issue. It runs only after the local mutation was persisted, while
// the caller still holds the store lock so same-checkout close/reopen
// commands cannot reorder their remote writes.
//
// Local state is canonical: every failure (missing tea, invalid metadata,
// ambiguous links, HTTP errors, verification mismatches) keeps the local
// mutation and the ordinary confirmation, prints exactly one stderr
// warning, and returns nil so the command still exits 0:
//
//	warning: <id> saved locally; external state push failed: <reason>; retry with awit update <id> --status <status>
//
// remoteState is the Gitea state ("open" or "closed"); the retry hint uses
// the item's freshly saved local status. A nil external link with no
// problem is a silent no-op. --no-push performs no tea discovery, auth, or
// network operation, even when the linked metadata is malformed.
func maybePushExternalState(ctx context.Context, cmd *cli.Command, all []*item.Item, it *item.Item, remoteState string) {
	if cmd.Bool("no-push") {
		return
	}
	if it.External == nil && it.ExternalProblem == "" {
		return
	}
	status := string(it.Status)
	warn := func(reason string) {
		fmt.Fprintf(cmd.Root().ErrWriter, "warning: %s saved locally; external state push failed: %s; retry with awit update %s --status %s\n",
			it.ID, reason, it.ID, status)
	}
	if it.ExternalProblem != "" {
		warn(it.ExternalProblem)
		return
	}
	ext := *it.External
	if dups := duplicateExternalLinks(all, ext); len(dups) > 1 {
		warn(fmt.Sprintf("ambiguous external link %s#%d matches %s; refusing to push", ext.Repo, ext.ID, joinIDs(dups)))
		return
	}
	client, err := teax.Open(ctx, ext, cmd.String("tea-login"))
	if err != nil {
		warn(err.Error())
		return
	}
	if err := client.SetState(ctx, ext.ID, remoteState); err != nil {
		warn(err.Error())
		return
	}
}
