---
id: AWIT-0ND5743G
title: "goreleaser, version embedding, pre-commit hook docs, README agent loop"
brief: >-
  Land .goreleaser.yaml v2 for linux/darwin/windows x amd64/arm64 with CGO_ENABLED=0 and Version ldflags, a tag-triggered release.yml, a two-line pre-commit hook, and a README covering install, commands (including label), the agent loop, status, pre-commit, and docs/schema.md.
status: closed
deps: [AWIT-0ND56B3G, AWIT-0ND5713G]
labels: [phase5, p1]
refs_base: repo
refs:
  - plan/implementation-guide.md
  - plan/awit-implementation-plan.md
---

## Summary

After this ticket tagged `v*` pushes build six static binaries (linux, darwin, windows × amd64, arm64) via GoReleaser v2 with `CGO_ENABLED=0` and `-X github.com/eisenwinter/awit/internal/cli.Version={{.Version}}`. `.github/workflows/release.yml` is a new workflow (do not edit `ci.yml`). `go build -ldflags "-X github.com/eisenwinter/awit/internal/cli.Version=v0.0.0-test"` prints `awit v0.0.0-test`. `docs/hooks/pre-commit` is exactly `#!/bin/sh` then `exec awit validate`. `README.md` is the user-facing document: Install, a commands table that includes `label`, the five-step agent loop, Status, Pre-commit, and a Layout section that links `docs/schema.md`. No Go source changes except the ldflags verification build.

## Context (read first)

- Guide §1 - module `github.com/eisenwinter/awit`, binary `cmd/awit`, Go `1.27.1` from `go.mod`. Allowed extra deps are urfave and yaml; this ticket does not `go get` anything. `internal/cli.Version` default is `"dev"` (`AWIT-0ND5683G`); ldflags override it.
- Guide §3 layout lists `.goreleaser.yaml` and `.github/workflows/ci.yml`. CI already exists (`AWIT-0ND56B3G`). This ticket adds `.goreleaser.yaml` and `.github/workflows/release.yml`. **Do not edit** `ci.yml` jobs (`test`, `lint`).
- Guide §4.11 - `var Version = "dev"` with comment `-ldflags "-X github.com/eisenwinter/awit/internal/cli.Version=v1.2.3"`. `printVersion` writes `awit %s\n` using `Version`. Do not change `printVersion` or `Version`.
- Guide §9 - this ticket depends on `AWIT-0ND56B3G` (CI matrix) and `AWIT-0ND5713G` (e2e agent loop). Both must be `status: closed` before you start so the README can describe a working loop.
- Spec Phase 5 - "Documented pre-commit hook running `awit validate`"; "goreleaser config, version embedding, `README` with the agent loop". Exit criterion: "Tagged binaries for linux/windows/darwin via goreleaser".
- Spec CLI matrix - thirteen commands including `label`. The README table must list all thirteen plus global flags.
- Spec agent surface loop:

  ```text
  awit prime
  awit next --claim
  awit show <id> --full
  awit comment <id>
  awit close <id>
  ```

- `AWIT-0ND56B3G` Out of scope already points here for goreleaser, ldflags, the tag workflow, pre-commit docs, and the README agent loop.
- `AWIT-0ND5753G` owns `docs/schema.md`. This ticket's README Layout section **must** link `[docs/schema.md](docs/schema.md)` even if that file does not exist yet (same pattern as CI attributes for future fixtures). Do not write `docs/schema.md` here.
- Existing `README.md` is a pre-alpha stub. Replace it with the full document in Step 7. Keep the BSD 2-Clause license line and the link to `plan/implementation-guide.md`.

## Files

- Create: `.goreleaser.yaml`
- Create: `.github/workflows/release.yml`
- Create: `docs/hooks/pre-commit`
- Modify: `README.md` - replace contents with the document in Step 7.
- Modify: none of `internal/cli`, `cmd/awit`, `ci.yml`, `go.mod`.

## Interfaces

- Consumes (do not change):
  ```go
  // package cli (internal/cli)
  var Version = "dev"
  func printVersion(cmd *cli.Command) // writes "awit %s\n", Version
  func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int
  ```
- Produces: no Go API. File-level contracts later tickets must not rename:
  - `.goreleaser.yaml` `version: 2`
  - ldflags path `github.com/eisenwinter/awit/internal/cli.Version`
  - workflow path `.github/workflows/release.yml`, workflow `name: release`, trigger `push.tags: ["v*"]`
  - hook path `docs/hooks/pre-commit` with bytes `#!/bin/sh\nexec awit validate\n`
  - README sections in this order: opening paragraph, Install, Commands, Agent loop, Status, Pre-commit, Layout, Contributing, License

## Steps

- [ ] **Step 1: Record the version baseline.**

  ```bash
  go build -o /tmp/awit-dev ./cmd/awit
  /tmp/awit-dev --version
  ```

  Expected: `awit dev`, exit 0. If this fails, stop - the fix belongs to `AWIT-0ND5683G`, not here.

  Confirm `Version` is still the default in source:

  ```bash
  python3 - <<'PY'
  import pathlib
  p = pathlib.Path('internal/cli/app.go').read_text()
  assert 'var Version = "dev"' in p, p
  print('Version default ok')
  PY
  ```

  Expected: `Version default ok`.

- [ ] **Step 2: Write the failing goreleaser check.**

  ```bash
  python3 -c "import yaml; yaml.safe_load(open('.goreleaser.yaml'))"
  ```

  Expected failure:

  ```text
  FileNotFoundError: [Errno 2] No such file or directory: '.goreleaser.yaml'
  ```

  Exit code 1.

- [ ] **Step 3: Create `.goreleaser.yaml`.**

  Write exactly this file (GoReleaser config version 2):

  ```yaml
  version: 2

  project_name: awit

  builds:
    - id: awit
      main: ./cmd/awit
      binary: awit
      env:
        - CGO_ENABLED=0
      goos:
        - linux
        - darwin
        - windows
      goarch:
        - amd64
        - arm64
      flags:
        - -trimpath
      ldflags:
        - -s -w -X github.com/eisenwinter/awit/internal/cli.Version={{.Version}}

  archives:
    - id: default
      formats: [tar.gz]
      format_overrides:
        - goos: windows
          formats: [zip]
      name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

  checksum:
    name_template: checksums.txt

  changelog:
    sort: asc
    filters:
      exclude:
        - "^docs:"
        - "^test:"
        - "^tickets:"
        - "^cli:"
        - "^item:"
        - "^lock:"

  release:
    draft: false
    prerelease: auto
  ```

  Six binaries: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64. `CGO_ENABLED=0` is required so the Windows and Darwin artifacts do not need a C toolchain. The ldflags string must contain the literal substring `-X github.com/eisenwinter/awit/internal/cli.Version={{.Version}}`.

- [ ] **Step 4: Assert goreleaser shape, then commit.**

  ```bash
  python3 - <<'PY'
  import yaml
  c = yaml.safe_load(open('.goreleaser.yaml'))
  assert c['version'] == 2, c['version']
  assert c['project_name'] == 'awit', c['project_name']
  b = c['builds'][0]
  assert b['main'] == './cmd/awit', b
  assert b['binary'] == 'awit', b
  assert b['env'] == ['CGO_ENABLED=0'], b
  assert b['goos'] == ['linux', 'darwin', 'windows'], b
  assert b['goarch'] == ['amd64', 'arm64'], b
  ld = b['ldflags']
  if isinstance(ld, str):
      ld = [ld]
  joined = ' '.join(ld)
  assert 'CGO_ENABLED=0' in str(b['env'])
  assert '-X github.com/eisenwinter/awit/internal/cli.Version={{.Version}}' in joined, joined
  oses = set(b['goos'])
  archs = set(b['goarch'])
  assert oses == {'linux', 'darwin', 'windows'}
  assert archs == {'amd64', 'arm64'}
  print('goreleaser shape ok')
  PY
  ```

  Expected: `goreleaser shape ok`, exit 0.

  Optional, if `goreleaser` is on PATH:

  ```bash
  goreleaser check
  ```

  Expected: no error. If the binary is missing, skip this command; the Python assertion is the required check.

  ```bash
  git add .goreleaser.yaml
  git commit -m "build: add goreleaser v2 linux/darwin/windows matrix"
  ```

- [ ] **Step 5: Verify ldflags embed `v0.0.0-test`.**

  This is the version-embedding acceptance. Do not change Go source.

  ```bash
  go build -ldflags "-s -w -X github.com/eisenwinter/awit/internal/cli.Version=v0.0.0-test" -o /tmp/awit-rel ./cmd/awit
  /tmp/awit-rel --version
  rm -f /tmp/awit-rel
  ```

  Expected stdout (exactly, plus newline):

  ```text
  awit v0.0.0-test
  ```

  Exit 0. If you see `awit dev`, the `-X` path is wrong - it must be `github.com/eisenwinter/awit/internal/cli.Version`. If you see `awit version v0.0.0-test`, `printVersion` was overwritten; restore the `AWIT-0ND5683G` hook instead of changing it here.

- [ ] **Step 6: Write the failing release-workflow check, then create it.**

  ```bash
  python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml'))"
  ```

  Expected failure: `FileNotFoundError` for `.github/workflows/release.yml`.

  Write `.github/workflows/release.yml`:

  ```yaml
  name: release

  on:
    push:
      tags: ["v*"]

  permissions:
    contents: write

  jobs:
    goreleaser:
      runs-on: ubuntu-latest
      defaults:
        run:
          shell: bash
      steps:
        - uses: actions/checkout@v4
          with:
            fetch-depth: 0
        - uses: actions/setup-go@v5
          with:
            go-version-file: go.mod
            cache: true
        - uses: goreleaser/goreleaser-action@v6
          with:
            distribution: goreleaser
            version: "~> v2"
            args: release --clean
          env:
            GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
  ```

  Then:

  ```bash
  python3 - <<'PY'
  import yaml
  w = yaml.safe_load(open('.github/workflows/release.yml'))
  assert w['name'] == 'release', w['name']
  trig = w.get('on', w.get(True))
  assert trig['push']['tags'] == ['v*'], trig
  assert w['permissions'] == {'contents': 'write'}, w['permissions']
  job = w['jobs']['goreleaser']
  assert job['runs-on'] == 'ubuntu-latest', job
  uses = [s.get('uses', '') for s in job['steps']]
  assert any(u.startswith('goreleaser/goreleaser-action@') for u in uses), uses
  print('release workflow shape ok')
  PY
  ```

  Expected: `release workflow shape ok`.

  Confirm `ci.yml` is untouched:

  ```bash
  python3 - <<'PY'
  import yaml
  w = yaml.safe_load(open('.github/workflows/ci.yml'))
  assert w['name'] == 'ci'
  assert 'test' in w['jobs'] and 'lint' in w['jobs']
  assert 'goreleaser' not in w['jobs']
  print('ci.yml untouched')
  PY
  ```

  Expected: `ci.yml untouched`.

  ```bash
  git add .github/workflows/release.yml
  git commit -m "ci: release on v* tags via goreleaser"
  ```

- [ ] **Step 7: Pre-commit hook file and full README.**

  Write `docs/hooks/pre-commit` with **exactly** these bytes (trailing newline, no extra lines, no comments):

  ```text
  #!/bin/sh
  exec awit validate
  ```

  Confirm:

  ```bash
  python3 - <<'PY'
  b = open('docs/hooks/pre-commit', 'rb').read()
  assert b == b'#!/bin/sh\nexec awit validate\n', b
  print('pre-commit bytes ok')
  PY
  ```

  Expected: `pre-commit bytes ok`.

  Replace `README.md` with exactly this document:

  ````markdown
  # awit

  Zero-daemon Go CLI that turns Markdown files under `.awit/` into a
  dependency graph for humans and agents. Offline, versioned in Git, no
  database, no daemon. Every command rebuilds the graph from
  `.awit/items/*.md`; a clone is the whole state.

  ## Install

  From source (Go 1.27+):

  ```bash
  go install github.com/eisenwinter/awit/cmd/awit@latest
  ```

  Or download a tagged binary from GitHub Releases. Each tag `v*` publishes
  linux, darwin and windows builds for amd64 and arm64, all with
  `CGO_ENABLED=0`.

  Verify the embed:

  ```bash
  awit --version
  ```

  A release build prints `awit <version>`. A `go build` without ldflags
  prints `awit dev`.

  ## Commands

  | Command                       | Flags                                                                        | Purpose                                                 |
  | ----------------------------- | ---------------------------------------------------------------------------- | ------------------------------------------------------- |
  | `awit init`                   | `--prefix`                                                                   | Create `.awit/`, `config.yaml`, gitignore `.awit/.lock` |
  | `awit create <title>`         | `--brief`, `-d` deps, `-l` labels, `--assign`, `--id`                        | Mint a snowflake ID, write a lean item                  |
  | `awit list`                   | `-s` status, `-l` label, `--ready`, `--blocked`, `--quarantined`, `--format` | Index view                                              |
  | `awit label`                  | `--state open\|closed\|all`, `--format`                                      | Label vocabulary with usage counts                      |
  | `awit show <id>`              | `--full`, `--refs-only`                                                      | Core ticket or full resolved ref tree                   |
  | `awit comment <id> [text]`    | `--file <path>`, `--author`                                                  | Timestamped comment or attached file; append to `refs`  |
  | `awit update <id>`            | `--status`, `--brief`, `--assign`, `--label`, `--unlabel`, `--title`         | Mutate frontmatter with a minimal diff                  |
  | `awit close <id>`             | `--reason`, `--author`                                                       | Set `closed`, clear `claimed_at`; does not git-commit   |
  | `awit release <id>`           | -                                                                            | Set `open`, clear `assignee` and `claimed_at`           |
  | `awit dep add\|rm <id> <dep>` | -                                                                            | Edit `deps` with cycle pre-check                        |
  | `awit validate`               | `--stale-claims`                                                             | Integrity report; non-zero exit on `FAIL`               |
  | `awit prime`                  | `--max-tokens`, `-l` label                                                   | Deterministic state graph for prompt injection          |
  | `awit next`                   | `-l` label, `--claim`, `--no-commit`, `--seed`                               | Top unblocked item; optional claim                      |

  Global flags: `--format compact|table|json`, `--repo <path>` (directory that
  contains `.awit/`), `--no-color` (accepted, no-op). Exit codes: `0` success,
  `1` expected non-success (`next` with no candidates, `validate` with FAIL),
  `2` usage error.

  ## Agent loop

  Each step is one process. State between steps lives only in files and Git.

  ```text
  awit prime                 # token-light snapshot of the graph
  awit next --claim          # claim the top unblocked item
  awit show <id> --full      # ticket + resolved refs
  awit comment <id> "..."    # research notes
  awit close <id>            # unblocks downstream
  ```

  Then `prime` again. `--claim` writes `status: in_progress`, `assignee`,
  `claimed_at`, and commits `awit: claim <id>` unless `--no-commit`.
  `close` is a plain file write; the human or agent commits when done.
  Filter with `-l p0` (AND across repeated flags, OR inside one comma list).

  ## Status

  v1 targets tagged binaries for linux, darwin and windows (amd64 and
  arm64) via GoReleaser. CI runs `go vet`, `staticcheck` and `go test ./...`
  on ubuntu-latest and windows-latest. This repository dogfoods itself:
  implementation tickets live in [`.awit/items/`](.awit/items/).

  ## Pre-commit

  `awit validate` is the intended pre-commit hook. A copy lives at
  [`docs/hooks/pre-commit`](docs/hooks/pre-commit):

  ```sh
  #!/bin/sh
  exec awit validate
  ```

  Install it with:

  ```bash
  cp docs/hooks/pre-commit .git/hooks/pre-commit
  chmod +x .git/hooks/pre-commit
  ```

  The hook exits non-zero on any `FAIL` (cycles, dangling deps, conflict
  markers, parse errors, duplicate ids, id mismatches), which blocks the
  commit.

  ## Layout

  ```text
  .awit/
  ├── config.yaml            # prefix, default_labels, stale_claim, agent_id
  ├── items/PREFIX-XXXXXXXX.md
  └── comments/PREFIX-XXXXXXXX/<UTC seconds>-<author>.md
  ```

  An item is YAML frontmatter (`id`, `title`, `brief`, `status`, `deps`,
  `labels`, `assignee`, `claimed_at`, `refs`) followed by a Markdown body.
  Priority is a label by convention (`p0`…`p4`). Blocked is derived, never
  stored. The on-disk schema, including the reserved `external:` key, is
  documented in [docs/schema.md](docs/schema.md).

  ## Contributing

  Read [plan/implementation-guide.md](plan/implementation-guide.md) first.
  It holds the resolved design decisions, the package layout, every shared
  Go interface, and the ticket index with dependency order. Then pick a
  ticket from `.awit/items/` whose `deps` are all closed.

  ## License

  BSD 2-Clause. See [LICENSE](LICENSE).
  ````

  Confirm required README phrases:

  ```bash
  python3 - <<'PY'
  t = open('README.md', encoding='utf-8').read()
  for s in [
      '## Install',
      '## Commands',
      '## Agent loop',
      '## Status',
      '## Pre-commit',
      '## Layout',
      'awit label',
      'awit prime',
      'awit next --claim',
      'awit show <id> --full',
      'awit comment',
      'awit close',
      'docs/hooks/pre-commit',
      'docs/schema.md',
      'go install github.com/eisenwinter/awit/cmd/awit@latest',
  ]:
      assert s in t, s
  print('README sections ok')
  PY
  ```

  Expected: `README sections ok`.

  ```bash
  git add docs/hooks/pre-commit README.md
  git commit -m "docs: README agent loop, install, pre-commit hook"
  ```

- [ ] **Step 8: Close ticket.**

  Re-run the checks from Steps 4–7 so the closing comment has fresh output:

  ```bash
  python3 -c "import yaml; yaml.safe_load(open('.goreleaser.yaml')); yaml.safe_load(open('.github/workflows/release.yml'))"
  go build -ldflags "-X github.com/eisenwinter/awit/internal/cli.Version=v0.0.0-test" -o /tmp/awit-rel ./cmd/awit
  /tmp/awit-rel --version
  rm -f /tmp/awit-rel
  ```

  - Set `status: closed` in `.awit/items/AWIT-0ND5743G.md`.
  - Create `.awit/comments/AWIT-0ND5743G/<YYYYMMDDTHHMMSSZ>-<author>.md` containing the `awit v0.0.0-test` line, `goreleaser shape ok`, `release workflow shape ok`, `pre-commit bytes ok`, and `README sections ok`.
  - Append the ref `../comments/AWIT-0ND5743G/<file>.md` to this ticket's `refs` (forward slashes, block style).
  - Commit:

    ```bash
    git add .awit/items/AWIT-0ND5743G.md .awit/comments/AWIT-0ND5743G
    git commit -m "tickets: close AWIT-0ND5743G"
    ```

## Acceptance Criteria

- `.goreleaser.yaml` has `version: 2`, `CGO_ENABLED=0`, `goos: [linux, darwin, windows]`, `goarch: [amd64, arm64]`, and ldflags `-X github.com/eisenwinter/awit/internal/cli.Version={{.Version}}`.
- `.github/workflows/release.yml` triggers on `push.tags: ["v*"]`, job `goreleaser`, `contents: write`. `.github/workflows/ci.yml` is byte-identical to before this ticket (no goreleaser job added there).
- `go build -ldflags "-X github.com/eisenwinter/awit/internal/cli.Version=v0.0.0-test" -o /tmp/awit-rel ./cmd/awit && /tmp/awit-rel --version` prints exactly `awit v0.0.0-test`.
- `docs/hooks/pre-commit` bytes are `#!/bin/sh\nexec awit validate\n`.
- `README.md` contains sections Install, Commands, Agent loop, Status, Pre-commit, Layout; the commands table includes `awit label`; the loop lists `prime`, `next --claim`, `show <id> --full`, `comment`, `close`; Layout links `docs/schema.md`.
- `internal/cli.Version` is still `"dev"` in source. No new Go files. `go test ./...` still passes (no behaviour change).

## Out of scope

- Writing `docs/schema.md` or reserving `external:` - `AWIT-0ND5753G`.
- `pkg/lock` / mutating-command locking - `AWIT-0ND5723G`.
- `validate --stale-claims` - `AWIT-0ND5733G`.
- Editing `.github/workflows/ci.yml`, `internal/cli/app.go`, `printVersion`, or `go.mod`.
- Publishing a real GitHub release in this ticket (the workflow is the deliverable; tagging `v*` is a human action after merge).
- Homebrew, Docker images, npm wrappers, colour, or extra `goos`/`goarch` values (386, arm, windows/arm).
