# Domains and Specs

Domains are one of the most important concepts in forgectl. Getting them wrong
at config time causes init rejections, misrouted batches, and broken spec queues.
Read this before you author your first `spec-queue.json`.

---

## What a domain is

A **domain** is a named grouping that maps to a directory in your project. It is
the unit forgectl uses to scope specs, batch work, cross-reference consistency
checks, and produce implementation plans.

Think of a domain as the boundary of a coherent subsystem. Everything inside that
directory — specs, plans, workspace artifacts — belongs to that domain and is
processed as a unit.

---

## Declaring domains in config

Add `[[domains]]` entries to `.forgectl/config` **before** you run `init`. Config
is locked into the state file at init time; adding domains mid-session does nothing
for the current session.

```toml
[[domains]]
name = "optimizer"
path = "optimizer"

[[domains]]
name = "portal"
path = "portal"
```

Each domain requires exactly two fields:

| Field | Type | Purpose |
|-------|------|---------|
| `name` | string | Identifier used in queue files, CLI flags, and state tracking |
| `path` | string | Directory path relative to project root |

Domains are **optional**. If you omit the `[[domains]]` section entirely, forgectl
derives domain groupings from spec file paths at runtime. For small projects or
a first run, omitting domains is fine. For any multi-subsystem product, declare
them explicitly — it prevents path ambiguity and gives you cleaner cross-reference
boundaries.

---

## Key constraints

These are enforced at `init` — violate them and init fails with the offending value
named:

1. **No prefix nesting.** No domain path may be a prefix of another.
   - Rejected: `domains/users` and `domains/users/employees`
   - Valid: `domains/users` and `domains/employees`

2. **Unique names.** Domain names must not repeat within the config.

3. **Spec paths must match.** When domains are declared, every spec's `"file"` in
   the queue must live under that domain's `<path>/specs/` directory. A spec with
   `"domain": "optimizer"` whose file is `portal/specs/foo.md` will fail validation.

---

## Where specs live

Specs for a domain live in `<path>/specs/`:

```
optimizer/
  specs/
    repository-loading.md
    caching-strategy.md
portal/
  specs/
    dashboard.md
```

Eval outputs conventionally go under `<path>/specs/.eval/`. Workspace artifacts
(implementation plans, notes) live under `<path>/<paths.workspace_dir>/`, where
`workspace_dir` defaults to `.forge_workspace`.

---

## Domain in spec-queue.json

Every entry in your `spec-queue.json` must carry a `"domain"` field that matches
a declared domain name:

```json
{
  "specs": [
    {
      "name": "Repository Loading",
      "domain": "optimizer",
      "topic": "The optimizer clones or locates a repository on disk ...",
      "file": "optimizer/specs/repository-loading.md",
      "planning_sources": ["docs/design/optimizer.md"],
      "depends_on": []
    },
    {
      "name": "Dashboard Layout",
      "domain": "portal",
      "topic": "The portal renders a dashboard with summary widgets ...",
      "file": "portal/specs/dashboard.md",
      "planning_sources": ["docs/design/portal.md"],
      "depends_on": []
    }
  ]
}
```

If the `"file"` path does not fall under the declared domain's `<path>/specs/`
directory, `init` rejects with the mismatch named.

---

## Domain in plan-queue.json

The same `"domain"` field carries into the planning queue. Each entry groups the
specs from one domain into a single plan:

```json
{
  "plans": [
    {
      "name": "Optimizer Implementation Plan",
      "domain": "optimizer",
      "kind": "code",
      "file": "optimizer/.forge_workspace/implementation_plan/plan.json",
      "specs": ["optimizer/specs/repository-loading.md", "..."],
      "code_search_roots": ["optimizer/"]
    }
  ]
}
```

`kind` routes the downstream phase: `"code"` → `implementing`, `"ui"` → `ui_implementing`.

---

## Adding specs mid-session

To add a spec while a session is in progress, use `add-queue-item`:

```bash
forgectl add-queue-item \
  --name "Caching Strategy" \
  --domain optimizer \
  --topic "The optimizer caches cloned repositories to avoid redundant network calls ..." \
  --file "optimizer/specs/caching-strategy.md"
```

- At `DRAFT` or `CROSS_REFERENCE_REVIEW`: `--domain` is inferred from the current
  working domain — you may omit it.
- At `DONE` or `RECONCILE_REVIEW`: `--domain` is required.

---

## How domains shape the specifying loop

Batching during the specifying phase is domain-scoped. A batch **never mixes
domains**. If `specifying.batch = 3` and domain `optimizer` has five specs,
forgectl runs two optimizer batches before touching any `portal` spec.

After the **last batch** of a domain is accepted, forgectl automatically enters a
**CROSS_REFERENCE** phase for that domain:

- It reads **all specs in the domain** — both newly written and pre-existing files.
- It checks them for consistency: naming collisions, contradictory behavior, missing
  interfaces between specs.
- Only after cross-reference passes does forgectl move to the next domain.

This is why domains are not just an organizational nicety — they are the boundary
that makes cross-reference tractable. A domain with 50 files is reviewable; a
cross-reference across your entire codebase is not.

---

## Quick rules

- Declare domains in `.forgectl/config` **before** `init`.
- Every spec queue entry needs `"domain"` matching a declared name; its `"file"`
  must live under `<domain_path>/specs/`.
- No prefix nesting, unique names — or `init` rejects with the offending value.
- Batches never mix domains; cross-reference runs per-domain after the last batch.
- Without a `[[domains]]` section, forgectl derives groupings from file paths.
  Explicit is always safer.
