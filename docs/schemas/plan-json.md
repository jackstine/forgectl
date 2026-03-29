# plan.json Schema

> Structured implementation plan manifest created during the **planning phase**
> and consumed during the **implementing phase**.
> Lives at `<domain>/.workspace/implementation_plan/plan.json`.

---

## Root

Only these 4 top-level fields are allowed.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `context` | Context | **yes** | Domain and project metadata. |
| `refs` | Ref[] | no | Reference files (specs, notes). Omitted if empty. |
| `layers` | Layer[] | **yes** | Ordered dependency tiers grouping items. |
| `items` | Item[] | **yes** | Implementation work items. |

---

## Context

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `domain` | string | **yes** | Domain name (matches top-level directory). Non-empty. |
| `module` | string | **yes** | Go module name or package identifier. Non-empty. |

---

## Ref

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | **yes** | Short identifier (e.g., `"spec-logging"`, `"notes-config"`). |
| `path` | string | **yes** | Path relative to plan.json directory. Must exist on disk. **No `#anchor` fragments** — forgectl runs `os.Stat()` on the raw string. |

**Important:** Refs must be objects `{"id": "...", "path": "..."}`, not strings.

### Path Resolution

All paths in `refs[].path` and `items[].ref` are resolved relative to `filepath.Dir(plan.json)`.

Example: if plan.json is at `api/.workspace/implementation_plan/plan.json`, then `../../specs/foo.md` resolves to `api/specs/foo.md`.

---

## Layer

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | **yes** | Layer identifier (e.g., `"L0"`, `"L1"`). |
| `name` | string | **yes** | Human-readable name (e.g., `"Foundation"`, `"Core"`). |
| `items` | string[] | **yes** | Item IDs in this layer, in suggested implementation order. |

### Layer Rules

- Every item must appear in exactly one layer.
- Items in layer N may only depend on items in layers 0..N.
- Layer ordering is significant — L0 before L1 before L2.

---

## Item

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | **yes** | Unique identifier. Convention: `<package>.<concern>` (e.g., `"config.load"`). |
| `name` | string | **yes** | Short action-oriented name. |
| `description` | string | **yes** | One or two sentences explaining what and why. |
| `depends_on` | string[] | **yes** | Item IDs that must complete first. Use `[]` for no deps. **Never null.** |
| `steps` | string[] | no | Ordered implementation instructions. Omitted if empty. |
| `files` | string[] | no | File paths to create/modify, relative to domain root. Omitted if empty. |
| `spec` | string | no | Spec filename reference. Single string only. No `#anchors`. |
| `ref` | string | no | Notes file path relative to plan.json directory. Must exist on disk. No `#anchors`. |
| `tests` | Test[] | **yes** | Acceptance criteria. Use `[]` for items with no tests. **Never null.** |

### Fields Added by Phase Transition

When forgectl transitions from planning to implementing, it adds these to every item:

| Field | Type | Initial | Description |
|-------|------|---------|-------------|
| `passes` | string | `"pending"` | Status: `"pending"` → `"done"` → `"passed"` or `"failed"` |
| `rounds` | int | `0` | Evaluation round counter. Incremented after each eval cycle. |

**Do not include `passes` or `rounds` when drafting.** Forgectl adds them automatically.

---

## Test

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `category` | string | **yes** | One of: `"functional"`, `"rejection"`, `"edge_case"` |
| `description` | string | **yes** | What this test verifies — specific enough to evaluate unambiguously. |

### Test Categories

| Category | Maps to | Purpose |
|----------|---------|---------|
| `functional` | Spec Behavior sections | Core happy-path behavior |
| `rejection` | Spec Rejection table | Invalid inputs rejected correctly |
| `edge_case` | Spec Edge Cases | Boundary conditions handled |

---

## Validation Summary

Forgectl validates plan.json at two points: during DRAFT advance and during implementing PHASE_SHIFT.

1. Only `context`, `refs`, `layers`, `items` allowed at top level.
2. `context.domain` and `context.module` must be non-empty strings.
3. All `refs[].path` and `items[].ref` paths must exist on disk. No `#anchors`.
4. Item IDs must be unique.
5. Every item must appear in exactly one layer.
6. Items can only depend on same-layer or earlier-layer items.
7. No circular dependencies (DAG enforced via DFS).
8. All `depends_on` IDs must reference existing items.
9. `depends_on` and `tests` must be arrays, never null.
10. Test categories must be `"functional"`, `"rejection"`, or `"edge_case"`.

---

## Example

```json
{
  "context": {
    "domain": "launcher",
    "module": "spectacular/launcher"
  },
  "refs": [
    {"id": "spec-config", "path": "../../specs/service-configuration.md"},
    {"id": "notes-config", "path": "notes/config.md"}
  ],
  "layers": [
    {"id": "L0", "name": "Foundation", "items": ["config.types", "config.load"]},
    {"id": "L1", "name": "Core", "items": ["daemon.spawn", "daemon.health"]}
  ],
  "items": [
    {
      "id": "config.types",
      "name": "Service config type definitions",
      "description": "Go structs for validated service endpoint configuration.",
      "depends_on": [],
      "files": ["internal/config/types.go"],
      "steps": ["Define ServiceEndpoint struct", "Define ServicesConfig struct"],
      "spec": "service-configuration.md",
      "ref": "notes/config.md",
      "tests": [
        {"category": "functional", "description": "Three named fields, not a map"}
      ]
    },
    {
      "id": "config.load",
      "name": "Load YAML, apply defaults, validate",
      "description": "Parse config file, apply default values, reject invalid config.",
      "depends_on": ["config.types"],
      "files": ["internal/config/load.go", "internal/config/load_test.go"],
      "steps": ["Implement LoadConfig()", "Add default port logic", "Add validation"],
      "spec": "service-configuration.md",
      "ref": "notes/config.md",
      "tests": [
        {"category": "functional", "description": "Default ports applied when services are empty"},
        {"category": "rejection", "description": "Missing services section rejected"},
        {"category": "edge_case", "description": "Duplicate ports allowed"}
      ]
    },
    {
      "id": "daemon.spawn",
      "name": "Spawn detached process",
      "description": "Start a system process in a new process group.",
      "depends_on": ["config.load"],
      "files": ["internal/daemon/spawn.go"],
      "spec": "launching-system-processes.md",
      "ref": "notes/daemon.md",
      "tests": [
        {"category": "functional", "description": "Process starts in new process group"}
      ]
    },
    {
      "id": "daemon.health",
      "name": "Health check spawned process",
      "description": "Poll process health endpoint until ready or timeout.",
      "depends_on": ["daemon.spawn"],
      "files": ["internal/daemon/health.go"],
      "spec": "launching-system-processes.md",
      "ref": "notes/daemon.md",
      "tests": [
        {"category": "functional", "description": "Returns healthy after endpoint responds"},
        {"category": "edge_case", "description": "Times out after configured duration"}
      ]
    }
  ]
}
```

---

## Source

- Type definitions: `forgectl/state/types.go` (`PlanJSON`, `PlanItem`, `PlanLayerDef`, `PlanRef`, `PlanTest`)
- Validation: `forgectl/state/validate.go` (`ValidatePlanJSON`)
- Phase mutation: `forgectl/state/advance.go` (`advancePhaseShift`)
- Authoritative reference: `skills/implementation_planning/references/plan-format.json`
