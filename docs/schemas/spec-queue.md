# spec-queue.json Schema

> Input file for initializing the **specifying phase**.
> Passed via `forgectl init --phase specifying --from spec-queue.json`

---

## Root

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `specs` | SpecEntry[] | **yes** | Non-empty array of specs to draft. No other top-level fields allowed. |

---

## SpecEntry

All 6 fields are required on every entry. No extra fields allowed.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | **yes** | Human-readable spec name. Must be unique across all entries. Used for display and `depends_on` cross-references. |
| `domain` | string | **yes** | Domain grouping (typically a top-level project directory). |
| `topic` | string | **yes** | One-sentence topic of concern. Describes the single responsibility this spec addresses. |
| `file` | string | **yes** | Target file path relative to project root. Convention: `<domain>/specs/<kebab-name>.md` |
| `planning_sources` | string[] | **yes** | Paths to planning documents this spec is derived from, relative to project root. Can be empty `[]`. |
| `depends_on` | string[] | **yes** | Names of other specs this one depends on. Can be empty `[]`. Every value must match a `name` in another entry. |

---

## Validation Rules

- Top-level must have exactly one key: `"specs"`.
- `specs` array must be non-empty.
- Each entry must have exactly the 6 fields listed — no more, no fewer.
- All `depends_on` values must reference a `name` in another entry.
- No circular dependencies.
- All field names and values are case-sensitive.

---

## Example

```json
{
  "specs": [
    {
      "name": "Service Configuration",
      "domain": "launcher",
      "topic": "The launcher loads and validates service endpoint configuration from YAML",
      "file": "launcher/specs/service-configuration.md",
      "planning_sources": [
        ".workspace/planning/launcher/config-loading.md"
      ],
      "depends_on": []
    },
    {
      "name": "Launching System Processes",
      "domain": "launcher",
      "topic": "The launcher spawns and health-checks detached system processes",
      "file": "launcher/specs/launching-system-processes.md",
      "planning_sources": [
        ".workspace/planning/launcher/process-lifecycle.md"
      ],
      "depends_on": ["Service Configuration"]
    }
  ]
}
```

---

## Source

- Type definitions: `forgectl/state/types.go` (`SpecQueueInput`, `SpecQueueEntry`)
- Validation: `forgectl/state/validate.go` (`ValidateSpecQueue`)
