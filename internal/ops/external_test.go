package ops_test

import (
	"context"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/internal/ops"
	"github.com/eisenwinter/awit/pkg/item"
)

func TestCheckOneLine(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // no tea/glab: fetch fails deterministically
	ctx := context.Background()
	unlinked := &item.Item{ID: "AWIT-X"}
	if got := ops.CheckOneLine(ctx, unlinked, ""); got != "ERROR AWIT-X: invalid external: no external link" {
		t.Fatalf("unlinked = %q", got)
	}
	problem := &item.Item{ID: "AWIT-Y", ExternalProblem: "invalid external: tracker must be gitea or gitlab"}
	if got := ops.CheckOneLine(ctx, problem, ""); got != "ERROR AWIT-Y: invalid external: tracker must be gitea or gitlab" {
		t.Fatalf("problem = %q", got)
	}
	linked := &item.Item{ID: "AWIT-Z", External: &item.External{Tracker: "gitea", Repo: "o/r", ID: 3, URL: "https://forge.example/o/r/issues/3"}}
	if got := ops.CheckOneLine(ctx, linked, ""); !strings.HasPrefix(got, "ERROR AWIT-Z https://forge.example/o/r/issues/3: ") {
		t.Fatalf("unfetchable = %q", got)
	}
	if row := ops.CheckOne(ctx, linked, ""); row.Result != "error" || row.URL != linked.External.URL {
		t.Fatalf("row = %+v", row)
	}
}

func TestExternalBaseDispatch(t *testing.T) {
	if _, err := ops.ExternalBase(item.External{Tracker: "jira"}); err == nil || err.Error() != "invalid external: tracker must be gitea or gitlab" {
		t.Fatalf("err = %v", err)
	}
	if _, err := ops.GetExternalIssue(context.Background(), item.External{Tracker: "jira"}, ""); err == nil || err.Error() != `unsupported tracker "jira"` {
		t.Fatalf("err = %v", err)
	}
}
