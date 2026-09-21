---
id: AWIT-0NPYAWT8
title: 'cli/create: add --body and --body-file'
brief: >-
  create can only take its body from config.template or the built-in skeleton, so a filled-in body must be written by hand afterwards. Adds --body and --body-file, mutually exclusive and resolved before the lock and the mint.
status: closed
deps: [AWIT-0NPYAVTA]
labels: [phase7, p0]
refs_base: repo
refs: [.awit/comments/AWIT-0NPYAWT8/20260921T140830Z-orchestrator.md, .awit/comments/AWIT-0NPYAWT8/20260921T140830Z-orchestrator-2.md]
assignee: agent/orchestrator
---

## Summary

`create` sources its body only from `config.template` or the built-in skeleton, so a
filled-in body must be written into `.awit/items/<id>.md` by hand afterwards. `--body` and
`--body-file` let the caller supply it directly, making item creation atomic.

Precedence: `--body`/`--body-file` > `config.template` > `item.DefaultBody`. The two flags
are mutually exclusive and resolved before the lock and the mint, so a usage error costs
nothing.

## Context (read first)

- `plan/implementation-guide.md` §1 — global constraints; §5 — why flags are detected by **value**, never `cmd.IsSet` (the command tree is reused across `Main` calls, so `hasBeenSet` sticks).
- `internal/cli/create.go:172` — where the body is resolved today.
- `internal/cli/create.go:212` — `if body != nil { it.SetBody(body) }`; unchanged by this item, a nil body still falls through to `item.New`'s default.
- `internal/cli/create_test.go` — existing convention: pass `--id AWIT-TEST0001` for a deterministic ID and put the title **last**. No output scraping needed.
- Depends on `readTemplateBody` and `validateBodyBytes` from the `awit template` item.

## Files

- `internal/cli/template.go` — append `resolveBodyFlags`; add `io` to imports.
- `internal/cli/create.go` — two flags after `brief`, body resolution ahead of the lock.
- `internal/cli/create_test.go` — five new tests.

## Interfaces

```go
// resolveBodyFlags returns the body named by --body or --body-file, or (nil, nil)
// when neither is set. "-" reads stdin. The two are mutually exclusive. Callers
// must invoke it before any mutation: a usage error must not leave a minted ID
// or a half-written item behind.
//
// --body "" is indistinguishable from unset, the same tri-state limit the reused
// command tree forces on every string flag. An empty body is reachable only
// through --body-file naming an empty file.
func resolveBodyFlags(cmd *cli.Command) ([]byte, error)
```

## Steps

- [ ] Append to `internal/cli/create_test.go` (ensure `os`, `path/filepath`, `strings` are imported; `writeConfigTemplate` comes from the template item):

```go
func TestCreateBodyFlagIsVerbatim(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", "AWIT-TEST0001", "--body", "## Only\n", "T")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if got := string(readItem(t, dir, "AWIT-TEST0001").Body()); got != "## Only\n" {
		t.Errorf("body = %q, want %q", got, "## Only\n")
	}
}

func TestCreateBodyFileFromStdin(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := runStdin(t, "## Piped\n", "--repo", dir, "create", "--brief", "B.", "--id", "AWIT-TEST0001", "--body-file", "-", "T")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if got := string(readItem(t, dir, "AWIT-TEST0001").Body()); got != "## Piped\n" {
		t.Errorf("body = %q, want %q", got, "## Piped\n")
	}
}

func TestCreateBodyBeatsConfigTemplate(t *testing.T) {
	dir := initRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "t.md"), []byte("## FromTemplate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeConfigTemplate(t, dir, "t.md")

	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--id", "AWIT-TEST0001", "--body", "## FromFlag\n", "T")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if got := string(readItem(t, dir, "AWIT-TEST0001").Body()); got != "## FromFlag\n" {
		t.Errorf("body = %q, want the flag to win over config.template", got)
	}
}

func TestCreateBodyFlagsAreMutuallyExclusive(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--body", "x", "--body-file", "y.md", "T")
	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stderr %q)", code, stderr)
	}
	entries, err := os.ReadDir(filepath.Join(dir, ".awit", "items"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("items dir has %d files, want 0: no item may be minted", len(entries))
	}
}

func TestCreateBodyRefusesConflictMarkers(t *testing.T) {
	dir := initRepo(t)
	// Assembled at runtime to keep literal conflict markers out of tracked
	// source, where they confuse merge drivers and diff viewers.
	marker := strings.Repeat("<", 7) + " HEAD\nx\n" + strings.Repeat("=", 7) + "\ny\n" + strings.Repeat(">", 7) + " other\n"
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "B.", "--body", marker, "T")
	if code == 0 {
		t.Fatalf("exit = 0, want non-zero")
	}
	if !strings.Contains(stderr, "conflict markers") {
		t.Errorf("stderr = %q, want it to mention conflict markers", stderr)
	}
	entries, err := os.ReadDir(filepath.Join(dir, ".awit", "items"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("items dir has %d files, want 0", len(entries))
	}
}
```

- [ ] Run `go test ./internal/cli/ -run TestCreateBody -v` — see it FAIL: `flag provided but not defined: -body`
- [ ] Append to `internal/cli/template.go`, adding `io` to its imports:

```go
func resolveBodyFlags(cmd *cli.Command) ([]byte, error) {
	text := cmd.String("body")
	file := cmd.String("body-file")
	switch {
	case text != "" && file != "":
		return nil, cli.Exit("Incorrect usage: --body and --body-file cannot be combined", 2)
	case text != "":
		if err := validateBodyBytes("--body", []byte(text)); err != nil {
			return nil, err
		}
		return []byte(text), nil
	case file == "":
		return nil, nil
	}
	var (
		data []byte
		err  error
	)
	if file == "-" {
		data, err = io.ReadAll(cmd.Root().Reader)
	} else {
		data, err = os.ReadFile(file)
	}
	if err != nil {
		return nil, fmt.Errorf("--body-file %s: %w", file, err)
	}
	if err := validateBodyBytes("--body-file "+file, data); err != nil {
		return nil, err
	}
	return data, nil
}
```

- [ ] Add to the `createCmd` `Flags` slice after the `brief` flag:

```go
		&cli.StringFlag{Name: "body", Usage: "item body below the frontmatter, verbatim; overrides config.yaml template:"},
		&cli.StringFlag{Name: "body-file", Usage: "read the body from `PATH`, or - for stdin; overrides config.yaml template:"},
```

- [ ] Replace the body resolution at `internal/cli/create.go:172` with:

```go
	body, err := resolveBodyFlags(cmd)
	if err != nil {
		return err
	}
	if body == nil {
		body, err = readTemplateBody(s.Root, s.Config.Template)
		if err != nil {
			return err
		}
	}
```

  and move the `resolveBodyFlags` call **above** `s.Lock` and the mint. Resulting order in
  `createAction`: parse external mapping → `openStore` → `resolveBodyFlags` →
  `noteWalkedUp` → `s.Lock` → `readTemplateBody` (only when body is nil) → mint.

- [ ] Run `go test ./internal/cli/ -run TestCreate -v` — PASS, every pre-existing create test included
- [ ] Verify the round trip by hand (POSIX shell assumed; on Windows substitute a build path): build the binary, `awit init --no-skills` in a temp dir, then `awit template > body.md`, `awit create "A" --brief "x" --body-file body.md`, `awit create "B" --brief "x"` — both items must carry identical bodies. A mismatch means `item.DefaultBody` and what `template` prints have drifted
- [ ] Commit: `cli/create: add --body and --body-file`

## Acceptance Criteria

- `create --body "## Only\n"` stores exactly those bytes as the body.
- `create --body-file -` with piped stdin stores the piped bytes.
- With `config.template` set, `create --body` wins.
- `create --body x --body-file y.md` exits 2 and `.awit/items/` stays empty.
- `create --body` containing conflict markers exits non-zero, names them on stderr, and `.awit/items/` stays empty.
- `awit template > body.md && awit create … --body-file body.md` yields the same body as a plain `awit create …`.

## Out of scope

- `update --body` — its own work item.
- Making `--body ""` mean "empty body". It is indistinguishable from unset and falls through to `config.template`; this is a documented limit, recorded in the guide by the docs work item.

