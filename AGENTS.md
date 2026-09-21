# awit

Zero-daemon Go CLI. Design spec and contract: `docs/design-spec.md`. On-disk schema: `docs/schema.md`. Closed v1 work items: `.awit/archive/`.

- Module `github.com/eisenwinter/awit`. CLI `github.com/urfave/cli/v3` (not cobra). YAML `gopkg.in/yaml.v3`.
- Implement one work item at a time. Package map: spec §7; exact signatures live in code (`go doc`). TDD.
- Orchestrator (`.omp/agents/orchestrator.md`) commits. `dev` never runs git.
- Record external holds with `awit block <id> --reason "<obstacle and release condition>"`; clear with `awit unblock <id>` only after the condition resolves. A `blocked` label never pauses work, and `awit release` never clears a hold.
