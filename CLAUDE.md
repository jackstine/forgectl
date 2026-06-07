# Forgectl Root

Spec-driven development harness. Compiles planning documents into production code through specifying, planning, and implementing phases.

## External Resources
### Context 7 MCP resources
- [cobra](https://context7.com/spf13/cobra)
- [go-yaml](https://context7.com/goccy/go-yaml)
- [golangci-lint](https://context7.com/golangci/golangci-lint)
- [go-git](https://context7.com/go-git/go-git) Version 5 only (Version 6 is in pre-release at this moment)
- [toml] (https://context7.com/burntsushi/toml)


## Agent Handoffs

When an agent generates a file to be handed off to the next step, the agent must register that file with forgectl rather than just leaving it on disk:

```bash
forgectl <command> file_name.md
```

Registering the file makes forgectl aware that a handoff is pending, so the next `forgectl advance` tells the operator (or next agent) that there is a file provided by the agent which needs to be reviewed before the workflow can move forward. This closes the gap where an agent's output can be silently passed over between phases.

## Skills

Do not update skill files (`skills/`) until implementation planning has begun. Skills consume forgectl — updating them before the work is even planned creates a chicken-and-egg problem. Once a phase reaches implementation planning, its skill may be drafted and committed alongside the plan.

## Build

```bash
make build           # build forgectl binary
make install-global  # install to ~/.local/bin/forgectl
```

## Keeping Docs in Sync

When specifications, Go code, or configurations change, the derived documentation must be updated to match.

### Diagrams (`docs/diagrams/`)

Update any diagram affected by architecture changes — state machines, workflows, CLI commands, evaluation criteria, skills, or data flow. Read the diagram files to determine which ones are impacted.

### Schemas (`docs/schemas/`)

Update schema docs when JSON structures change — fields added/removed/renamed in Go types, validation rules changed, or path resolution behavior changed. Read the schema files to determine which ones are impacted.

### Source of truth

- `forgectl/state/types.go` — authoritative source for all JSON schemas
- `forgectl/state/validate.go` — what forgectl accepts and rejects
- `forgectl/specs/` — intended behavior
- Diagrams and schema docs are derived — they follow the code and specs, not lead them.


