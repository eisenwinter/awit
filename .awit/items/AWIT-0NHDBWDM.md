---
id: AWIT-0NHDBWDM
title: 'prime: preserve the top ready item before spending budget on framing'
brief: >-
  Replace scaffolding-first admission with deterministic payload-preserving truncation. Keep at least one complete ready row for every positive budget when a filtered ready item exists, and document the unavoidable soft-budget floor.
status: closed
deps: [AWIT-0ND56W3G, AWIT-0ND56V3G]
labels: [phase3, p1]
refs_base: repo
refs: [.awit/comments/AWIT-0NHDBWDM/20260919T151901Z-orchestrator.md]
---
## Summary

A budgeted snapshot must not consist only of headers and a critical path while hiding every ready item. Keep the highest-ranked complete ready row, retain all graph warning details, and shed lower-value material deterministically.

## Context (read first)

- `pkg/prime/prime.go`: `Render`, `assemble`, `EstimateTokens`.
- `pkg/prime/prime_test.go:TestPrimeMaxTokens` currently pins critical-path survival; that expectation is superseded.
- Guide §2 token estimate remains `len(bytes)/4`. Unlimited snapshot ordering remains unchanged.

## Files

- Modify `pkg/prime/prime.go`, `pkg/prime/prime_test.go`, `internal/cli/prime.go`, relevant CLI tests.
- Update guide §§2/4.8, spec Agent surface/prime, README flag description, embedded skill Reading prime.

## Interfaces

Keep `Options` and `Render` signatures. `MaxTokens == 0` remains unlimited; negative CLI budgets are usage errors, exit 2.

Algorithm:
1. Render complete deterministic content using existing ready and blocked ordering. Compute full rendered byte costs including separators and omission notices; do not repeatedly rebuild growing candidate strings.
2. If it fits, emit it byte-for-byte as today.
3. While over budget, remove BLOCKED rows from the end first; then READY rows from the end, **never the first ready row**. Keep retained rows as prefixes; never skip a long higher-ranked row to admit a shorter lower-ranked row.
4. If still over, remove the critical-path section as a whole. This is contextual payload, not more important than the work the agent can claim.
5. If still over, remove optional scaffolding: headers for empty omitted sections, omission suffix, READY/BLOCKED headers and extra separators. Keep warning reason/detail lines in full; their section heading is optional at this last stage.
6. Stop at the safety floor: all warning details plus one full top ready row if any exists after filtering. If that minimum exceeds N, emit it anyway. `--max-tokens` is explicitly a **soft budget with this minimum**, not an impossible promise that one full row fits into one token.

Whenever displayed, counts remain post-filter/pre-truncation; `(+N more)` counts omitted ready+blocked rows, not removed headers or critical-path nodes. Preserve exactly one terminal LF. No timestamps or random ordering.

## Steps

- [ ] Add failing regression cases for `MaxTokens=1`, a long top ready row, warnings larger than the budget, filtered no-ready graphs, and accurate costs including the omission suffix. Replace the old critical-path-always-survives assertion with the new priority invariant; do not weaken unlimited goldens.
- [ ] Run `go test ./pkg/prime ./internal/cli -run 'Prime' -count=1`; record the intended failures.
- [ ] Replace `assemble`’s admission logic with prefix counts and precomputed lengths; choose the retained form before writing it once. Preserve `EstimateTokens` exactly.
- [ ] Verify budgets reduce retained prefixes deterministically, warnings never disappear, and the top ready row survives any positive budget. Check both byte-equal repeated renders and unchanged unlimited clean/cyclic goldens.
- [ ] Rerun the scoped tests, update budgeting docs, and send the orchestrator evidence.

## Acceptance Criteria

- `go test ./pkg/prime ./internal/cli -run Prime -count=1` passes.
- `awit --repo testdata/fixtures/clean prime --max-tokens 1` includes the complete AWIT-TEST0001 ready row, not just its ID inside a critical path.
- Two identical budgeted invocations produce identical bytes. Unlimited outputs retain existing goldens.
- If output exceeds the estimate, tests demonstrate it is exactly the documented mandatory floor, not forgotten scaffolding or omission accounting.

## Out of scope

Changing ranking, adding a tokenizer dependency, truncating item titles mid-row, hiding graph faults, making a mathematically impossible hard one-token guarantee.

