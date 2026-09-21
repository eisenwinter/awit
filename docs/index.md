# awit

**Agent work item tool.** A zero-daemon Go CLI that turns Markdown files under
`.awit/` into a dependency graph that humans and agents work from — offline,
versioned in Git, no database, no daemon, no server.

Every command rebuilds the graph from `.awit/items/*.md`, operates on it, and
writes back at most one file. A clone is the whole state: nothing to migrate,
nothing to repair beyond text files. Work items carry `deps`, so the tool can
answer the only question an agent actually needs answered — *what can I start
right now, and what does finishing it unblock?*

- **Ready, not assigned.** Eligibility is derived from the graph, not stored.
- **Priority is a label.** `p0`…`p4` by convention; one ranking axis, not two.
- **Every fault is one mechanism.** Cycles, dangling deps, unparseable
  frontmatter, Git conflict markers and duplicate IDs all become quarantine,
  and `awit validate` prints the command that fixes each one.
- **Git is the sync protocol.** Two agents racing a claim produce a merge
  conflict, which quarantine surfaces — instead of a silent double-claim.

[Install and first steps](usage.md){ .md-button .md-button--primary }
[How it works](how-it-works.md){ .md-button }

## A single agent working the queue

<!-- TODO: replace with a real captured session. Should show:
     awit prime → awit next --claim → awit show <id> --full →
     awit comment <id> → awit close <id> → awit prime again,
     with the unblock counts visibly changing across the two primes. -->

```text
TODO: paste a real single-agent session log here.
```

## An orchestrator with parallel workers

<!-- TODO: replace with a real captured session. Should show:
     one orchestrator running awit prime, handing two independent ready items
     to two workers, each claiming with awit next --claim -l <label>,
     one worker hitting an obstacle and recording it with
     awit block <id> --reason "...", and the orchestrator's next prime
     showing the hold and the remaining ready set. -->

```text
TODO: paste a real multi-agent orchestration log here.
```

## Where to go next

| Page | What it answers |
| --- | --- |
| [How it works](how-it-works.md) | The store, the graph, ranking, quarantine, archive |
| [Setup & Usage](usage.md) | Install, `awit init`, the agent loop, every command |
| [Design Spec](design-spec.md) | Decisions, command matrix, package layout — the contract |
| [On-disk Schema](schema.md) | The v1 file format, key by key |
| [FAQ](faq.md) | Short answers to recurring questions |
