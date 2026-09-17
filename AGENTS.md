# awit

Zero-daemon Go CLI. Spec: `plan/awit-implementation-plan.md`. Contract: `plan/implementation-guide.md`. Tickets: `.awit/items/`.

- Module `github.com/eisenwinter/awit`. CLI `github.com/urfave/cli/v3` (not cobra). YAML `gopkg.in/yaml.v3`.
- Implement one ticket at a time. Guide §4 signatures are the API. TDD.
- Orchestrator (`.omp/agents/orchestrator.md`) commits. `dev` never runs git.
