# Git Commit Guidelines

Rules for committing code during implementation work.

## Commit Message Rules

- Write a descriptive message summarizing the changes.
- Do not mention that you are an AI, LLM, or Claude Code in the commit message.
- Do not append `Co-Authored-By: <model> <noreply@anthropic.com>` to the commit message.

## When Commits Happen

There are two commit points in the forgectl workflow:

1. **IMPLEMENT (round 1)** — `forgectl advance --message "<msg>"` auto-commits per item. You do not need to run `git add` or `git commit` manually.
2. **COMMIT state** — You stage and commit manually before advancing:
   ```bash
   git add -A && git commit -m "<descriptive message>"
   forgectl advance --message "<commit message>"
   ```

## At COMMIT State

1. Ensure tests pass for all code in the batch.
2. Add a log entry to `{domain}/.forge_workspace/implementation/IMPLEMENTATION_LOG.md`.
3. Stage all relevant files: `git add -A`
4. Commit with a descriptive message: `git commit -m "<message>"`
5. Advance the scaffold: `forgectl advance --message "<message>"`
