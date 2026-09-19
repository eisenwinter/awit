---
id: AWIT-0NJC0BDX
title: 'import: derive an omitted brief from the remote title or body'
brief: >-
  Make import derive a short brief from the remote title, falling back to the body's first sentence. Preserve explicit summaries, exact imported content, and create's required-brief contract.
status: open
deps: []
labels: [phase5, p2]
refs_base: repo
refs: []
---
## Summary

Make `awit import <issue-url> [--brief <summary>]` accept an omitted brief. Derive it from a normalized remote title when nonblank; otherwise derive the body's first sentence using the existing validator's punctuation-boundary convention. Limit automatically derived briefs to 240 Unicode code points. Keep import's title/body/labels/status snapshot behavior unchanged, and keep `create --brief` required.

## Context (read first)

- `AGENTS.md`; guide §§1, 2, 4.11 and 5.
- `.awit/archive/AWIT-0NHDBJDR.md`, closed. Its mandatory-import-brief/no-heuristic decision is superseded for import only; leave the archived record untouched.
- `internal/cli/import.go`: `importCmd`, `importAction`, `validateImportCandidate`, `refuseDuplicateImport`.
- `internal/cli/import_test.go`: `TestImportRequiresBrief`, `TestImportFaithful`, `TestImportByteExactBody`, `TestImportNullBody`, `TestImportGitLabFaithful`, and the existing tea/glab test helpers.
- `internal/cli/validate.go`: `sentenceCount` and existing missing/long-brief warnings.
- `internal/cli/create.go`: retain its independent required-brief guard.

## Files

- Modify `internal/cli/import.go` and `internal/cli/import_test.go`.
- Leave `create.go`, tracker transport schemas, item parsing/serialization and `validate.go` behavior unchanged. Keep and run existing create-required-brief regressions.
- Update `docs/schema.md` Optional known keys/external import prose, `plan/implementation-guide.md` §2 import-summary decision, `plan/awit-implementation-plan.md` Data model/CLI command matrix, `README.md` Commands, and `internal/skill/assets/driving-awit.body.md` Adding work items/Quick reference.
- Regenerate `.omp/skills/driving-awit/SKILL.md`.

## Interfaces

Add a private pure helper in the existing import file, not a new package:

```go
func deriveImportBrief(title string, body []byte) string
```

Exact behavior:

1. `brief := cmd.String("brief")`. If nonempty, use those bytes unchanged: no normalization, truncation or sentence extraction. This preserves caller-controlled brief behavior, including existing validator warnings for whitespace-only or overly long explicit values. As required by guide §5's reusable-command convention, `--brief=""` is the unset value and invokes derivation; do not use `cmd.IsSet`.
2. For automatic derivation, choose the title if it contains any non-whitespace rune. Otherwise choose the body. A blank title is **not** replaced in the stored title field; the fallback affects only the brief.
3. Normalize the chosen source by trimming leading/trailing Unicode whitespace and collapsing each internal Unicode whitespace run to one ASCII space. CRLF, LF, tabs and blank lines therefore become spaces. Do not strip Markdown syntax, decode entities, alter case or rewrite punctuation.
4. For the **title**, use the whole normalized title, subject to the cap. Do not split it at punctuation.
5. For the **body**, stop at the first `.`, `!` or `?` immediately followed by whitespace or end-of-source, including the punctuation. A newline alone is not a sentence boundary. Punctuation inside a token does not end the sentence; there are no abbreviation or quote heuristics. If no boundary exists, use the normalized body subject to the cap. This deliberately matches `sentenceCount`'s simple boundary definition.
6. Cap only derived values at 240 Unicode code points, not bytes. If the candidate is at most 240 runes, preserve it. If it is longer, take its first 239 runes, remove any trailing normalized ASCII space from that prefix, and append U+2026 (`…`). Thus the result is at most 240 runes, valid UTF-8, and has no space immediately before the ellipsis. Do not search backward for word boundaries.
7. Avoid constructing a normalized copy of an arbitrarily long remote body: scan runes and accumulate at most the 240-rune candidate plus sufficient lookahead to distinguish exact fit, truncation and the punctuation boundary. Stop once the final output is determined. Reuse stdlib Unicode/UTF-8 primitives; add no text/NLP dependency.
8. If both title and body normalize to empty and no nonempty explicit brief was supplied, refuse before locking/minting/saving with exit 1 and exactly:

```text
Error: cannot derive import brief: remote title and body are empty; pass --brief
```

   A nonempty explicit brief still allows this otherwise valid snapshot. Missing/null remote title fields remain existing transport-schema errors, not fallback inputs. Null body continues to mean empty.
9. Remove `Required: true` only from import's brief flag and remove import's unconditional required-brief action guard. Derive after remote fetch/identity/state validation and before the mutation lock. Pass the selected value to the existing `item.New`; keep `it.SetBody(issue.Body)` byte-exact and retain all duplicate/candidate validation.
10. Do not change `validate`: derived nonblank values avoid missing-brief warnings, but a multi-sentence title or explicit brief can still receive the existing `brief is longer than 3 sentences` warning. An explicit whitespace-only value retains the existing `missing brief` warning. Warnings remain advisory, not new import errors.

Examples:

| Title | Body | Derived brief |
| --- | --- | --- |
| `  Fix\tparser\r\nbehavior  ` | anything | `Fix parser behavior` |
| whitespace only | `First line\ncontinues. Next sentence!` | `First line continues.` |
| empty | `Version 1.2 works! Next.` | `Version 1.2 works!` |
| empty | `No punctuation\nsecond line` | `No punctuation second line` |
| 241 repetitions of `é` | anything | 239 repetitions of `é`, then `…` |

## Steps

- [ ] Replace `TestImportRequiresBrief` with a command test asserting import without the flag succeeds and stores the remote title as brief while preserving remote body bytes. Retain `TestImportFaithful` as explicit-override coverage, and add a GitLab no-brief import case using the existing portable fixture.
- [ ] Add `TestDeriveImportBrief` table cases for title precedence, Unicode whitespace/CRLF normalization, blank-title body fallback, punctuation followed by whitespace/EOS, embedded punctuation, line breaks without punctuation, no terminator, 240/241-rune boundaries, multibyte truncation and empty sources. Add command cases for empty-source error/no creation, nonempty explicit override on empty sources, and repeated `Main` calls with explicit then omitted/empty brief.

Representative replacement command regression:

```go
func TestImportDefaultsBriefFromTitle(t *testing.T) {
    repo, stub := importRepo(t)
    writeTeaIssue(t, stub, 127, faithfulIssue)
    code, stdout, stderr := run(t, "--repo", repo, "import",
        "https://forge.example/owner/repo/issues/127", "--tea-login", "sandbox")
    if code != 0 { t.Fatalf("exit %d, stderr %q", code, stderr) }
    it := readItem(t, repo, itemIDFromCompact(t, stdout))
    if it.Brief != "Fix header parsing" {
        t.Fatalf("brief = %q", it.Brief)
    }
    if string(it.Body()) != "Line one.\n\nLine two.\n" {
        t.Fatalf("body changed: %q", it.Body())
    }
}
```

- [ ] Run `go test ./internal/cli -run 'Import|DeriveImportBrief|Create' -count=1`; record the new omission/fallback failures before implementation.
- [ ] Implement `deriveImportBrief` in `import.go` using the specified bounded rune scan. Remove import-only mandatory-brief checks, resolve explicit versus derived brief after the existing fetch validation, refuse an empty derivation before mint/save, and pass the result to `item.New`. Leave `createAction` and transport/body handling untouched.
- [ ] Run the scoped command to green. Smoke the built CLI against disposable subprocess-backed issues: omitted brief derives; explicit brief wins; both sources empty fails without a new item; `create` without a brief still exits 2. Inspect stored body bytes as well as displayed brief.
- [ ] Update all listed docs with the exact derivation and empty-source rules, clarify that the skill's mandatory `--brief` statement is specific to `create`, and regenerate the skill copy. Run `go test ./internal/skill -run TestDogfoodOmpCopyMatchesRenderer -count=1`; submit scoped proof for orchestrator review/commit, not project-wide validation.

## Acceptance Criteria

- `go test ./internal/cli -run 'Import|DeriveImportBrief|Create' -count=1` passes with Gitea and GitLab command coverage.
- `awit import <disposable-issue-url>` succeeds without `--brief`, stores the normalized remote title or specified first-sentence fallback, and retains exact remote title/body snapshot bytes, mapping, labels and status.
- `awit import <disposable-issue-url> --brief 'Chosen summary.'` stores exactly `Chosen summary.` regardless of remote title/body length. Explicit text is not capped or reformatted. Empty flag value follows the documented unset convention.
- Automatically derived output is valid UTF-8 and at most 240 runes; 240-rune exact fit is not truncated, while longer input uses the specified single-character ellipsis. Newlines alone never prematurely terminate fallback sentences.
- Blank title plus null/blank body, without a nonempty override, exits 1 with the specified message and creates no item. Missing remote title remains the existing malformed-response error. No placeholder summary or remote title replacement is fabricated.
- `awit create 'Example'` without `--brief` still exits 2 and creates no item, including after repeated in-process calls. `validate` retains its existing advisory brief warnings.
- Import remains read-only remotely, does not commit, and keeps duplicate detection, candidate validation and byte-exact body behavior. Docs and renderer parity checks pass.

## Out of scope

Making create's brief optional; generated/NLP summaries; Markdown stripping; changing stored titles or remote bodies; enforcing a new global brief length limit; changing validator sentence counting; import-as-update; remote schema relaxation; editing archived work-item history.

# E. Simplification Analysis


