# Git Commit Guidelines

Rules for committing code during implementation work.

## Commit Message Rules

Forgectl synthesizes the primary commit message itself — at IMPLEMENT, from the item's `description` field in plan.json; at the batch-terminal commit, from the batch's item descriptions. You do not write these messages. If you also want to add context, pass `--message "<text>"` on the `advance` call — that text is **appended** to the synthesized message, not a replacement for it:

- Keep any appended `--message` text descriptive and focused on the "why," matching the synthesized message's tone.
- Do not mention that you are an AI, LLM, or Claude Code in any appended `--message` text.
- Do not append `Co-Authored-By: <model> <noreply@anthropic.com>` to any appended `--message` text.

## When Commits Happen

There are two commit points in the forgectl workflow, both automatic when `enable_commits: true` — you never run `git add` or `git commit` yourself:

1. **IMPLEMENT (every round)** — `forgectl advance` auto-commits the current item, first round and every round after an eval FAIL alike (message synthesized from the item's `description`; `--message` optional and appended).
   ```bash
   forgectl advance
   # or, to add context:
   forgectl advance --message "<extra context>"
   ```
2. **Terminal EVALUATE (batch-terminal commit)** — the moment EVALUATE reaches a terminal verdict (PASS at/above `min_rounds`, or FAIL force-accepted at `max_rounds`), that same `advance` auto-commits the batch inline (message synthesized from the batch's item descriptions) and proceeds directly to ORIENT/DONE:
   ```bash
   forgectl advance --eval-report <path> --verdict PASS
   ```
   There is no separate COMMIT state to visit when `enable_commits: true` — the commit and the transition happen together.

## When `enable_commits: false`

The COMMIT state still exists in the state machine as a bookkeeping checkpoint, but no git operation occurs there or anywhere else. Advance through it with `forgectl advance` (no flags, no manual `git add`/`git commit` required by the scaffold — though you remain free to commit manually on your own schedule since forgectl isn't doing it for you).
