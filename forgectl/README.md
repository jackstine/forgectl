# forgectl

A CLI scaffold that guides software development through three sequential phases: **specifying**, **planning**, and **implementing**. It enforces a JSON-backed state machine with validated transitions, evaluation loops, and durable state management.

## Phases

1. **Specifying** — Draft specs from planning documents, evaluate via sub-agent rounds, refine on deficiency, accept, then reconcile cross-references across all specs.
2. **Planning** — Study accepted specs, codebase, and packages. Draft a structured implementation plan (`plan.json`). Validate and evaluate through iterative rounds until accepted.
3. **Implementing** — Receive plan items one at a time in dependency-ordered batches. Implement each, then an evaluation sub-agent verifies the batch. Commit at batch boundaries.

Between phases, a **PHASE_SHIFT** checkpoint stops work and prompts a context refresh.

## Quick Start

```bash
# Build
cd forgectl
go build -o forgectl .

# Initialize a specifying session
forgectl init --from specs-queue.json --phase specifying

# Check current state
forgectl status

# Advance to next state
forgectl advance

# Get evaluation context for sub-agent
forgectl eval
```

## State File

All state lives in `.forgectl/state/forgectl-state.json`. The `.forgectl/` directory serves as the project marker (like `.git/`). Writes are atomic (tmp → backup → rename) with crash recovery on startup. The state file is gitignored; completed sessions are archived to `sessions/`.
