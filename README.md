# awit

Zero-daemon Go CLI that turns Markdown files under `.awit/` into a dependency
graph that humans and agents work from. Offline, versioned in Git, no database.
Every command rebuilds the graph from `.awit/items/*.md`; a clone is the whole
state.

**Status:** pre-alpha. The design is fixed
([plan/awit-implementation-plan.md](plan/awit-implementation-plan.md)); the
code is being built ticket by ticket. This repository dogfoods itself: the
implementation tickets live in [`.awit/items/`](.awit/items/) in the exact
format `awit list` will read once phase 2 lands.

## The loop

```
awit prime                 # token-light snapshot of the graph
awit next --claim          # lock the top unblocked item
awit show <id> --full      # ticket + resolved refs
awit comment <id> "..."    # research notes
awit close <id>            # unblocks downstream
```

Each step is one process invocation. State between steps lives only in files
and in Git.

## Layout

```
.awit/
├── config.yaml            # prefix, default_labels, stale_claim, agent_id
├── items/PREFIX-XXXXXXXX.md
└── comments/PREFIX-XXXXXXXX/<UTC seconds>-<author>.md
```

An item is YAML frontmatter (`id`, `title`, `brief`, `status`, `deps`,
`labels`, `assignee`, `claimed_at`, `refs`) followed by a Markdown body.
Priority is a label by convention (`p0`…`p4`). Blocked is derived, never
stored.

The on-disk schema, including the reserved `external:` key, is
documented in [docs/schema.md](docs/schema.md).

## Commands

`init` · `create` · `list` · `label` · `show` · `comment` · `update` · `close` ·
`release` · `dep add|rm` · `validate` · `prime` · `next`

Global flags: `--format compact|table|json`, `--repo <path>`, `--no-color`.

## Building

```
go build ./cmd/awit
go test ./...
```

Requires Go 1.27+. CLI framework: [urfave/cli v3](https://github.com/urfave/cli).

## Contributing / implementing

Read [plan/implementation-guide.md](plan/implementation-guide.md) first. It
holds the resolved design decisions, the package layout, every shared Go
interface, and the ticket index with dependency order. Then pick a ticket from
`.awit/items/` whose `deps` are all closed.

## License

BSD 2-Clause. See [LICENSE](LICENSE).
