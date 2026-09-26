---
id: AWIT-0P1XYZSP
title: 'config: expose Load''s rules — Normalize, ValidPrefix, Duration.String'
brief: >-
  Refactors pkg/config so Load = Unmarshal + Normalize, exports the init prefix grammar as ValidPrefix (init.go cut over) and gives Duration a String method; no behaviour change, groundwork for the lazyawit Config tab's validation.
status: closed
deps: []
labels: [tui, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---
## Summary

The Config tab must refuse exactly what `Load` refuses, plus the prefix grammar `awit init` enforces, before it writes `.awit/config.yaml`. Today those rules are the body of `Load` (unexported helpers `validateTemplatePath`, `normalizeLabels`) and `internal/cli/init.go`'s `prefixRE`. This item lifts them into three exported names on `pkg/config` — `(Config).Normalize`, `ValidPrefix`, `(Duration).String` — and makes `Load`, `MarshalYAML` and `initAction` call them. `Load` output and every error string stay byte-identical; `Load` still does not enforce the prefix grammar (existing repositories keep loading).

## Context (read first)

- `docs/superpowers/specs/2026-09-26-config-tab-plan.md` §C (`pkg/config` facts), §D.1 (this item), §E (why the grammar is not added to `Load`).
- `pkg/config/config.go:40-67` — `Duration`, `MarshalYAML` (the `0s` / `Nh` / `Nm` / `String()` cases), `UnmarshalYAML`.
- `pkg/config/config.go:76-100` — `Load`: prefix required → stale default → `validateTemplatePath` → `normalizeLabels`. Lines 85-98 become `Normalize`.
- `pkg/config/config.go:143-196` — `validateTemplatePath`, `windowsAbs`, `normalizeLabels` (unchanged, still unexported).
- `internal/cli/init.go:20` (`prefixRE`) and `:48-51` (`initAction` check, message `prefix must be 2-8 uppercase alphanumerics starting with a letter`); `internal/cli/init_test.go:73` `TestInitBadPrefix` pins the message and the rejected inputs `A`, `ABCDEFGHI`, `awit`, `1AB`, `AB-C`, `Ab`, `""`.
- `pkg/config/config_test.go` — `package config` tests; existing `TestLoad*`, `TestTemplateLoad*`, `TestDeclaredLabelsLoad`, `TestWrite*RoundTrip` must stay green unmodified.
- `docs/schema.md:49-58` — the rule table these functions implement.

## Files

- `pkg/config/config.go` — `Duration.String`, `ValidPrefix` (+ `prefixRE` var, `regexp` import), `Normalize`; `Load` and `MarshalYAML` rewritten to call them.
- `pkg/config/config_test.go` — `TestNormalize`, `TestValidPrefix`, `TestDurationString` added.
- `internal/cli/init.go` — `prefixRE` deleted, `regexp` import dropped, `initAction` calls `config.ValidPrefix`.

## Interfaces

```go
// pkg/config/config.go

// String renders d the way Write emits it: "0s", whole hours as "2h",
// whole minutes as "90m", otherwise time.Duration's form ("1m30s").
func (d Duration) String() string

func (d Duration) MarshalYAML() (any, error) { return d.String(), nil }

var prefixRE = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,7}$`)

// ValidPrefix reports whether p matches the id prefix grammar
// ^[A-Z][A-Z0-9]{1,7}$ that awit init enforces (2-8 uppercase
// alphanumerics starting with a letter). Load does not call it: existing
// repositories keep loading whatever prefix they were initialised with.
func ValidPrefix(p string) bool { return prefixRE.MatchString(p) }

// Normalize applies Load's post-decode rules to a copy of c and returns
// it: prefix required, zero stale_claim → 2h, template path checks, labels
// entry rules with in-memory dedupe. default_labels are not checked.
func (c Config) Normalize() (Config, error)

// Load reads awitDir/config.yaml: yaml.Unmarshal followed by Normalize.
func Load(awitDir string) (Config, error)

## Comments

### 2026-09-25T17:48:07Z jan

implemented
