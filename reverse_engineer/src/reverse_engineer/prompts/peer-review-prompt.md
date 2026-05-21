# Peer Review Prompt

## Purpose
Sent as a follow-up to the primary agent after the initial draft. Instructs the agent to spawn reviewer sub-agents in parallel to evaluate the spec and then synthesize their feedback into the spec file.

## Used By
EXECUTE state — primary agent follow-up, when `peer_review` mode is enabled. Sent `peer_review.rounds` times after the initial draft.

## Interpolation Fields
- `{peer_review.reviewers}` — number of reviewer sub-agents to spawn
- `{peer_review.subagents.model}` — model for reviewer sub-agents
- `{peer_review.subagents.type}` — type for reviewer sub-agents
- `{file}` — the spec file path to review
- `{code_search_roots}` — directories to verify spec accuracy against

## Prompt

TODO: Write the full peer review prompt content. The prompt must cover:

- You have drafted a specification at `{file}`
- Spawn `{peer_review.reviewers}` `{peer_review.subagents.model}` `{peer_review.subagents.type}` sub-agents in parallel to review your work
- Each sub-agent receives:
  - The spec file to review: `{file}`
  - The source code to verify against: `{code_search_roots}`
  - The spec format reference (included below)
- Each reviewer evaluates the spec against:
  - Topic of concern: single sentence, no "and", describes an activity
  - Declarative voice: no "should", "could", "might"
  - Every behavior has testing criteria (Given/When/Then)
  - Error handling is exhaustive: every failure mode named
  - Edge cases have scenario, expected behavior, and rationale
  - Invariants are always-true, testable properties
  - Observability: INFO for success, ERROR for failures, DEBUG for diagnostics
  - No open questions or TBDs
  - Code accuracy: does the spec match what the code actually does?
- Each reviewer reports back:
  - Issues found (with specific section references)
  - Missing behaviors found in code but not in spec
  - Suggested corrections
- After all reviewers report back, synthesize their feedback and update the spec file at `{file}`
- Resolve conflicting feedback using your judgment
- Do not add reviewer notes to the spec — incorporate the fixes directly
- The spec format reference is appended below for reviewer context
