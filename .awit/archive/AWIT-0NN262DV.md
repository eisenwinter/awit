---
id: AWIT-0NN262DV
title: 'skill: apply the reviewed driving-awit trim and correctness fixes'
brief: >-
  Apply both prose reviews to the driving-awit body asset: must-fix corrections plus the measured token trim, then regenerate the skill copy.
status: closed
deps: []
labels: [phase6, p1]
refs_base: repo
refs: []
assignee: agent/orchestrator
---
## Summary

Apply the two prose reviews (`agent://ProseSkillTrim`, `agent://ProseSkillTrim2`) to `internal/skill/assets/driving-awit.body.md` and regenerate `.omp/skills/driving-awit/SKILL.md`. The patch combines must-fix correctness corrections with a measured ~1,030-1,200 token trim (~20-23% of the injected document). Every operational fact must survive; the second review verified the combined patch loses none.

## Context (read first)

- `AGENTS.md`; guide §§1, 5.
- `agent://ProseSkillTrim` — first review: section-by-section keep/cut/rewrite with exact replacement texts, must-fix vs nice-to-have split, ~700-1,000 token estimate.
- `agent://ProseSkillTrim2` — second review: Part 1 validates all 26 prior entries (20 CONFIRM, 5 PARTIAL, 1 REJECT) with measured deltas; Part 2 adds findings N1-N13 with exact replacements; final split (must-fix / nice-to-have / reject / do-not-touch) and independent estimate. This is the authoritative application order.
- `internal/skill/skill.go`, `internal/skill/skill_test.go:116-141` — regen via `skill.Render`; `TestDogfoodOmpCopyMatchesRenderer` pins the generated copy byte-for-byte. Any asset edit MUST be followed by regeneration or the test fails.
- Fact witnesses cited by review 2: `internal/cli/author.go:22-36`, `comment.go:22`, `update_test.go:246,249-250`, `create_test.go:32,579-580`, `external_state.go:67-91`, `next.go`, `git log -S '## Common mistakes'` (heading lost in `b5530e7`).

## Files

- Modify `internal/skill/assets/driving-awit.body.md` only (prose + structure; no renderer logic).
- Regenerate `.omp/skills/driving-awit/SKILL.md` through the existing renderer; never hand-edit.
- No production code, no behavior change.

## Interfaces

No code interfaces. Text contract: every command/flag, both identity chains, tracker setup/version, all ten escalation rows, exit codes, report-status order, commit ownership, push config/precedence, store-lock ordering, quarantine handling, and file paths survive verbatim in meaning. Exact replacement texts are in the two agent reports; apply review 2's Split, which supersedes review 1 on conflicts (N11 over line-180 rewrite; N9 pipe-free rows over `\|` escaping; `--why` demoted to nit).

## Steps

- [ ] Read both agent reports in full plus the source asset. Confirm the regen command and run the parity tests before editing (baseline green).
- [ ] Apply must-fix first, exactly as specified:
  - N1: line 6 `create` body — replace "leaves empty" claim with skeleton/template truth (review 2 exact text).
  - Identity chains: both resolution paths with `config.agent_id` in the author chain (review 1 text as amended by N2: drop the `Error: no author` string, escalation table carries it).
  - N5: scope line 69 one-claim rule (review 2 six-word fix).
  - N8: Orchestrator step 4 — add `awit unblock <id>` after resolution before re-dispatch (review 2 exact text).
  - N10: restore `## Common mistakes` heading, remove stray blank line.
  - Quick-reference table: repair via N9 pipe-free rows (not backslash escaping).
- [ ] Apply nice-to-have rewrites/cuts per review 2's Split (all review 1 Should-Consider rewrites except the rejected line-180 rewrite; N2, N3, N4, N6, N11, N12; N7 count fix). Cut the line-180 bullet, fold its four unique words into the labels row (N11).
- [ ] Respect do-not-touch: line 66 branch set (self-repetition only, N4), shell spine command lines, prime example's three BLOCKED forms, create example, all ten escalation rows, both exact warning strings.
- [ ] Regenerate the committed copy; `git diff` must show only the intended prose changes in the two files.
- [ ] Run `go test ./internal/skill -count=1` to green. Submit scoped proof for orchestrator review/commit, not project-wide validation.

## Acceptance Criteria

- `go test ./internal/skill -count=1` passes, including `TestDogfoodOmpCopyMatchesRenderer` and `TestRenderIsDeterministic`.
- N1, N5, N8, N10, both identity chains, and the repaired quick-reference table are all present in the asset; `git diff` shows no other-file changes.
- Measured character reduction is within 10% of the predicted ~6,700 (report `wc -c` before/after in a comment).
- No operational fact lost: verify by grepping the patched asset for each command/flag, error string, and file path named in the reports' preservation lists.
- `awit validate` prints PASS.

## Out of scope

Renderer changes; workflow redesign (one-claim scoping is wording only); N13 `--push`+`--no-push` addition (owner call — ask in a comment, do not add silently); rewording beyond the two reports; touching seeded target dirs beyond the committed copy.

## Comments

### 2026-09-20T20:37:32Z jan

Patch applied (must-fix + nice-to-have per review 2 Split) and copy regenerated via renderer. wc -c: body 22755 -> 15861 (-6894, -30.3%), copy 23176 -> 16282 (-6894). Predicted ~6700 reduction: within tolerance. go test ./internal/skill -count=1 green (incl. TestDogfoodOmpCopyMatchesRenderer + TestRenderIsDeterministic); awit validate PASS; git status shows only the two intended files. N13 owner call: review 2 proposes appending 'Combining --push and --no-push is exit 2.' to the --push clause (~12 tokens, an addition not a saving). Add it or leave out?

### 2026-09-20T20:43:06Z agent/orchestrator

implemented
