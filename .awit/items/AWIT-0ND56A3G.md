---
id: AWIT-0ND56A3G
title: 'pkg/config: config.yaml load, write, agent resolution'
brief: >-
  Implement pkg/config: load and atomically write config.yaml, a Duration type
  that round-trips Go duration strings, Default/Load/Write, WriteAtomic for
  later Store.Save reuse, and Agent resolution (flag then AWIT_AGENT then
  agent_id) with no prefixing.
status: open
deps: []
labels: [phase0, p0]
refs:
  - ../../plan/implementation-guide.md
  - ../../plan/awit-implementation-plan.md
---

## Summary
After this ticket `pkg/config/config.go` and `pkg/config/atomic.go` exist with every signature in guide §4.2 copied verbatim. `Load` reads `awitDir/config.yaml`, rejects a missing prefix with `config: prefix is required`, and fills a zero `stale_claim` with 2h. `Write` encodes YAML at indent 2 and calls `WriteAtomic`. `WriteAtomic` is `os.CreateTemp(dir, ".tmp-*")`, write, `Close`, `Chmod 0o644`, `Rename`, and `Remove` the temp on failure — ticket `AWIT-0ND56E3G` will call it from `Store.Save`. `Agent` is flag → `AWIT_AGENT` → `AgentID` → `""` and never prepends `agent/`. No CLI, no item store, no git.

## Context (read first)
- Guide §4.2 `pkg/config` — copy the signatures; do not rename. `FileName`, `Config`, `Duration`, `Default`, `Load`, `Write`, `WriteAtomic`, `Agent` are the whole public surface.
- Guide §1: allowed dependency `gopkg.in/yaml.v3` (CLI skeleton AWIT-0ND5683G already `go get`s it; if this ticket is implemented first, `go get gopkg.in/yaml.v3` yourself). Stdlib otherwise. **Never hardcode `/`** — `filepath.Join`. Every file write is temp-then-rename in the **same** directory. Must pass on Linux **and** Windows (`os.Rename` will not replace an existing file on Windows — remove the destination and retry).
- Guide §1 tests: `testing` only, table-driven, `t.TempDir()` for the filesystem. No golden files.
- Guide §2 decision 4 (comment author source): `--author` → `AWIT_AGENT` → `config.agent_id` → git name. **`Config.Agent` does not add `agent/`**. Prefixing happens later in the comment/claim tickets. This method returns the raw string.
- Spec `plan/awit-implementation-plan.md` §Data model: `config.yaml` holds `prefix`, optional `default_labels`, `stale_claim` (duration, default `2h`), and `agent_id` (overridden by `AWIT_AGENT`).
- Spec Phase 0: `config.yaml` load with defaults; `AWIT_AGENT` / `AWIT_WORKER` env handling. `AWIT_WORKER` belongs to `pkg/id` (AWIT-0ND5693G), not this package.
- `Duration.MarshalYAML` starts from `time.Duration(d).String()`. That method of `2*time.Hour` is `"2h0m0s"`; guide §4.2 examples are `"2h"` and `"90m"`, and `TestWriteThenLoadRoundTrip` pins the file bytes `prefix: AWIT\nstale_claim: 2h\n`. After `String()`, strip a trailing `"0s"` then a trailing `"0m"` so 2h encodes as `2h`. `UnmarshalYAML` is `time.ParseDuration` on the scalar.
- `Load`: missing file → return the `os.ReadFile` error (`errors.Is(err, os.ErrNotExist)` must hold). Missing/empty `prefix` → `errors.New("config: prefix is required")` (that exact string). Zero `StaleClaim` after unmarshal → `Duration(2*time.Hour)`.
- `Write` uses `yaml.NewEncoder`, `SetIndent(2)`, `Encode`, `Close`, then `WriteAtomic(filepath.Join(awitDir, FileName), buf.Bytes())`. Do **not** use `yaml.Marshal` (indent 4).
- `WriteAtomic(path, data)`: `dir := filepath.Dir(path)`; `os.CreateTemp(dir, ".tmp-*")`; write all bytes; `Close`; `os.Chmod(tmp, 0o644)`; `os.Rename(tmp, path)`; on any failure after create, `os.Remove(tmp)`.
- `Agent(flag)`: if `flag != ""` return it; else if `os.Getenv("AWIT_AGENT") != ""` return that; else return `c.AgentID` (possibly `""`). No trimming, no prefixing, no git lookup.

## Files
- Create: `pkg/config/config.go`
- Create: `pkg/config/atomic.go`
- Create: `pkg/config/config_test.go`
- Modify: `go.mod` / `go.sum` only if `gopkg.in/yaml.v3` is not yet a requirement (see Step 3). Do not run `go mod tidy` — it may drop `github.com/urfave/cli/v3` if the CLI skeleton has not imported it yet.
- Fixtures/golden: none. Every test writes YAML into `t.TempDir()`.

## Interfaces
- Consumes: `gopkg.in/yaml.v3`. No other awit package.
- Produces (verbatim from guide §4.2):

```go
package config

const FileName = "config.yaml"

type Config struct {
    Prefix        string        `yaml:"prefix"`
    DefaultLabels []string      `yaml:"default_labels,omitempty"`
    StaleClaim    Duration      `yaml:"stale_claim"`           // default 2h
    AgentID       string        `yaml:"agent_id,omitempty"`
}

type Duration time.Duration
func (d Duration) MarshalYAML() (any, error)
func (d *Duration) UnmarshalYAML(n *yaml.Node) error

func Default(prefix string) Config
func Load(awitDir string) (Config, error)
func (c Config) Write(awitDir string) error
func WriteAtomic(path string, data []byte) error
func (c Config) Agent(flag string) string
```

- Produces for **AWIT-0ND56E3G** (do not implement Store here): `WriteAtomic` is the shared temp-then-rename helper. `Store.Save` will call `config.WriteAtomic(s.ItemPath(it.ID), data)`. Keep the signature `func WriteAtomic(path string, data []byte) error` stable.
- Future consumers (**not** implemented here): `item.Store.Open`/`Init` (`Load`/`Default`/`Write`), `item.Store.Save` (`WriteAtomic`), `internal/cli` claim/comment author resolution (`Agent`).

## Steps

- [ ] **Step 1: Write the failing tests.**
  Create `pkg/config/config_test.go`. Do not create `config.go` / `atomic.go` yet.

```go
package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("prefix: AWIT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Prefix != "AWIT" {
		t.Fatalf("Prefix = %q, want AWIT", c.Prefix)
	}
	if time.Duration(c.StaleClaim) != 2*time.Hour {
		t.Fatalf("StaleClaim = %s, want 2h", time.Duration(c.StaleClaim))
	}
}

func TestLoadMissingPrefix(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("stale_claim: 1h\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil || err.Error() != "config: prefix is required" {
		t.Fatalf("Load() error = %v, want config: prefix is required", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(t.TempDir())
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load(missing) error = %v, want os.ErrNotExist", err)
	}
}

func TestLoadInvalidDuration(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("prefix: AWIT\nstale_claim: banana\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load(banana) = nil, want error")
	}
}

func TestWriteThenLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := Default("AWIT")
	if err := c.Write(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	want := "prefix: AWIT\nstale_claim: 2h\n"
	if string(data) != want {
		t.Fatalf("Write() bytes = %q, want %q", data, want)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Prefix != "AWIT" || time.Duration(loaded.StaleClaim) != 2*time.Hour {
		t.Fatalf("Load after Write = %+v", loaded)
	}
}

func TestWritePreservesAgentAndLabels(t *testing.T) {
	dir := t.TempDir()
	c := Default("AWIT")
	c.AgentID = "claude"
	c.DefaultLabels = []string{"p0", "auth"}
	if err := c.Write(dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AgentID != "claude" {
		t.Fatalf("AgentID = %q, want claude", loaded.AgentID)
	}
	if len(loaded.DefaultLabels) != 2 || loaded.DefaultLabels[0] != "p0" || loaded.DefaultLabels[1] != "auth" {
		t.Fatalf("DefaultLabels = %q, want [p0 auth]", loaded.DefaultLabels)
	}
}

func TestAgentResolution(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		env     string
		setenv  bool
		agentID string
		want    string
	}{
		{name: "flag over env and config", flag: "from-flag", env: "from-env", setenv: true, agentID: "from-cfg", want: "from-flag"},
		{name: "env over config", flag: "", env: "from-env", setenv: true, agentID: "from-cfg", want: "from-env"},
		{name: "config fallback", flag: "", setenv: true, env: "", agentID: "from-cfg", want: "from-cfg"},
		{name: "empty", flag: "", setenv: true, env: "", agentID: "", want: ""},
		{name: "flag not prefixed", flag: "claude", setenv: true, env: "", agentID: "", want: "claude"},
		{name: "env not prefixed", flag: "", env: "claude", setenv: true, agentID: "", want: "claude"},
		{name: "config not prefixed", flag: "", setenv: true, env: "", agentID: "claude", want: "claude"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setenv {
				t.Setenv("AWIT_AGENT", tt.env)
			}
			c := Config{AgentID: tt.agentID}
			if got := c.Agent(tt.flag); got != tt.want {
				t.Fatalf("Agent(%q) = %q, want %q", tt.flag, got, tt.want)
			}
		})
	}
}

func TestWriteAtomicReplaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := WriteAtomic(path, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("two")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "two" {
		t.Fatalf("got %q, want two", data)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("leftover temp files: %q", matches)
	}
}

func TestWriteAtomicBadDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-dir", "f.txt")
	if err := WriteAtomic(path, []byte("x")); err == nil {
		t.Fatal("WriteAtomic(missing dir) = nil, want error")
	}
}
```

- [ ] **Step 2: Run it, see it fail to compile.**
  ```bash
  go test ./pkg/config -run TestLoad -v
  ```
  Expected failure (`config.go` does not exist):
  ```text
  # github.com/eisenwinter/awit/pkg/config [github.com/eisenwinter/awit/pkg/config.test]
  pkg/config/config_test.go: undefined: Load
  FAIL	github.com/eisenwinter/awit/pkg/config [build failed]
  ```
  The compiler will also list `FileName`, `Default`, `WriteAtomic`, `Config`. That is the red step.

- [ ] **Step 3: Implement `WriteAtomic`, `Duration`, `Default`, `Load`, `Write`, `Agent`.**
  If `go test` later reports `cannot find package "gopkg.in/yaml.v3"`:
  ```bash
  go get gopkg.in/yaml.v3
  ```
  Do **not** run `go mod tidy`.

  Create `pkg/config/atomic.go`:

```go
package config

import (
	"os"
	"path/filepath"
)

// WriteAtomic writes data to path via temp-then-rename in the same directory.
// Used by Config.Write and by pkg/item.Store.Save (AWIT-0ND56E3G).
func WriteAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	name := f.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(name)
		}
	}()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, 0o644); err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		// Windows cannot rename over an existing file (guide §1).
		if rmErr := os.Remove(path); rmErr == nil {
			err = os.Rename(name, path)
		}
		if err != nil {
			return err
		}
	}
	ok = true
	return nil
}
```

  Create `pkg/config/config.go`:

```go
package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const FileName = "config.yaml"

type Config struct {
	Prefix        string   `yaml:"prefix"`
	DefaultLabels []string `yaml:"default_labels,omitempty"`
	StaleClaim    Duration `yaml:"stale_claim"` // default 2h
	AgentID       string   `yaml:"agent_id,omitempty"`
}

type Duration time.Duration

func (d Duration) MarshalYAML() (any, error) {
	s := time.Duration(d).String()
	s = strings.TrimSuffix(s, "0s")
	s = strings.TrimSuffix(s, "0m")
	if s == "" {
		s = "0s"
	}
	return s, nil
}

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

func Default(prefix string) Config {
	return Config{
		Prefix:     prefix,
		StaleClaim: Duration(2 * time.Hour),
	}
}

func Load(awitDir string) (Config, error) {
	data, err := os.ReadFile(filepath.Join(awitDir, FileName))
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	if c.Prefix == "" {
		return Config{}, errors.New("config: prefix is required")
	}
	if c.StaleClaim == 0 {
		c.StaleClaim = Duration(2 * time.Hour)
	}
	return c, nil
}

func (c Config) Write(awitDir string) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(c); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return WriteAtomic(filepath.Join(awitDir, FileName), buf.Bytes())
}

func (c Config) Agent(flag string) string {
	if flag != "" {
		return flag
	}
	if v := os.Getenv("AWIT_AGENT"); v != "" {
		return v
	}
	return c.AgentID
}
```

  Notes that must survive `gofmt`:
  - `UnmarshalYAML` takes `*yaml.Node` (yaml.v3), not the old `func(any) error` callback.
  - `MarshalYAML` must emit `2h` for `2*time.Hour` or `TestWriteThenLoadRoundTrip` fails on the exact-bytes pin. Returning `time.Duration(d).String()` raw yields `2h0m0s`.
  - `Load` does not wrap the `os.ReadFile` error — `errors.Is(..., os.ErrNotExist)` has to work.
  - `Write` field order is the struct order: `prefix`, `default_labels` (omitempty), `stale_claim`, `agent_id` (omitempty). That is why `Default("AWIT").Write` is exactly `prefix: AWIT\nstale_claim: 2h\n`.
  - `CreateTemp` pattern is `".tmp-*"` (leading dot). `Chmod 0o644` after `Close`, before `Rename`.
  - `Agent` never returns `"agent/"+x`.

- [ ] **Step 4: Run Load tests, see them pass.**
  ```bash
  go test ./pkg/config -run TestLoad -v
  ```
  Expected:
  ```text
  === RUN   TestLoadDefaults
  --- PASS: TestLoadDefaults (0.00s)
  === RUN   TestLoadMissingPrefix
  --- PASS: TestLoadMissingPrefix (0.00s)
  === RUN   TestLoadMissingFile
  --- PASS: TestLoadMissingFile (0.00s)
  === RUN   TestLoadInvalidDuration
  --- PASS: TestLoadInvalidDuration (0.00s)
  PASS
  ok  	github.com/eisenwinter/awit/pkg/config	0.00s
  ```
  Commit:
  ```bash
  gofmt -l pkg/config
  git add pkg/config
  git commit -m "config: load yaml with prefix and stale_claim defaults"
  ```
  (`gofmt -l` must print nothing. If you `go get`'d yaml.v3 in this ticket, `git add go.mod go.sum` too.)

- [ ] **Step 5: Run Write, Agent and WriteAtomic tests, see them pass.**
  ```bash
  go test ./pkg/config -run 'TestWrite|TestAgent' -v
  ```
  Expected:
  ```text
  === RUN   TestWriteThenLoadRoundTrip
  --- PASS: TestWriteThenLoadRoundTrip (0.00s)
  === RUN   TestWritePreservesAgentAndLabels
  --- PASS: TestWritePreservesAgentAndLabels (0.00s)
  === RUN   TestAgentResolution
  === RUN   TestAgentResolution/flag_over_env_and_config
  === RUN   TestAgentResolution/env_over_config
  === RUN   TestAgentResolution/config_fallback
  === RUN   TestAgentResolution/empty
  === RUN   TestAgentResolution/flag_not_prefixed
  === RUN   TestAgentResolution/env_not_prefixed
  === RUN   TestAgentResolution/config_not_prefixed
  --- PASS: TestAgentResolution (0.00s)
  === RUN   TestWriteAtomicReplaces
  --- PASS: TestWriteAtomicReplaces (0.00s)
  === RUN   TestWriteAtomicBadDir
  --- PASS: TestWriteAtomicBadDir (0.00s)
  PASS
  ok  	github.com/eisenwinter/awit/pkg/config	0.00s
  ```
  If `TestWriteThenLoadRoundTrip` fails with `stale_claim: 2h0m0s`, the MarshalYAML compacting is missing. If it fails with a leading `---` document marker, you used a multi-doc encoder setting — `Encode` once and `Close` is enough. If `TestWriteAtomicReplaces` fails on Windows with `cannot replace`, the `os.Remove`+retry path is missing.
  Commit:
  ```bash
  git add pkg/config
  git commit -m "config: atomic write and agent resolution"
  ```

- [ ] **Step 6: Run the whole package, build and vet.**
  ```bash
  go test ./pkg/config -v
  go build ./...
  go vet ./pkg/config
  gofmt -l pkg/config
  ```
  Expected: nine tests PASS (`TestLoadDefaults`, `TestLoadMissingPrefix`, `TestLoadMissingFile`, `TestLoadInvalidDuration`, `TestWriteThenLoadRoundTrip`, `TestWritePreservesAgentAndLabels`, `TestAgentResolution` with seven subtests, `TestWriteAtomicReplaces`, `TestWriteAtomicBadDir`), then `ok  	github.com/eisenwinter/awit/pkg/config`; `go build` and `go vet` print nothing and exit `0`; `gofmt -l` prints nothing.

- [ ] **Step 7: Close ticket.**
  - Set `status: closed` in the frontmatter of `.awit/items/AWIT-0ND56A3G.md`.
  - Create `.awit/comments/AWIT-0ND56A3G/<YYYYMMDDTHHMMSSZ>-<author>.md` (UTC stamp):
    ```markdown
    ---
    author: agent/claude
    created: 2026-09-17T15:22:33Z
    ---

    Acceptance output:

    $ go test ./pkg/config -v
    (all nine tests PASS; paste the real output here)

    $ go build ./...
    (no output, exit 0)

    $ go vet ./pkg/config
    (no output, exit 0)

    $ gofmt -l pkg/config
    (no output)
    ```
  - Append the ref `../comments/AWIT-0ND56A3G/<file>.md` to this ticket's `refs` list (forward slashes, block style, after the two plan refs).
  - Commit:
    ```bash
    git add .awit/items/AWIT-0ND56A3G.md .awit/comments/AWIT-0ND56A3G
    git commit -m "tickets: close AWIT-0ND56A3G"
    ```

## Acceptance Criteria
- `go test ./pkg/config -v` → all nine tests `PASS`, final line `ok  	github.com/eisenwinter/awit/pkg/config`, exit code `0`.
- `go test ./pkg/config -run TestLoadDefaults -v` → `PASS`; file `prefix: AWIT\n` loads with `StaleClaim == 2h`.
- `go test ./pkg/config -run TestLoadMissingPrefix -v` → `PASS`; error string is exactly `config: prefix is required`.
- `go test ./pkg/config -run TestLoadMissingFile -v` → `PASS`; `errors.Is(err, os.ErrNotExist)`.
- `go test ./pkg/config -run TestWriteThenLoadRoundTrip -v` → `PASS`; file bytes are exactly `prefix: AWIT\nstale_claim: 2h\n`.
- `go test ./pkg/config -run TestAgentResolution -v` → `PASS`; flag wins, then env, then `agent_id`, then `""`; none of the values gain an `agent/` prefix.
- `go test ./pkg/config -run TestWriteAtomicReplaces -v` → `PASS`; second write replaces contents and `filepath.Glob(dir, ".tmp-*")` is empty.
- `go test ./pkg/config -run TestWriteAtomicBadDir -v` → `PASS`.
- `go build ./...` → no output, exit `0`.
- `go vet ./pkg/config` → no output, exit `0`.
- `gofmt -l pkg/config` → no output.
- `WriteAtomic` is exported from `pkg/config` for `AWIT-0ND56E3G`: `grep -c '^func WriteAtomic' pkg/config/atomic.go` → `1`.

## Out of scope
- `item.Store.Save` / `Store.Open` / `Init` writing `config.yaml` on `awit init` — `AWIT-0ND56E3G` and `AWIT-0ND56G3G`. They **call** `WriteAtomic` / `Default` / `Load` / `Write`; they do not reimplement them.
- `AWIT_WORKER` parsing — `AWIT-0ND5693G` (`id.Worker`).
- Prepending `agent/` to identities — `AWIT-0ND56Y3G` (comment author) and `AWIT-0ND56X3G` (`next --claim`). `Config.Agent` returns the raw value.
- `git config user.name` fallback — `internal/gitx.UserName` (`AWIT-0ND56C3G`) and the CLI author chain.
- CLI flags `--agent` / `--author`, urfave wiring, `internal/cli`.
- Lock files, flock, `pkg/lock` — `AWIT-0ND5723G`.
- Stale-claim *checking* (`validate --stale-claims`) — `AWIT-0ND5733G`. This ticket only stores the duration.
- Colour, formatters, golden files.
