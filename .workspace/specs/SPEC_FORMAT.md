# Specification Format

This document defines how specifications are structured in Spectacular. If you are new to the project or have never written a spec before, this is your guide. Read it fully before writing your first spec.

---

## What a Spec Is

A specification is a **permanent, authoritative contract** for a single topic of concern. It defines what the system does, what it accepts, what it rejects, and what it guarantees — independent of how or where the code is organized.

Specs are the source of truth. Code implements specs, not the other way around. When code and spec disagree, the code is wrong — or the spec needs an intentional update with a clear reason.

---

## What a Spec Is Not

- **Not a plan.** Plans propose what the system *should* do. Specs commit to what the system *does*. If you find yourself writing "the system should..." or "we could...", you are planning, not specifying.
- **Not code documentation.** Specs do not describe how the code is organized, what files exist, or what classes are named. The code implements the spec; the spec does not describe the code.
- **Not a tutorial.** Specs are reference contracts, not walkthroughs. They are precise, not narrative.

---

## How Specs Differ from Plans

Plans and specs serve different purposes at different stages of the project lifecycle.

**Plans** answer: *"What should the system do and why?"*
They describe behavior, architecture, data contracts, and flows as proposals. Plans are technology-agnostic — they never reference specific languages, frameworks, or libraries. Plans are temporary. Once a spec incorporates a plan, the plan is removed.

**Specs** answer: *"What does the system do, accept, reject, and guarantee?"*
They define precise contracts with constraints, rejection conditions, invariants, edge cases, and testing criteria. Specs are technology-aware — they reference wire formats, protocols, and serialization contracts because these are interface commitments. Specs are permanent. They evolve with the system.

| Dimension | Plan | Spec |
|-----------|------|------|
| Question answered | "What should the system do and why?" | "What does the system do, accept, reject, and guarantee?" |
| Scope | Topic of concern (activity) | Topic of concern (contract) |
| Lifetime | Temporary — removed once incorporated into a spec | Permanent — evolves with the system |
| Technology | Agnostic | Aware (references implementation stack) |
| Error handling | Mentioned if obvious | Exhaustively enumerated |
| Validation | Implied | Explicit rules with rejection behavior |
| Testability | Not a concern | Testing criteria define how contracts are verified |
| Voice | "The system should..." | "The system does..." |
| Open questions | Welcome — plans are proposals under discussion | Not permitted — unresolved questions stay in plans |

**In short:**
- A plan tells you *what to build*.
- A spec tells you *what "correct" means*.
- A plan says: "The system models ideas with three fields."
- A spec says: "An Idea with an empty value_proposition cannot exist. Whitespace-only strings are treated as empty. Construction raises a validation error."

---

## Spec Format

Every spec follows this structure. Sections are included when relevant to the topic — not every spec needs every section. But the order is fixed so readers always know where to find information.

```markdown
# [Title — Activity-Oriented]

## Topic of Concern
> One sentence describing the single topic this spec addresses.
> Same rule as plans: describable without the word "and" conjoining
> unrelated capabilities.

## Context
Why this spec exists. What problem it addresses. References to the
planning documents it incorporates.

## Depends On
Specs this contract requires. These are cross-references — if this
spec assumes behavior defined elsewhere, that dependency is listed
here. Only list upstream dependencies. The inverse (what depends on
this spec) is derivable and not maintained separately.

## Integration Points
Other specs and components that interact with this topic. Each entry
describes the relationship: what data flows between them, in which
direction, and under what conditions. This gives readers a map of
how this spec fits into the broader system.

| Spec | Relationship |
|------|-------------|
| [Spec name] | [What flows between them and how] |

---

## Interface

### Inputs
What this topic accepts — message types, arguments, events, config.
Each with: type, constraints, required vs optional.

### Outputs
What this topic produces — return values, events emitted, side
effects (files written, state mutated, messages sent).

### Rejection
What this topic refuses and how it signals refusal. Invalid inputs,
precondition violations, state conflicts. Each with: the condition,
the rejection signal, and the rationale.

---

## Behavior

### [Behavior Name]
A specific behavior or capability. Each behavior is a contract:
given preconditions, when a trigger occurs, then a result is
guaranteed.

#### Preconditions
What must be true before this behavior can execute.

#### Steps
What happens, in order.

#### Postconditions
What is guaranteed to be true after this behavior completes.

#### Error Handling
What happens when each step fails. Specific failure modes with
specific recovery or propagation behavior. Not "errors are
handled" — name the failure, name the response.

---

## Configuration
When the topic involves configurable behavior, define the
parameters here. Each with: name, type, default value, and
description. If there is a precedence order (e.g., CLI argument
overrides environment variable overrides config file), state it
explicitly.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| [name] | [type] | [default] | [what it controls] |

---

## Observability
What this topic logs and at what level. What metrics it emits.
Observability is externally observable — log formats, metric names,
and log levels are interface commitments that monitoring systems
and dashboards depend on.

### Logging
| Level | What is logged |
|-------|---------------|
| INFO | [events worth noting in normal operation] |
| WARN | [recoverable issues that may need attention] |
| ERROR | [failures that affect correctness or availability] |
| DEBUG | [detailed diagnostic information for troubleshooting] |

### Metrics
| Metric | Type | Description |
|--------|------|-------------|
| [name] | [counter/gauge/histogram] | [what it measures] |

---

## Invariants
Properties that must always hold, regardless of what operations
have been performed or in what order. These are assertions that
should never be violated. A test suite can check these at any
point and they must pass.

Think of invariants as the rules of the system that are always
true — not just after a specific operation, but always.

---

## Edge Cases
Scenarios that are unusual but must be handled correctly. Each
with three parts:

- **Scenario:** What happens (the unusual condition)
- **Expected behavior:** What the system does
- **Rationale:** Why this is the right choice

Edge cases are where much of the spec's unique value lives. Plans
leave these to the implementor's judgment. Specs capture the
decisions so every implementor makes the same choice.

---

## Testing Criteria
How the contracts in this spec are verified. Each criterion maps
to a behavior, invariant, or edge case and describes what a test
must prove. Testing criteria are the bridge between "what the
system guarantees" and "how we know it does."

If you cannot write a testing criterion for a contract, the
contract is too vague. Go back and make it more precise.

### [Criterion Name]
- **Verifies:** Which behavior, invariant, or edge case this tests.
- **Given:** Setup / preconditions for the test.
- **When:** The action or trigger.
- **Then:** The expected outcome.

---

## Implements
Which planning documents this spec incorporates. When a plan is
fully covered by one or more specs, the plan is removed from
planning/.
```

---

## Principles

These are the rules that govern how specs are written. They are not suggestions — they are constraints on the spec itself.

### 1. Topic of Concern scoping
A spec covers one topic. If you need "and" to describe unrelated capabilities, split the spec. The test: *"Can I describe this topic in one sentence without conjoining unrelated capabilities?"*

- **Pass:** "The optimizer represents ideas and their scores as structured transport models" — one cohesive data model, not unrelated concerns.
- **Fail:** "The system handles authentication, profiles, and billing" — three unrelated capabilities. Split into three specs.

### 2. No codebase references
Specs do not reference file paths, directory structure, or module locations. If you write "this lives in `api/src/state/manager.go`" and then the code is refactored, the spec is immediately stale. The spec describes *what the system does*, not *where the code lives*.

### 3. Specs are the source of truth
When code and spec disagree, the code is wrong — or the spec needs an intentional update. Specs are not retroactive documentation. They are the authority that the code implements.

### 4. No open questions
Specs do not have open questions. If something is unresolved, it stays in the plan until it is decided. A spec is a commitment — you cannot commit to something you haven't decided. If you find yourself writing "TBD" or "open question," the topic is not ready to be specified.

### 5. Technology-aware, not technology-coupled
Specs reference technology when it is part of the contract — when changing it would break a consumer. Specs do not prescribe internal implementation choices.

**Technology-aware (belongs in specs):** Decisions that are externally observable — if you changed them, something outside this component would break.
- Data formats: JSON, YAML, Markdown, JSONL
- Transport protocols: WebSocket, HTTP, TCP
- Serialization contracts: "discriminated by `type` field"
- File conventions: ".md with YAML frontmatter"
- API shapes: REST endpoints, health check paths, port conventions
- Log formats and metric names (monitoring systems depend on these)

These are **interface commitments**. Two components agree on them. Changing one side without the other breaks the system.

**Implementation details (not in specs):** Decisions that are internally contained — you could change them and nothing outside the component would notice.
- Concurrency strategy: mutexes, channels, actors, thread pools
- Internal data structures: hash maps, arrays, trees
- Design patterns: singleton, factory, observer
- Error recovery internals: retry counts, backoff algorithms
- Memory management: pooling, caching strategies
- Code organization: file structure, class hierarchy, helper functions

These are **engineering choices**. They live inside the component. The implementor picks them; the spec doesn't care.

**The test:** Ask *"If I changed this, would I need to update another component, another spec, or a consumer?"* Yes → it belongs in the spec. No → it's an implementation detail.

### 6. Testing criteria close the loop
Every behavior and invariant should have a corresponding testing criterion. If you can't write a testing criterion for a contract, the contract is too vague. Go back and sharpen it until you can express it as Given/When/Then.

### 7. Edge cases capture judgment calls
Plans leave edge cases to the implementor. Specs capture the decisions. Without edge cases, two implementors reading the same spec might make different choices at the boundaries. The edge cases section eliminates that ambiguity.

### 8. Error handling is exhaustive
Do not write "errors are handled appropriately." Name the failure. Name the response. Every step in a behavior that can fail must say what happens when it does. If a step cannot fail, say nothing — silence means success is guaranteed.

### 9. Invariants are always true
Invariants are not "usually true" or "true after this operation." They are true at every point in time, regardless of what operations have been performed or in what order. If an invariant can be temporarily violated (e.g., during a transaction), it is not an invariant — it is a postcondition of a specific behavior.

### 10. Specs live with the project they govern
Specs are permanent artifacts that live alongside the source code they define — not in the workspace. Each project has a `specs/` directory adjacent to its `src/` directory:

```
optimizer/
├── src/           # source code
├── specs/         # specifications for this project
│   └── idea-transport-models.md
└── ...

api/
├── src/
├── specs/
└── ...

web-portal/
├── src/
├── specs/
└── ...

launcher/
├── src/
├── specs/
└── ...
```

The workspace (`planning/`, `tabled/`) is for temporary planning documents. Specs are not temporary — they travel with the project they define. If a project is moved, its specs move with it.

### 11. Protocol specs define cross-domain communication layers
When two domains communicate (e.g., portal ↔ API, API ↔ optimizer), the message contract between them is a **protocol spec**. Protocol specs are bilateral contracts — both sides depend on them, neither side owns them unilaterally.

Protocol specs live in their own project directory, separate from either domain:

```
protocols/
├── ws1/
│   └── specs/         # portal ↔ API message contract
└── ws2/
    └── specs/         # API ↔ optimizer message contract
```

**What a protocol spec defines:**
- Message types (commands and events) with their schemas
- Discriminated union definitions
- Connection model (persistent, reconnection behavior)
- Message ordering guarantees
- The direction of each message (who sends, who receives)

**What a protocol spec does NOT define:**
- How either side handles a message internally (that belongs in the domain's spec)
- Store mutations, state transitions, or UI behavior triggered by messages
- Internal data models (the domain that produces the data owns its spec; the protocol references it)

**Why protocols are separate:**
A protocol is the agreement between two domains. If it lived inside one domain, the other domain would depend on an internal artifact of a project it doesn't own. By placing the protocol at the boundary, both sides have equal visibility and equal responsibility to maintain compatibility.

**Breaking changes:** When a protocol spec changes, both domains that depend on it must update. The `Depends On` and `Integration Points` sections in each domain's specs create a traceable dependency chain — you can see exactly which specs on both sides are affected by a protocol change.

**Data models are not protocols.** Data models (e.g., `Idea`, `Score`, `ScoredIdea`) are defined by the domain that produces them. Protocol specs reference these models in their message schemas but do not define them. The rule: whoever creates the data owns its spec. The protocol is the envelope; the domain defines what's inside.

---

## Spec Lifecycle

1. **A plan proposes** — planning documents describe what the system should do.
2. **Discussion resolves open questions** — all ambiguities are decided before specifying.
3. **A spec commits** — the spec captures the contract with constraints, rejection, invariants, edge cases, and testing criteria.
4. **The plan is removed** — once fully incorporated into one or more specs, the planning document is deleted from `planning/`.
5. **The spec evolves** — as the system changes, specs are updated. The spec always reflects what the system *currently* does and guarantees. Old behavior is removed, not accumulated.
6. **Code implements the spec** — implementors read the spec to understand what "correct" means. Tests verify the testing criteria.

---

## Quick Reference: Writing Your First Spec

If you have never written a spec before, here is a step-by-step guide:

1. **Read the plan.** Understand the topic of concern, the proposed behavior, and any resolved questions.
2. **Write the Topic of Concern.** One sentence, no "and" joining unrelated capabilities.
3. **Define the Interface.** What goes in, what comes out, what gets rejected. Be explicit about types and constraints.
4. **Write the Behaviors.** For each capability: preconditions, steps, postconditions, error handling. Use declarative language ("the system does X") not aspirational language ("the system should X").
5. **Identify Invariants.** What must always be true? These are the rules that never break.
6. **Decide Edge Cases.** What happens at the boundaries? Unusual inputs? Extreme values? Missing data? Race conditions? Decide and document.
7. **Write Testing Criteria.** For every behavior, invariant, and edge case: Given/When/Then. If you can't write a test for it, the contract is too vague.
8. **Add Configuration and Observability.** If the topic is configurable, define the parameters. If it logs or emits metrics, define what and at what level.
9. **Check for open questions.** If any remain, the spec is not ready. Send the question back to the plan for discussion.
10. **List what this spec Implements.** Reference the planning documents this spec incorporates.
