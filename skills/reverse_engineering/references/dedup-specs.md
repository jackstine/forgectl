# Deduplicating Specs

A topic of concern has **exactly one** specification. Before you write a new spec, and whenever you
finish one, you are responsible for ensuring no second spec describes the same topic and that the
spec agrees with the code. This reference defines how a reverse-engineering pass keeps specs
de-duplicated.

It assumes you are producing a single spec per topic per the
[reverse-engineering-one-spec](reverse-engineering-one-spec.md) prompt, writing in pure behavioral
language per [../../shared/spec-format.md](../../shared/spec-format.md), and scoping to one topic per
[../../shared/topic-of-concern.md](../../shared/topic-of-concern.md).

---

## Search before you write

Before producing a spec for a topic, search the existing specs for one that already covers it.

- Match on **topic of concern**, not on title or filename. Two specs describe the same topic when
  they make claims about the same observable behavior — the same inputs, outputs, and side effects —
  even if they are named differently.
- A spec whose topic *overlaps* yours but is not identical is not a duplicate. It may share a
  boundary with your topic; document the crossing per the boundary rules, but do not merge the two.
- If a spec for your topic already exists, do not create a second one. Continue under **Update in
  place** below.

---

## Update in place

When a spec for the topic already exists, update that spec rather than creating a new one.

- Bring the existing spec into agreement with the code as it exists now. Add behaviors the code has
  gained, remove behaviors the code no longer performs, and correct any claim the code contradicts.
- Preserve the existing spec's structure and identity. You are editing the authoritative document,
  not replacing it with a fresh draft.
- The result is still one spec for the topic — never two specs that must be reconciled against each
  other later.

---

## Reconcile spec to code

The code is the source of truth. When a spec and the code disagree, the code is correct and the spec
is stale.

- A spec that describes behavior the code does not perform is wrong. Remove or correct that claim.
- A behavior the code performs that the spec omits is missing. Add it.
- This reconciliation is the same forensic act as writing the spec in the first place: you document
  what the code *actually* does, including incorrect, inconsistent, or surprising behavior, and you
  add nothing the code does not implement.

---

## What this reference does **not** cover

- It does not define how behavior shared across multiple topics is handled — inlining, flagging, and
  the canonical spec for shared behavior live in
  [cross-spec-shared-behavior](cross-spec-shared-behavior.md).
- It does not change the behavior-not-implementation mandate. Searching, updating, and reconciling
  are all done against observable behavior; the specs you compare and edit never reference function
  names, file paths, variable names, libraries, or patterns.

---

## Reminders

- **One topic, one spec.** Two specs for the same topic of concern is the defect this reference
  exists to prevent.
- **Search by behavior, not by name.** Duplicates hide behind different titles.
- **Update, don't re-create.** When a spec exists, edit it in place; do not draft a parallel copy.
- **The code wins.** When spec and code disagree, the spec is stale and the code defines reality.
