# Plan Queue Input Format

> Defines the JSON input file required to start the forgectl planning phase.
> This file is passed via `forgectl init --phase planning --from <plan-queue.json>`.

---

## Purpose

The plan queue tells forgectl which implementation plans to produce in this session. Each entry identifies a domain, the specs to study, and where the resulting `plan.json` will be written.

---

## Schema

```json
{
  "plans": [
    {
      "name": "<string>",
      "domain": "<string>",
      "file": "<string>",
      "specs": ["<string>", ...],
      "spec_commits": ["<string>", ...],
      "code_search_roots": ["<string>", ...]
    }
  ]
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Display name for the plan (shown in `forgectl status`) |
| `domain` | string | yes | Domain this plan covers (e.g., `launcher`, `api`) |
| `file` | string | yes | Target path for the output `plan.json`, relative to project root |
| `specs` | string[] | yes | Spec file paths to study during STUDY_SPECS. May be empty. |
| `spec_commits` | string[] | yes | Git commit hashes associated with specs for viewing diffs. May be empty. |
| `code_search_roots` | string[] | yes | Directory roots for codebase exploration during STUDY_CODE. May be empty. |

---

## Validation Rules

Forgectl validates the queue strictly on `init`. If validation fails, it prints errors and the expected schema, then exits with code 1.

- Top-level key must be `"plans"` (no other fields allowed)
- `plans` must be a non-empty array
- Each entry must have all 6 fields listed above
- No extra fields permitted beyond the 6 listed
- `spec_commits` is an array and may be empty (`[]`)
- `specs` and `code_search_roots` are arrays and may be empty (`[]`)

---

## Example

```json
{
  "plans": [
    {
      "name": "Service Configuration",
      "domain": "launcher",
      "file": "launcher/.forge_workspace/implementation_plan/plan.json",
      "specs": [
        "launcher/specs/service-configuration.md",
        "launcher/specs/launching-system-processes.md"
      ],
      "spec_commits": ["7cede10", "8743b1d"],
      "code_search_roots": ["launcher/", "api/"]
    }
  ]
}
```

---

## Output Location

The planning phase produces files at the path specified in `file`:

```
<domain>/.forge_workspace/implementation_plan/
├── plan.json          # The implementation plan manifest
└── notes/             # Reference notes per package
    ├── <package>.md
    └── ...
```

The `plan.json` format is defined in the forgectl docs at `forgectl/docs/PLAN_FORMAT.md`.

---

## How to Start

```bash
forgectl init --phase planning --from plan-queue.json
```

All batch sizes, round limits, and guided settings are configured in `.forgectl/config` (TOML).

After init, run `forgectl status` to see the session overview and `forgectl advance` to begin.
