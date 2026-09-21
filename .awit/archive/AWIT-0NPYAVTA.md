---
id: AWIT-0NPYAVTA
title: 'cli: add the template command'
brief: >-
  awit has no way to print the work item body template, so an agent must read the guide to learn which sections are required. Adds awit template, printing the config.yaml template: bytes or the built-in skeleton, and promotes readCreateTemplate to a shared readTemplateBody.
status: closed
deps: []
labels: [phase7, p0]
refs_base: repo
refs: []
assignee: agent/orchestrator
---

## Summary

`awit` cannot print the body template it would use. `config.yaml template:` is read only
inside `create`, so an agent learns the required section list from guide §6 prose or not at
all. `awit template` writes those exact bytes to stdout, making a
template → fill → `create --body-file` loop possible.

Read-only: builds no graph, so it prints no `warning: N items quarantined` line — the same
silence rule `label` follows. Ignores `--format`; it emits a document, not rows.

## Context (read first)

- `plan/implementation-guide.md` §1 — global constraints: Linux **and** Windows, no hardcoded `/`, temp-then-rename writes, stderr `Error: ` prefix, exit `0`/`1`/`2`, stdlib-only table-driven tests, commit message `<scope>: <imperative summary>`.
- `internal/cli/create.go:254-278` — `readCreateTemplate`, the function being moved and renamed. Its one caller is `create.go:172`.
- `pkg/item/item.go:237` — `body: []byte("\n## Summary\n\n## Acceptance Criteria\n\n")`, the literal to export.
- `internal/cli/label.go` — precedent for a command that builds no graph and stays silent.
- Test helpers already in `internal/cli/helpers_test.go`: `run`, `runStdin`, `initRepo`, `copyFixture`, `readItem`, `golden`.

## Files

- `pkg/item/item.go` — add `const DefaultBody`, use it in `New` at line 237.
- `internal/cli/template.go` — new: `templateCmd`, `readTemplateBody`, `validateBodyBytes`.
- `internal/cli/template_test.go` — new.
- `internal/cli/create.go` — delete lines 254-278, repoint line 172, drop three imports.
- `internal/cli/app.go:129-149` — register `templateCmd` after `labelCmd`.

## Interfaces

```go
// pkg/item
const DefaultBody = "\n## Summary\n\n## Acceptance Criteria\n\n"

// internal/cli
func validateBodyBytes(source string, data []byte) error
func readTemplateBody(root, rel string) ([]byte, error)
```

## Steps

- [ ] Create `internal/cli/template_test.go` (imports `os`, `path/filepath`, `strings`, `testing`):

```go
func TestTemplateDefaultSkeleton(t *testing.T) {
	dir := initRepo(t)
	code, stdout, stderr := run(t, "--repo", dir, "template")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	const want = "\n## Summary\n\n## Acceptance Criteria\n\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestTemplateFromConfigIsVerbatim(t *testing.T) {
	dir := initRepo(t)
	body := "## Summary\n## Steps\n- [ ] do it\n"
	if err := os.MkdirAll(filepath.Join(dir, "tpl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tpl", "b.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	writeConfigTemplate(t, dir, "tpl/b.md")

	code, stdout, stderr := run(t, "--repo", dir, "template")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != body {
		t.Errorf("stdout = %q, want %q", stdout, body)
	}
}

func TestTemplateReportsBrokenConfig(t *testing.T) {
	dir := initRepo(t)
	writeConfigTemplate(t, dir, "tpl/missing.md")

	code, stdout, stderr := run(t, "--repo", dir, "template")
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr %q)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "template tpl/missing.md") {
		t.Errorf("stderr = %q, want it to name the template path", stderr)
	}
}

func TestTemplateIsSilentOnQuarantinedGraph(t *testing.T) {
	dir := copyFixture(t, "cyclic")
	code, _, stderr := run(t, "--repo", dir, "template")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty: template builds no graph", stderr)
	}
}

// writeConfigTemplate appends a template: key to the repo's config.yaml.
// Reused by the create and update body work items.
func writeConfigTemplate(t *testing.T, repo, rel string) {
	t.Helper()
	p := filepath.Join(repo, ".awit", "config.yaml")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(data, []byte("template: "+rel+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] Run `go test ./internal/cli/ -run TestTemplate -v` — see it FAIL: no `template` command is registered, so urfave exits 2
- [ ] In `pkg/item/item.go`, add above `New` and use it at line 237 (`body: []byte(DefaultBody)`):

```go
// DefaultBody is the body New writes when no config template applies. The
// awit template command prints these exact bytes, so the two must not drift.
const DefaultBody = "\n## Summary\n\n## Acceptance Criteria\n\n"
```

- [ ] Create `internal/cli/template.go`:

```go
package cli

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"unicode/utf8"

	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var templateCmd = &cli.Command{
	Name:  "template",
	Usage: "Print the work item body template",
	Description: `Writes the body create would use: the file named by config.yaml
template:, or the built-in skeleton when none is set. Fill it in and pass it
back with awit create --body-file, or awit update <id> --body-file.

Builds no graph, so it never reports quarantined items. Ignores --format.`,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		body, err := readTemplateBody(s.Root, s.Config.Template)
		if err != nil {
			return err
		}
		if body == nil {
			body = []byte(item.DefaultBody)
		}
		_, err = cmd.Root().Writer.Write(body)
		return err
	},
}

// validateBodyBytes rejects bodies awit cannot store: invalid UTF-8, and Git
// conflict markers that would quarantine the item on its next load. source
// names the origin for the message ("template docs/t.md", "--body").
func validateBodyBytes(source string, data []byte) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("%s: not valid UTF-8", source)
	}
	if item.HasConflictMarkers(data) {
		return fmt.Errorf("%s: conflict markers", source)
	}
	return nil
}

// readTemplateBody reads the config.yaml template: file. An empty rel means
// no template is configured and returns (nil, nil), which leaves the caller
// on item.DefaultBody. Shared by create and template.
func readTemplateBody(root, rel string) ([]byte, error) {
	if rel == "" {
		return nil, nil
	}
	cleaned := path.Clean(rel)
	abs := filepath.Join(root, filepath.FromSlash(cleaned))
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("template %s: %w", rel, err)
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("template %s: is a directory", rel)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("template %s: %w", rel, err)
	}
	if err := validateBodyBytes("template "+rel, data); err != nil {
		return nil, err
	}
	return data, nil
}
```

- [ ] Delete `readCreateTemplate` from `internal/cli/create.go` (lines 254-278) and change its one call site at line 172 to `body, err := readTemplateBody(s.Root, s.Config.Template)`
- [ ] Remove `path`, `path/filepath` and `unicode/utf8` from `create.go` imports — they appear only at lines 258, 259 and 271, all inside the moved function. `os` **stays**: `detectFormat` at line 139 uses `*os.File` at lines 140-141
- [ ] Add `templateCmd,` to the `Commands` slice in `internal/cli/app.go`, after `labelCmd` (line 147)
- [ ] Run `go test ./internal/cli/ -run TestTemplate -v` — see all four PASS
- [ ] Run `go test ./... && go vet ./...` — PASS. `create`'s existing template error strings are byte-identical because `validateBodyBytes("template "+rel, …)` reproduces them exactly
- [ ] Commit: `cli/template: add template command printing the body template`

## Acceptance Criteria

- `awit template` in a repo without `template:` writes exactly `"\n## Summary\n\n## Acceptance Criteria\n\n"` to stdout, nothing to stderr, exit 0.
- `awit template` with `template:` set writes that file's bytes verbatim.
- `awit template` whose `template:` names a missing file exits 1, writes nothing to stdout, and names the path on stderr.
- `awit template` against the `cyclic` fixture writes nothing to stderr — it builds no graph.
- `go test ./... && go vet ./...` passes on Linux and Windows.

## Out of scope

- `awit template <name>` for multiple named templates. One `template:` key today; YAGNI.
- Any change to what `create` writes when no template is configured.


## Comments

### 2026-09-21T13:54:31Z agent/orchestrator

awit template prints config template bytes or the built-in skeleton; readCreateTemplate promoted to shared readTemplateBody

### 2026-09-21T13:54:31Z agent/orchestrator

Implemented by TemplateWorker (TDD: 4 template tests red then green; go test ./... && go vet ./... clean). Reviewed by reviewer agent: correct, error strings byte-identical to old readCreateTemplate; one gofmt nit (trailing blank line create.go) folded in, gofmt now clean, tests re-verified.
