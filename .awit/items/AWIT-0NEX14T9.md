---
id: AWIT-0NEX14T9
title: 'skill: driving-awit misstates author resolution for comment and claim'
brief: >-
  The driving-awit skill claims comment and next --claim both fail with 'Error: no author' when AWIT_AGENT is unset. Neither is true: comment silently falls back to git user.name and signs agent work with the human's name, and claim fails with a different message, 'no agent identity'. An agent that forgets the export is never told.
status: open
deps: []
labels: [phase5, p1]
refs:
  - ../../plan/implementation-guide.md
  - ../comments/AWIT-0NEX14T9/20260918T122427Z-claude.md
---

## Summary

`.omp/skills/driving-awit/SKILL.md` tells agents, twice, that identity is
enforced:

- line 19: "Without `AWIT_AGENT`, `next --claim` and `comment` fail with
  `Error: no author`."
- line 112, escalation ladder: `Error: no author` → "Identity not set." →
  `export AWIT_AGENT=<name>` and rerun.

Neither command behaves that way. The two commands resolve identity through
**two different chains**, and the skill describes a third thing that exists
in neither.

`comment` uses `resolveAuthor` (`internal/cli/author.go`):

    --author → AWIT_AGENT → config.agent_id → git user.name → error

`next --claim` uses `config.Config.Agent` (`pkg/config/config.go:90`):

    --agent → AWIT_AGENT → config.agent_id → error

So with `AWIT_AGENT` unset:

- `awit comment <id> "…"` **succeeds**. It falls through to git
  `user.name`, lowercased with spaces hyphenated, used **verbatim — no
  `agent/` prefix**. Reproduced on this work item's parent while writing it:
  the comment was filed as author `jan`, the repo owner, for work an agent
  did.
- `awit next --claim` **fails**, but with `no agent identity; pass --agent
  or set AWIT_AGENT` — not `no author`. An agent that learned the string
  `Error: no author` from the skill will not match it.

The implementation is not the bug. `plan/implementation-guide.md:42` and
`docs/schema.md:150` both specify exactly this chain, deliberately: humans
get their git name, agents get `agent/<name>`. `awit comment --help` states
it correctly too. Only the skill is wrong.

## Context (read first)

- `internal/cli/author.go` `resolveAuthor` — the comment chain, including
  the git fallback and the `withAgentPrefix` asymmetry.
- `pkg/config/config.go:90` `Agent` — the claim chain, no git fallback.
- `internal/cli/next.go:99` — the `no agent identity` error text.
- `plan/implementation-guide.md:42` — decision 4, the intended contract.
  This is the source of truth; the skill must be corrected to match it,
  not the other way round.
- `docs/schema.md:150` — the comment `author` field, already correct.

## Why this is p1 and blocks AWIT-0NEWKJTD

AWIT-0NEWKJTD embeds this skill into the binary and seeds it into every
`.claude`, `.omp`, `.opencode`, `.agents` and `.pi` directory it finds. A
wrong sentence that today lives in one repo would ship to every repo that
runs `awit init`. Fix the text before it is distributed, not after.

The audit-trail consequence is the part worth weighing: an agent that
forgets `export AWIT_AGENT` does not get an error, it gets a comment signed
with the human's name. Nothing in the output says so. The skill currently
guarantees the opposite.

## Files

- Modify: `.omp/skills/driving-awit/SKILL.md` — the Setup paragraph
  (line 19) and the escalation-ladder row (line 112). If AWIT-0NEWKJTD has
  already moved the body to `internal/skill/assets/driving-awit.body.md`,
  edit it there and regenerate.

## Interfaces

None. Documentation only — no Go signature changes, no behaviour change.

## Steps

- [ ] Rewrite the Setup sentence to state both chains and their different
      failure modes: `comment` falls back to git `user.name` and never
      fails; `next --claim` refuses with `no agent identity`.
- [ ] Make the consequence explicit for an agent: without `AWIT_AGENT`
      your comments are attributed to the repo's git user, unprefixed, and
      nothing warns you. Setting it is what makes the audit trail true.
- [ ] Replace the escalation-ladder row `Error: no author` with
      `Error: no agent identity` (the string `next --claim` actually
      prints), same remedy.
- [ ] Check the rest of the skill for the same conflation — the Vocabulary
      table's `assignee` row and the Quick reference table both assume
      identity is always `agent/<name>`.
- [ ] Re-read `plan/implementation-guide.md:42` against the final text,
      word by word. The skill must not invent a third contract.

## Acceptance Criteria

- [ ] `grep -n "no author" .omp/skills/driving-awit/SKILL.md` returns only
      the `description` frontmatter line (which lists it as a symptom
      phrase users may type), or nothing.
- [ ] The skill states that `comment` succeeds without `AWIT_AGENT` and
      names git `user.name` as the fallback.
- [ ] The skill states the exact string `next --claim` prints.
- [ ] Every identity claim in the skill matches
      `plan/implementation-guide.md:42` and `pkg/config/config.go:90`.
- [ ] `awit validate` prints PASS.

## Out of scope

- Changing `resolveAuthor` or `Config.Agent`. The behaviour is the
  documented decision; only the skill is wrong.
- Warning on stderr when the git fallback is used. Defensible, and it
  would make the silent misattribution visible — but it is a behaviour
  change to a shipped contract and belongs in its own work item with its own
  argument.
- Unifying the two chains into one.
