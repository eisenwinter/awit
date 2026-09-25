package ops

import (
	"fmt"
	"strings"
	"time"

	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

// CloseItem marks it closed, clearing the claim timestamp and any manual
// block, and saves it. A non-empty reason is recorded as a comment by
// author at now (AddComment saves); with an empty reason author is unused.
func CloseItem(s *item.Store, it *item.Item, reason, author string, now time.Time) error {
	it.SetStatus(item.StatusClosed)
	it.SetClaimedAt(nil)
	if err := it.SetBlockedReason(""); err != nil {
		return err
	}
	if reason != "" {
		_, err := s.AddComment(it, author, now, reason)
		return err
	}
	return s.Save(it)
}

// BlockItem records reason as its manual block, reopens it and clears
// its claim, then saves. A closed item is refused (exit 1) and an invalid
// reason is a usage error (exit 2); neither writes anything.
func BlockItem(s *item.Store, it *item.Item, reason string) error {
	if it.Status == item.StatusClosed {
		return cli.Exit(fmt.Sprintf("%s is closed; awit release %s to reopen it before blocking", it.ID, it.ID), 1)
	}
	if err := it.SetBlockedReason(reason); err != nil {
		return cli.Exit("block requires --reason with a non-empty, single-line explanation", 2)
	}
	it.SetStatus(item.StatusOpen)
	it.SetAssignee("")
	it.SetClaimedAt(nil)
	return s.Save(it)
}

// UnblockItem removes only the manual block and saves.
func UnblockItem(s *item.Store, it *item.Item) error {
	if err := it.SetBlockedReason(""); err != nil {
		return err
	}
	return s.Save(it)
}

// ReleaseItem returns it to open and clears its claim; any manual block
// stays. Saves.
func ReleaseItem(s *item.Store, it *item.Item) error {
	it.SetStatus(item.StatusOpen)
	it.SetAssignee("")
	it.SetClaimedAt(nil)
	return s.Save(it)
}

// RefuseClaim errors when the exact item cannot be claimed: quarantined,
// closed, blocked, or already claimed by someone. Messages carry no
// "Error: " prefix; Main prints the cli.Exit body as-is with exit 1.
func RefuseClaim(n *graph.Node) error {
	id := n.Item.ID
	if n.Quarantined() {
		var reasons []string
		seen := map[string]bool{}
		for _, f := range n.Faults {
			if !seen[string(f.Reason)] {
				seen[string(f.Reason)] = true
				reasons = append(reasons, "["+string(f.Reason)+"]")
			}
		}
		return cli.Exit(fmt.Sprintf("%s is quarantined %s; run awit validate", id, strings.Join(reasons, ", ")), 1)
	}
	if n.Item.Status == item.StatusClosed {
		return cli.Exit(fmt.Sprintf("%s is closed; awit release %s to reopen it", id, id), 1)
	}
	if n.Item.BlockedReason != "" {
		return cli.Exit(fmt.Sprintf("%s is manually blocked (%s); awit unblock %s once resolved", id, n.Item.BlockedReason, id), 1)
	}
	if n.Blocked {
		return cli.Exit(fmt.Sprintf("%s is blocked by %s", id, strings.Join(n.OpenDepIDs(), ", ")), 1)
	}
	if n.Item.Status == item.StatusInProgress && n.Item.Assignee != "" {
		return cli.Exit(fmt.Sprintf("%s is claimed by %s; awit release %s", id, n.Item.Assignee, id), 1)
	}
	return nil
}

// ClaimItem sets it in_progress, assigned to agent (agent/ prefix added
// when missing) and claimed at now in UTC truncated to the second, then
// saves. The caller checks identity and claimability first.
func ClaimItem(s *item.Store, it *item.Item, agent string, now time.Time) error {
	at := now.UTC().Truncate(time.Second)
	it.SetStatus(item.StatusInProgress)
	it.SetAssignee(WithAgentPrefix(agent))
	it.SetClaimedAt(&at)
	return s.Save(it)
}
