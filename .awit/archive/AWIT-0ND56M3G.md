---
id: AWIT-0ND56M3G
title: "awit update, close, release"
brief: >-
  Add `awit update`, `awit close` and `awit release`, plus shared `loadItem` and `resolveAuthor`. `update --status` must change exactly one line; `close` does not git-commit.
status: closed
deps: [AWIT-0ND56G3G]
labels: [phase1, p1]
refs_base: repo
refs:
  - plan/implementation-guide.md
  - plan/awit-implementation-plan.md
---

## Summary

Three commands mutate one item file: `update` patches frontmatter (`nothing to
update` when no flag is set), `close` sets `closed` and clears `claimed_at`
(optional `--reason` comment, no git commit), `release` sets `open` and clears
assignee plus `claimed_at`. `loadItem` and `resolveAuthor` live in `author.go`
for AWIT-0ND56Y3G. Tests seed with `item.New`+`Save` on `initRepo` - do not
call `awit create` (sibling, not a dep).

## Context (read first)

- **guide §1** - `filepath`, `Error: ` via `report`, exit 0/1/2, Node YAML so
  unchanged keys stay byte-identical.
- **guide §2 #3** - `close` does **not** git-commit. Do not import `gitx.Commit`.
- **guide §2 #4** - `--author` verbatim → `AWIT_AGENT` (`agent/` unless value
  contains `/`) → `config.agent_id` (same prefix rule) → `gitx.UserName`
  lowercased, spaces→`-` (no prefix) → `no author; pass --author or set AWIT_AGENT`.
  Empty flag/env/config = unset.
- **guide §2 keys** - `SetAssignee("")` / `SetClaimedAt(nil)` **delete** the key.
  `update --status closed` also clears `claimed_at`; other statuses leave
  assignee/`claimed_at`. `close` does not clear assignee.
- **guide §4.3–4.5** - `ParseStatus` (return unchanged), setters, `Load`
  (`os.ErrNotExist` or `*item.BrokenError`), `AddComment` (writes, appends
  forward-slash ref, saves), `gitx.UserName`.
- **guide §4.11 / spec matrix** - `update <id>` `--status --brief --assign
--label --unlabel --title`; `close <id>` `--reason --author`; `release <id>`
  no flags. Register on `newRoot` Commands. Phase-1 exit: `update --status`
  changes exactly one line to `status: in_progress`.
- **AWIT-0ND56G3G** - `openStore`, `run`, `initRepo`, `readItem`. Do not
  redeclare them, nor `parseIDList`/`detectFormat`. This ticket uses
  `splitFlagCSV` for `--label`/`--unlabel`.
- `loadItem`: missing → `unknown item %s`; `*BrokenError` →
  `%s: %s: %s (fix the file, then retry)` (ID, Reason, Detail).
- `--label` adds (skip dups, keep order, append); `--unlabel` removes (missing
  = no-op). Adds first, then removes.

## Files

- Create: `internal/cli/author.go`, `update.go`, `close.go`, `release.go`, `update_test.go`
- Modify: `internal/cli/app.go` - append `updateCmd`, `closeCmd`, `releaseCmd`

## Interfaces

Consumes: `openStore`, `Store.Load/Save/AddComment`, `ParseStatus`, `item.New`,
setters, `gitx.UserName`, `item.BrokenError`.

Produces (package-private; AWIT-0ND56Y3G calls these - do not rename):

```go
func loadItem(s *item.Store, id string) (*item.Item, error)
func resolveAuthor(flagAuthor, repoRoot string, cfg config.Config) (string, error)
func withAgentPrefix(v string) string // contains "/" → verbatim; else "agent/"+v
var updateCmd, closeCmd, releaseCmd *cli.Command
```

Success is silent (empty stdout, exit 0).

## Steps

- [ ] **Step 1: Failing tests.** Create `internal/cli/update_test.go`:

```go
package cli

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/eisenwinter/awit/internal/gitx"
	"github.com/eisenwinter/awit/pkg/config"
	"github.com/eisenwinter/awit/pkg/item"
)

func seedItem(t *testing.T, repo, id, title, brief string, labels []string) {
	t.Helper()
	st, err := item.Open(repo)
	if err != nil { t.Fatal(err) }
	if err := st.Save(item.New(id, title, brief, nil, labels)); err != nil { t.Fatal(err) }
}

func TestUpdateStatusOneLineDiff(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "One line", "Brief.", nil)
	path := filepath.Join(dir, ".awit", "items", id+".md")
	before, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	code, stdout, stderr := run(t, "--repo", dir, "update", id, "--status", "in_progress")
	if code != 0 || stdout != "" { t.Fatalf("exit %d stdout %q stderr %q", code, stdout, stderr) }
	after, err := os.ReadFile(path)
	if err != nil { t.Fatal(err) }
	b, a := strings.Split(string(before), "\n"), strings.Split(string(after), "\n")
	if len(b) != len(a) { t.Fatalf("line count %d -> %d\n%s\n%s", len(b), len(a), before, after) }
	var changed []string
	for i := range b {
		if b[i] != a[i] {
			changed = append(changed, a[i])
		}
	}
	if len(changed) != 1 || changed[0] != "status: in_progress" { t.Fatalf("changed = %q, want [status: in_progress]", changed) }
}

func TestUpdateTitleAndBrief(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "Old", "Old brief.", nil)
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--title", "New title", "--brief", "New brief.")
	if code != 0 { t.Fatalf("exit %d stderr %q", code, stderr) }
	it := readItem(t, dir, "AWIT-TEST0001")
	if it.Title != "New title" || it.Brief != "New brief." { t.Fatalf("title=%q brief=%q", it.Title, it.Brief) }
}

func TestUpdateLabelsAddRemove(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "L", "B.", []string{"keep", "drop"})
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--label", "add,keep", "--unlabel", "drop")
	if code != 0 { t.Fatalf("exit %d stderr %q", code, stderr) }
	if g := strings.Join(readItem(t, dir, "AWIT-TEST0001").Labels, ","); g != "keep,add" { t.Fatalf("labels = %s", g) }
}

func TestUpdateNothing(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "T", "B.", nil)
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001")
	if code != 1 || stderr != "Error: nothing to update\n" { t.Fatalf("exit %d stderr %q", code, stderr) }
}

func TestUpdateBadStatus(t *testing.T) {
	dir := initRepo(t)
	seedItem(t, dir, "AWIT-TEST0001", "T", "B.", nil)
	_, perr := item.ParseStatus("banana")
	if perr == nil { t.Fatal("ParseStatus(banana) succeeded") }
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--status", "banana")
	if code != 1 || stderr != "Error: "+perr.Error()+"\n" { t.Fatalf("exit %d stderr %q", code, stderr) }
}

func TestUpdateUnknownItem(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--title", "x")
	if code != 1 || stderr != "Error: unknown item AWIT-TEST0001\n" { t.Fatalf("exit %d stderr %q", code, stderr) }
}

func TestUpdateBrokenItem(t *testing.T) {
	dir := initRepo(t)
	p := filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md")
	if err := os.WriteFile(p, []byte("---\nid: AWIT-TEST0001\ntitle: [unclosed\nstatus: open\n---\n"), 0o644); err != nil { t.Fatal(err) }
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--title", "x")
	if code != 1 { t.Fatalf("exit %d stderr %q", code, stderr) }
	if !strings.Contains(stderr, "Error: AWIT-TEST0001:") || !strings.Contains(stderr, string(item.ReasonParse)) || !strings.Contains(stderr, "(fix the file, then retry)") { t.Fatalf("stderr = %q", stderr) }
}

func TestCloseClearsClaimedAt(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	st, err := item.Open(dir)
	if err != nil { t.Fatal(err) }
	it, err := st.Load(id)
	if err != nil { t.Fatal(err) }
	ts := time.Date(2026, 9, 17, 14, 32, 5, 0, time.UTC)
	it.SetAssignee("agent/claude")
	it.SetClaimedAt(&ts)
	if err := st.Save(it); err != nil { t.Fatal(err) }
	code, _, stderr := run(t, "--repo", dir, "close", id)
	if code != 0 { t.Fatalf("exit %d stderr %q", code, stderr) }
	got := readItem(t, dir, id)
	if got.Status != item.StatusClosed || got.ClaimedAt != nil || got.Assignee != "agent/claude" { t.Fatalf("status=%q claimed=%v assignee=%q", got.Status, got.ClaimedAt, got.Assignee) }
}

func TestCloseWithReasonWritesComment(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	code, _, stderr := run(t, "--repo", dir, "close", id, "--reason", "done", "--author", "jane")
	if code != 0 { t.Fatalf("exit %d stderr %q", code, stderr) }
	got := readItem(t, dir, id)
	if got.Status != item.StatusClosed || len(got.Refs) != 1 || !strings.HasPrefix(got.Refs[0], "../comments/"+id+"/") || strings.Contains(got.Refs[0], "\\") { t.Fatalf("status=%q refs=%v", got.Status, got.Refs) }
	ents, err := os.ReadDir(filepath.Join(dir, ".awit", "comments", id))
	if err != nil || len(ents) != 1 { t.Fatalf("comments: %v %v", ents, err) }
	body, err := os.ReadFile(filepath.Join(dir, ".awit", "comments", id, ents[0].Name()))
	if err != nil { t.Fatal(err) }
	if !bytes.Contains(body, []byte("author: jane")) || !bytes.Contains(body, []byte("done")) { t.Fatalf("comment = %s", body) }
}

func TestCloseReasonNeedsAuthor(t *testing.T) {
	dir := initRepo(t)
	t.Setenv("AWIT_AGENT", "")
	if name := gitx.UserName(dir); name != "" { t.Skipf("git user.name is %q", name) }
	seedItem(t, dir, "AWIT-TEST0001", "T", "B.", nil)
	code, _, stderr := run(t, "--repo", dir, "close", "AWIT-TEST0001", "--reason", "done")
	if code != 1 || stderr != "Error: no author; pass --author or set AWIT_AGENT\n" { t.Fatalf("exit %d stderr %q", code, stderr) }
}

func TestReleaseClearsAssignee(t *testing.T) {
	dir := initRepo(t)
	const id = "AWIT-TEST0001"
	seedItem(t, dir, id, "T", "B.", nil)
	st, err := item.Open(dir)
	if err != nil { t.Fatal(err) }
	it, err := st.Load(id)
	if err != nil { t.Fatal(err) }
	ts := time.Date(2026, 9, 17, 14, 32, 5, 0, time.UTC)
	it.SetStatus(item.StatusInProgress)
	it.SetAssignee("bob")
	it.SetClaimedAt(&ts)
	if err := st.Save(it); err != nil { t.Fatal(err) }
	code, _, stderr := run(t, "--repo", dir, "release", id)
	if code != 0 { t.Fatalf("exit %d stderr %q", code, stderr) }
	got := readItem(t, dir, id)
	if got.Status != item.StatusOpen || got.Assignee != "" || got.ClaimedAt != nil { t.Fatalf("status=%q assignee=%q claimed=%v", got.Status, got.Assignee, got.ClaimedAt) }
	raw, err := os.ReadFile(filepath.Join(dir, ".awit", "items", id+".md"))
	if err != nil { t.Fatal(err) }
	if strings.Contains(string(raw), "assignee:") || strings.Contains(string(raw), "claimed_at:") { t.Fatalf("keys not deleted:\n%s", raw) }
}

func TestResolveAuthorPrecedence(t *testing.T) {
	tests := []struct{ name, flag, env, agentID, gitName, want, wantErr string }{
		{name: "flag verbatim", flag: "Jane Doe", want: "Jane Doe"},
		{name: "flag with slash", flag: "human/jane", want: "human/jane"},
		{name: "env prefixed", env: "claude", want: "agent/claude"},
		{name: "env already prefixed", env: "agent/claude", want: "agent/claude"},
		{name: "config prefixed", agentID: "codex", want: "agent/codex"},
		{name: "config already prefixed", agentID: "agent/codex", want: "agent/codex"},
		{name: "git sanitised", gitName: "Jane Doe", want: "jane-doe"},
		{name: "none", wantErr: "no author; pass --author or set AWIT_AGENT"},
		{name: "flag beats env", flag: "x", env: "y", want: "x"},
		{name: "env beats config", env: "e", agentID: "c", want: "agent/e"},
		{name: "config beats git", agentID: "c", gitName: "G", want: "agent/c"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("AWIT_AGENT", tt.env)
			if tt.gitName != "" {
				if _, err := exec.LookPath("git"); err != nil {
					t.Skip("git not installed")
				}
				git := func(args ...string) {
					t.Helper()
					cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
					if out, err := cmd.CombinedOutput(); err != nil {
						t.Fatalf("git %s: %v: %s", args, err, out)
					}
				}
				git("init", "-q")
				git("config", "user.name", tt.gitName)
			}
			if tt.name == "none" && gitx.UserName(dir) != "" {
				t.Skipf("git user.name is %q", gitx.UserName(dir))
			}
			got, err := resolveAuthor(tt.flag, dir, config.Config{AgentID: tt.agentID})
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("got %q err %v, want %q", got, err, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run it, see it fail.**

```bash
go test ./internal/cli -run 'TestUpdate|TestClose|TestRelease|TestResolveAuthor' -v
```

Expected: `undefined: resolveAuthor` (and friends) -
`FAIL github.com/eisenwinter/awit/internal/cli [build failed]`.

- [ ] **Step 3: Implement helpers and commands.**

  `internal/cli/author.go`:

```go
package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/eisenwinter/awit/internal/gitx"
	"github.com/eisenwinter/awit/pkg/config"
	"github.com/eisenwinter/awit/pkg/item"
)

func withAgentPrefix(v string) string {
	if strings.Contains(v, "/") { return v }
	return "agent/" + v
}

func resolveAuthor(flagAuthor, repoRoot string, cfg config.Config) (string, error) {
	if flagAuthor != "" { return flagAuthor, nil }
	if v := os.Getenv("AWIT_AGENT"); v != "" { return withAgentPrefix(v), nil }
	if cfg.AgentID != "" { return withAgentPrefix(cfg.AgentID), nil }
	if name := gitx.UserName(repoRoot); name != "" { return strings.ReplaceAll(strings.ToLower(name), " ", "-"), nil }
	return "", fmt.Errorf("no author; pass --author or set AWIT_AGENT")
}

func loadItem(s *item.Store, id string) (*item.Item, error) {
	it, err := s.Load(id)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("unknown item %s", id)
		}
		var br *item.BrokenError
		if errors.As(err, &br) {
			return nil, fmt.Errorf("%s: %s: %s (fix the file, then retry)", br.Broken.ID, br.Broken.Reason, br.Broken.Detail)
		}
		return nil, err
	}
	return it, nil
}
```

`internal/cli/update.go`:

```go
package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var updateCmd = &cli.Command{
	Name: "update", Usage: "Change fields on an existing item", ArgsUsage: "<id>",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "status"},
		&cli.StringFlag{Name: "brief"},
		&cli.StringFlag{Name: "assign"},
		&cli.StringFlag{Name: "title"},
		&cli.StringSliceFlag{Name: "label", Aliases: []string{"l"}},
		&cli.StringSliceFlag{Name: "unlabel"},
	},
	Action: updateAction,
}

func splitFlagCSV(values []string) []string {
	var out []string
	for _, v := range values {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
	}
	return out
}

func updateAction(_ context.Context, cmd *cli.Command) error {
	id := cmd.Args().First()
	if id == "" { return fmt.Errorf("update needs an item id") }
	add, remove := splitFlagCSV(cmd.StringSlice("label")), splitFlagCSV(cmd.StringSlice("unlabel"))
	if !cmd.IsSet("status") && !cmd.IsSet("brief") && !cmd.IsSet("assign") && !cmd.IsSet("title") && len(add) == 0 && len(remove) == 0 { return fmt.Errorf("nothing to update") }
	s, err := openStore(cmd)
	if err != nil { return err }
	it, err := loadItem(s, id)
	if err != nil { return err }
	if cmd.IsSet("status") {
		st, err := item.ParseStatus(cmd.String("status"))
		if err != nil {
			return err
		}
		it.SetStatus(st)
		if st == item.StatusClosed {
			it.SetClaimedAt(nil)
		}
	}
	if cmd.IsSet("brief") {
		it.SetBrief(cmd.String("brief"))
	}
	if cmd.IsSet("assign") {
		it.SetAssignee(cmd.String("assign"))
	}
	if cmd.IsSet("title") {
		it.SetTitle(cmd.String("title"))
	}
	if len(add) > 0 || len(remove) > 0 {
		seen := map[string]bool{}
		var labels []string
		for _, l := range it.Labels {
			seen[l] = true
			labels = append(labels, l)
		}
		for _, l := range add {
			if !seen[l] {
				seen[l] = true
				labels = append(labels, l)
			}
		}
		drop := map[string]bool{}
		for _, l := range remove {
			drop[l] = true
		}
		kept := []string{}
		for _, l := range labels {
			if !drop[l] {
				kept = append(kept, l)
			}
		}
		it.SetLabels(kept)
	}
	return s.Save(it)
}
```

`internal/cli/close.go`:

```go
package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var closeCmd = &cli.Command{
	Name: "close", Usage: "Mark an item closed and clear its claim", ArgsUsage: "<id>",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "reason"},
		&cli.StringFlag{Name: "author"},
	},
	Action: closeAction,
}

func closeAction(_ context.Context, cmd *cli.Command) error {
	id := cmd.Args().First()
	if id == "" { return fmt.Errorf("close needs an item id") }
	s, err := openStore(cmd)
	if err != nil { return err }
	it, err := loadItem(s, id)
	if err != nil { return err }
	it.SetStatus(item.StatusClosed)
	it.SetClaimedAt(nil)
	if reason := cmd.String("reason"); reason != "" {
		author, err := resolveAuthor(cmd.String("author"), s.Root, s.Config)
		if err != nil {
			return err
		}
		_, err = s.AddComment(it, author, time.Now().UTC(), reason)
		return err
	}
	return s.Save(it)
}
```

`AddComment` already saves. Do not call `gitx.Commit`. Do not clear assignee.

`internal/cli/release.go`:

```go
package cli

import (
	"context"
	"fmt"

	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var releaseCmd = &cli.Command{
	Name: "release", Usage: "Return an item to open and clear its claim", ArgsUsage: "<id>",
	Action: releaseAction,
}

func releaseAction(_ context.Context, cmd *cli.Command) error {
	id := cmd.Args().First()
	if id == "" { return fmt.Errorf("release needs an item id") }
	s, err := openStore(cmd)
	if err != nil { return err }
	it, err := loadItem(s, id)
	if err != nil { return err }
	it.SetStatus(item.StatusOpen)
	it.SetAssignee("")
	it.SetClaimedAt(nil)
	return s.Save(it)
}
```

In `newRoot` Commands, append `updateCmd`, `closeCmd`, `releaseCmd`. Keep
`initCmd`. Include `createCmd` only if that identifier already exists.

- [ ] **Step 4: Pass and commit.**

```bash
go test ./internal/cli -run 'TestUpdate|TestClose|TestRelease|TestResolveAuthor' -v
```

Expected PASS: `TestUpdateStatusOneLineDiff`, `TestUpdateTitleAndBrief`,
`TestUpdateLabelsAddRemove`, `TestUpdateNothing`, `TestUpdateBadStatus`,
`TestUpdateUnknownItem`, `TestUpdateBrokenItem`, `TestCloseClearsClaimedAt`,
`TestCloseWithReasonWritesComment`, `TestCloseReasonNeedsAuthor` (or SKIP),
`TestReleaseClearsAssignee`, `TestResolveAuthorPrecedence`.

```bash
go test ./internal/cli -count=1
gofmt -w internal/cli
git add internal/cli/author.go internal/cli/update.go internal/cli/close.go internal/cli/release.go internal/cli/update_test.go internal/cli/app.go
git commit -m "cli: add update, close, release and author resolution"
```

- [ ] **Step 5: Full check and close.**

```bash
go build ./... && go vet ./... && go test ./internal/cli -count=1
```

`status: closed`, comment file with output, ref, `tickets: close AWIT-0ND56M3G`.

## Acceptance Criteria

- `go test ./internal/cli -run TestUpdateStatusOneLineDiff -v` PASS - file
  differs by exactly one line, `status: in_progress` (phase-1 exit criterion).
- `go test ./internal/cli -run 'TestUpdate|TestClose|TestRelease|TestResolveAuthor' -v`
  - every test in Step 4 PASSes (`TestCloseReasonNeedsAuthor` may SKIP).
- `close.go` does not import `gitx` or call `Commit`.
- `release` deletes `assignee` and `claimed_at` keys.
- `resolveAuthor("Jane Doe", dir, cfg)` → `Jane Doe`; `AWIT_AGENT=claude` →
  `agent/claude`; `AWIT_AGENT=agent/x` → `agent/x`.
- `gofmt -l internal/cli` prints nothing.

## Out of scope

- `awit create` (seed with `item.New`+`Save`). `awit comment` command
  (AWIT-0ND56Y3G reuses `resolveAuthor`/`loadItem`). `next --claim`, git
  commits, graph, format rendering. Clearing assignee on `close` or
  `update --status closed`. Redeclaring `run`, `initRepo`, `readItem`,
  `openStore`, `parseIDList`, `detectFormat`.
