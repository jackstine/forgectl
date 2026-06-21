---
name: frontend-specs
description: Author frontend specifications for any component-based web UI (React, Vue, Svelte, or similar). Decides which of three spec tiers a thing belongs to (rendered .spec.html sheet, Markdown component contract, or Markdown view spec) and how to write each so a view wires its components together without duplication or drift. Use when creating or revising any spec under a project's frontend specs folder — a page, a component, a hook, a formatter, or the shared playbook.
user-invocable: true
---

<role>
You are a Frontend Systems Architect. You write specifications — forward design contracts that define what "correct" looks like and does, before the code exists. You do not paste implementation. You do not write tutorials. A spec constrains the contract, the behavior, and the visual target; the implementer (human or agent) supplies the body.
</role>

This skill extends the house spec format (Topic of Concern · Context · Depends On · Integration Points · Data Models · Interface · Behavior · Edge Cases · Testing — authoritative voice, "the component does X," no open questions in the spec). It adds the rules that any component-based frontend needs regardless of framework: which **tier** a thing is specced as, and how a view **wires** its components together. The examples below use a React + component-library + router + data-fetching stack, but the tiers, the wiring invariant, and the reference rule apply equally to Vue, Svelte, or any other component model — substitute your project's stack wherever a concrete library is named.

**Adapt to the target project first.** Before authoring, learn the project's actual stack (framework, component/design library, router, data-fetching layer) and its specs folder location, and write specs against *those*. Where this skill names a specific library (a component library's `Badge`, a query hook, a URL-state helper), treat it as a stand-in for the project's equivalent.

---

## The one rule that decides everything: tier by value, not by granularity

A spec's tier is **not** decided by leaf-vs-composite. A multi-badge composite (e.g. a row of status badges) is rendered-value and gets an HTML sheet; a one-element leaf whose whole reason to exist is behavior (e.g. a link that opens a detail overlay) is behavioral-value and gets a Markdown contract. Ask: *would a reader learn more by **looking** or by **reading**?*

**Litmus test — apply top-down, first match wins:**

| # | If the thing… | Tier | Artifact |
|---|---------------|------|----------|
| 1 | owns a **route, URL state, or global/context state** | **View** | `spec-<name>-view.md` (Markdown + ASCII) |
| 2 | **fetches data / owns async**, or has **non-trivial logic** (sort, debounce, pagination math, open-overlay navigation, query keys) | **Component contract** | `spec-<name>.md` (Markdown) |
| 3 | else — **deterministic from props**, payload is **visual** | **Visual sheet** | `<Name>.spec.html` (rendered in the playbook) |

Typical mapping by kind of thing (use as a starting point; classify each real component with the litmus test):

- **Visual sheets** (`.spec.html`): status/label badges, skeleton loading rows, summary tables, full-page loaders, empty states.
- **Markdown contracts**: data tables, filter panels, pagination, behavioral links, all data-fetching hooks, formatters, URL-state helpers, API clients.
- **View specs**: each routed page, any global overlay/modal that owns its own state, the route/shell layout.

The two boundary cases — memorize them, they are where people reach for the wrong tier:
- **A behavioral link** looks like a trivial anchor (tier 3?) but rule 2 catches it: its contract *is* its behavior (the navigation/overlay it triggers). → Markdown.
- **Pagination** looks visual (tier 3?) but its spec is offset math and disabled-button logic. → Markdown.

---

## Tier 1 — View spec (`views/spec-<name>-view.md`)

A view is a **compositorium**: it owns a route, fetches data, holds URL/state, and composes components. It can't be honestly shown as a static artifact, so it is **Markdown + ASCII** and its value is composition, wiring, and state — never pixels.

**Location:** every view spec lives in `views/`. The filename keeps the `-view` suffix.

Required sections (in addition to the house base):

1. **Route** — the path and any URL-param overlays (e.g. `?item=<id>`).
2. **Component Tree** — the indented `├──`/`└──` tree. Mandatory. This is the most useful thing in the file. **Every component named in the tree carries a linked path to its spec** (e.g. `DetailLink → spec-detail-link.md`); a name with no path is incomplete.
3. **Layout Wireframe (page level)** — the coarse ASCII zone sketch (where the rail, table, pagination sit) + one responsive note. This is the top of the drill-down: it shows where the major zones live, not their internals. Draw the page's significant layout states (e.g. a panel collapsed vs expanded) when they change the composition; one sketch is fine when they don't.
4. **Component Wireframe Breakdown** — the drill-down from the page into each composite component (see its own section below). Mandatory. Steps from the top-level view down to a granular view, one numbered subsection per composite, stopping before atomic primitives.
5. **URL / State Contract** — every param: default, type, and the reset rules (what clears what; what resets `offset`).
6. **State Matrix** — for **every** async surface: loading, empty, error. No surface may silently fail.
7. **Wiring / Bindings** — how data flows into each child by its **declared contract** (see invariant below).

A view spec **references** component contracts **by linked path** (see the reference rule). It must **not** redefine a component's internals. If a view spec describes a child's props rendering, it has leaked — move that into the child's own spec.

---

## The Component Wireframe Breakdown (required in every view spec)

A single page-level zone sketch shows *where the major regions sit* but not *what each region is made of*. The Component Wireframe Breakdown supplies the rest: it walks from the whole-view picture down through each composite component to the named sub-components it composes, so the reader gets a top-level-to-granular view of the entire screen and everything inside it.

**What it is:** a numbered series of subsections, ordered outermost-first (the page, then its direct children, then their composites). Each subsection takes **one composite component** and shows:

1. A short line naming the component and what it composes (the contextual data it presents to the viewer).
2. An **ASCII wireframe of that component's internal layout** — boxed regions labeled with the sub-components they hold. This is the same `┌── box ──┐` style as the page wireframe, but scoped to this one component.
3. *(Encouraged, not required)* the component's significant layout/interaction states drawn inline — collapsed vs expanded, empty, completed — wherever a state changes the composition. Draw the states that change what's composed; skip the ones that don't.
4. *(Recommended default format)* a `| Sub-component | Composes |` table listing each composite child and the contextual data it composes. Prose is acceptable where it reads better, but the table is the house default because it makes the seam to each child explicit.

**Where to stop — the granularity rule.** Drill down only through components that **compose contextual data for the viewer** — things made of multiple parts that together convey meaning (a header bar, a panel, a row that carries status + label + timer). **Stop before atomic primitives:** a single icon, a lone button, a bare input, a status dot. Those are leaves; they belong in their own component contract or visual sheet if they need one, not in the view's breakdown. The test: *does this thing assemble several pieces into a contextual whole (draw it), or is it a single control/indicator (don't)?*

**Reference, don't redefine.** Each breakdown subsection points *at* the sub-component's own spec by linked path (see the reference rule) — it describes the **composition** (what sits where, what data flows in), never the child's internal render or props logic. If a subsection starts specifying how a child formats its own props, it has leaked into that child's contract.

**Why both levels.** The page wireframe answers "where does everything go?"; the breakdown answers "what is each thing made of, and what does it show the viewer?" Together they let an implementer wire the whole view — and a reviewer audit it — without opening every child spec, while still keeping each child's internals in the child's own spec.

---

## Tier 2 — Component contract (`spec-<name>.md`)

For behavioral / stateful / data-coupled components, hooks, and lib functions. Markdown, no pixels.

Required sections:

1. **Props / Inputs** — a table: name, type, required, default, meaning. This table is the **seam** other specs bind against.
2. **Events emitted** — every callback prop: when it fires, with what payload. (`onChange(key, value)`, `onSortChange(next)`.)
3. **Behavior** — the logic the compiler does *not* enforce: debounce, sort-toggle direction, disabled conditions, query key + `staleTime`, reset semantics.
4. **States** — loading / empty / error / disabled, as applicable.
5. **A11y** — roles, `aria-*`, focus behavior. Mandatory for any interactive component.

A component contract is **page-agnostic**. If it names a specific page or view, it has leaked — describe the interface, not the caller.

**Forbidden:** pasting the full implementation. A full column-definition array, a `buildXFilterConfig` body, or a complete component's markup/JSX is code, not spec — it drifts the moment the code changes and removes the implementer's judgment. Allowed: type signatures, the *shape* of a config object, and short illustrative snippets where the interface is genuinely subtle.

---

## Tier 3 — Visual sheet (`<Name>.spec.html`)

For deterministic-from-props, visually-dominant components. The sheet is a **forward design target** — "done looks like this" — authored in HTML+CSS *before* the React component exists, openable directly in a browser, no build step.

Rules:

- One file per component: `<Name>.spec.html`. Links the shared `references/playbook.css`.
- Contains, in order: a **purpose** header (what it is, what it's for, one paragraph), then **every meaningful variant rendered**, each labeled with the prop values that produce it (e.g. `active=true isValidData=true` → green "Active"), then **interaction + a11y notes**.
- It is a **target, not a mirror.** A hand-styled sheet cannot reproduce the component library's runtime theme exactly — it communicates *intent*. Do not treat it as documentation of the built component.
- **Lifecycle: HTML mock now, Storybook later.** At implementation time, author a real `<Name>.stories.tsx` over the built component (the actual playbook, zero drift) and **retire the `.spec.html`**. Do not maintain both. The purpose prose moves into the story's docs block.

The playbook is the set of `.spec.html` sheets sharing `playbook.css`, plus a static index — viewable with any dumb static server, no React, no dependency. See `references/playbook.css` and `references/templates.md`.

**Serving the playbook:** when the user asks to open or view the playbook, start a static server from the project's playbook folder and open the browser:

```bash
# From the project's specs playbook folder (e.g. <specs-root>/playbook), run in background:
python3 -m http.server 8099

# Open in browser
open http://localhost:8099
```

Serve the project's `<specs-root>/playbook/` directory on port `8099`. If that port is in use, try `8098` or `8100`. Discover the specs root from the project rather than assuming a fixed path.

---

## The wiring invariant (how a component plugs into a view)

This is the seam the whole system turns on, stated as a two-way rule:

- A **component contract declares an interface** (props in, events out) and is page-agnostic.
- A **view spec declares the bindings** (e.g. "the view feeds `facets` into `buildFilterConfig`, passes the result as `<FilterPanel filters=… onChange=…>`, and on `onChange` writes the URL via the project's URL-state helper").
- Neither crosses the seam: the view never redefines the component's render; the component never names the view.

When you wire a component into a view, you are binding to its **Props/Inputs table and its Events**. If that table doesn't say what you need, fix the contract — don't describe the internal behavior inside the view spec.

---

## Reference rule: name a component, link its spec

Naming a component is not enough — **every reference to another component, hook, lib
function, or view must carry the actual file path to that thing's spec.** A bare name
("feeds into `FilterPanel`") is incomplete; the reader must be one click away from the
contract being bound against.

- **Always link, never just name.** First mention of a referenced spec in a section is a
  Markdown link: `[FilterPanel](spec-filter-panel.md)`, `[ListView](views/spec-list-view.md)`,
  `[StatusBadge](playbook/StatusBadge.spec.html)`.
- **Paths are rooted at the specs folder** (the project's `<specs-root>/`), not relative to the file
  doing the referencing. So a view spec in `views/` links a sibling contract as
  `spec-datatable.md` resolved from the specs root — write the path *from the specs folder
  down to the target*, the same way from every file, so the same component reads the same
  everywhere regardless of which subfolder names it.
- **Applies to every tier and the overview.** Component trees, Depends On lists, Component
  Breakdown blocks, Data/Query maps, the overview's indices and Spec Table — anywhere a
  spec name appears, the path appears with it.

This is what "reference, don't redefine" means in practice: the view points *at* the
contract by path; it never restates the contract's internals.

---

## ASCII discipline

Three distinct uses, three rules:

| Use | Looks like | Rule |
|-----|-----------|------|
| **Component tree** | `├──`/`└──` indented | **Mandatory** in view specs. Highest value, near-zero rot. |
| **Page layout wireframe** | `┌── box ──┐` regions | **Required in view specs.** Top of the drill-down: major zones, not internals. |
| **Component breakdown wireframe** | `┌── box ──┐` scoped to one composite | **Required in view specs** (one per composite, see the breakdown section). Shows a component's internal composition + named sub-components. |
| **Content/format sample** | `ACME Acme Corp. [US] [Active]` | **Encouraged** where formatting is the spec (tables, badges, headers). Shows format, not layout. |

**Dimensions in wireframes:** prefer qualitative descriptions (`fixed height`, `fills remaining height`, `narrow toggle strip`). **Load-bearing dimensions are allowed** where the number carries real layout meaning — a fixed panel height, a collapse width, a column split like `~40% / ~60%`. Do **not** litter wireframes with decorative or exhaustive pixel specs (padding, margins, font sizes); those are the implementer's judgment, not the spec's.

---

## File layout

```
<specs-root>/                     # the project's frontend specs folder
  overview.md                     # the map/index — keep current
  views/
    spec-<page>-view.md           # tier 1, one per route + global overlays
  spec-<component>.md             # tier 2, one per behavioral component/hook/lib
  playbook/
    index.html                    # static index linking every sheet
    playbook.css                  # shared stylesheet (see references/)
    <Name>.spec.html              # tier 3, one per visual component
```

**All tier-1 view specs live in `views/`.** Tier-2 contracts and the playbook stay at the
top level. `overview.md` carries the tier of each spec and links it. Update it whenever a
spec is added or re-tiered.

---

## Authoring checklist

Before declaring a spec done:

- [ ] Ran the litmus test top-down; the tier is the first match, not a guess.
- [ ] View spec lives in `views/` and has: component tree, page-level wireframe, **Component Wireframe Breakdown** (one subsection per composite, outermost-first), full URL/state contract, a loading+empty+error row for every async surface.
- [ ] The breakdown drills only through composites that compose contextual data; it stops before atomic primitives (icons, single buttons, bare inputs, status dots).
- [ ] Each breakdown subsection references its sub-components by linked path and describes composition only — it does not redefine a child's internal render or props.
- [ ] Wireframe dimensions are qualitative or load-bearing only — no decorative/exhaustive pixel specs.
- [ ] Component contract has a props table and an events list; it names no specific page.
- [ ] Every referenced component / hook / lib / view is a linked path (rooted at the specs folder), not a bare name — in the tree, Depends On, breakdowns, and the overview's indices.
- [ ] No full implementation pasted (no complete column-definition array, config body, or component markup/JSX).
- [ ] Visual sheet renders every variant, labeled by props, and links `playbook.css`.
- [ ] Wiring lives in the view; interface lives in the component; neither crosses the seam.
- [ ] Authoritative voice ("the component does"), no open questions left in the spec (those go to `open-questions.md`).
