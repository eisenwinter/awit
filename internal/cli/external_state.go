package cli

import (
	"context"
	"fmt"

	"github.com/eisenwinter/awit/internal/glabx"
	"github.com/eisenwinter/awit/internal/teax"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

// maybePushExternalState propagates a local status change to the linked
// external issue. It runs only after the local mutation was persisted,
// while the caller still holds the store lock so same-checkout close/reopen
// commands cannot reorder their remote writes.
//
// Local state is canonical: every failure (missing tea/glab, invalid
// metadata, ambiguous links, HTTP errors, verification mismatches) keeps
// the local mutation and the ordinary confirmation, prints exactly one
// stderr warning, and returns nil so the command still exits 0:
//
//	warning: <id> saved locally; external state push failed: <reason>; retry with awit update <id> --status <status>
//
// remoteState is "open" or "closed" on both trackers (GitLab wire opened
// maps to local open); the retry hint uses the item's freshly saved local
// status. A nil external link with no problem is a silent no-op. --no-push
// performs no tool discovery, auth, or network operation, even when the
// linked metadata is malformed. --tea-login is Gitea-only and is never
// passed to glab.
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
	if err := setExternalState(ctx, ext, cmd.String("tea-login"), remoteState); err != nil {
		warn(err.Error())
		return
	}
}

// setExternalState pushes the local status change to the linked issue,
// dispatching on tracker: teax for Gitea (with the tea login), glabx for
// GitLab (which ignores the tea login and translates open/closed to
// state_event reopen/close itself). It changes no body, title, or labels.
// Unsupported trackers are refused before any mutation.
func setExternalState(ctx context.Context, ext item.External, teaLogin, state string) error {
	switch ext.Tracker {
	case "gitea":
		client, err := teax.Open(ctx, ext, teaLogin)
		if err != nil {
			return err
		}
		return client.SetState(ctx, ext.ID, state)
	case "gitlab":
		client, err := glabx.Open(ctx, ext)
		if err != nil {
			return err
		}
		return client.SetState(ctx, ext.ID, state)
	default:
		return fmt.Errorf("unsupported tracker %q", ext.Tracker)
	}
}
