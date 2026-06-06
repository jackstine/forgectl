# Cross-Spec Shared Behavior

When you reverse-engineer more than one topic, you will encounter behavior that several topics rely
on — a common validation, a shared transformation, a retry or backoff rule, a formatting or
encoding step, a utility drawn from the project's standard library of shared components. This
reference defines how a reverse-engineered specification handles that recurring behavior.

It assumes you are already producing a single spec per topic per the
[reverse-engineering-one-spec](reverse-engineering-one-spec.md) prompt, writing in pure behavioral
language per [../../shared/spec-format.md](../../shared/spec-format.md), and scoping to one topic per
[../../shared/topic-of-concern.md](../../shared/topic-of-concern.md).

---

## What counts as shared behavior

Behavior is **shared** when more than one topic depends on the same observable contract — the same
inputs produce the same outputs and side effects, and a change to that behavior would change the
outcomes of every topic that depends on it.

Signals that you are looking at shared behavior:

- The same logic is reached from the entry points of multiple topics.
- Multiple topics route through the same component, helper, or standard-library utility.
- You find yourself describing the same input → output → side-effect contract in more than one
  spec.

Shared behavior is identified by its **observable contract**, never by the fact that two topics call
the same code. Two topics that happen to use the same helper but depend on *different* observable
guarantees are not sharing behavior — they are two behaviors that coincide in the implementation.

---

## How to handle it

### 1. Inline it fully — each spec stays self-contained

In every spec that depends on the shared behavior, describe the behavior's full observable contract
inline: its inputs, outputs, side effects, ordering, and edge cases, in behavioral language. Do not
replace the description with a pointer that says "see the other spec" for the substance of what
happens. A reader of any single spec must be able to reimplement that topic — including the shared
behavior it relies on — without opening another document.

### 2. Flag it as shared

Where the shared behavior appears inline, mark it as shared and name the canonical topic it belongs
to (see step 3). This is a tracking marker: it tells a reader that this behavior recurs and where its
authoritative description lives, without removing the self-contained inline description.

### 3. Give it its own canonical spec

The shared behavior is itself a topic of concern. Produce a canonical spec for it, scoped with the
same one-sentence-without-"and" test as any other topic. That canonical spec is the authoritative
statement of the behavior's contract. The inline copies in dependent specs must agree with it — when
the behavior changes, the canonical spec and every inline copy change together.

---

## Reconciling integration points across the spec library

A reverse-engineered spec records its **Integration Points** from the code alone. The agent producing
a single spec works one topic at a time and does not know the rest of the spec library — so it names
each adjacent concern as the code presents it, without trying to match it to an existing spec or guess
its canonical name. This is deliberate: the one-topic agent should not invent cross-spec knowledge it
does not have.

Reconciling those observed boundaries is a separate role in the pipeline. That role takes the
integration points each reverse-engineered spec produced and reshapes them so they mesh with the
system as a whole:

- **Matching observed concerns to existing specs.** Where an observed boundary corresponds to a
  concern that already has a spec, the integration point is rewritten to name that spec and align with
  its contract.
- **Reshaping for consistency.** Names, directions, and assumptions recorded independently by separate
  one-topic passes are made consistent across the specs that share a boundary.
- **Accounting for what is not specced yet.** Some observed boundaries point at parts of the system
  that have not been reverse-engineered, or that have no spec at all. These are kept as observed
  concerns and carried forward, so the gap stays visible rather than being silently dropped.

The one-topic agent records what it sees; this role decides how those records fit together. Keeping
the two separate is what lets each spec be produced in isolation, without the agent needing global
knowledge of every other spec.

---

## What this reference does **not** cover

- It does not tell you to search for or avoid duplicate specs, or to update existing specs in place.
  That is a separate concern, covered in [dedup-specs](dedup-specs.md).
- It does not change the behavior-not-implementation mandate. Shared behavior is described by its
  observable contract only — no function names, file paths, variable names, libraries, or patterns,
  in either the inline copies or the canonical spec.

---

## Reminders

- **Self-contained first.** Inlining the full contract in every dependent spec is the default; the
  canonical spec is in addition to that, not a replacement for it.
- **Contract, not call site.** Behavior is shared because topics depend on the same observable
  guarantee — not because they reach the same code.
- **Canonical and copies move together.** When the shared behavior changes, every place that
  describes it changes in the same step.
- **Boundaries are recorded, then reconciled.** The one-topic agent records integration points as
  observed; a separate cross-spec step matches them to real specs and carries forward what is not
  specced yet.
