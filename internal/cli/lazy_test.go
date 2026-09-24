package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/eisenwinter/awit/internal/lazy"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

var _ lazy.Ops = (*lazyOps)(nil)

func fixedNow() time.Time { return time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC) }

func newLazyOps(t *testing.T, fixture string) (*lazyOps, string) {
	t.Helper()
	dir := copyFixture(t, fixture)
	return &lazyOps{s: openTestStore(t, dir), agent: "claude", now: fixedNow}, dir
}

func idsOf(ns []*graph.Node) []string {
	var out []string
	for _, n := range ns {
		out = append(out, n.Item.ID)
	}
	return out
}

func idsFromCompact(out string) []string {
	var ids []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if strings.HasPrefix(line, "[") {
			ids = append(ids, line[1:strings.Index(line, "]")])
		}
	}
	return ids
}

func TestLazyListParity(t *testing.T) {
	o, dir := newLazyOps(t, "clean")
	g, _ := o.Load()
	cases := []struct {
		args []string
		f    graph.Filter
	}{
		{nil, graph.Filter{}},
		{[]string{"--ready"}, graph.Filter{Ready: true}},
		{[]string{"--blocked"}, graph.Filter{Blocked: true}},
		{[]string{"--ready", "--blocked"}, graph.Filter{Ready: true, Blocked: true}},
		{[]string{"-s", "open,closed"}, graph.Filter{Statuses: []item.Status{item.StatusOpen, item.StatusClosed}}},
		{[]string{"-l", "auth"}, graph.Filter{Labels: [][]string{{"auth"}}}},
		{[]string{"-l", "auth,db", "-l", "p1"}, graph.Filter{Labels: [][]string{{"auth", "db"}, {"p1"}}}},
		{[]string{"--ready", "-l", "db"}, graph.Filter{Ready: true, Labels: [][]string{{"db"}}}},
	}
	for _, c := range cases {
		args := append([]string{"--repo", dir, "--format", "compact", "list"}, c.args...)
		_, stdout, _ := run(t, args...)
		want := idsFromCompact(stdout)
		got := idsOf(g.Filter(c.f))
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("list %v: tui=%v cli=%v", c.args, got, want)
		}
		var lines []string
		for _, n := range g.Filter(c.f) {
			lines = append(lines, o.Line(n))
		}
		if cliLines := strings.TrimSpace(stdout); strings.Join(lines, "\n") != cliLines {
			t.Errorf("list %v rows differ:\n%s\nwant\n%s", c.args, strings.Join(lines, "\n"), cliLines)
		}
	}
}

func TestLazyDetailOverviewQueueParity(t *testing.T) {
	o, dir := newLazyOps(t, "clean")
	g, _ := o.Load()
	for id := range g.Nodes {
		_, stdout, _ := run(t, "--repo", dir, "show", id, "--full")
		if got := o.Detail(g, id); got != stdout {
			t.Errorf("Detail(%s) differs from show --full", id)
		}
	}
	_, prime, _ := run(t, "--repo", dir, "prime")
	m := lazy.New(o, context.Background(), lazy.Options{})
	if got := lazy.OverviewText(m); got != prime {
		t.Errorf("overview:\n%s\nwant\n%s", got, prime)
	}
	readySection := prime[:strings.Index(prime, "\n=== BLOCKED")]
	if got, want := strings.Join(lazy.QueueIDs(m), ","), strings.Join(idsFromCompact(readySection), ","); got != want {
		t.Errorf("queue = %s, want %s", got, want)
	}
	_, validate, _ := run(t, "--repo", dir, "validate")
	if o.Validate(g) != validate {
		t.Error("Validate differs from validate stdout")
	}
}

func TestLazyOpsBytesMatchCLI(t *testing.T) {
	type op struct {
		name string
		id   string
		tui  func(o *lazyOps) error
		cli  []string
	}
	ops := []op{
		{"close", "AWIT-TEST0001", func(o *lazyOps) error { return o.Close("AWIT-TEST0001", "") }, []string{"close", "AWIT-TEST0001", "--no-push"}},
		{"block", "AWIT-TEST0002", func(o *lazyOps) error { return o.Block("AWIT-TEST0002", "vendor") }, []string{"block", "AWIT-TEST0002", "--reason", "vendor"}},
		{"release", "AWIT-TEST0006", func(o *lazyOps) error { return o.Release("AWIT-TEST0006") }, []string{"release", "AWIT-TEST0006", "--no-push"}},
		{"claim", "AWIT-TEST0002", func(o *lazyOps) error { return o.Claim("AWIT-TEST0002") }, []string{"next", "AWIT-TEST0002", "--claim", "--agent", "claude", "--commit=false"}},
	}
	for _, c := range ops {
		t.Run(c.name, func(t *testing.T) {
			o, _ := newLazyOps(t, "clean")
			b := copyFixture(t, "clean")
			if err := c.tui(o); err != nil {
				t.Fatal(err)
			}
			if code, _, stderr := run(t, append([]string{"--repo", b}, c.cli...)...); code != 0 {
				t.Fatalf("cli exit %d: %s", code, stderr)
			}
			got, _ := os.ReadFile(o.s.ItemPath(c.id))
			want, _ := os.ReadFile(b + "/.awit/items/" + c.id + ".md")
			if c.name == "claim" {
				// claimed_at differs by wall clock: compare with the timestamp line removed.
				got, want = stripLine(got, "claimed_at:"), stripLine(want, "claimed_at:")
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("bytes differ:\n%s\n---\n%s", got, want)
			}
		})
	}
	t.Run("unblock", func(t *testing.T) {
		o, _ := newLazyOps(t, "clean")
		if err := o.Block("AWIT-TEST0002", "vendor"); err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(o.s.ItemPath("AWIT-TEST0002"))
		if err := o.Unblock("AWIT-TEST0002"); err != nil {
			t.Fatal(err)
		}
		after, _ := os.ReadFile(o.s.ItemPath("AWIT-TEST0002"))
		if bytes.Equal(before, after) || bytes.Contains(after, []byte("blocked_reason")) {
			t.Fatalf("unblock did not remove the hold:\n%s", after)
		}
	})
	t.Run("close with reason and comment", func(t *testing.T) {
		o, dir := newLazyOps(t, "clean")
		t.Setenv("AWIT_AGENT", "claude")
		if err := o.Close("AWIT-TEST0001", "done"); err != nil {
			t.Fatal(err)
		}
		if err := o.Comment("AWIT-TEST0002", "note"); err != nil {
			t.Fatal(err)
		}
		cs, _ := o.s.Comments("AWIT-TEST0001")
		if len(cs) != 1 || cs[0].Text != "done" || cs[0].Author != "agent/claude" {
			t.Fatalf("close comment = %+v", cs)
		}
		cs, _ = o.s.Comments("AWIT-TEST0002")
		if len(cs) != 1 || cs[0].Text != "note" || len(readItem(t, dir, "AWIT-TEST0002").Refs) != 1 {
			t.Fatalf("comment = %+v", cs)
		}
		if err := o.Comment("AWIT-TEST0002", "  "); err == nil || err.Error() != "empty comment" {
			t.Fatalf("empty comment err = %v", err)
		}
	})
}

func stripLine(b []byte, prefix string) []byte {
	var out [][]byte
	for _, l := range bytes.Split(b, []byte("\n")) {
		if !bytes.HasPrefix(l, []byte(prefix)) {
			out = append(out, l)
		}
	}
	return bytes.Join(out, []byte("\n"))
}

func TestLazyOpsRefusalsMatchCLI(t *testing.T) {
	o, dir := newLazyOps(t, "clean")
	if err := o.Block("AWIT-TEST0005", "x"); err == nil || !strings.Contains(err.Error(), "AWIT-TEST0005 is closed; awit release AWIT-TEST0005") {
		t.Fatalf("block closed: %v", err)
	}
	if err := o.Claim("AWIT-TEST0003"); err == nil || !strings.Contains(err.Error(), "is blocked by AWIT-TEST0001") {
		t.Fatalf("claim blocked: %v", err)
	}
	if err := o.Claim("AWIT-TEST0006"); err == nil || !strings.Contains(err.Error(), "is claimed by") {
		t.Fatalf("claim claimed: %v", err)
	}
	if err := o.Block("AWIT-TEST0002", "hold"); err != nil {
		t.Fatal(err)
	}
	if err := o.Claim("AWIT-TEST0002"); err == nil || !strings.Contains(err.Error(), "manually blocked (hold)") {
		t.Fatalf("claim held: %v", err)
	}
	t.Setenv("AWIT_AGENT", "")
	o.agent = ""
	if err := o.Claim("AWIT-TEST0001"); err == nil || err.Error() != "no agent identity; pass --agent or set AWIT_AGENT" {
		t.Fatalf("claim without identity: %v", err)
	}
	if got := o.ExternalCheck(context.Background(), "AWIT-TEST0001"); got != "ERROR AWIT-TEST0001: invalid external: no external link" {
		t.Fatalf("external check = %q", got)
	}
	_ = dir
}

func TestLazyArchiveOps(t *testing.T) {
	o, dir := newLazyOps(t, "archive")
	if code, _, stderr := run(t, "--repo", dir, "archive"); code != 0 {
		t.Fatalf("archive: %s", stderr)
	}
	items, err := o.LoadArchive()
	if err != nil || len(items) == 0 {
		t.Fatalf("LoadArchive = %v, %v", items, err)
	}
	if line := o.ArchiveLine(items[0]); !strings.HasPrefix(line, "["+items[0].ID+"] closed ") {
		t.Fatalf("ArchiveLine = %q", line)
	}
	txt, err := o.ArchiveDetail(items[0].ID)
	if err != nil || !strings.Contains(txt, "## Comments") {
		t.Fatalf("ArchiveDetail = %q, %v", txt, err)
	}
}

func TestLazyHumanRegisteredAndQuits(t *testing.T) {
	code, stdout, _ := run(t, "lazy-human", "--help")
	if code != 0 || !strings.Contains(stdout, "--agent") {
		t.Fatalf("help: exit %d\n%s", code, stdout)
	}
	dir := copyFixture(t, "clean")
	done := make(chan int, 1)
	go func() {
		var out, errb bytes.Buffer
		done <- Main([]string{"--repo", dir, "lazy-human"}, strings.NewReader("q"), &out, &errb)
	}()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("lazy-human did not exit on q within 10s")
	}
}
