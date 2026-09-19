---
id: AWIT-0ND56X3G
title: 'awit next with seeded tie-break, -l, --claim, --no-commit'
brief: >-
  Add `awit next`: pick the top ready item by UnblockCount, shuffle the first equal-count group with math/rand/v2 PCG, filter with -l, and optionally claim with a git commit unless --no-commit.
status: closed
deps: [AWIT-0ND56Q3G, AWIT-0ND56M3G, AWIT-0ND56C3G]
labels: [phase3, p0]
refs_base: repo
refs:
  - plan/implementation-guide.md
  - plan/awit-implementation-plan.md
---

## Summary
After this ticket `internal/cli/next.go` registers `awit next`. Candidates are `graph.FilterLabels(g.Ready(), SplitLabels(-l))`. Rank is UnblockCount descending (already the Ready() order). The first run of equal UnblockCount is shuffled with `math/rand/v2` `rand.NewPCG(seed, seed)`; `--seed` 0 (the default) uses `time.Now().UnixNano()`. Empty candidates print `No ready items` and, when `-l` is set, ` (labels: p0)`, then exit 1 via `cli.Exit` (no `Error:` prefix). `--claim` requires an agent identity (`--agent` / `AWIT_AGENT` / `config.agent_id`), writes `in_progress` + `assignee` (`agent/`+value unless the value contains `/`) + `claimed_at`, `Save`s, then `gitx.Commit` with `awit: claim <id>` unless `--no-commit`. A commit error after a successful write is `claimed <id> but git commit failed: ...` exit 1.

## Context (read first)
- Guide §2 `next` tie-break — `math/rand/v2` PCG(seed, seed). Shuffle **only** the leading equal-UnblockCount group; do not shuffle the whole candidate list.
- Guide §2 Git commit on `--claim` — `gitx.Commit(root, []string{it.Path}, "awit: claim "+id)`. Path is absolute (`item.Path`). Commit failure is an error **after** the file was written.
- Guide §2 decision 3 — only `--claim` commits. `--no-commit` skips `gitx.Commit`. `--no-commit` without `--claim` is a no-op.
- Guide §4.2 — `Config.Agent(flag)` is flag → `AWIT_AGENT` → `c.AgentID` → `""` and never adds `agent/`. Prefix with `withAgentPrefix` from AWIT-0ND56M3G (`contains "/" → verbatim`, else `"agent/"+v`).
- Guide §4.5 — `gitx.Commit`. Guide §4.7 — `format.WriteOne` for compact/table/json of one `Entry`. Guide §4.11 — `loadGraph`, `toEntry` (owned by AWIT-0ND56J3G). This ticket's deps do **not** include J3G, so define both if absent (exact code in Step 3).
- Guide §4.11 / spec — flags `-l`, `--claim`, `--no-commit`, `--seed`. Agent identity: `--agent`, else `AWIT_AGENT`, else `config.agent_id`; none → refuse `--claim`.
- 683G `report` — `cli.Exit("No ready items", 1)` prints `No ready items\n` on stderr with **no** `Error:` prefix. Use that, not `fmt.Errorf`.
- AWIT-0ND56M3G — `withAgentPrefix`, `loadItem` (not needed here; claim uses the graph node then `s.Load`/`Save`), `seedItem` in `update_test.go` (same package — reuse it). `initRepo`, `run`, `copyFixture`, `readItem` from G3G.
- Guide §8 `clean`: Ready is 0001 (UnblockCount 2), 0002 (0), 0006 (0). `next` without `-l` always returns 0001 because it uniquely holds the top count. `-l p0` filters Ready to empty (0004 is p0 but blocked) → `No ready items (labels: p0)`.
- `--seed` is `Int64Flag`, default 0. Tests always pass a non-zero seed.
- `--agent` is `StringFlag` with `Sources: cli.EnvVars("AWIT_AGENT")`. Then `s.Config.Agent(cmd.String("agent"))`.
- Claimed timestamp: `time.Now().UTC().Truncate(time.Second)` stored via `SetClaimedAt`.
- Print the compact/json line **after** a successful claim (so JSON `status` is `in_progress`) and **after** a successful commit. If commit fails, do not print the line; return the claimed-but-failed error.
- `detectFormat` — define if absent (H3G). Tests pass `--format compact` except `TestNextJSON`.

## Files
- Create: `internal/cli/next.go`
- Create: `internal/cli/next_test.go`
- Modify: `internal/cli/app.go` — append `nextCmd`; add `loadGraph` / `toEntry` / `detectFormat` only if they are not already defined.

## Interfaces
- Consumes:
  ```go
  func openStore(cmd *cli.Command) (*item.Store, error)
  func SplitLabels(flags []string) [][]string
  func FilterLabels(nodes []*graph.Node, groups [][]string) []*graph.Node
  func (g *Graph) Ready() []*Node
  func (c Config) Agent(flag string) string
  func withAgentPrefix(v string) string
  func Commit(root string, paths []string, message string) error
  func WriteOne(w io.Writer, f format.Format, e format.Entry) error
  func (it *Item) SetStatus(s Status)
  func (it *Item) SetAssignee(s string)
  func (it *Item) SetClaimedAt(t *time.Time)
  func (s *Store) Save(it *Item) error
  func (s *Store) Load(id string) (*Item, error)
  ```
- Produces (package-private):
  ```go
  var nextCmd *cli.Command
  func nextAction(_ context.Context, cmd *cli.Command) error
  func pickNext(cands []*graph.Node, seed int64) *graph.Node
  func noReadyMessage(labelFlags []string) string
  func loadGraph(s *item.Store) (*graph.Graph, error)           // define if absent
  func toEntry(n *graph.Node) format.Entry                      // define if absent
  func detectFormat(cmd *cli.Command) (format.Format, error)    // define if absent
  ```

## Steps

- [ ] **Step 1: Write the failing tests.**

  Create `internal/cli/next_test.go`:

  ```go
  package cli

  import (
  	"encoding/json"
  	"os"
  	"os/exec"
  	"path/filepath"
  	"strings"
  	"testing"
  	"time"

  	"github.com/eisenwinter/awit/pkg/item"
  )

  func TestNextTopByUnblocks(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--seed", "1")
  	if code != 0 || stderr != "" {
  		t.Fatalf("exit %d stderr %q stdout %q", code, stderr, stdout)
  	}
  	want := "[AWIT-TEST0001] Implement OAuth2 bearer token extraction | auth,p1 | Unblocks: 2\n"
  	if stdout != want {
  		t.Fatalf("stdout = %q, want %q", stdout, want)
  	}
  }

  func TestNextSeededTieBreak(t *testing.T) {
  	dir := initRepo(t)
  	seedItem(t, dir, "AWIT-TEST0001", "Alpha", "A.", nil)
  	seedItem(t, dir, "AWIT-TEST0002", "Beta", "B.", nil)

  	code1, out1, err1 := run(t, "--repo", dir, "--format", "compact", "next", "--seed", "1")
  	if code1 != 0 || err1 != "" {
  		t.Fatalf("seed 1: exit %d stderr %q", code1, err1)
  	}
  	code1b, out1b, err1b := run(t, "--repo", dir, "--format", "compact", "next", "--seed", "1")
  	if code1b != 0 || err1b != "" {
  		t.Fatalf("seed 1 repeat: exit %d stderr %q", code1b, err1b)
  	}
  	if out1 != out1b {
  		t.Fatalf("same seed must be deterministic: %q vs %q", out1, out1b)
  	}
  	if !strings.HasPrefix(out1, "[AWIT-TEST0001]") && !strings.HasPrefix(out1, "[AWIT-TEST0002]") {
  		t.Fatalf("seed 1 picked %q, want 0001 or 0002", out1)
  	}

  	_, out7, err7 := run(t, "--repo", dir, "--format", "compact", "next", "--seed", "7")
  	if err7 != "" {
  		t.Fatalf("seed 7 stderr %q", err7)
  	}
  	if !strings.HasPrefix(out7, "[AWIT-TEST0001]") && !strings.HasPrefix(out7, "[AWIT-TEST0002]") {
  		t.Fatalf("seed 7 picked %q, want 0001 or 0002", out7)
  	}
  }

  func TestNextLabelFilterEmpty(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "-l", "p0", "--seed", "1")
  	if code != 1 {
  		t.Fatalf("exit %d, want 1 stdout %q stderr %q", code, stdout, stderr)
  	}
  	if stdout != "" {
  		t.Fatalf("stdout = %q, want empty", stdout)
  	}
  	if stderr != "No ready items (labels: p0)\n" {
  		t.Fatalf("stderr = %q, want %q", stderr, "No ready items (labels: p0)\n")
  	}
  }

  func TestNextClaimNoCommit(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	before := time.Now().UTC().Add(-time.Second).Truncate(time.Second)
  	code, _, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--no-commit", "--agent", "claude", "--seed", "1")
  	if code != 0 || stderr != "" {
  		t.Fatalf("exit %d stderr %q", code, stderr)
  	}
  	got := readItem(t, dir, "AWIT-TEST0001")
  	if got.Status != item.StatusInProgress {
  		t.Fatalf("status = %q, want in_progress", got.Status)
  	}
  	if got.Assignee != "agent/claude" {
  		t.Fatalf("assignee = %q, want agent/claude", got.Assignee)
  	}
  	if got.ClaimedAt == nil {
  		t.Fatal("claimed_at is nil")
  	}
  	if got.ClaimedAt.Before(before) || got.ClaimedAt.After(time.Now().UTC().Add(time.Second)) {
  		t.Fatalf("claimed_at = %v out of range", got.ClaimedAt)
  	}
  	if _, err := os.Stat(filepath.Join(dir, ".git")); !os.IsNotExist(err) {
  		t.Fatalf("fixture must not be a git repo; --no-commit must not require one: %v", err)
  	}
  }

  func gitLookPath(t *testing.T) {
  	t.Helper()
  	if _, err := exec.LookPath("git"); err != nil {
  		t.Skip("git not installed")
  	}
  }

  func gitRun(t *testing.T, dir string, args ...string) string {
  	t.Helper()
  	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
  	out, err := cmd.CombinedOutput()
  	if err != nil {
  		t.Fatalf("git %s: %v: %s", args, err, out)
  	}
  	return strings.TrimSpace(string(out))
  }

  func TestNextClaimCommits(t *testing.T) {
  	gitLookPath(t)
  	dir := copyFixture(t, "clean")
  	gitRun(t, dir, "init", "-q")
  	gitRun(t, dir, "config", "user.name", "tester")
  	gitRun(t, dir, "config", "user.email", "tester@example.com")
  	gitRun(t, dir, "add", ".")
  	gitRun(t, dir, "commit", "-q", "-m", "init")

  	code, _, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--agent", "claude", "--seed", "1")
  	if code != 0 || stderr != "" {
  		t.Fatalf("exit %d stderr %q", code, stderr)
  	}
  	subject := gitRun(t, dir, "log", "-1", "--pretty=%s")
  	if subject != "awit: claim AWIT-TEST0001" {
  		t.Fatalf("subject = %q, want %q", subject, "awit: claim AWIT-TEST0001")
  	}
  	got := readItem(t, dir, "AWIT-TEST0001")
  	if got.Status != item.StatusInProgress || got.Assignee != "agent/claude" {
  		t.Fatalf("status=%q assignee=%q", got.Status, got.Assignee)
  	}
  	files := gitRun(t, dir, "show", "--name-only", "--pretty=format:", "HEAD")
  	if !strings.Contains(files, "AWIT-TEST0001.md") {
  		t.Fatalf("committed files = %q, want the claimed item", files)
  	}
  }

  func TestNextClaimWithoutAgent(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	t.Setenv("AWIT_AGENT", "")
  	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--claim", "--no-commit", "--seed", "1")
  	if code != 1 {
  		t.Fatalf("exit %d, want 1 stdout %q stderr %q", code, stdout, stderr)
  	}
  	if stderr != "Error: no agent identity; pass --agent or set AWIT_AGENT\n" {
  		t.Fatalf("stderr = %q", stderr)
  	}
  	got := readItem(t, dir, "AWIT-TEST0001")
  	if got.Status != item.StatusOpen || got.Assignee != "" {
  		t.Fatalf("item was claimed without an agent: status=%q assignee=%q", got.Status, got.Assignee)
  	}
  }

  func TestNextJSON(t *testing.T) {
  	dir := copyFixture(t, "clean")
  	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "next", "--seed", "1")
  	if code != 0 || stderr != "" {
  		t.Fatalf("exit %d stderr %q", code, stderr)
  	}
  	var row struct {
  		ID       string `json:"id"`
  		Title    string `json:"title"`
  		Status   string `json:"status"`
  		State    string `json:"state"`
  		Unblocks int    `json:"unblocks"`
  	}
  	if err := json.Unmarshal([]byte(stdout), &row); err != nil {
  		t.Fatalf("json: %v\n%s", err, stdout)
  	}
  	if row.ID != "AWIT-TEST0001" || row.State != "ready" || row.Unblocks != 2 {
  		t.Fatalf("row = %+v, want 0001 ready unblocks 2", row)
  	}
  	if !strings.HasSuffix(stdout, "\n") {
  		t.Fatal("json must end with a newline")
  	}
  }

  func TestNextNoReadyUnfiltered(t *testing.T) {
  	dir := initRepo(t)
  	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "next", "--seed", "1")
  	if code != 1 || stdout != "" || stderr != "No ready items\n" {
  		t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr)
  	}
  }
  ```

- [ ] **Step 2: Run it, see it fail.**

  ```bash
  go test ./internal/cli -run 'TestNext' -v
  ```

  Expected: `next` is unknown (exit 2) or `undefined: seedItem` if M3G is not closed — M3G is a dep and must be `status: closed` before you start. Valid red:

  ```text
  --- FAIL: TestNextTopByUnblocks
      next_test.go: exit 2 stderr "unknown command \"next\" (run \"awit --help\")"
  ```

- [ ] **Step 3: Implement helpers (if absent) and `next.go`.**

  Define these **only if they are not already in package `cli`**. Bodies must match AWIT-0ND56J3G / AWIT-0ND56H3G.

  `loadGraph`:

  ```go
  func loadGraph(s *item.Store) (*graph.Graph, error) {
  	items, broken, err := s.LoadAll()
  	if err != nil {
  		return nil, err
  	}
  	return graph.Build(items, broken), nil
  }
  ```

  `toEntry`:

  ```go
  func toEntry(n *graph.Node) format.Entry {
  	e := format.Entry{
  		ID:       n.Item.ID,
  		Title:    n.Item.Title,
  		Brief:    n.Item.Brief,
  		Status:   string(n.Item.Status),
  		Labels:   n.Item.Labels,
  		Deps:     n.Item.Deps,
  		Assignee: n.Item.Assignee,
  		Unblocks: n.UnblockCount,
  	}
  	if e.Labels == nil {
  		e.Labels = []string{}
  	}
  	if e.Deps == nil {
  		e.Deps = []string{}
  	}
  	switch {
  	case n.Quarantined():
  		e.State = "quarantined"
  		for _, f := range n.Faults {
  			e.Faults = append(e.Faults, "["+string(f.Reason)+"] "+f.Detail)
  		}
  	case n.Item.Status == item.StatusClosed:
  		e.State = "closed"
  	case n.Ready:
  		e.State = "ready"
  	default:
  		e.State = "blocked"
  	}
  	return e
  }
  ```

  `detectFormat`:

  ```go
  func detectFormat(cmd *cli.Command) (format.Format, error) {
  	var stdout *os.File
  	if f, ok := cmd.Root().Writer.(*os.File); ok {
  		stdout = f
  	}
  	return format.Detect(cmd.Root().String("format"), stdout)
  }
  ```

  Create `internal/cli/next.go`:

  ```go
  package cli

  import (
  	"context"
  	"fmt"
  	"math/rand/v2"
  	"strings"
  	"time"

  	"github.com/eisenwinter/awit/internal/gitx"
  	"github.com/eisenwinter/awit/pkg/format"
  	"github.com/eisenwinter/awit/pkg/graph"
  	"github.com/eisenwinter/awit/pkg/item"
  	"github.com/urfave/cli/v3"
  )

  var nextCmd = &cli.Command{
  	Name:  "next",
  	Usage: "Print the top unblocked item, optionally claiming it",
  	Flags: []cli.Flag{
  		&cli.StringSliceFlag{Name: "label", Aliases: []string{"l"}, Usage: "AND across flags, OR within a flag"},
  		&cli.BoolFlag{Name: "claim", Usage: "set in_progress, assignee, claimed_at, and commit"},
  		&cli.BoolFlag{Name: "no-commit", Usage: "with --claim, skip the git commit"},
  		&cli.Int64Flag{Name: "seed", Usage: "tie-break RNG seed; 0 (default) uses time.Now().UnixNano()"},
  		&cli.StringFlag{
  			Name:    "agent",
  			Usage:   "agent identity for --claim",
  			Sources: cli.EnvVars("AWIT_AGENT"),
  		},
  	},
  	Action: nextAction,
  }

  func noReadyMessage(labelFlags []string) string {
  	msg := "No ready items"
  	groups := SplitLabels(labelFlags)
  	if len(groups) == 0 {
  		return msg
  	}
  	var parts []string
  	for _, g := range groups {
  		parts = append(parts, strings.Join(g, ","))
  	}
  	return msg + " (labels: " + strings.Join(parts, ", ") + ")"
  }

  // pickNext returns the winner. cands must already be Ready()-ordered
  // (UnblockCount desc, ID asc). The leading equal-UnblockCount group is
  // shuffled with PCG(seed, seed); the rest of the slice is ignored.
  func pickNext(cands []*graph.Node, seed int64) *graph.Node {
  	if len(cands) == 0 {
  		return nil
  	}
  	top := cands[0].UnblockCount
  	end := 1
  	for end < len(cands) && cands[end].UnblockCount == top {
  		end++
  	}
  	group := append([]*graph.Node(nil), cands[:end]...)
  	r := rand.New(rand.NewPCG(uint64(seed), uint64(seed)))
  	r.Shuffle(len(group), func(i, j int) {
  		group[i], group[j] = group[j], group[i]
  	})
  	return group[0]
  }

  func nextAction(_ context.Context, cmd *cli.Command) error {
  	s, err := openStore(cmd)
  	if err != nil {
  		return err
  	}
  	g, err := loadGraph(s)
  	if err != nil {
  		return err
  	}
  	labelFlags := cmd.StringSlice("label")
  	cands := graph.FilterLabels(g.Ready(), SplitLabels(labelFlags))
  	if len(cands) == 0 {
  		return cli.Exit(noReadyMessage(labelFlags), 1)
  	}
  	seed := cmd.Int64("seed")
  	if seed == 0 {
  		seed = time.Now().UnixNano()
  	}
  	n := pickNext(cands, seed)

  	if cmd.Bool("claim") {
  		agent := s.Config.Agent(cmd.String("agent"))
  		if agent == "" {
  			return fmt.Errorf("no agent identity; pass --agent or set AWIT_AGENT")
  		}
  		it, err := s.Load(n.Item.ID)
  		if err != nil {
  			return err
  		}
  		now := time.Now().UTC().Truncate(time.Second)
  		it.SetStatus(item.StatusInProgress)
  		it.SetAssignee(withAgentPrefix(agent))
  		it.SetClaimedAt(&now)
  		if err := s.Save(it); err != nil {
  			return err
  		}
  		n.Item = it
  		if !cmd.Bool("no-commit") {
  			if err := gitx.Commit(s.Root, []string{it.Path}, "awit: claim "+it.ID); err != nil {
  				return fmt.Errorf("claimed %s but git commit failed: %w", it.ID, err)
  			}
  		}
  	}

  	f, err := detectFormat(cmd)
  	if err != nil {
  		return err
  	}
  	return format.WriteOne(cmd.Writer, f, toEntry(n))
  }
  ```

  In `newRoot`, append `nextCmd` to `Commands`. Keep every existing command.

- [ ] **Step 4: Run it, see it pass, commit.**

  ```bash
  go test ./internal/cli -run 'TestNext' -v
  ```

  Expected PASS: `TestNextTopByUnblocks`, `TestNextSeededTieBreak`, `TestNextLabelFilterEmpty`, `TestNextClaimNoCommit`, `TestNextClaimCommits` (or SKIP if git is missing), `TestNextClaimWithoutAgent`, `TestNextJSON`, `TestNextNoReadyUnfiltered`.

  ```bash
  go test ./internal/cli -count=1
  gofmt -w internal/cli/next.go internal/cli/next_test.go internal/cli/app.go
  git add internal/cli/next.go internal/cli/next_test.go internal/cli/app.go
  git commit -m "cli/next: seeded tie-break, label filter, and optional claim"
  ```

- [ ] **Step 5: Full check and close.**

  ```bash
  go build ./... && go vet ./... && go test ./internal/cli -count=1
  ```

  Set `status: closed` on this file, write `.awit/comments/AWIT-0ND56X3G/<YYYYMMDDTHHMMSSZ>-<author>.md` with the acceptance output, append that ref, commit `tickets: close AWIT-0ND56X3G`.

## Acceptance Criteria
- `go test ./internal/cli -run 'TestNext' -v` — all PASS (`TestNextClaimCommits` may SKIP).
- `next --seed 1` on clean prints the 0001 compact line, exit 0.
- Two ready items with equal UnblockCount: `--seed 1` twice is identical; the pick is one of the two IDs.
- `next -l p0` on clean: stderr `No ready items (labels: p0)\n`, stdout empty, exit 1, no `Error:` prefix.
- `--claim --no-commit --agent claude` sets 0001 to `in_progress` / `agent/claude` / non-nil `claimed_at` without requiring git.
- `--claim --agent claude` in a git repo commits `awit: claim AWIT-TEST0001` touching only that item (`TestNextClaimCommits`).
- `--claim` without agent/env/config: exit 1, `Error: no agent identity; pass --agent or set AWIT_AGENT`, item unchanged.
- `--format json` is one object, `id` 0001, `state` ready, `unblocks` 2.
- `gofmt -l internal/cli/next.go` prints nothing.

## Out of scope
- `awit prime`. Stale-claim checks. `close` committing. Changing `gitx.Commit` or `FilterLabels`. Registering `--seed` on any other command.
