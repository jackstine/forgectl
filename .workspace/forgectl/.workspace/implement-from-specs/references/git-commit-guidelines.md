# Git Commit Guidelines

Rules for committing code during implementation work.

## Commit Message Rules

- Write a descriptive message summarizing the changes.
- Do not mention that you are an AI, LLM, or Claude Code in the commit message.
- Do not append `Co-Authored-By: <model> <noreply@anthropic.com>` to the commit message.

## Commit Workflow

1. Ensure tests pass for the code you changed.
2. Add a log entry to `{domain}/.workspace/implement-from-specs/IMPLEMENTATION_LOG.md`.
3. Stage all relevant files: `git add -A`
4. Commit with a descriptive message: `git commit -m "<message>"`
