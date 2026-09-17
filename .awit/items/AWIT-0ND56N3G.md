---
id: AWIT-0ND56N3G
title: 'pkg/graph Build: nodes, edges, dangling, broken carry-through + fixtures'
brief: >-
  Implement pkg/graph Build: one Node per item, ID-sorted Order, Deps/Unblocks
  wiring, dangling-dep faults, and carry-through of Broken files into g.Broken
  and g.Faults. Land every testdata/fixtures tree from guide §8. Cycle
  detection, classify, and unblock counts are empty seams called in that order.
status: closed
deps: [AWIT-0ND56E3G]
labels: [phase2, p0]
refs:
  - ../../plan/implementation-guide.md
  - ../../plan/awit-implementation-plan.md
---

## Summary
After this ticket `pkg/graph/graph.go` exists with types `Fault`, `Node`, `Graph`, node methods `Quarantined` / `DepIDs` / `OpenDepIDs`, and `Build`. `Build` creates one node per parsed item, sorts `Order` by ID, wires resolved `Deps`/`Unblocks`, quarantines unresolved deps, carries every `item.Broken` into `g.Broken` and `g.Faults`, then calls `g.detectCycles(); g.classify(); g.countUnblocks()` in that order. Those three methods have empty bodies — they are seams, not stubs; later tickets replace the bodies without changing the call site. All eight guide-§8 fixture trees are committed under `testdata/fixtures/`. Cycle SCCs, ready/blocked classification, unblock counts, `WouldCycle`, `FilterLabels`, and `CriticalPath` are not implemented here.

## Context (read first)
- Guide §4.6 `pkg/graph` — exact exported types and `Build` signature. Copy them. Do not rename fields.
- Guide §1: module `github.com/eisenwinter/awit`; stdlib plus `pkg/item` only in this package; tests are `testing` stdlib, table-driven, no extra deps; never hardcode `/` in filesystem paths — use `path/filepath`.
- Guide §2 unblock count / quarantine: quarantined nodes are excluded from `next`; this ticket only *marks* them by putting `Fault` values on `Node.Faults` (and on `g.Faults`). `UnblockCount` stays 0 until the countUnblocks seam is filled.
- Guide §4.3 `item.Reason` constants used here: `ReasonParse`, `ReasonConflict`, `ReasonIDMismatch`, `ReasonDuplicate`, `ReasonDangling`. `ReasonCycle` is not produced in this ticket.
- Guide §4.3 `Broken`: `ID` is the filename stem, `Path` is absolute, `Detail` is the human sentence from the store.
- Guide §4.4 `item.Open(repoRoot)` opens `repoRoot/.awit`; `LoadAll() ([]*Item, []Broken, error)` never errors on a bad file. Tests load fixtures with `item.Open` then `LoadAll` then `graph.Build`.
- Guide §5: fixtures live at `testdata/fixtures/<name>/.awit/…`. IDs are `AWIT-TEST0001`..`AWIT-TEST00NN`. The `conflicted` fixture contains literal `<<<<<<< HEAD` lines; `.gitattributes` must mark `testdata/fixtures/conflicted/** -merge`.
- Guide §8 fixture catalogue (this ticket creates every row).
- Spec `plan/awit-implementation-plan.md` §Graph engine and §Quarantine: one mechanism covers cycles, dangling deps, unparseable frontmatter, Git conflict markers, and duplicate IDs. Cycles are the next ticket; this ticket does dangling + broken carry-through.
- Dep ticket `AWIT-0ND56E3G` must be `status: closed` before you start (Store `Open` / `LoadAll` / `Broken`).
- Seams: `detectCycles`, `classify`, `countUnblocks` are package-private methods with **empty bodies**. Call them a seam in comments. Do not panic, do not TODO, do not leave them undeclared.

## Files
- Create: `pkg/graph/graph.go`
- Create: `pkg/graph/graph_test.go`
- Create: `.gitattributes` (or append the one line if the file already exists)
- Create: `testdata/fixtures/clean/.awit/config.yaml`
- Create: `testdata/fixtures/clean/.awit/items/AWIT-TEST0001.md`
- Create: `testdata/fixtures/clean/.awit/items/AWIT-TEST0002.md`
- Create: `testdata/fixtures/clean/.awit/items/AWIT-TEST0003.md`
- Create: `testdata/fixtures/clean/.awit/items/AWIT-TEST0004.md`
- Create: `testdata/fixtures/clean/.awit/items/AWIT-TEST0005.md`
- Create: `testdata/fixtures/clean/.awit/items/AWIT-TEST0006.md`
- Create: `testdata/fixtures/cyclic/.awit/config.yaml`
- Create: `testdata/fixtures/cyclic/.awit/items/AWIT-TEST0001.md`
- Create: `testdata/fixtures/cyclic/.awit/items/AWIT-TEST0002.md`
- Create: `testdata/fixtures/cyclic/.awit/items/AWIT-TEST0003.md`
- Create: `testdata/fixtures/cyclic/.awit/items/AWIT-TEST0004.md`
- Create: `testdata/fixtures/cyclic/.awit/items/AWIT-TEST0005.md`
- Create: `testdata/fixtures/dangling/.awit/config.yaml`
- Create: `testdata/fixtures/dangling/.awit/items/AWIT-TEST0001.md`
- Create: `testdata/fixtures/dangling/.awit/items/AWIT-TEST0002.md`
- Create: `testdata/fixtures/conflicted/.awit/config.yaml`
- Create: `testdata/fixtures/conflicted/.awit/items/AWIT-TEST0001.md`
- Create: `testdata/fixtures/conflicted/.awit/items/AWIT-TEST0002.md`
- Create: `testdata/fixtures/duplicate-id/.awit/config.yaml`
- Create: `testdata/fixtures/duplicate-id/.awit/items/AWIT-TEST0001.md`
- Create: `testdata/fixtures/duplicate-id/.awit/items/awit-test0001.md`
- Create: `testdata/fixtures/id-mismatch/.awit/config.yaml`
- Create: `testdata/fixtures/id-mismatch/.awit/items/AWIT-TEST0001.md`
- Create: `testdata/fixtures/parse-error/.awit/config.yaml`
- Create: `testdata/fixtures/parse-error/.awit/items/AWIT-TEST0001.md`
- Create: `testdata/fixtures/loop/.awit/config.yaml`
- Create: `testdata/fixtures/loop/.awit/items/AWIT-TEST0001.md`
- Create: `testdata/fixtures/loop/.awit/items/AWIT-TEST0002.md`
- Create: `testdata/fixtures/loop/.awit/items/AWIT-TEST0003.md`
- Create: `testdata/fixtures/loop/docs/spec.md`

## Interfaces
- Consumes (already implemented by `AWIT-0ND56E3G` / `AWIT-0ND56D3G`, do not reimplement):
  ```go
  package item

  func Open(repoRoot string) (*Store, error)
  func (s *Store) LoadAll() ([]*Item, []Broken, error)

  type Item struct {
      ID        string
      Title     string
      Brief     string
      Status    Status
      Deps      []string
      Labels    []string
      Assignee  string
      ClaimedAt *time.Time
      Refs      []string
      Path      string
  }
  const StatusClosed Status = "closed"

  type Reason string
  const (
      ReasonParse      Reason = "PARSE ERROR"
      ReasonConflict   Reason = "CONFLICT MARKERS"
      ReasonIDMismatch Reason = "ID MISMATCH"
      ReasonDuplicate  Reason = "DUPLICATE ID"
      ReasonDangling   Reason = "DANGLING DEP"
      ReasonCycle      Reason = "CYCLE"
  )
  type Broken struct {
      ID     string
      Path   string
      Reason Reason
      Detail string
  }
  ```
- Produces (verbatim from guide §4.6, this ticket):
  ```go
  package graph

  type Fault struct {
      Reason item.Reason
      IDs    []string
      Detail string
      Fix    string
  }

  type Node struct {
      Item         *item.Item
      Deps         []*Node
      Unblocks     []*Node
      Faults       []Fault
      Ready        bool
      Blocked      bool
      UnblockCount int
  }
  func (n *Node) Quarantined() bool
  func (n *Node) DepIDs() []string
  func (n *Node) OpenDepIDs() []string

  type Graph struct {
      Nodes  map[string]*Node
      Order  []*Node
      Broken []item.Broken
      Faults []Fault
  }

  func Build(items []*item.Item, broken []item.Broken) *Graph
  ```
- Produces (package-private, this ticket):
  ```go
  func (g *Graph) detectCycles()  // empty seam; AWIT-0ND56P3G replaces the body
  func (g *Graph) classify()      // empty seam; AWIT-0ND56Q3G replaces the body
  func (g *Graph) countUnblocks() // empty seam; AWIT-0ND56Q3G replaces the body
  func reasonFix(b item.Broken) string
  func sortFaults(faults []Fault)
  ```
- Not produced here: `Ready`, `Blocked`, `Quarantined`, `Closed`, `CriticalPath`, `WouldCycle`, `FilterLabels`. Declaring them with panic/TODO bodies is forbidden; they land in later tickets.

## Steps

- [ ] **Step 1: Write every fixture and `.gitattributes`.**

  Every fixture uses this exact `config.yaml` (trailing newline). Copy it to each `testdata/fixtures/<name>/.awit/config.yaml`:

  ```yaml
  prefix: AWIT
  stale_claim: 2h
  ```

  Write `.gitattributes` at the repo root (create the file, or append this line if it exists and does not already contain it):

  ```text
  testdata/fixtures/conflicted/** -merge
  ```

  Write each item file with **exactly** the bytes below (trailing newline on every file). Do not reflow briefs, do not drop `refs: []`.

  `testdata/fixtures/clean/.awit/items/AWIT-TEST0001.md`:

  ```markdown
  ---
  id: AWIT-TEST0001
  title: Implement OAuth2 bearer token extraction
  brief: Fix header parsing so URL-safe bearer tokens authenticate.
  status: open
  deps: []
  labels: [auth, p1]
  refs: []
  ---

  ## Summary

  The auth middleware splits on spaces and assumes the token is strict base64.
  URL-safe tokens with `-` and `_` are dropped before signature verification.

  ## Acceptance Criteria

  - Tokens using the URL-safe base64 alphabet authenticate successfully
  - Invalid signatures still return a structured 401
  ```

  `testdata/fixtures/clean/.awit/items/AWIT-TEST0002.md`:

  ```markdown
  ---
  id: AWIT-TEST0002
  title: Update database migration scripts
  brief: Add the missing users.token_hash column and backfill existing rows.
  status: open
  deps: []
  labels: [db]
  refs: []
  ---

  ## Summary

  The token_hash column is referenced by the auth service but never created.

  ## Acceptance Criteria

  - Migration adds users.token_hash
  - Existing rows are backfilled without downtime
  ```

  `testdata/fixtures/clean/.awit/items/AWIT-TEST0003.md`:

  ```markdown
  ---
  id: AWIT-TEST0003
  title: Add E2E auth tests
  brief: Cover login, refresh, and 401 paths against a running gateway.
  status: open
  deps: [AWIT-TEST0001]
  labels: []
  refs: []
  ---

  ## Summary

  Happy-path login is untested. Failures currently show up only in staging.

  ## Acceptance Criteria

  - Login, refresh, and invalid-signature cases are asserted
  - Tests run in CI without a live network
  ```

  `testdata/fixtures/clean/.awit/items/AWIT-TEST0004.md`:

  ```markdown
  ---
  id: AWIT-TEST0004
  title: Rotate API tokens
  brief: Re-issue production API tokens after the extractor fix and E2E coverage.
  status: open
  deps: [AWIT-TEST0001, AWIT-TEST0003]
  labels: [p0]
  refs: []
  ---

  ## Summary

  Tokens minted under the old extractor cannot be distinguished from forgeries
  once the parser is fixed. Rotation is the cutover.

  ## Acceptance Criteria

  - Every production token is re-issued
  - Old tokens are rejected within one TTL
  ```

  `testdata/fixtures/clean/.awit/items/AWIT-TEST0005.md`:

  ```markdown
  ---
  id: AWIT-TEST0005
  title: Write auth middleware spec
  brief: Document the expected Authorization header grammar and error codes.
  status: closed
  deps: []
  labels: []
  refs: []
  ---

  ## Summary

  The middleware spec is the contract the extractor and E2E tests implement.

  ## Acceptance Criteria

  - Header grammar is specified
  - 401 payload shape is specified
  ```

  `testdata/fixtures/clean/.awit/items/AWIT-TEST0006.md`:

  ```markdown
  ---
  id: AWIT-TEST0006
  title: Implement refresh-token rotation
  brief: Issue a new refresh token on every use and revoke the previous one.
  status: in_progress
  deps: [AWIT-TEST0005]
  labels: []
  assignee: agent/claude
  claimed_at: 2026-09-17T14:32:05Z
  refs: []
  ---

  ## Summary

  Reuse of a stolen refresh token currently goes undetected.

  ## Acceptance Criteria

  - Each refresh call mints a new token and invalidates the old one
  - Concurrent reuse of the old token is rejected
  ```

  `testdata/fixtures/cyclic/.awit/items/AWIT-TEST0001.md`:

  ```markdown
  ---
  id: AWIT-TEST0001
  title: Implement OAuth2 bearer token extraction
  brief: Fix header parsing so URL-safe bearer tokens authenticate.
  status: open
  deps: [AWIT-TEST0002]
  labels: []
  refs: []
  ---

  ## Summary

  First vertex of the three-item cycle used by SCC tests.

  ## Acceptance Criteria

  - File parses as an open item depending on AWIT-TEST0002
  ```

  `testdata/fixtures/cyclic/.awit/items/AWIT-TEST0002.md`:

  ```markdown
  ---
  id: AWIT-TEST0002
  title: Add E2E auth tests
  brief: Cover login, refresh, and 401 paths against a running gateway.
  status: open
  deps: [AWIT-TEST0003]
  labels: []
  refs: []
  ---

  ## Summary

  Second vertex of the three-item cycle used by SCC tests.

  ## Acceptance Criteria

  - File parses as an open item depending on AWIT-TEST0003
  ```

  `testdata/fixtures/cyclic/.awit/items/AWIT-TEST0003.md`:

  ```markdown
  ---
  id: AWIT-TEST0003
  title: Rotate API tokens
  brief: Re-issue production API tokens after the extractor fix and E2E coverage.
  status: open
  deps: [AWIT-TEST0001]
  labels: []
  refs: []
  ---

  ## Summary

  Third vertex of the three-item cycle used by SCC tests.

  ## Acceptance Criteria

  - File parses as an open item depending on AWIT-TEST0001
  ```

  `testdata/fixtures/cyclic/.awit/items/AWIT-TEST0004.md`:

  ```markdown
  ---
  id: AWIT-TEST0004
  title: Rewrite the parser in place
  brief: Replace the extractor without a staging flag, then revert if it fails.
  status: open
  deps: [AWIT-TEST0004]
  labels: []
  refs: []
  ---

  ## Summary

  Self-loop used by SCC tests. The item depends on itself.

  ## Acceptance Criteria

  - File parses as an open item whose deps list contains only its own ID
  ```

  `testdata/fixtures/cyclic/.awit/items/AWIT-TEST0005.md`:

  ```markdown
  ---
  id: AWIT-TEST0005
  title: Update the changelog
  brief: Record the extractor fix once the cycle is broken and the work ships.
  status: open
  deps: []
  labels: []
  refs: []
  ---

  ## Summary

  Independent item in the cyclic fixture; it must stay off the quarantine list.

  ## Acceptance Criteria

  - File parses as an open item with no dependencies
  ```

  `testdata/fixtures/dangling/.awit/items/AWIT-TEST0001.md`:

  ```markdown
  ---
  id: AWIT-TEST0001
  title: Implement OAuth2 bearer token extraction
  brief: Fix header parsing so URL-safe bearer tokens authenticate.
  status: open
  deps: [AWIT-TEST9999]
  labels: [auth, p1]
  refs: []
  ---

  ## Summary

  Depends on an ID that has no file. Build must quarantine this node.

  ## Acceptance Criteria

  - File parses; the missing dep is AWIT-TEST9999
  ```

  `testdata/fixtures/dangling/.awit/items/AWIT-TEST0002.md`:

  ```markdown
  ---
  id: AWIT-TEST0002
  title: Add E2E auth tests
  brief: Cover login, refresh, and 401 paths against a running gateway.
  status: open
  deps: [AWIT-TEST0001]
  labels: []
  refs: []
  ---

  ## Summary

  Depends on the dangling item. It is a real node; Build must still wire the edge.

  ## Acceptance Criteria

  - File parses as an open item depending on AWIT-TEST0001
  ```

  `testdata/fixtures/conflicted/.awit/items/AWIT-TEST0001.md` (the `<<<<<<< ` / `=======` / `>>>>>>> ` lines are load-bearing; a line must *start* with those bytes including the space after `<<<<<<< `):

  ```markdown
  ---
  id: AWIT-TEST0001
  title: Implement OAuth2 bearer token extraction
  brief: Fix header parsing so URL-safe bearer tokens authenticate.
  <<<<<<< HEAD
  status: open
  =======
  status: in_progress
  >>>>>>> branch-b
  deps: []
  labels: [auth, p1]
  refs: []
  ---

  ## Summary

  Frontmatter contains Git conflict markers so LoadAll returns ReasonConflict.

  ## Acceptance Criteria

  - LoadAll reports CONFLICT MARKERS for this stem
  ```

  `testdata/fixtures/conflicted/.awit/items/AWIT-TEST0002.md`:

  ```markdown
  ---
  id: AWIT-TEST0002
  title: Update database migration scripts
  brief: Add the missing users.token_hash column and backfill existing rows.
  status: open
  deps: []
  labels: [db]
  refs: []
  ---

  ## Summary

  Sibling of the conflicted file; it must remain a healthy node.

  ## Acceptance Criteria

  - File parses as an open item with no dependencies
  ```

  `testdata/fixtures/duplicate-id/.awit/items/AWIT-TEST0001.md`:

  ```markdown
  ---
  id: AWIT-TEST0001
  title: Implement OAuth2 bearer token extraction
  brief: Fix header parsing so URL-safe bearer tokens authenticate.
  status: open
  deps: []
  labels: [auth, p1]
  refs: []
  ---

  ## Summary

  Upper-case stem of the case-insensitive duplicate pair.

  ## Acceptance Criteria

  - LoadAll reports DUPLICATE ID on this stem
  ```

  `testdata/fixtures/duplicate-id/.awit/items/awit-test0001.md` (id matches this file's stem, so the only fault is DUPLICATE ID, not ID MISMATCH):

  ```markdown
  ---
  id: awit-test0001
  title: Implement OAuth2 bearer token extraction (lowercase copy)
  brief: Fix header parsing so URL-safe bearer tokens authenticate.
  status: open
  deps: []
  labels: [auth, p1]
  refs: []
  ---

  ## Summary

  Lower-case stem of the case-insensitive duplicate pair.

  ## Acceptance Criteria

  - LoadAll reports DUPLICATE ID on this stem
  ```

  `testdata/fixtures/id-mismatch/.awit/items/AWIT-TEST0001.md` (filename stem `AWIT-TEST0001`, id key `AWIT-TEST0009`):

  ```markdown
  ---
  id: AWIT-TEST0009
  title: Implement OAuth2 bearer token extraction
  brief: Fix header parsing so URL-safe bearer tokens authenticate.
  status: open
  deps: []
  labels: [auth, p1]
  refs: []
  ---

  ## Summary

  The id key does not equal the filename stem.

  ## Acceptance Criteria

  - LoadAll reports ID MISMATCH; Broken.ID is the stem AWIT-TEST0001
  ```

  `testdata/fixtures/parse-error/.awit/items/AWIT-TEST0001.md` (`title: [unclosed` is intentional invalid YAML):

  ```markdown
  ---
  id: AWIT-TEST0001
  title: [unclosed
  status: open
  deps: []
  labels: []
  refs: []
  ---

  ## Summary

  Frontmatter is intentionally invalid YAML.

  ## Acceptance Criteria

  - LoadAll reports PARSE ERROR for stem AWIT-TEST0001
  ```

  `testdata/fixtures/loop/.awit/items/AWIT-TEST0001.md`:

  ```markdown
  ---
  id: AWIT-TEST0001
  title: Implement OAuth2 bearer token extraction
  brief: Fix header parsing so URL-safe bearer tokens authenticate.
  status: open
  deps: []
  labels: [auth, p1]
  refs:
    - ../../docs/spec.md
  ---

  ## Summary

  First item of the three-item E2E chain. The spec ref is relative to `.awit/items/`.

  ## Acceptance Criteria

  - File parses with refs containing ../../docs/spec.md
  - Item has no dependencies so it is ready
  ```

  `testdata/fixtures/loop/.awit/items/AWIT-TEST0002.md`:

  ```markdown
  ---
  id: AWIT-TEST0002
  title: Add E2E auth tests
  brief: Cover login, refresh, and 401 paths against a running gateway.
  status: open
  deps: [AWIT-TEST0001]
  labels: [auth]
  refs: []
  ---

  ## Summary

  Second item of the E2E chain; blocked on the extractor.

  ## Acceptance Criteria

  - File parses as an open item depending on AWIT-TEST0001
  ```

  `testdata/fixtures/loop/.awit/items/AWIT-TEST0003.md`:

  ```markdown
  ---
  id: AWIT-TEST0003
  title: Rotate API tokens
  brief: Re-issue production API tokens after the extractor fix and E2E coverage.
  status: open
  deps: [AWIT-TEST0002]
  labels: [p0]
  refs: []
  ---

  ## Summary

  Third item of the E2E chain; blocked on the E2E tests.

  ## Acceptance Criteria

  - File parses as an open item depending on AWIT-TEST0002
  ```

  `testdata/fixtures/loop/docs/spec.md` (fixture **root**, not under `.awit/`; two paragraphs):

  ```markdown
  The Authorization header is Bearer followed by a single token. The token
  uses the URL-safe base64 alphabet and is verified against the signing key.

  A failed verification returns HTTP 401 with a JSON body containing error
  invalid_token. The gateway never returns 500 for a bad header.
  ```

- [ ] **Step 2: Write the failing tests.**

  Create `pkg/graph/graph_test.go`:

  ```go
  package graph

  import (
  	"os"
  	"path/filepath"
  	"slices"
  	"testing"

  	"github.com/eisenwinter/awit/pkg/item"
  )

  func fixtureRoot(t *testing.T, name string) string {
  	t.Helper()
  	return filepath.Join("..", "..", "testdata", "fixtures", name)
  }

  func buildFixture(t *testing.T, name string) *Graph {
  	t.Helper()
  	st, err := item.Open(fixtureRoot(t, name))
  	if err != nil {
  		t.Fatalf("item.Open(%s): %v", name, err)
  	}
  	items, broken, err := st.LoadAll()
  	if err != nil {
  		t.Fatalf("LoadAll(%s): %v", name, err)
  	}
  	return Build(items, broken)
  }

  func nodeIDs(nodes []*Node) []string {
  	out := make([]string, len(nodes))
  	for i, n := range nodes {
  		out[i] = n.Item.ID
  	}
  	return out
  }

  func unblockIDs(n *Node) []string {
  	out := make([]string, len(n.Unblocks))
  	for i, u := range n.Unblocks {
  		out[i] = u.Item.ID
  	}
  	return out
  }

  func depIDs(n *Node) []string {
  	out := make([]string, len(n.Deps))
  	for i, d := range n.Deps {
  		out[i] = d.Item.ID
  	}
  	return out
  }

  func TestBuildClean(t *testing.T) {
  	g := buildFixture(t, "clean")
  	if len(g.Nodes) != 6 {
  		t.Fatalf("len(Nodes) = %d, want 6", len(g.Nodes))
  	}
  	wantOrder := []string{
  		"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0003",
  		"AWIT-TEST0004", "AWIT-TEST0005", "AWIT-TEST0006",
  	}
  	if got := nodeIDs(g.Order); !slices.Equal(got, wantOrder) {
  		t.Fatalf("Order IDs = %v, want %v", got, wantOrder)
  	}
  	n1 := g.Nodes["AWIT-TEST0001"]
  	n3 := g.Nodes["AWIT-TEST0003"]
  	n4 := g.Nodes["AWIT-TEST0004"]
  	n5 := g.Nodes["AWIT-TEST0005"]
  	n6 := g.Nodes["AWIT-TEST0006"]
  	if n1 == nil || n3 == nil || n4 == nil || n5 == nil || n6 == nil {
  		t.Fatalf("missing node in clean graph: %+v", nodeIDs(g.Order))
  	}
  	if got := depIDs(n3); !slices.Equal(got, []string{"AWIT-TEST0001"}) {
  		t.Fatalf("0003.Deps = %v, want [AWIT-TEST0001]", got)
  	}
  	if got := depIDs(n4); !slices.Equal(got, []string{"AWIT-TEST0001", "AWIT-TEST0003"}) {
  		t.Fatalf("0004.Deps = %v, want [AWIT-TEST0001 AWIT-TEST0003]", got)
  	}
  	gotUnblocks := unblockIDs(n1)
  	if !slices.Contains(gotUnblocks, "AWIT-TEST0003") || !slices.Contains(gotUnblocks, "AWIT-TEST0004") {
  		t.Fatalf("0001.Unblocks = %v, want to contain 0003 and 0004", gotUnblocks)
  	}
  	if n1.Quarantined() {
  		t.Fatal("0001 Quarantined() = true, want false")
  	}
  	if n5.Item.Status != item.StatusClosed {
  		t.Fatalf("0005 status = %q, want closed", n5.Item.Status)
  	}
  	if n6.Item.Assignee != "agent/claude" {
  		t.Fatalf("0006 assignee = %q, want agent/claude", n6.Item.Assignee)
  	}
  	if got := n4.DepIDs(); !slices.Equal(got, []string{"AWIT-TEST0001", "AWIT-TEST0003"}) {
  		t.Fatalf("0004.DepIDs() = %v, want sorted [0001 0003]", got)
  	}
  	if got := n4.OpenDepIDs(); !slices.Equal(got, []string{"AWIT-TEST0001", "AWIT-TEST0003"}) {
  		t.Fatalf("0004.OpenDepIDs() = %v, want [0001 0003] (both open)", got)
  	}
  	if got := n6.OpenDepIDs(); len(got) != 0 {
  		t.Fatalf("0006.OpenDepIDs() = %v, want empty (0005 is closed)", got)
  	}
  	if len(g.Broken) != 0 {
  		t.Fatalf("Broken = %d, want 0", len(g.Broken))
  	}
  	if len(g.Faults) != 0 {
  		t.Fatalf("Faults = %d, want 0", len(g.Faults))
  	}
  }

  func TestBuildDangling(t *testing.T) {
  	g := buildFixture(t, "dangling")
  	n1 := g.Nodes["AWIT-TEST0001"]
  	n2 := g.Nodes["AWIT-TEST0002"]
  	if n1 == nil || n2 == nil {
  		t.Fatal("dangling fixture missing 0001 or 0002")
  	}
  	if !n1.Quarantined() {
  		t.Fatal("0001 Quarantined() = false, want true")
  	}
  	if n2.Quarantined() {
  		t.Fatal("0002 Quarantined() = true, want false")
  	}
  	if len(n1.Faults) != 1 {
  		t.Fatalf("0001 Faults = %d, want 1", len(n1.Faults))
  	}
  	f := n1.Faults[0]
  	if f.Reason != item.ReasonDangling {
  		t.Fatalf("reason = %q, want %q", f.Reason, item.ReasonDangling)
  	}
  	wantDetail := "AWIT-TEST0001 depends on unknown AWIT-TEST9999"
  	if f.Detail != wantDetail {
  		t.Fatalf("Detail = %q, want %q", f.Detail, wantDetail)
  	}
  	wantFix := "awit dep rm AWIT-TEST0001 AWIT-TEST9999"
  	if f.Fix != wantFix {
  		t.Fatalf("Fix = %q, want %q", f.Fix, wantFix)
  	}
  	if !slices.Equal(f.IDs, []string{"AWIT-TEST0001"}) {
  		t.Fatalf("IDs = %v, want [AWIT-TEST0001]", f.IDs)
  	}
  	if len(n1.Deps) != 0 {
  		t.Fatalf("0001.Deps = %v, want empty (dangling excluded)", depIDs(n1))
  	}
  	if got := n1.OpenDepIDs(); !slices.Equal(got, []string{"AWIT-TEST9999"}) {
  		t.Fatalf("0001.OpenDepIDs() = %v, want [AWIT-TEST9999]", got)
  	}
  	if got := depIDs(n2); !slices.Equal(got, []string{"AWIT-TEST0001"}) {
  		t.Fatalf("0002.Deps = %v, want [AWIT-TEST0001]", got)
  	}
  	var dangling []Fault
  	for _, ff := range g.Faults {
  		if ff.Reason == item.ReasonDangling {
  			dangling = append(dangling, ff)
  		}
  	}
  	if len(dangling) != 1 {
  		t.Fatalf("g.Faults dangling count = %d, want 1", len(dangling))
  	}
  	if dangling[0].Fix != wantFix {
  		t.Fatalf("g.Faults Fix = %q, want %q", dangling[0].Fix, wantFix)
  	}
  }

  func TestBuildDependsOnBroken(t *testing.T) {
  	items := []*item.Item{{
  		ID:     "AWIT-TEST0002",
  		Title:  "ok",
  		Status: item.StatusOpen,
  		Deps:   []string{"AWIT-TEST0001"},
  	}}
  	broken := []item.Broken{{
  		ID:     "AWIT-TEST0001",
  		Path:   filepath.Join("x", "AWIT-TEST0001.md"),
  		Reason: item.ReasonParse,
  		Detail: "yaml: boom",
  	}}
  	g := Build(items, broken)
  	n := g.Nodes["AWIT-TEST0002"]
  	if n == nil {
  		t.Fatal("missing depender node")
  	}
  	if !n.Quarantined() {
  		t.Fatal("depender Quarantined() = false, want true")
  	}
  	if len(n.Deps) != 0 {
  		t.Fatalf("Deps = %v, want empty (broken ID is not a node)", depIDs(n))
  	}
  	if len(n.Faults) != 1 {
  		t.Fatalf("Faults = %d, want 1", len(n.Faults))
  	}
  	f := n.Faults[0]
  	if f.Reason != item.ReasonDangling {
  		t.Fatalf("reason = %q, want %q", f.Reason, item.ReasonDangling)
  	}
  	wantDetail := "depends on quarantined AWIT-TEST0001"
  	if f.Detail != wantDetail {
  		t.Fatalf("Detail = %q, want %q", f.Detail, wantDetail)
  	}
  	wantFix := "fix AWIT-TEST0001 first"
  	if f.Fix != wantFix {
  		t.Fatalf("Fix = %q, want %q", f.Fix, wantFix)
  	}
  }

  func TestBuildCarriesBroken(t *testing.T) {
  	tests := []struct {
  		fixture string
  		reason  item.Reason
  		id      string
  		fix     func(b item.Broken) string
  		healthy []string
  	}{
  		{
  			fixture: "conflicted",
  			reason:  item.ReasonConflict,
  			id:      "AWIT-TEST0001",
  			fix: func(b item.Broken) string {
  				return "resolve the git conflict in " + b.Path
  			},
  			healthy: []string{"AWIT-TEST0002"},
  		},
  		{
  			fixture: "id-mismatch",
  			reason:  item.ReasonIDMismatch,
  			id:      "AWIT-TEST0001",
  			fix: func(item.Broken) string {
  				return "rename the file or fix the id: key"
  			},
  		},
  		{
  			fixture: "parse-error",
  			reason:  item.ReasonParse,
  			id:      "AWIT-TEST0001",
  			fix: func(item.Broken) string {
  				return "edit the frontmatter until `awit validate` passes"
  			},
  		},
  	}
  	for _, tt := range tests {
  		t.Run(tt.fixture, func(t *testing.T) {
  			g := buildFixture(t, tt.fixture)
  			if len(g.Broken) != 1 {
  				t.Fatalf("len(Broken) = %d, want 1", len(g.Broken))
  			}
  			b := g.Broken[0]
  			if b.ID != tt.id {
  				t.Fatalf("Broken.ID = %q, want %q", b.ID, tt.id)
  			}
  			if b.Reason != tt.reason {
  				t.Fatalf("Broken.Reason = %q, want %q", b.Reason, tt.reason)
  			}
  			var found *Fault
  			for i := range g.Faults {
  				if g.Faults[i].Reason == tt.reason {
  					found = &g.Faults[i]
  					break
  				}
  			}
  			if found == nil {
  				t.Fatalf("no %q fault in g.Faults (%v)", tt.reason, g.Faults)
  			}
  			if !slices.Equal(found.IDs, []string{tt.id}) {
  				t.Fatalf("Fault.IDs = %v, want [%s]", found.IDs, tt.id)
  			}
  			if found.Detail != b.Detail {
  				t.Fatalf("Fault.Detail = %q, want Broken.Detail %q", found.Detail, b.Detail)
  			}
  			wantFix := tt.fix(b)
  			if found.Fix != wantFix {
  				t.Fatalf("Fault.Fix = %q, want %q", found.Fix, wantFix)
  			}
  			if g.Nodes[tt.id] != nil {
  				t.Fatalf("Nodes[%s] exists; Broken files must not become nodes", tt.id)
  			}
  			if got := nodeIDs(g.Order); !slices.Equal(got, tt.healthy) {
  				t.Fatalf("healthy Order = %v, want %v", got, tt.healthy)
  			}
  		})
  	}
  }

  func TestBuildCarriesDuplicate(t *testing.T) {
  	root := fixtureRoot(t, "duplicate-id")
  	ents, err := os.ReadDir(filepath.Join(root, ".awit", "items"))
  	if err != nil {
  		t.Fatal(err)
  	}
  	if len(ents) < 2 {
  		t.Skip("duplicate-id fixture collapsed on a case-insensitive filesystem")
  	}
  	g := buildFixture(t, "duplicate-id")
  	if len(g.Broken) != 2 {
  		t.Fatalf("len(Broken) = %d, want 2", len(g.Broken))
  	}
  	for _, b := range g.Broken {
  		if b.Reason != item.ReasonDuplicate {
  			t.Fatalf("Broken %s reason = %q, want %q", b.ID, b.Reason, item.ReasonDuplicate)
  		}
  	}
  	var n int
  	for _, f := range g.Faults {
  		if f.Reason != item.ReasonDuplicate {
  			continue
  		}
  		n++
  		if f.Fix != "rename one of the files" {
  			t.Fatalf("Fix = %q, want %q", f.Fix, "rename one of the files")
  		}
  	}
  	if n != 2 {
  		t.Fatalf("duplicate faults = %d, want 2", n)
  	}
  	if len(g.Nodes) != 0 {
  		t.Fatalf("len(Nodes) = %d, want 0", len(g.Nodes))
  	}
  }

  func TestFaultsSorted(t *testing.T) {
  	items := []*item.Item{{
  		ID:     "AWIT-TEST0002",
  		Title:  "dangling",
  		Status: item.StatusOpen,
  		Deps:   []string{"AWIT-MISSING"},
  	}}
  	broken := []item.Broken{
  		{ID: "AWIT-TEST0005", Reason: item.ReasonConflict, Detail: "c5", Path: "p5"},
  		{ID: "AWIT-TEST0001", Reason: item.ReasonConflict, Detail: "c1", Path: "p1"},
  		{ID: "AWIT-TEST0004", Reason: item.ReasonIDMismatch, Detail: "m", Path: "p4"},
  		{ID: "AWIT-TEST0003", Reason: item.ReasonParse, Detail: "p", Path: "p3"},
  	}
  	g := Build(items, broken)
  	got := make([]string, len(g.Faults))
  	for i, f := range g.Faults {
  		got[i] = string(f.Reason) + ":" + f.IDs[0]
  	}
  	want := []string{
  		"CONFLICT MARKERS:AWIT-TEST0001",
  		"CONFLICT MARKERS:AWIT-TEST0005",
  		"DANGLING DEP:AWIT-TEST0002",
  		"ID MISMATCH:AWIT-TEST0004",
  		"PARSE ERROR:AWIT-TEST0003",
  	}
  	if !slices.Equal(got, want) {
  		t.Fatalf("Faults = %v, want %v", got, want)
  	}
  }

  func TestNodeDepIDsSorted(t *testing.T) {
  	a := &item.Item{ID: "AWIT-TEST0001", Title: "a", Status: item.StatusOpen, Deps: []string{"AWIT-TEST0003", "AWIT-TEST0002"}}
  	b := &item.Item{ID: "AWIT-TEST0002", Title: "b", Status: item.StatusOpen}
  	c := &item.Item{ID: "AWIT-TEST0003", Title: "c", Status: item.StatusClosed}
  	g := Build([]*item.Item{a, b, c}, nil)
  	n := g.Nodes["AWIT-TEST0001"]
  	if got := n.DepIDs(); !slices.Equal(got, []string{"AWIT-TEST0002", "AWIT-TEST0003"}) {
  		t.Fatalf("DepIDs() = %v, want sorted", got)
  	}
  	if got := n.OpenDepIDs(); !slices.Equal(got, []string{"AWIT-TEST0002"}) {
  		t.Fatalf("OpenDepIDs() = %v, want [0002] (0003 is closed)", got)
  	}
  	if got := depIDs(n); !slices.Equal(got, []string{"AWIT-TEST0003", "AWIT-TEST0002"}) {
  		t.Fatalf("Node.Deps order = %v, want Item.Deps order", got)
  	}
  }

  func TestLoopFixtureLoads(t *testing.T) {
  	g := buildFixture(t, "loop")
  	if got := nodeIDs(g.Order); !slices.Equal(got, []string{"AWIT-TEST0001", "AWIT-TEST0002", "AWIT-TEST0003"}) {
  		t.Fatalf("Order = %v", got)
  	}
  	n1 := g.Nodes["AWIT-TEST0001"]
  	if !slices.Equal(n1.Item.Refs, []string{"../../docs/spec.md"}) {
  		t.Fatalf("0001.Refs = %v, want [../../docs/spec.md]", n1.Item.Refs)
  	}
  	if got := depIDs(g.Nodes["AWIT-TEST0002"]); !slices.Equal(got, []string{"AWIT-TEST0001"}) {
  		t.Fatalf("0002.Deps = %v", got)
  	}
  	if got := depIDs(g.Nodes["AWIT-TEST0003"]); !slices.Equal(got, []string{"AWIT-TEST0002"}) {
  		t.Fatalf("0003.Deps = %v", got)
  	}
  }
  ```

- [ ] **Step 3: Run it, see it fail.**

  ```bash
  go test ./pkg/graph -run 'TestBuildClean|TestBuildDangling|TestBuildCarriesBroken|TestFaultsSorted' -v
  ```

  Expected failure (no `graph.go` yet, so `Build` is undefined):

  ```text
  # github.com/eisenwinter/awit/pkg/graph [github.com/eisenwinter/awit/pkg/graph.test]
  pkg/graph/graph_test.go: undefined: Build
  FAIL	github.com/eisenwinter/awit/pkg/graph [build failed]
  ```

- [ ] **Step 4: Implement types, node methods, Build, and the three empty seams.**

  Create `pkg/graph/graph.go` with exactly this behaviour (names and call order are load-bearing):

  ```go
  // Package graph builds the in-memory dependency graph from parsed items.
  package graph

  import (
  	"sort"
  	"strings"

  	"github.com/eisenwinter/awit/pkg/item"
  )

  type Fault struct {
  	Reason item.Reason
  	IDs    []string
  	Detail string
  	Fix    string
  }

  type Node struct {
  	Item         *item.Item
  	Deps         []*Node
  	Unblocks     []*Node
  	Faults       []Fault
  	Ready        bool
  	Blocked      bool
  	UnblockCount int
  }

  func (n *Node) Quarantined() bool {
  	return len(n.Faults) > 0
  }

  func (n *Node) DepIDs() []string {
  	out := append([]string(nil), n.Item.Deps...)
  	sort.Strings(out)
  	return out
  }

  func (n *Node) OpenDepIDs() []string {
  	resolved := make(map[string]*Node, len(n.Deps))
  	for _, d := range n.Deps {
  		resolved[d.Item.ID] = d
  	}
  	var out []string
  	for _, id := range n.Item.Deps {
  		d, ok := resolved[id]
  		if !ok {
  			out = append(out, id)
  			continue
  		}
  		if d.Quarantined() || d.Item.Status != item.StatusClosed {
  			out = append(out, id)
  		}
  	}
  	sort.Strings(out)
  	return out
  }

  type Graph struct {
  	Nodes  map[string]*Node
  	Order  []*Node
  	Broken []item.Broken
  	Faults []Fault
  }

  // detectCycles, classify, and countUnblocks are seams. Build always calls
  // them in this order. Later tickets replace the bodies; do not change the
  // call site.
  func (g *Graph) detectCycles()  {}
  func (g *Graph) classify()      {}
  func (g *Graph) countUnblocks() {}

  func reasonFix(b item.Broken) string {
  	switch b.Reason {
  	case item.ReasonParse:
  		return "edit the frontmatter until `awit validate` passes"
  	case item.ReasonConflict:
  		return "resolve the git conflict in " + b.Path
  	case item.ReasonIDMismatch:
  		return "rename the file or fix the id: key"
  	case item.ReasonDuplicate:
  		return "rename one of the files"
  	default:
  		return ""
  	}
  }

  func sortFaults(fs []Fault) {
  	sort.SliceStable(fs, func(i, j int) bool {
  		if fs[i].Reason != fs[j].Reason {
  			return fs[i].Reason < fs[j].Reason
  		}
  		return strings.Join(fs[i].IDs, ",") < strings.Join(fs[j].IDs, ",")
  	})
  }

  func Build(items []*item.Item, broken []item.Broken) *Graph {
  	g := &Graph{
  		Nodes:  make(map[string]*Node, len(items)),
  		Broken: append([]item.Broken(nil), broken...),
  	}
  	for _, it := range items {
  		n := &Node{Item: it}
  		g.Nodes[it.ID] = n
  		g.Order = append(g.Order, n)
  	}
  	sort.Slice(g.Order, func(i, j int) bool {
  		return g.Order[i].Item.ID < g.Order[j].Item.ID
  	})

  	brokenByID := make(map[string]item.Broken, len(broken))
  	for _, b := range broken {
  		brokenByID[b.ID] = b
  		g.Faults = append(g.Faults, Fault{
  			Reason: b.Reason,
  			IDs:    []string{b.ID},
  			Detail: b.Detail,
  			Fix:    reasonFix(b),
  		})
  	}

  	for _, n := range g.Order {
  		for _, depID := range n.Item.Deps {
  			if dep, ok := g.Nodes[depID]; ok {
  				n.Deps = append(n.Deps, dep)
  				dep.Unblocks = append(dep.Unblocks, n)
  				continue
  			}
  			f := Fault{
  				Reason: item.ReasonDangling,
  				IDs:    []string{n.Item.ID},
  			}
  			if _, ok := brokenByID[depID]; ok {
  				f.Detail = "depends on quarantined " + depID
  				f.Fix = "fix " + depID + " first"
  			} else {
  				f.Detail = n.Item.ID + " depends on unknown " + depID
  				f.Fix = "awit dep rm " + n.Item.ID + " " + depID
  			}
  			n.Faults = append(n.Faults, f)
  			g.Faults = append(g.Faults, f)
  		}
  	}

  	for _, n := range g.Order {
  		sort.Slice(n.Unblocks, func(i, j int) bool {
  			return n.Unblocks[i].Item.ID < n.Unblocks[j].Item.ID
  		})
  	}

  	g.detectCycles()
  	g.classify()
  	g.countUnblocks()
  	sortFaults(g.Faults)
  	return g
  }
  ```

  Rules the implementation must keep:
  - `Node.Deps` follows `Item.Deps` order and **omits** unknown / Broken IDs.
  - `Unblocks` is the reverse edge; after wiring, sort each node's `Unblocks` by ID.
  - One `Fault` per unresolved dep, attached to **both** `n.Faults` (so `Quarantined()` is true) and `g.Faults`.
  - Unknown dep Detail is `<id> depends on unknown <dep>` and Fix is `awit dep rm <id> <dep>` with no extra words.
  - Dep ID present in `broken` (not in `Nodes`): Detail is exactly `depends on quarantined <dep>` and Fix is exactly `fix <dep> first`.
  - Broken files never become nodes. `g.Broken` is a copy of the input slice.
  - `g.Faults` is sorted by `Reason` (string order) then `strings.Join(IDs, ",")`. Sort **after** the three seam calls so later tickets can append and still get a stable order.
  - `Build` never returns a nil `*Graph`. `Nodes` is a non-nil map even when `items` is empty.
  - The three seam methods have empty bodies. Do not put cycle/ready/unblock logic in them.

- [ ] **Step 5: Run the tests, see them pass, commit.**

  ```bash
  go test ./pkg/graph -v
  ```

  Expected:

  ```text
  === RUN   TestBuildClean
  --- PASS: TestBuildClean
  === RUN   TestBuildDangling
  --- PASS: TestBuildDangling
  === RUN   TestBuildDependsOnBroken
  --- PASS: TestBuildDependsOnBroken
  === RUN   TestBuildCarriesBroken
  === RUN   TestBuildCarriesBroken/conflicted
  === RUN   TestBuildCarriesBroken/id-mismatch
  === RUN   TestBuildCarriesBroken/parse-error
  --- PASS: TestBuildCarriesBroken
  === RUN   TestBuildCarriesDuplicate
  --- PASS: TestBuildCarriesDuplicate
  === RUN   TestFaultsSorted
  --- PASS: TestFaultsSorted
  === RUN   TestNodeDepIDsSorted
  --- PASS: TestNodeDepIDsSorted
  === RUN   TestLoopFixtureLoads
  --- PASS: TestLoopFixtureLoads
  PASS
  ok  	github.com/eisenwinter/awit/pkg/graph
  ```

  (`TestBuildCarriesDuplicate` may skip on a case-insensitive filesystem; that is a pass.)

  ```bash
  gofmt -w pkg/graph/graph.go pkg/graph/graph_test.go
  git add pkg/graph testdata/fixtures .gitattributes
  git commit -m "graph: Build nodes, edges, dangling faults, broken carry-through"
  ```

  `gofmt -l pkg/graph` must print nothing.

## Acceptance Criteria
- `go test ./pkg/graph -count=1` passes.
- `Build` of the clean fixture yields 6 nodes; `Order` is ID-ascending; `0003.Deps` is `[0001]`; `0001.Unblocks` contains `0003` and `0004`.
- `Build` of the dangling fixture quarantines `0001` with exactly one `DANGLING DEP` fault, Detail `AWIT-TEST0001 depends on unknown AWIT-TEST9999`, Fix `awit dep rm AWIT-TEST0001 AWIT-TEST9999`. `0002` is a non-quarantined node whose `Deps` points at `0001`.
- A hand-built item whose dep ID is in `broken` gets Detail `depends on quarantined <dep>` and Fix `fix <dep> first`.
- `Build` of conflicted / id-mismatch / parse-error copies `Broken` into `g.Broken` and `g.Faults` with Fix `resolve the git conflict in <path>`, `rename the file or fix the id: key`, and `edit the frontmatter until \`awit validate\` passes` respectively. Duplicate-id yields two `DUPLICATE ID` faults, Fix `rename one of the files`, skipped on case-insensitive FS.
- `g.Faults` is ordered by Reason then joined IDs (`TestFaultsSorted`).
- `Build` calls `detectCycles`, `classify`, `countUnblocks` in that order. Those three methods have empty bodies in this ticket.
- All eight fixture trees from guide §8 exist with the bytes in Step 1, including `testdata/fixtures/loop/docs/spec.md` and `.gitattributes` `testdata/fixtures/conflicted/** -merge`.

## Out of scope
- Filling `detectCycles` (Tarjan / `ReasonCycle`) — `AWIT-0ND56P3G`.
- Filling `classify` / `countUnblocks`, and adding `Ready` / `Blocked` / `Quarantined` / `Closed` / `WouldCycle` / `FilterLabels` — `AWIT-0ND56Q3G`.
- `CriticalPath` — `AWIT-0ND56V3G`.
- CLI commands (`dep`, `validate`, `prime`, `next`, `list`).
- Changing `pkg/item` or fixture IDs away from `AWIT-TEST000N`.
