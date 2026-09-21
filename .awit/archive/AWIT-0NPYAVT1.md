---
id: AWIT-0NPYAVT1
title: 'graph/cli: expose the transitive unblock set'
brief: >-
  Unblocks: N is printed by list, next and prime, but the set behind N is printed nowhere, even though reachableUnblocks already walks it and throws it away. Returns that set as graph.ReachableUnblocks and adds an awit show --unblocks view.
status: closed
deps: []
labels: [phase7, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

`Unblocks: N` appears in `list`, `next` and `prime`, but the set behind `N` is printed
nowhere. `reachableUnblocks` already walks exactly that set and returns only its length.
This exports it as `graph.ReachableUnblocks` returning the nodes, and adds
`awit show <id> --unblocks` — answering "what does finishing this actually release".

`--unblocks` is its own view, like `--refs-only`, so no existing golden changes.

## Context (read first)

- `plan/implementation-guide.md` §1 — global constraints (Linux **and** Windows, deterministic output, stdlib-only tests, commit message format).
- `pkg/graph/rank.go:44-71` — `countUnblocks` (line 44) and the `reachableUnblocks` BFS (lines 54-71) being widened. `sort` is already imported.
- `pkg/graph/rank_test.go` — `TestUnblockCountsIgnoreClosedDownstream` shows the construction style: `&item.Item{…}` literals into `Build(items, nil)`. There is no `mkItem`/`buildGraph` helper; do not add one. `slices` is already imported at line 4.
- `internal/cli/show.go:71-99` — the view dispatch. **The `--format json` branch sits before `refs-only` and `full`**, so the insertion point decides behaviour.
- `internal/cli/show_full_test.go:157` — asserts `"Error: pass either --full or --refs-only\n"` verbatim. The existing guard must not be touched.

## Files

- `pkg/graph/rank.go` — `reachableUnblocks` → exported `ReachableUnblocks` returning `[]*Node`; `countUnblocks` takes `len()`.
- `pkg/graph/rank_test.go` — one new test.
- `internal/cli/show.go` — `--unblocks` flag and view.
- `internal/cli/show_test.go` — three new tests plus a `mkChain` helper.

## Interfaces

```go
// ReachableUnblocks returns the unique non-closed, non-quarantined nodes reachable
// from start via Unblocks edges, sorted by ID. Closed and quarantined nodes are
// walked through but never returned. UnblockCount is its length.
func ReachableUnblocks(start *Node) []*Node
```

## Steps

- [ ] Append to `pkg/graph/rank_test.go`:

```go
func TestReachableUnblocksReturnsTheCountedSet(t *testing.T) {
	// Unblocks edges: A → B → C, and A → D(closed) → E.
	// From A the set is {B, C, E}: closed D is walked through but never
	// returned, so it cannot hide open E.
	a := &item.Item{ID: "AWIT-TEST0001", Title: "A", Status: item.StatusOpen}
	b := &item.Item{ID: "AWIT-TEST0002", Title: "B", Status: item.StatusOpen, Deps: []string{"AWIT-TEST0001"}}
	c := &item.Item{ID: "AWIT-TEST0003", Title: "C", Status: item.StatusOpen, Deps: []string{"AWIT-TEST0002"}}
	d := &item.Item{ID: "AWIT-TEST0004", Title: "D", Status: item.StatusClosed, Deps: []string{"AWIT-TEST0001"}}
	e := &item.Item{ID: "AWIT-TEST0005", Title: "E", Status: item.StatusOpen, Deps: []string{"AWIT-TEST0004"}}
	g := Build([]*item.Item{a, b, c, d, e}, nil)

	var ids []string
	for _, n := range ReachableUnblocks(g.Nodes["AWIT-TEST0001"]) {
		ids = append(ids, n.Item.ID)
	}
	want := []string{"AWIT-TEST0002", "AWIT-TEST0003", "AWIT-TEST0005"}
	if !slices.Equal(ids, want) {
		t.Errorf("ReachableUnblocks = %v, want %v (sorted by ID, closed D excluded)", ids, want)
	}
	if n := g.Nodes["AWIT-TEST0001"].UnblockCount; n != len(want) {
		t.Errorf("UnblockCount = %d, want %d: it must equal len(ReachableUnblocks)", n, len(want))
	}
}
```

- [ ] Run `go test ./pkg/graph/ -run TestReachableUnblocks -v` — see it FAIL: `undefined: ReachableUnblocks`
- [ ] Replace `reachableUnblocks` at `pkg/graph/rank.go:54-71` with:

```go
// ReachableUnblocks returns the unique non-closed, non-quarantined nodes
// reachable from start via Unblocks edges, sorted by ID. The walk passes
// through closed and quarantined nodes so a closed middle node does not hide
// an open descendant; those nodes are walked but never returned.
// UnblockCount is len(ReachableUnblocks(n)) for every non-quarantined node.
func ReachableUnblocks(start *Node) []*Node {
	seen := map[string]bool{start.Item.ID: true}
	var out []*Node
	queue := append([]*Node(nil), start.Unblocks...)
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		if seen[n.Item.ID] {
			continue
		}
		seen[n.Item.ID] = true
		if !n.Quarantined() && n.Item.Status != item.StatusClosed {
			out = append(out, n)
		}
		queue = append(queue, n.Unblocks...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Item.ID < out[j].Item.ID })
	return out
}
```

- [ ] At `rank.go:50` inside `countUnblocks`, set `n.UnblockCount = len(ReachableUnblocks(n))`
- [ ] Run `go test ./pkg/graph/ -v` — PASS, every existing unblock-count assertion unchanged; only the return type moved
- [ ] Append to `internal/cli/show_test.go`:

```go
// mkChain creates Root ← Mid ← Leaf with deterministic IDs.
func mkChain(t *testing.T) string {
	t.Helper()
	dir := initRepo(t)
	steps := [][]string{
		{"create", "--brief", "B.", "--id", "AWIT-TEST0001", "Root"},
		{"create", "--brief", "B.", "--id", "AWIT-TEST0002", "-d", "AWIT-TEST0001", "Mid"},
		{"create", "--brief", "B.", "--id", "AWIT-TEST0003", "-d", "AWIT-TEST0002", "Leaf"},
	}
	for _, s := range steps {
		if code, _, stderr := run(t, append([]string{"--repo", dir}, s...)...); code != 0 {
			t.Fatalf("%v: exit %d stderr %q", s, code, stderr)
		}
	}
	return dir
}

func TestShowUnblocksListsTheTransitiveSet(t *testing.T) {
	dir := mkChain(t)

	code, stdout, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0001", "--unblocks", "--format", "compact")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	for _, want := range []string{"AWIT-TEST0002", "AWIT-TEST0003"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout %q missing %s", stdout, want)
		}
	}
	if strings.Contains(stdout, "AWIT-TEST0001") {
		t.Errorf("stdout %q must not list the item itself", stdout)
	}
}

func TestShowUnblocksEmptySetPrintsNothing(t *testing.T) {
	dir := mkChain(t)

	// The leaf unblocks nothing.
	code, stdout, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0003", "--unblocks", "--format", "compact")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

func TestShowUnblocksRejectsCombinedViews(t *testing.T) {
	dir := mkChain(t)

	code, _, stderr := run(t, "--repo", dir, "show", "AWIT-TEST0001", "--unblocks", "--full")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stderr %q)", code, stderr)
	}
}
```

- [ ] Run `go test ./internal/cli/ -run TestShowUnblocks -v` — see it FAIL: `flag provided but not defined: -unblocks`
- [ ] Add to the `showCmd` `Flags` slice in `internal/cli/show.go`:

```go
		&cli.BoolFlag{Name: "unblocks", Usage: "list the open items this one transitively unblocks, instead of the item"},
```

- [ ] Insert the view in `showOne` **immediately after `n := g.Nodes[id]` (show.go:71), before `itemsDir := s.ItemsDir()`**:

```go
	if cmd.Bool("unblocks") {
		if cmd.Bool("full") || cmd.Bool("refs-only") {
			return cli.Exit("Error: --unblocks cannot be combined with --full or --refs-only", 2)
		}
		var entries []format.Entry
		for _, u := range graph.ReachableUnblocks(n) {
			entries = append(entries, toEntry(u))
		}
		return format.Write(cmd.Root().Writer, f, entries)
	}
```

  **The anchor is not negotiable.** `showOne`'s dispatch order is:

```
:71  n := g.Nodes[id]
:79  if refsOnly && full   → exit 2
:82  if f == format.JSON   → showJSON envelope, RETURNS
:90  if refsOnly           → view
:95  if full               → view
:99  defaultView
```

  The JSON branch sits **before** `refsOnly`/`full`. Placed anywhere after line 82,
  `--unblocks --format json` silently emits the `showJSON` envelope instead of the row
  array — and none of the three tests above would catch it, since none use `--format json`.
  Two details follow: `refsOnly`/`full` are not bound until lines 77-78, so the guard reads
  `cmd.Bool(…)` directly; and the guard carries the `Error: ` prefix inline to match the
  existing guard at line 80, which **must not** be folded into, because
  `show_full_test.go:157` asserts its exact string.

- [ ] Run `go test ./internal/cli/ -run TestShow -v` — PASS, existing show tests included
- [ ] Run `go test ./... && go vet ./...` — PASS
- [ ] Commit: `cli/show: add --unblocks listing the transitive unblock set`

## Acceptance Criteria

- For `Root ← Mid ← Leaf`, `awit show <root> --unblocks --format compact` lists Mid and Leaf and never the root itself.
- `awit show <leaf> --unblocks --format compact` writes nothing to stdout, exit 0.
- `awit show <id> --unblocks --full` exits 2.
- `awit show <id> --unblocks --format json` emits a bare entry array, not the `showJSON` envelope.
- `go test ./pkg/graph/ -v` passes with every pre-existing unblock-count assertion unchanged.
- `show_full_test.go:157` still passes — its guard string is untouched.

Intended edge cases, no test required: `show <broken-file-id> --unblocks` prints the
broken-file view and ignores the flag (broken resolution returns before line 71); a
quarantined start node still lists its downstream set.

## Out of scope

- `--unblocks` on `list` or `next`.
- Any change to `UnblockCount` semantics or to ranking.


## Comments

### 2026-09-21T13:56:27Z agent/orchestrator

ReachableUnblocks exported from the counting BFS; show --unblocks lists the transitive set as its own view

### 2026-09-21T13:56:27Z agent/orchestrator

Implemented by UnblocksWorker (TDD: both red steps observed, then green; go test ./... and vet clean; smoke: --unblocks --format json bare entry array, not the showJSON envelope). Reviewed by reviewer agent: SATISFIED, zero findings — view anchored before the JSON branch, refs-only flag restoration exact, guard string untouched, gofmt clean.
