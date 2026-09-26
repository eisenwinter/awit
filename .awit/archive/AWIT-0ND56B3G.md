---
id: AWIT-0ND56B3G
title: CI matrix linux + windows (vet, staticcheck, test)
brief: >-
  Add a GitHub Actions workflow that builds, vets and tests awit on ubuntu-latest and windows-latest plus a pinned staticcheck lint job, and a .gitattributes that keeps golden files and the conflicted fixture byte-stable on both operating systems.
status: closed
deps: [AWIT-0ND5683G]
labels: [phase0, p1]
refs_base: repo
refs:
  - plan/implementation-guide.md
  - plan/awit-implementation-plan.md
---

## Summary

After this ticket the repository has `.gitattributes` and `.github/workflows/ci.yml`. Every push to `main` and every pull request runs `go build ./...`, `go vet ./...` and `go test ./...` on both `ubuntu-latest` and `windows-latest`, plus a separate `staticcheck ./...` job on Linux with the staticcheck version pinned to an exact release. `.gitattributes` normalises text files to LF, exempts `*.golden` from any normalisation, and marks the `conflicted` fixture as binary-and-unmergeable so Git never rewrites or merge-mangles its deliberate conflict markers. No release pipeline, no goreleaser config, no fixture files and no Go code are produced here.

## Context (read first)

- Guide §1 global constraints: the code must compile and pass `go vet`, `staticcheck` and `go test ./...` on **Linux and Windows**; that dual-OS promise is exactly what this workflow enforces. Module path is `github.com/eisenwinter/awit`, Go `1.27.1` as declared in `go.mod`.
- Guide §1: "Do not run formatters/linters project-wide inside a ticket beyond `gofmt` on files you touched; CI runs `go vet` and `staticcheck` once." This ticket is that one place.
- Guide §1: output compared in tests is deterministic (no timestamps, no map order). Determinism is worthless if Git rewrites the bytes between the Linux and Windows checkouts, hence `.gitattributes`.
- Guide §3 repository layout lists `.github/workflows/ci.yml` and `.goreleaser.yaml`. Only the first belongs to this ticket.
- Guide §5 testing conventions: golden files live in `testdata/golden/*.golden` and are compared with `bytes.Equal`; fixtures are complete `.awit/` trees committed to Git; the `conflicted` fixture contains literal Git conflict markers, so `.gitattributes` must mark `testdata/fixtures/conflicted/** -merge`.
- Guide §5 Windows note: `item.Split` accepts `\r\n` but `Bytes()` always writes `\n`. A golden file that Git converted to CRLF on the Windows runner would fail `bytes.Equal` against `\n` output even though the code is correct.
- Guide §9 ticket index: this ticket depends on `AWIT-0ND5683G` (CLI skeleton) because `go build ./...` needs at least one buildable package (`cmd/awit` + `internal/cli`) for the workflow to be meaningful.
- Spec `plan/awit-implementation-plan.md` §"Phased plan" → "Phase 0 - skeleton", line item: "GitHub/GitLab CI matrix: linux + windows, `go vet`, `staticcheck`, tests". Only the GitHub variant is in scope.
- Spec §"Paths and platforms": paths are built with `filepath`, refs are forward slashes. CI on Windows is the only mechanism that catches a regression in that rule.

## Files

- Create: `.gitattributes`
- Create: `.github/workflows/ci.yml`
- Modify: none.
- Test: none - there is no Go test for a YAML workflow. Verification is the exact command list in **Acceptance Criteria** (local YAML parse, local build/vet/test, `git check-attr`, and both matrix legs green on the pull request).
- Fixtures/golden: none created here. `.gitattributes` references `testdata/fixtures/conflicted/**` and `*.golden`, which are produced later by `AWIT-0ND56N3G` (fixtures) and `AWIT-0ND56F3G` (golden files). Git attributes for paths that do not exist yet are legal and inert; `git check-attr` answers for any path string, existing or not.

## Interfaces

- Consumes: nothing from Go. The only dependency is that ticket `AWIT-0ND5683G` already produced a compilable tree exposing, per guide §4.11:
  ```go
  // package cli (internal/cli)
  func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int
  var Version = "dev"
  ```
  and `cmd/awit/main.go` calling `os.Exit(cli.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))`. This ticket does not call, change or reference those symbols; it only needs `go build ./...` to have something to build.
- Produces: no Go API. The contracts this ticket establishes for later tickets are file-level and must not be renamed:
  - Workflow file path: `.github/workflows/ci.yml`, workflow `name: ci`.
  - Job ids: `test` (matrix over `ubuntu-latest`, `windows-latest`) and `lint` (ubuntu only). `AWIT-0ND5743G` adds a separate release workflow file and must not edit these jobs.
  - Pinned linter version string `honnef.co/go/tools/cmd/staticcheck@2025.1.1`, recorded in one place (the `lint` job).
  - `.gitattributes` patterns `* text=auto eol=lf`, `*.golden -text`, `testdata/fixtures/conflicted/** -merge -text`.

## Steps

- [ ] **Step 1: Record the local baseline.**
      Confirm the tree from `AWIT-0ND5683G` is green before adding CI, so that a later red job is CI's fault and not the code's.

  ```bash
  go build ./... && go vet ./... && go test ./...
  ```

  Expected: `go build` and `go vet` print nothing; `go test` prints one line per package, e.g. `ok  	github.com/eisenwinter/awit/internal/cli	0.004s` and `?   	github.com/eisenwinter/awit/cmd/awit	[no test files]`. Exit code `0`. If anything is red, stop - fix belongs to the ticket that owns that package, not here.

- [ ] **Step 2: Write the failing attribute check.**
      This is the "test" for `.gitattributes`: ask Git what attributes it would apply to a path inside the conflicted fixture and to a golden file. The path does not need to exist - `git check-attr` matches the pattern against the string.

  ```bash
  git check-attr -a testdata/fixtures/conflicted/x.md
  git check-attr -a testdata/golden/prime.golden
  ```

  Expected failure: both commands print **nothing** and exit `0`, because no `.gitattributes` exists yet, so no attribute is set. That empty output is the red state: the conflicted fixture is currently subject to merge and text normalisation.

- [ ] **Step 3: Create `.gitattributes`, see the check pass, commit.**
      Write exactly this file:

  ```gitattributes
  # Normalise every text file to LF in the repository and in the working tree.
  # awit writes "\n" unconditionally (item.Bytes, guide §5); a CRLF checkout on
  # the Windows runner would make byte comparisons fail for correct code.
  * text=auto eol=lf

  # Golden files are compared with bytes.Equal. "-text" switches off end-of-line
  # conversion and any future normalisation entirely, so the bytes committed are
  # the bytes both runners read.
  *.golden -text

  # The "conflicted" fixture deliberately contains literal Git conflict marker
  # lines (a "<<<<<<< HEAD" line, a bare seven-equals line, a ">>>>>>> other"
  # line) inside its frontmatter so that item.HasConflictMarkers has something
  # real to detect. Without "-merge", Git's merge driver treats those lines as an
  # unresolved conflict of its own during any merge or rebase touching the file
  # and rewrites them; "-merge" makes Git keep our version and report the file as
  # conflicting instead of editing it. "-text" additionally stops eol/text
  # normalisation so the fixture bytes are identical on Linux and Windows.
  testdata/fixtures/conflicted/** -merge -text
  ```

  Then re-run the checks from Step 2:

  ```bash
  git check-attr -a testdata/fixtures/conflicted/x.md
  git check-attr -a testdata/golden/prime.golden
  ```

  Expected output now (attribute order within a path may vary; both attributes must be present):

  ```text
  testdata/fixtures/conflicted/x.md: text: unset
  testdata/fixtures/conflicted/x.md: merge: unset
  testdata/golden/prime.golden: text: unset
  ```

  `text: unset` is how `git check-attr -a` reports `-text`, and `merge: unset` is how it reports `-merge`. Also verify the LF rule reaches a normal source file:

  ```bash
  git check-attr -a internal/cli/app.go
  ```

  Expected: `internal/cli/app.go: text: auto` and `internal/cli/app.go: eol: lf`.
  Commit:

  ```bash
  git add .gitattributes
  git commit -m "ci: keep fixtures and goldens byte-stable"
  ```

- [ ] **Step 4: Write the failing workflow check.**
      The workflow's "test" is that it is valid YAML with the expected job/matrix shape. Start red:

  ```bash
  python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"
  ```

  Expected failure:

  ```text
  Traceback (most recent call last):
    File "<string>", line 1, in <module>
  FileNotFoundError: [Errno 2] No such file or directory: '.github/workflows/ci.yml'
  ```

  Exit code `1`.

- [ ] **Step 5: Create the workflow.**

  ```bash
  mkdir -p .github/workflows
  ```

  Write `.github/workflows/ci.yml` with exactly this content:

  ```yaml
  name: ci

  on:
    push:
      branches: [main]
    pull_request: {}

  permissions:
    contents: read

  jobs:
    test:
      strategy:
        fail-fast: false
        matrix:
          os: [ubuntu-latest, windows-latest]
      runs-on: ${{ matrix.os }}
      defaults:
        run:
          shell: bash
      steps:
        - uses: actions/checkout@v4
        - uses: actions/setup-go@v5
          with:
            go-version-file: go.mod
            cache: true
        - name: build
          run: go build ./...
        - name: vet
          run: go vet ./...
        # -race needs a working cgo C toolchain. The windows-latest image has no
        # gcc on PATH, so the race detector is Linux-only and Windows runs the
        # plain test suite. Both legs still execute every test.
        - name: test (race)
          if: matrix.os == 'ubuntu-latest'
          run: go test ./... -race
        - name: test
          if: matrix.os == 'windows-latest'
          run: go test ./...

    lint:
      runs-on: ubuntu-latest
      defaults:
        run:
          shell: bash
      steps:
        - uses: actions/checkout@v4
        - uses: actions/setup-go@v5
          with:
            go-version-file: go.mod
            cache: true
        - name: put GOPATH/bin on PATH
          run: echo "$(go env GOPATH)/bin" >> "$GITHUB_PATH"
        # The version is pinned on purpose. With "@latest" a new staticcheck
        # release can turn a previously green commit red without anybody
        # changing the repository, and a re-run of an old commit no longer
        # reproduces its original result. Bumping the pin is then a reviewable
        # one-line diff instead of a surprise.
        - name: install staticcheck
          run: go install honnef.co/go/tools/cmd/staticcheck@2025.1.1
        - name: staticcheck
          run: staticcheck ./...
  ```

  Notes that must survive future edits:
  - `defaults.run.shell: bash` makes the `run:` blocks identical on both operating systems; without it Windows would use PowerShell and `$(go env GOPATH)`-style syntax would break.
  - `fail-fast: false` keeps the Windows leg running when Linux fails, which is the whole point of a portability matrix - you want to see both results in one run.
  - `go-version-file: go.mod` reads `1.27.1` from `go.mod`, so the toolchain follows the module and never drifts from guide §1.
  - `permissions: contents: read` drops the default write token; CI only reads the repository.

- [ ] **Step 6: Run the workflow checks, see them pass.**

  ```bash
  python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"
  ```

  Expected: no output, exit code `0`.
  Then assert the shape, so a silent typo in a key name cannot slip through:

  ```bash
  python3 - <<'PY'
  import yaml
  w = yaml.safe_load(open('.github/workflows/ci.yml'))
  assert w['name'] == 'ci', w['name']
  # PyYAML parses the bare key "on" as the boolean True; index with True.
  trig = w.get('on', w.get(True))
  assert trig['push']['branches'] == ['main'], trig
  assert 'pull_request' in trig, trig
  assert w['permissions'] == {'contents': 'read'}, w['permissions']
  t = w['jobs']['test']
  assert t['strategy']['fail-fast'] is False, t['strategy']
  assert t['strategy']['matrix']['os'] == ['ubuntu-latest', 'windows-latest'], t
  assert t['runs-on'] == '${{ matrix.os }}', t['runs-on']
  assert t['defaults']['run']['shell'] == 'bash', t['defaults']
  runs = [s.get('run') for s in t['steps'] if 'run' in s]
  assert 'go build ./...' in runs, runs
  assert 'go vet ./...' in runs, runs
  assert 'go test ./... -race' in runs, runs
  assert 'go test ./...' in runs, runs
  guards = {s['run']: s.get('if') for s in t['steps'] if 'run' in s}
  assert guards['go test ./... -race'] == "matrix.os == 'ubuntu-latest'", guards
  assert guards['go test ./...'] == "matrix.os == 'windows-latest'", guards
  l = w['jobs']['lint']
  assert l['runs-on'] == 'ubuntu-latest', l['runs-on']
  lruns = [s.get('run') for s in l['steps'] if 'run' in s]
  assert 'go install honnef.co/go/tools/cmd/staticcheck@2025.1.1' in lruns, lruns
  assert 'staticcheck ./...' in lruns, lruns
  print('workflow shape ok')
  PY
  ```

  Expected output: `workflow shape ok`, exit code `0`.

- [ ] **Step 7: Reproduce the CI steps locally, then commit.**
      Run the same commands the runners will run:

  ```bash
  go build ./... && go vet ./... && go test ./...
  ```

  Expected: all three green, exit `0` (same output as Step 1).
  Optionally prove the pinned linter works on this tree before pushing:

  ```bash
  go install honnef.co/go/tools/cmd/staticcheck@2025.1.1
  "$(go env GOPATH)/bin/staticcheck" ./...
  ```

  Expected: no output, exit `0`. If this pinned release refuses the `1.27.1` toolchain (it prints a line containing `unsupported version of Go`), pick the newest staticcheck release that accepts it, change the single version string in the `lint` job to that exact release (never to `@latest`), and record the chosen version and the refusal message in the closing comment of Step 9.
  Commit:

  ```bash
  git add .github/workflows/ci.yml
  git commit -m "ci: add linux+windows matrix"
  ```

- [ ] **Step 8: Push and confirm both matrix legs are green.**

  ```bash
  git push -u origin HEAD
  ```

  Then watch the run for the pushed branch (a pull request against `main` triggers the `pull_request` event; a push to `main` triggers the `push` event - either is fine as long as all three jobs report success):

  ```bash
  gh run list --workflow=ci.yml --limit 1
  gh run watch "$(gh run list --workflow=ci.yml --limit 1 --json databaseId --jq '.[0].databaseId')" --exit-status
  gh run view "$(gh run list --workflow=ci.yml --limit 1 --json databaseId --jq '.[0].databaseId')" --json jobs --jq '.jobs[] | "\(.name) \(.conclusion)"'
  ```

  Expected output of the last command (three lines, order may vary):

  ```text
  test (ubuntu-latest) success
  test (windows-latest) success
  lint success
  ```

  `gh run watch --exit-status` exits `0` only when the run concluded successfully. Without `gh` installed, open the run in the Actions tab and confirm the same three job results by eye; paste the job list into the closing comment either way.
  If the Windows leg fails on a path or line-ending assertion, that is a real portability bug in the package the test belongs to: report it and let the owning ticket fix it. Do not weaken the matrix, do not add `continue-on-error`, and do not skip tests on Windows to make this ticket green.

- [ ] **Step 9: Close ticket.**
  - Set `status: closed` in the frontmatter of `.awit/items/AWIT-0ND56B3G.md`.
  - Create `.awit/comments/AWIT-0ND56B3G/<YYYYMMDDTHHMMSSZ>-<author>.md` (UTC stamp, e.g. `20260917T151204Z-claude.md`) with this shape, filling in the real captured output:

    ```markdown
    ---
    author: agent/claude
    created: 2026-09-17T15:12:04Z
    ---

    Acceptance output:

    $ python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"
    (no output, exit 0)

    $ git check-attr -a testdata/fixtures/conflicted/x.md
    testdata/fixtures/conflicted/x.md: text: unset
    testdata/fixtures/conflicted/x.md: merge: unset

    $ go build ./... && go vet ./... && go test ./...
    (no output from build/vet; ok lines from test; exit 0)

    CI run jobs:
    test (ubuntu-latest) success
    test (windows-latest) success
    lint success

    staticcheck pinned at honnef.co/go/tools/cmd/staticcheck@2025.1.1
    ```

  - Append the ref `../comments/AWIT-0ND56B3G/<file>.md` to this ticket's `refs` list (forward slashes, block style, after the two plan refs).
  - Commit:
    ```bash
    git add .awit/items/AWIT-0ND56B3G.md .awit/comments/AWIT-0ND56B3G
    git commit -m "tickets: close AWIT-0ND56B3G"
    ```

## Acceptance Criteria

- `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml'))"` → no output, exit code `0`.
- The Step 6 shape assertion script prints `workflow shape ok` and exits `0`.
- `git check-attr -a testdata/fixtures/conflicted/x.md` → prints `testdata/fixtures/conflicted/x.md: text: unset` and `testdata/fixtures/conflicted/x.md: merge: unset` (the path need not exist; `check-attr` matches the pattern against the string).
- `git check-attr -a testdata/golden/prime.golden` → prints `testdata/golden/prime.golden: text: unset`.
- `git check-attr -a internal/cli/app.go` → prints `internal/cli/app.go: text: auto` and `internal/cli/app.go: eol: lf`.
- `go build ./...` → no output, exit code `0`.
- `go vet ./...` → no output, exit code `0`.
- `go test ./...` → every package line starts with `ok` or `?`, exit code `0`.
- `staticcheck ./...` (from the pinned install) → no output, exit code `0`.
- The CI run for the branch reports three successful jobs: `test (ubuntu-latest)`, `test (windows-latest)`, `lint`. `gh run watch <id> --exit-status` exits `0`.
- `.github/workflows/ci.yml` contains the literal string `honnef.co/go/tools/cmd/staticcheck@2025.1.1` and does not contain `@latest`: `grep -c '@latest' .github/workflows/ci.yml` → `0` with exit code `1` (grep's no-match exit).

## Out of scope

- goreleaser config, version embedding via `-ldflags`, the tag-triggered release workflow, the pre-commit hook documentation and the README agent loop - all of that is `AWIT-0ND5743G`, which depends on this ticket.
- Creating `testdata/fixtures/**` (including the `conflicted` fixture whose marker lines motivate `-merge`) - that is `AWIT-0ND56N3G`. This ticket only writes the attribute pattern.
- Creating `testdata/golden/*.golden` or the `-update` flag - that is `AWIT-0ND56F3G`.
- Any Go code, including `internal/cli`, `cmd/awit` and `go.mod` edits. If `go build ./...` fails, the fix belongs to `AWIT-0ND5683G`.
- Adding `golangci-lint`, `gofumpt`, coverage upload, codecov, caching beyond `setup-go`'s `cache: true`, dependabot, or a darwin matrix leg. The matrix is exactly the two operating systems guide §1 promises.
- Loosening the matrix to make it pass: no `continue-on-error`, no `-short` on Windows, no `if:` guards that skip whole test packages per OS beyond the documented `-race` split.
