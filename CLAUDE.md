# Forgectl Root

Spec-driven development harness. Compiles planning documents into production code through specifying, planning, and implementing phases.

## External Resources
### Context 7 MCP resources
- [cobra](https://context7.com/spf13/cobra)
- [go-yaml](https://context7.com/goccy/go-yaml)
- [golangci-lint](https://context7.com/golangci/golangci-lint)
- [go-git](https://context7.com/go-git/go-git) Version 5 only (Version 6 is in pre-release at this moment)



## Skills

Do not update skill files (`skills/`) until the forgectl binary has been built and the specs are implemented. Skills consume forgectl — updating them before the tool exists creates a chicken-and-egg problem. Build forgectl first, then update skills to match.

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
