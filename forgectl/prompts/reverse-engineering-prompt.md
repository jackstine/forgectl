# Reverse Engineering Prompt

## Purpose
Instructs the primary Claude Agent SDK session to read code within its assigned code search roots and produce or update a single spec file for the given topic of concern.

## Used By
EXECUTE state — primary agent. Concatenated with `spec-format-reference.md` to form the complete agent prompt.

## Interpolation Fields
- `{name}` — display name of the spec
- `{topic}` — one-sentence topic of concern
- `{file}` — target spec file path (relative to domain root)
- `{action}` — "create" or "update"
- `{code_search_roots}` — directories to examine (relative to domain root)
- `{existing_spec_content}` — current spec content (populated for updates, empty for creates)
- `{subagent_type}` — role for sub-agents (e.g., "explorer")
- `{subagent_model}` — model for sub-agents
- `{subagent_count}` — number of sub-agents to use

## Prompt

TODO: Write the full prompt content. The prompt must cover:

- The agent's role: reverse-engineer a specification from existing code
- The assigned topic of concern: `{topic}`
- The target output file: `{file}`
- Whether this is a create or update: `{action}`
- Where to look in the codebase: `{code_search_roots}`
- For updates: the existing spec content to revise: `{existing_spec_content}`
- Constraint: write or edit only the single file at `{file}` — no other files
- Constraint: read-only codebase — do not modify source code
- Constraint: capture what the code *does*, not what it *should* do
- Constraint: the Implements section references the reverse-engineered topic, not a planning document
- Sub-agent usage: spawn `{subagent_count}` `{subagent_type}` sub-agents at `{subagent_model}` to assist with code exploration
- The spec format is provided in a separate concatenated file — follow it exactly
