---
id: AWIT-0NPYAWTA
title: 'cli/update: add --body and --body-file'
brief: >-
  A work item body can only be corrected by editing the file directly, contradicting the skill's core rule that every state change goes through the CLI. Adds update --body and --body-file, replacing the body while leaving frontmatter untouched.
status: open
deps: [AWIT-0NPYAWT8]
labels: [phase7, p1]
refs_base: repo
refs: []
---

## Summary

A work item body can only be corrected by editing `.awit/items/<id>.md` directly, which is
the one carve-out in the driving-awit core rule that every state change goes through the
CLI. `update --body` and `--body-file` close it: the body below the closing frontmatter
fence is replaced, frontmatter untouched, so minimal-diff behaviour is preserved.

Without this, an agent that fills a body wrong at create time has no repair path short of a
human or close-and-recreate.

## Context (read first)

- `plan/implementation-guide.md` §1 — global constraints; §5 — flags are detected by value, never `cmd.IsSet`.
- **`internal/cli/update.go:70-72`** — the `nothing to update` guard. It tests ten flag values, returns `fmt.Errorf("nothing to update")` (exit 1), and runs **before** `openStore`. A `--body`-only update hits it and dies unless the guard is extended. This is the one thing that will silently break this item.
- `internal/cli/update.go:95` — `// Echo order is fixed: status, title, brief, assignee, labels, alias, external.` Goes stale when the body case lands.
- `internal/cli/update.go:116` — the `brief` case; the body case goes directly after it.
- `pkg/item/item.go` — `SetBody` owns a copy and never touches the YAML node, so frontmatter goldens are unaffected.
- Depends on `resolveBodyFlags` and `validateBodyBytes` from the create-body item.

## Files

- `internal/cli/update.go` — two flags, body resolution above the guard, the guard condition, the setter, the comment.
- `internal/cli/update_test.go` — four new tests.

## Interfaces

None new. Reuses `resolveBodyFlags`.

## Steps

- [ ] Append to `internal/cli/update_test.go`:

```go
func TestUpdateBodyReplacesBodyOnly(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "keep me", "--id", "AWIT-TEST0001", "-l", "auth", "T")
	if code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}

	code, _, stderr = run(t, "--repo", dir, "update", "AWIT-TEST0001", "--body", "## Replaced\n")
	if code != 0 {
		t.Fatalf("update: exit %d stderr %q", code, stderr)
	}

	it := readItem(t, dir, "AWIT-TEST0001")
	if got := string(it.Body()); got != "## Replaced\n" {
		t.Errorf("body = %q, want %q", got, "## Replaced\n")
	}
	if it.Brief != "keep me" {
		t.Errorf("brief = %q, want it untouched", it.Brief)
	}
	if len(it.Labels) != 1 || it.Labels[0] != "auth" {
		t.Errorf("labels = %v, want [auth] untouched", it.Labels)
	}
}

func TestUpdateBodyFileFromStdin(t *testing.T) {
	dir := initRepo(t)
	if code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", "AWIT-TEST0001", "T"); code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}

	code, _, stderr := runStdin(t, "## Piped\n", "--repo", dir, "update", "AWIT-TEST0001", "--body-file", "-")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if got := string(readItem(t, dir, "AWIT-TEST0001").Body()); got != "## Piped\n" {
		t.Errorf("body = %q, want %q", got, "## Piped\n")
	}
}

func TestUpdateBodyRefusesConflictMarkersWithoutWriting(t *testing.T) {
	dir := initRepo(t)
	if code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", "AWIT-TEST0001", "T"); code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}
	before := string(readItem(t, dir, "AWIT-TEST0001").Body())

	marker := strings.Repeat("<", 7) + " HEAD\nx\n" + strings.Repeat("=", 7) + "\ny\n" + strings.Repeat(">", 7) + " other\n"
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--body", marker)
	if code == 0 {
		t.Fatalf("exit = 0, want non-zero")
	}
	if !strings.Contains(stderr, "conflict markers") {
		t.Errorf("stderr = %q, want it to mention conflict markers", stderr)
	}
	if got := string(readItem(t, dir, "AWIT-TEST0001").Body()); got != before {
		t.Errorf("body changed to %q, want it unwritten", got)
	}
}

func TestUpdateBodyFlagsAreMutuallyExclusive(t *testing.T) {
	dir := initRepo(t)
	if code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", "AWIT-TEST0001", "T"); code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}

	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--body", "x", "--body-file", "y.md")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stderr %q)", code, stderr)
	}
}
```

- [ ] Run `go test ./internal/cli/ -run TestUpdateBody -v` — see it FAIL: `flag provided but not defined: -body`
- [ ] Add to the `updateCmd` `Flags` slice after the `brief` flag:

```go
		&cli.StringFlag{Name: "body", Usage: "replace the item body below the frontmatter, verbatim"},
		&cli.StringFlag{Name: "body-file", Usage: "replace the body from `PATH`, or - for stdin"},
```

- [ ] In `updateAction`, call `resolveBodyFlags` **above** the `nothing to update` guard at line 70:

```go
	body, err := resolveBodyFlags(cmd)
	if err != nil {
		return err
	}
```

  and add `body == nil` to that guard's condition. Without this a `--body`-only update exits
  1 with `Error: nothing to update` before any of the new code runs. The guard sits before
  `openStore` and `resolveBodyFlags` reads only flags and `cmd.Root().Reader`, so the order
  is safe and a usage error still costs nothing.

- [ ] Add the setter directly after the `brief` case at line 116:

```go
	if body != nil {
		it.SetBody(body)
		changed = append(changed, fmt.Sprintf("body=%d bytes", len(body)))
	}
```

  `fmt` is already imported.

- [ ] Update the echo-order comment at line 95 to: `// Echo order is fixed: status, title, brief, body, assignee, labels, alias, external.`
- [ ] Run `go test ./internal/cli/ -run TestUpdate -v` — PASS, every pre-existing update test and minimal-diff golden included
- [ ] Commit: `cli/update: add --body and --body-file`

## Acceptance Criteria

- A `--body`-only update **succeeds** — it must not hit `nothing to update`.
- `update <id> --body "## Replaced\n"` replaces the body and leaves `brief` and `labels` byte-identical.
- `update <id> --body-file -` stores piped stdin.
- `update <id> --body` containing conflict markers exits non-zero and the item on disk is unchanged.
- `update <id> --body x --body-file y.md` exits 2.
- Existing update goldens are unchanged.

## Out of scope

- Appending to a body rather than replacing it.
- Any frontmatter behaviour change; `--body` never pushes to a linked tracker.

