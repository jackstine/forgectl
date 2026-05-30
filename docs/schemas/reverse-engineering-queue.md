# reverse-engineering-queue.json Schema

> Produced during the **reverse engineering phase** at each domain's QUEUE state.
> Written to a fixed convention path owned by forgectl:
> `<project_root>/.forgectl/state/reverse-engineering-queue.json`

The path is **not** user-supplied. Forgectl owns it and records it in the state
file on the first QUEUE advance. `forgectl advance` takes **no** `--file` flag in
the QUEUE state. A single queue file accumulates entries across all domains —
each domain's QUEUE state appends that domain's entries to the same file.

All entry paths (`file`, `code_search_roots`) are relative to the **domain root**
(`<project_root>/<domain>/`), not the project root.

---

## Root

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `specs` | SpecEntry[] | **yes** | Ordered list of specs to create or update, across all processed domains. No other top-level fields allowed. |

---

## SpecEntry

All 7 fields are required on every entry. No extra fields allowed.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | **yes** | Display name for the spec. |
| `domain` | string | **yes** | Domain grouping. Must match a domain in the initialized domain list. The entry's `file` and `code_search_roots` resolve against the domain root `<project_root>/<domain>/`. |
| `topic` | string | **yes** | One-sentence topic of concern (single responsibility; no "and" conjoining unrelated capabilities). |
| `file` | string | **yes** | Target spec file path, relative to the domain root. Convention: `specs/<kebab-name>.md`. |
| `action` | string | **yes** | `"create"` or `"update"`. For updates, `file` is both source and destination. |
| `code_search_roots` | string[] | **yes** | Directories forming the root of the core code for this spec's topic of concern, relative to the domain root. May not be empty. See definition below. |
| `depends_on` | string[] | **yes** | Names of specs this one depends on. May be empty `[]`. Used by RECONCILE for cross-referencing; ignored by the execution loop. |

---

## `code_search_roots` Definition

Each root is the directory that forms the **root of the core code** implementing
this spec's topic of concern. Go as deep into the tree as needed: the root is the
deepest directory that still contains **all** the files for that one capability.
It is often a nested package several levels down (for example
`net/http/internal/httpcommon/` — four levels deep — for shared HTTP
request/response handling), not a broad top-level directory.

- A single topic of concern may span multiple such directories — hence a list.
- Each root is searched recursively to its full depth; every file beneath it is in scope.
- Roots are directories, not single files, relative to the domain root.
- Must be non-empty for every entry.

---

## Validation Rules

Validation runs at QUEUE advance.

- Top-level must have exactly one key: `"specs"`.
- Each entry must have exactly the 7 fields listed — no more, no fewer.
- `action` must be `"create"` or `"update"`.
- `code_search_roots` must be non-empty for every entry.
- Every entry's `domain` must match a domain in the initialized domain list.
  Unrecognized domains are rejected (error suggests `forgectl add-domain <domain>`).
- Every `code_search_roots` directory must exist on disk (resolved against the
  domain root). Missing directories are rejected.
- No circular dependencies in the `depends_on` graph.
- Entries are ordered by dependency: specs with no dependencies first.
- All field names and values are case-sensitive.

---

## Advance Behavior (QUEUE state)

`forgectl advance` never accepts a `--file` flag in QUEUE; the path is fixed.

| Advance | Behavior |
|---------|----------|
| First (no stored hash) | Read file at the convention path, compute and store content hash, validate schema + domain + path existence. Valid → advance. Invalid → error with violations. |
| Subsequent (hash stored) | Re-read file. Compare hash. Unchanged → error: `"Queue file has not changed. Update the file and retry."` Changed → recompute hash, re-validate. Valid → advance. |

Rejections:
- `--file` supplied → `"forgectl advance takes no --file flag in QUEUE. The queue file is fixed at .forgectl/state/reverse-engineering-queue.json."`
- File not found → error with the expected path.

---

## Example

```json
{
  "specs": [
    {
      "name": "Repository Loading",
      "domain": "optimizer",
      "topic": "The optimizer clones or locates a repository and provides its path for downstream modules",
      "file": "specs/repository-loading.md",
      "action": "create",
      "code_search_roots": ["src/repo/", "src/config/"],
      "depends_on": []
    },
    {
      "name": "URL Validation",
      "domain": "optimizer",
      "topic": "The optimizer validates repository URLs before cloning",
      "file": "specs/url-validation.md",
      "action": "update",
      "code_search_roots": ["src/repo/validate/"],
      "depends_on": ["Repository Loading"]
    }
  ]
}
```

---

## Consumption

- **EXECUTE_REVERSE_ENGINEER** loop: iterates entries in queue order, one item
  per iteration, across all domains. Reads code under `code_search_roots` and
  writes the spec at `<domain>/<file>`. `depends_on` is ignored here.
- **RECONCILE** loop: uses `depends_on` to wire `Depends On` and symmetric
  `Integration Points` cross-references between specs.

---

## Source

- Intended behavior: `forgectl/specs/reverse-engineering.md`
- (Type definitions and validation in `forgectl/state/` are added when the
  reverse engineering phase is implemented; this schema is derived from the
  spec until then.)
