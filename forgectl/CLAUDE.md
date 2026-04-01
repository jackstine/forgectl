# forgectl

Go CLI for managing the software development lifecycle scaffold.

## Commands

```bash
cd forgectl
go build -o forgectl .    # build
go test ./...             # run all tests
go test ./state/ -v       # state package tests
go test ./cmd/ -v         # command tests
```

## Structure

- `cmd/` — Cobra CLI commands (init, advance, status, eval, validate, add-queue-item, set-roots)
- `state/` — State machine types, transitions, validation, output, git operations
- `evaluators/` — Evaluation prompts for plan and implementation sub-agents
- `specs/` — Specification files
