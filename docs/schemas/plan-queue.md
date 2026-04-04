# plan-queue.json Schema

> Input file for initializing the **planning phase**.
> Passed via `forgectl init --phase planning --from plan-queue.json`
> or during phase shift: `forgectl advance --from plan-queue.json`
>
> Also generated automatically during the **generate_planning_queue phase** and written to `<state_dir>/plan-queue.json`.

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
| `file` | string | **yes** | Target path for the output `plan.json`, relative to project root. Convention: `<domain>/.forge_workspace/implementation_plan/plan.json` |
| `specs` | string[] | **yes** | Spec file paths to study during planning, relative to project root. Can be empty `[]`. |
| `spec_commits` | string[] | **yes** | Git commit hashes associated with the specs. Used to view spec diffs during planning. Can be empty `[]`. |
| `code_search_roots` | string[] | **yes** | Directory roots for codebase exploration during STUDY_CODE. Can be empty `[]`. |

---

## Validation Rules

- Top-level must have exactly one key: `"plans"`.
- `plans` array must be non-empty.
- Each entry must have exactly the 6 fields listed — no more, no fewer. (Note: `topic` was removed; `spec_commits` was added.)
- All field names and values are case-sensitive.

---

## Example

```json
{
  "plans": [
    {
      "name": "Launcher Implementation Plan",
      "domain": "launcher",
      "file": "launcher/.forge_workspace/implementation_plan/plan.json",
      "specs": [
        "launcher/specs/service-configuration.md",
        "launcher/specs/launching-system-processes.md"
      ],
      "spec_commits": ["7cede10", "8743b1d"],
      "code_search_roots": ["launcher/", "lib/"]
    }
  ]
}
```

---

## Output

For each entry, the planning phase produces:

```
<domain>/.forge_workspace/implementation_plan/
├── plan.json      # Implementation plan manifest (see plan-json.md)
└── notes/         # Per-package implementation guidance
    ├── <pkg>.md
    └── ...
```

---

## Auto-Generation (generate_planning_queue phase)

During the generate_planning_queue phase, forgectl auto-generates this file from completed specs, commit hashes, and code search roots collected during the specifying phase. The file is written to `<state_dir>/plan-queue.json`.

### Generation Logic

For each domain (in spec queue order — the order domains first appeared):

| Field | Source |
|-------|--------|
| `name` | `"<Domain> Implementation Plan"` (domain name capitalized) |
| `domain` | Domain name from completed specs |
| `file` | `<domain>/.forge_workspace/implementation_plan/plan.json` |
| `specs` | All completed spec file paths for the domain |
| `spec_commits` | Deduplicated list of all `commit_hashes` from the domain's completed specs |
| `code_search_roots` | From `specifying.domains[<domain>].code_search_roots` if set via `set-roots`; otherwise `["<domain>/"]` |

### Architect Review (REFINE state)

After auto-generation, the scaffold enters the REFINE state. The architect reviews `<state_dir>/plan-queue.json`, reorders domains, adjusts entries, or leaves it unchanged. Advancing from REFINE validates the file. If valid, transitions to PHASE_SHIFT.

The architect controls domain ordering — the generated order follows the spec queue but can be changed freely during REFINE.

---

## Source

- Type definitions: `forgectl/state/types.go` (`PlanQueueInput`, `PlanQueueEntry`)
- Validation: `forgectl/state/validate.go` (`ValidatePlanQueue`)
- Reference docs: `skills/implementation_planning/references/plan-queue-format.md`
- Auto-generation: `forgectl/state/advance.go` (generate_planning_queue ORIENT)
