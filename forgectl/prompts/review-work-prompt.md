# Review Work Prompt

## Purpose
Sent as a follow-up to the primary agent after the initial draft. Instructs the agent to re-read its spec file and critique it against the spec format, topic-of-concern rules, completeness, and common evaluation findings. The agent refines the spec in place.

## Used By
EXECUTE state — primary agent follow-up, when `review_work` is enabled. Sent `review_work.number_of_times` times after the initial draft.

## Interpolation Fields
None. This prompt is sent as-is after the initial drafting prompt completes.

## Prompt

TODO: Write the full review prompt content. The prompt must cover:

- Re-read the spec file you just wrote or updated
- Critique the spec against these checklists:
  - Topic of concern: single sentence, no "and", describes an activity
  - Declarative voice throughout: no "should", "could", "might"
  - Every behavior has testing criteria (Given/When/Then)
  - Error handling is exhaustive: every failure mode named with a specific response
  - Edge cases capture judgment calls with scenario, expected behavior, rationale
  - Invariants are always-true properties, not postconditions — each has a Given/When/Then test
  - No references to planning file paths
  - No open questions or TBDs
  - Observability section present: INFO for success, ERROR for failures, DEBUG for diagnostics
- Check for common evaluation findings:
  - Phantom observability entries: log entry references a behavior not defined in any Behavior section
  - Unverifiable invariants: invariant describes intent rather than a testable property
  - Untested invariants: every invariant needs a Given/When/Then test
  - Internal architecture as invariants: don't prescribe concurrency or data structures — reformulate as externally observable properties
  - Silent omissions: every identified behavior must be covered, explicitly excluded, or marked out of scope
- Fix any issues found by editing the spec file in place
- If no issues found, confirm the spec passes review
