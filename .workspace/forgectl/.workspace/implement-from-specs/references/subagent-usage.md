# Subagent Usage

Guidelines for using subagents effectively during implementation work.

## When to Use Subagents

- **Codebase search:** Before implementing anything, use a subagent to search the codebase and confirm the feature doesn't already exist.
- **Updating documents:** Use a subagent to update `{domain}/.workspace/implement-from-specs/IMPLEMENTATION_LOG.md` or `{domain}/CLAUDE.md` so the main agent stays focused on implementation.
- **Build and test:** Use only one subagent for build/test operations to avoid conflicts.

## When NOT to Use Subagents

- For trivial, single-file edits where the main agent can handle it directly.
- For operations that require sequential, stateful interaction (e.g., interactive debugging).

## Subagent Principles

- Subagents have no memory of previous sessions — provide complete context in the prompt.
- Keep subagent tasks focused and well-scoped.
- Prefer one subagent per concern (search, update docs, run tests) rather than one subagent doing everything.
