# plan-queue.json Schema

> Input file for initializing the **planning phase**.
> Passed via `forgectl init --phase planning --from plan-queue.json`
> or during phase shift: `forgectl advance --from plan-queue.json`

---

## Root

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `plans` | PlanEntry[] | **yes** | Non-empty array of plans to produce. No other top-level fields allowed. |

---

## PlanEntry

All 6 fields are required on every entry. No extra fields allowed.

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | **yes** | Display name for the plan. Shown in `forgectl status` output. |
| `domain` | string | **yes** | Domain this plan covers (typically a directory name). |
| `topic` | string | **yes** | One-sentence description of what this plan addresses. |
| `file` | string | **yes** | Target path for the output `plan.json`, relative to project root. Convention: `<domain>/.workspace/implementation_plan/plan.json` |
| `specs` | string[] | **yes** | Spec file paths to study during planning, relative to project root. Can be empty `[]`. |
| `code_search_roots` | string[] | **yes** | Directory roots for codebase exploration during STUDY_CODE. Can be empty `[]`. |

---

## Validation Rules

- Top-level must have exactly one key: `"plans"`.
- `plans` array must be non-empty.
- Each entry must have exactly the 6 fields listed — no more, no fewer.
- All field names and values are case-sensitive.

---

## Example

```json
{
  "plans": [
    {
      "name": "Service Configuration",
      "domain": "launcher",
      "topic": "Implementation plan for service configuration loading and validation",
      "file": "launcher/.workspace/implementation_plan/plan.json",
      "specs": [
        "launcher/specs/service-configuration.md",
        "launcher/specs/launching-system-processes.md"
      ],
      "code_search_roots": ["launcher/", "lib/"]
    }
  ]
}
```

---

## Output

For each entry, the planning phase produces:

```
<domain>/.workspace/implementation_plan/
├── plan.json      # Implementation plan manifest (see plan-json.md)
└── notes/         # Per-package implementation guidance
    ├── <pkg>.md
    └── ...
```

---

## Source

- Type definitions: `forgectl/state/types.go` (`PlanQueueInput`, `PlanQueueEntry`)
- Validation: `forgectl/state/validate.go` (`ValidatePlanQueue`)
- Reference docs: `skills/implementation_planning/references/plan-queue-format.md`
