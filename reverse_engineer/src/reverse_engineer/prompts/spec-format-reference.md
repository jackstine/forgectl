# Spec Format Reference

## Purpose
Provides the full specification format structure so the agent knows exactly how to structure its output. Sections, ordering, principles, topic-of-concern rules, and common evaluation findings to avoid.

## Used By
EXECUTE state — primary agent. Concatenated with `reverse-engineering-prompt.md` to form the complete agent prompt.

## Interpolation Fields
None. This is static content.

## Content

TODO: Write the full spec format reference. The content must cover:

- What a spec is: a permanent, authoritative contract for a single topic of concern
- What a spec is not: not a plan, not code documentation, not a tutorial
- The complete spec structure in order:
  - Title (activity-oriented)
  - Topic of Concern (one sentence, no "and", describes an activity)
  - Context (why the spec exists)
  - Depends On (upstream spec dependencies)
  - Integration Points (relationships with other specs)
  - Interface (inputs, outputs, rejection)
  - Behavior (preconditions, steps, postconditions, error handling)
  - Configuration (parameters, types, defaults)
  - Observability (logging levels, metrics)
  - Invariants (always-true properties)
  - Edge Cases (scenario, expected behavior, rationale)
  - Testing Criteria (Given/When/Then)
  - Implements (what this spec covers)
- Principles:
  - One topic of concern per spec
  - No codebase references (file paths, module names)
  - Declarative voice ("the system does", not "the system should")
  - No open questions or TBDs
  - Technology-aware, not technology-coupled
  - Error handling is exhaustive
  - Invariants are always true, not postconditions
  - Every behavior has testing criteria
  - Edge cases capture judgment calls
- Common evaluation findings to avoid:
  - Phantom observability entries (log entry with no corresponding behavior)
  - Unverifiable invariants (intent, not testable property)
  - Missing observability section
  - Untested invariants
  - Internal architecture prescribed as invariants
  - Silent omissions (behavior not covered, excluded, or marked out of scope)
