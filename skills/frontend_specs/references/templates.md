# Spec templates

Three skeletons, one per tier, plus a worked `.spec.html` example. Copy the matching
skeleton, delete sections that genuinely don't apply (the order is fixed so readers always
know where to look), and fill in. House base sections (Topic of Concern · Context · Depends
On · Out of Scope) precede the tier-specific sections shown here.

---

## Tier 1 — View spec (`views/spec-<name>-view.md`)

> Lives in `views/`. Every component named below is a **linked path** rooted at the specs
> folder (`spec-…md`, `playbook/….spec.html`), not a bare name — see the reference rule.

```markdown
# <Page Name> View

## Topic of Concern
> One sentence: the single view this spec governs.

## Route
`/path` — plus any URL-param overlay (e.g. `?item=<id>`).

## Depends On
- `GET /api/...` — what it returns
- [DataTable](spec-datatable.md), [FilterPanel](spec-filter-panel.md), [Pagination](spec-pagination.md), [DetailLink](spec-detail-link.md)
- [urlState](spec-lib-url-state.md), [formatters](spec-lib-formatters.md)

## Component Tree
Every node carries the path to its spec.
```
<PageName>                       (page, /path)
├── FilterPanel                  (left rail)        → spec-filter-panel.md
└── Results
    ├── DataTable                                   → spec-datatable.md
    │   └── (id cells) → DetailLink                 → spec-detail-link.md
    └── Pagination                                  → spec-pagination.md
```

## Layout Wireframe (page level)
The top of the drill-down: where the major zones sit, not their internals.
```
┌───────────────────────────────────────────────┐
│ [tabs / toggles]                               │
├──────────┬────────────────────────────────────┤
│ Filter   │  DataTable                          │
│ rail     │                                     │
│          │  Pagination                         │
└──────────┴────────────────────────────────────┘
```
Responsive: < `sm`, the filter rail collapses to a left `Drawer` toggled by a "Filters" button.
Draw the page's significant layout states (e.g. rail collapsed vs expanded) only when they
change the composition.

## Component Wireframe Breakdown
One numbered subsection per composite the view composes, ordered outermost-first. Drill only
through composites that compose contextual data; stop before atomic primitives (icons, single
buttons, bare inputs, status dots). Each subsection: a line naming the component + what it
composes, an ASCII wireframe of its internal layout (sub-components labeled in-place), and a
`| Sub-component | Composes |` table (recommended default; prose where it reads better).
Reference each sub-component by linked path; describe composition only, never its internal render.

### 1. <ChildComponent>
Renders <what it composes — the contextual data it presents to the viewer>.
```
┌── <ChildComponent> ────────────────────────────────────┐
│  ┌── <SubA> ──────────────┐  ┌── <SubB> ────────────┐ │
│  │  <what SubA shows>     │  │  <what SubB shows>   │ │
│  └────────────────────────┘  └──────────────────────┘ │
│  ── Empty state ────────────────────────────────────── │
│  <empty copy>                                          │
└────────────────────────────────────────────────────────┘
```
| Sub-component | Composes |
|---|---|
| [SubA](spec-sub-a.md) | <fields/contextual data SubA assembles> |
| [SubB](spec-sub-b.md) | <fields/contextual data SubB assembles> |

### 2. <NextComposite>
> Repeat for each composite, from the outer container inward. Encouraged: draw significant
> states (collapsed/expanded, empty, completed) inline where they change the composition.

## Data / Query Map
| Hook / query | Query key | Triggers when | staleTime / options |
|--------------|-----------|---------------|---------------------|
| `useX(params)` | `['x', params]` | `params` change | `keepPreviousData` |

## Entry & Exit Points
- **Entry** — how the user reaches this view (nav link, deep link, redirect).
- **Exit** — where its interactive elements lead (e.g. `DetailLink` → `?item=` overlay).

## URL / State Contract
| Param | Default | Type | Notes |
|-------|---------|------|-------|
| `limit` | `50` | number | |
| `offset` | `0` | number | |
| `order_by` | `...` | string | |

**Reset rules**
| Trigger | Reset |
|---------|-------|
| any filter change | `offset = 0` |
| <tab> change | clear filters, preserve `limit` |

## State Matrix
| Surface | Loading | Empty | Error |
|---------|---------|-------|-------|
| <results query> | `DataTable loading` skeleton rows | `<EmptyX onClear>` | `Alert` + retry |

## Empty / Error Copy
The actual user-facing strings for each non-success state.
| State | Copy | Action |
|-------|------|--------|
| empty (no results) | "No items match these filters." | "Clear filters" |
| error | "Couldn't load results." | "Retry" |

## Wiring / Bindings
- Reads URL via `useSearchParams` + `urlState` helpers; applies defaults.
- Feeds `facets` → `buildXFilterConfig(facets)` → `<FilterPanel filters onChange>`.
- `onChange(key, value)` → `urlState.setParams` → `setSearchParams` (resets `offset`).
- Passes `<DataTable columns rows sortState onSortChange>`; `onSortChange` writes `order_by`/`order_desc`.

## Performance Notes
- `keepPreviousData` on the results query for flicker-free pagination.
- Debounce search input before writing the URL param.
- Virtualize the table body only if row counts exceed <N>.

## Out of Scope
- ...
```

---

## Tier 2 — Component contract (`spec-<name>.md`)

```markdown
# <ComponentName>

## Topic of Concern
> One sentence: the contract this component/hook/function guarantees. Page-agnostic.

## Depends On
- component-library primitives used (the project's design-system components)
- upstream specs as linked paths, e.g. [formatters](spec-lib-formatters.md), [urlState](spec-lib-url-state.md)

## Props / Inputs
| Name | Type | Required | Default | Meaning |
|------|------|----------|---------|---------|
| `columns` | `ColumnDef<TRow>[]` (your table library's column type) | yes | — | column config |
| `loading` | `boolean` | no | `false` | replaces body with skeleton rows |

## Events Emitted
| Callback | Fires when | Payload |
|----------|-----------|---------|
| `onSortChange` | a sortable header is clicked | `{ orderBy, orderDesc }` |

## Behavior
- Sort toggle: clicked column == current `orderBy` → flip `orderDesc`; else set `orderBy`, `orderDesc = true`.
- Query key `['...', params]`, `staleTime` ..., `keepPreviousData` ... (for hooks).
- The logic the compiler does not enforce. No pasted implementation.

## States
- Loading: ...
- Empty: ...
- Error: ...

## A11y
- `role="grid"`; sortable headers carry `aria-sort`.

## Out of Scope
- ...
```

---

## Tier 3 — Visual sheet (`<Name>.spec.html`)

Worked example for `StatusBadge` (active/inactive/invalid → colored badge):

```html
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <title>StatusBadge — spec sheet</title>
  <link rel="stylesheet" href="./playbook.css" />
</head>
<body>
  <main class="pb-sheet">
    <h1 class="pb-title">StatusBadge</h1>
    <p class="pb-purpose">
      Renders an entity's lifecycle/data-quality status as a single colored badge.
      Deterministic from two booleans — no data fetching, no state. Used in the
      list table's status column. Design target; the built component uses the
      component library's <code>Badge</code> with its <code>color</code> prop.
    </p>

    <div class="pb-section-label">Variants</div>

    <div class="pb-variant">
      <div class="pb-props">active=true<br>isValidData=true</div>
      <div class="pb-render"><span class="pb-badge pb-badge--green">Active</span></div>
    </div>

    <div class="pb-variant">
      <div class="pb-props">active=false<br>isValidData=any</div>
      <div class="pb-render"><span class="pb-badge pb-badge--gray">Inactive</span></div>
    </div>

    <div class="pb-variant">
      <div class="pb-props">active=true<br>isValidData=false</div>
      <div class="pb-render"><span class="pb-badge pb-badge--yellow">Invalid Data</span></div>
    </div>

    <div class="pb-section-label">Notes</div>
    <p class="pb-note">Non-interactive. Inherits surrounding text size. No focus/hover state.</p>
    <p class="pb-note">a11y: color is not the only signal — the label text carries the meaning.</p>
  </main>
</body>
</html>
```

`playbook/index.html` links every sheet so the whole playbook opens in one static server
(`python -m http.server` from the project's `<specs-root>/playbook/`, or any static host).

---

## Index — `overview.md`

The map/index of the whole spec set. Tier-less (it governs no component). Section order is
fixed so readers always know where to look. Must be updated whenever a spec is added or
re-tiered.

```markdown
# Frontend UI Component Specs — Overview

## Purpose
> One line: this folder is the index/map for the frontend specs.

## Working Area
Which folder is source-of-truth vs working area, and what flows here (refinements,
open questions, todos).

## Stack
The project's actual stack — fill in with your real libraries (the row below is an example).
| Concern | Library | Rationale |
|---------|---------|-----------|
| Routing | `<your-router>` (e.g. `react-router v7`) | ... |

## Source Tree
```
src/
  ...                       # canonical layout, one comment per notable file
```

## Views Index
Per route: name, path, one-line summary, **linked path** to its view spec (in `views/`),
and the **components it uses as linked paths** (not bare names).

| View | Route | Spec | Components used | Summary |
|------|-------|------|-----------------|---------|
| Items | `/items` | [views/spec-items-view.md](views/spec-items-view.md) | [DataTable](spec-datatable.md), [FilterPanel](spec-filter-panel.md), [Pagination](spec-pagination.md), [DetailLink](spec-detail-link.md) | ... |

## Shared Components Index
`Spec` is a linked path, not a bare filename.

| Component | Spec | Summary |
|-----------|------|---------|
| DataTable | [spec-datatable.md](spec-datatable.md) | ... |

## Lib / Utilities Index
| File | Spec | Summary |
|------|------|---------|
| formatters | [spec-lib-formatters.md](spec-lib-formatters.md) | ... |

## API Client Index
| File | Spec | Summary |
|------|------|---------|
| api-client | [spec-api-client.md](spec-api-client.md) | ... |

## Cross-Cutting Rules
The global invariants every spec is written against (numbered list).

## Spec Table
| File | Tier | Status | Contents |
|------|------|--------|----------|
| [views/spec-items-view.md](views/spec-items-view.md) | 1 | draft / approved / implemented | ... |

The authoritative tier + status map. Every spec appears here as a linked path (tier-1 rows
point into `views/`).

## View Dependency Graph
Which views share global/overlay state (e.g. all views → a shared detail modal).

## Naming Conventions
File / component / hook naming rules.

## Playbook Index
| Sheet | Component | Summary |
|-------|-----------|---------|
```
