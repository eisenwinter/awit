# awit

> Cast away the eldritch horror of TICKETS.md. A zero-daemon, git-native task graph built for agents and humans alike.[^1]

**Agent work item tool.** A zero-daemon Go CLI that turns Markdown files under
`.awit/` into a dependency graph that humans and agents work from — offline,
versioned in Git, no database, no daemon, no server.

Every command rebuilds the graph from `.awit/items/*.md`, operates on it, and
writes back at most one file. A clone is the whole state: nothing to migrate,
nothing to repair beyond text files. Work items carry `deps`, so the tool can
answer the only question an agent actually needs answered — _what can I start
right now, and what does finishing it unblock?_

- **Ready, not assigned.** Eligibility is derived from the graph, not stored.
- **Every fault is one mechanism.** Cycles, dangling deps, unparseable
  frontmatter, Git conflict markers and duplicate IDs all become quarantine,
  and `awit validate` prints the command that fixes each one.
- **Git is the sync protocol.** Two agents racing a claim produce a merge
  conflict, which quarantine surfaces — instead of a silent double-claim.
- **Labels adapt to any workflow.** Free-form grouping sets with no enforced meaning — `p0`…`p4` for priority, areas, phases, whatever you need; filter them with `-l`.

[Install and first steps](usage.md){ .md-button .md-button--primary }
[How it works](how-it-works.md){ .md-button }

## Quickstart

Paste this to your agent:

```text
Bootstrap awit in the current repository: download the release binary for
this machine from github.com/eisenwinter/awit/releases (assets are named
awit_<version>_<os>_<arch>.tar.gz, .zip on Windows), put it on PATH, then
run awit init --skills in the repo root and report the .awit layout and
the seeded skills it created.
```

## A single agent working the queue

```text
well once i figure out how to get those session down to a reasonabel size they will be here :D
```

## An orchestrator with parallel workers

```text
same as above sorry :<
```

## Where to go next

| Page                            | What it answers                                          |
| ------------------------------- | -------------------------------------------------------- |
| [How it works](how-it-works.md) | The store, the graph, ranking, quarantine, archive       |
| [Setup & Usage](usage.md)       | Install, `awit init`, the agent loop, every command      |
| [Design Spec](design-spec.md)   | Decisions, command matrix, package layout — the contract |
| [On-disk Schema](schema.md)     | The v1 file format, key by key                           |
| [FAQ](faq.md)                   | Short answers to recurring questions                     |

[^1]: [WHY ?!](faq.md#why)
