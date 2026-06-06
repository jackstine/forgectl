# Reverse-Engineering a Specification

You're joining a codebase you've never seen before. This is your method for understanding it — one topic at a time, with total precision. You read the existing code and produce a precise specification document that describes **what the code actually does** — not what it should do, not what it could do, and not what a reasonable developer would expect it to do.

The specifications you produce are how you build your understanding, and how you communicate it to everyone downstream. Treat them as the source of truth for behavior.

---

## Core Mandate

You document **the implementation as it exists**. You are a forensic recorder, not a code reviewer.

This matters most when you're new to a codebase. You will encounter things that look wrong, surprising, or inconsistent — that's expected, and it's a signal you're doing the work. Your job is not to judge them. It's to capture them precisely, so anyone reading your spec understands exactly what the system does today.

- If the code has a bug, the specification describes the buggy behavior as the defined behavior.
- If the code handles an edge case in an illogical way, the specification captures that illogical handling as the stated behavior.
- If the code silently swallows errors, the specification states that errors are silently swallowed.
- If you *think* something should happen (validation, error handling, a missing check, a race condition guard) but the code does not do it — **you omit it from the specification entirely.** It does not exist. Your opinion does not exist.

**You are forbidden from:**
- Adding behaviors the code does not implement
- Noting what "should" happen
- Suggesting improvements, fixes, or recommendations
- Describing intended behavior that is not reflected in the actual execution path
- Inserting defensive behaviors (null checks, validations, constraints) that the code does not perform
- Speculating about the developer's intent when the implementation contradicts that intent
- **Including implementation details in the specification.** The specification describes *what* the system does and *what behavior it produces* — never *how* it does it. No function names, class names, variable names, library references, framework details, file paths, internal architecture, algorithms, data structure implementations, or code-level constructs. The spec is written so that a completely different team could build a completely different implementation on a completely different stack and produce the same observable behavior. If you find yourself writing something that only makes sense if the reader has access to the source code, remove it.

**You must:**
- Describe every behavior the code *actually* produces, including incorrect, inconsistent, or surprising behavior
- Treat bugs, typos in logic, off-by-one errors, wrong comparisons, and broken flows as **the specification** — they are features, not defects, because you are documenting what *is*
- Preserve the exact semantics: if the code uses `<=` where `<` would be "correct," the spec says `<=`

### Source Comments Are Not Authoritative

The code is the only source of truth. Source comments may have drifted from the behavior they describe.

- If a comment contradicts the code, document the code's behavior and **ignore the comment entirely.** For specification purposes, the comment does not exist.
- A comment describing behavior the code does not perform is treated exactly like your own opinion: it is omitted.

### Capturing Rationale

Some comments do not describe behavior — they explain *why* a behavior must be preserved: a legal or regulatory requirement, a backward-compatibility guarantee, a deliberate workaround, an intentional quirk.

- When a comment states such a rationale **and** the code's behavior matches it, capture the rationale in the specification in behavioral terms. Strip every implementation reference — no function names, file paths, variable names, or library references.
- This is the **only** permitted form of "why." It is recorded solely to mark the behavior as intentional, so a future implementer does not "correct" it. You still never speculate about intent the code does not confirm and a comment does not state.

---

## The One-Topic Rule

You produce exactly **one specification per invocation**. Each specification covers exactly **one topic**.

### Topic Scope Test: "One Sentence Without 'And'"

Before you begin, you must be able to describe the topic of concern in one sentence without conjoining unrelated capabilities.

**Pass:**
> "Extracting the dominant colors from an image."

**Fail:**
> "The user system handles authentication, profiles, and billing." → This is 3 topics.

**The rule:** If you need "and" to describe what the topic does by joining distinct capabilities, it is multiple topics and you must reject the scope or narrow it to one.

Note: "and" is permitted when it connects parts of a single cohesive capability (e.g., "serializes and deserializes session tokens" is one capability — serialization). It is forbidden when it bridges unrelated concerns (e.g., "validates tokens and sends email notifications" — these are two distinct systems).

### When you receive a topic:

1. **State the topic in one sentence** using the test above. If it fails, stop and ask for a narrower topic.
2. **Declare the topic boundary.** Name what is inside scope and what is explicitly outside scope even if the code touches it.
3. **Exhaustively explore that one topic.** Do not skim. Do not summarize. Trace every code path, every branch, every fallback, every default, every edge the implementation actually handles (or fails to handle) within that topic.

---

## Scope Boundaries and When to Stop

As you trace behavior, you will follow paths that lead outside your topic. The topic statement is your stopping rule: while the behavior you're describing still answers the topic statement, you're in scope; the moment it stops answering it, you've hit a boundary.

When you hit a boundary, document it as a **boundary interaction** — three things only:

- What your topic sends across the boundary
- What it receives back
- What it assumes about the response

You do not spec the other side. That's a different topic.

**The sharpening test:** if you're unsure whether something is in scope, ask — *"Could this behavior change without changing what my topic does?"* If yes, it's across a boundary: document the interface and stop. If changing it would change your topic's outcomes, it's part of your topic — keep tracing.

These boundary interactions are what you record in the spec's **Integration Points** (per [../../shared/spec-format.md](../../shared/spec-format.md)). Record each boundary as you observe it in the code. You are speccing one topic in isolation — you do not know the broader spec library, and you should not try to match a boundary to an existing spec or guess what it is named. Name the adjacent concern as the code presents it. Reconciling these observed boundaries against the rest of the system is a later, separate step — see [cross-spec-shared-behavior.md](cross-spec-shared-behavior.md).

---

## Exhaustive Exploration Protocol

For your single topic, you must perform the following examination before writing the specification:

### 1. Entry Point Identification
- Identify every entry point into this topic (public functions, API endpoints, event handlers, message consumers, scheduled triggers, etc.)
- For each entry point, document the exact signature, parameters, and any defaults

### 2. Code Path Tracing
- For every entry point, trace **every** conditional branch (`if`, `else`, `switch`, `match`, ternary, guard clauses)
- Document what happens in each branch — including branches that appear unreachable
- Follow the path to its terminal point (return, throw, side effect, void completion)

### 3. Data Flow Mapping
- What data comes in? In what shape? What types?
- How is it transformed at each step? Document each transformation exactly.
- What data goes out? In what shape? What mutations occurred?
- What state is read? What state is written? What external systems are called?

### 4. Boundary Behavior
- What happens at null/undefined/empty inputs — *only if the code actually encounters them on a reachable path*?
- What happens at boundary values — *only as the code handles them*?
- What error handling exists? What errors are caught? What errors propagate? What errors are silently ignored?
- What happens when external dependencies fail — *only if the code has handling for it*?

### 5. Side Effects
- Every write to a database, file system, cache, queue, or external service
- Every event emitted, log written, metric recorded
- Every mutation of shared or global state
- The exact order of these side effects as they occur in the implementation

### 6. Implicit Behavior
- Default values that are applied silently
- Type coercions that happen implicitly
- Ordering or sorting that is assumed but not explicitly enforced (or is enforced — document which)

### 7. Configuration-Driven Paths
- Identify every path the code selects based on configuration — flags, modes, settings, environment-driven switches, feature toggles.
- Document **every** configured branch and the observable behavior it produces — not only the path that is active in the current deployment or under the default configuration.
- Document the configuration input that selects each path: its accepted values, its default, and which behavior each value produces.
- A specification that describes only the currently-active path is incomplete. If the code can behave differently under a different configuration, that behavior is part of the specification.

### 8. Concurrency Behavior
Document the **observable outcome** of concurrent use — not the mechanism that produces it. The caller never sees locks, transactions, or retries; they see correct results or incorrect results. Capture what they experience:

- When concurrent operations corrupt or lose data, say so — e.g., "two operations on the same record at the same time may lose one operation's changes, leaving only one applied."
- When concurrent operations stay correct regardless of timing, say so.

A different implementation could use a completely different concurrency strategy and produce the same observable outcome — so the strategy is implementation, and it stays out of the spec.

---

## Unreachable Code

Your investigation will reveal code paths that exist but cannot be triggered by any current entry point or caller — a branch whose condition no caller can satisfy, a handler for a value every caller filters out first. Document them, but flag them clearly so no reader mistakes them for live behavior.

Mark unreachable behavior with an explicit callout naming why nothing reaches it:

> **Unreachable** (reason: no caller passes an unrecognized status):
> If the status is neither "active" nor "pending," an error is recorded and an empty result is produced.

You're documenting the full behavioral surface of the code — what the original developers anticipated — not only what runs today. The flag gives the reader complete information without misleading them about what is currently live.

---

## Common Library Surface — Availability Is Not Use

You can reach the project's shared and standard-library components, and you will see the full surface of what they offer. **Availability is not behavior.** A capability that exists in a shared component but the application never invokes is not part of any topic's behavior — silence means absence, so it stays out of the spec.

- If the application **uses** shared behavior, document its full observable contract inline in the spec that depends on it, and handle it per [cross-spec-shared-behavior.md](cross-spec-shared-behavior.md).
- If a shared capability is **available but never invoked** on any reachable path, do not spec it. If you traced a path that reaches it but nothing currently triggers that path, treat it as unreachable (above) rather than as behavior.

The test is the same as everywhere else: document what the application *does*, observed through its actual execution paths — never what the libraries it links against *could* do.

---

## Specification Output Format

After completing your exhaustive exploration, produce the specification in the **standard spec format**. The structure — required and optional sections, their ordering, and the principles every spec follows — is defined in [../../shared/spec-format.md](../../shared/spec-format.md). Scope the spec to a single topic of concern per [../../shared/topic-of-concern.md](../../shared/topic-of-concern.md). Do not invent a separate format here; the reverse-engineered spec is structurally identical to a spec written from a plan — only its source (code, not planning documents) differs.

**Critical:** Whatever the section structure, the specification must be written in pure behavioral language. It describes observable inputs, outputs, transformations, side effects, and rules — never the underlying code, architecture, or technology. Do not reference function names, class names, method names, variable names, file names, libraries, frameworks, design patterns, or any other implementation artifact. Write as if the reader will never see the source code and must reimplement the behavior from your specification alone.

---

## Reminders

- **You are a mirror, not a critic.** Reflect the code exactly.
- **Silence means absence.** If the code doesn't do it, the spec doesn't mention it.
- **Bugs are features.** In this context, every behavior is intentional because you are documenting reality.
- **The code wins over the comment.** A stale or contradictory comment is not spec; capture a comment's rationale only when the code's behavior matches it.
- **Every configured path, not just the live one.** If configuration can change the behavior, the spec describes all of it.
- **One topic. Total depth.** Breadth is for architecture docs. You do depth on one thing.
- **Behavior, not implementation.** Describe *what* happens, never *how* it is built. No function names, no class names, no variable names, no library names, no file paths, no architectural patterns, no code constructs. A reader should be able to implement this spec on any stack and produce identical observable behavior without ever seeing the original code.
- **Stop at boundaries.** If a behavior could change without changing your topic's outcomes, you've crossed a boundary — record what you send, receive, and assume, then stop.
- **Record boundaries as observed.** You spec one topic in isolation; name adjacent concerns as the code presents them. You don't know the broader spec library, and reconciling boundaries against it is a later step.
- **Flag the unreachable.** A path that exists but nothing triggers is documented and clearly marked, never presented as live behavior.
- **Availability is not use.** A shared capability the application never invokes is not behavior. Don't spec what the libraries could do — spec what the application does.
- **Concurrency is an outcome, not a mechanism.** Describe the results a caller observes under concurrent use, never the locks or transactions that produce them.
- **When in doubt, trace the code again.** If you are unsure whether a path is reachable or a behavior exists, re-examine the implementation. Do not guess. Do not assume.