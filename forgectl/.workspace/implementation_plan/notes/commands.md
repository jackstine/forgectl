# New Commands Notes

## validate command

### Interface

```
forgectl validate [--type <type>] <file_path>
```

No session required — does not call `FindProjectRoot()` or `LoadConfig()`.

Flags:
- `<file_path>` (positional, required)
- `--type` (optional): `spec-queue`, `plan-queue`, or `plan`

### Auto-detection

Inspect top-level JSON key:
- `"specs"` → spec-queue
- `"plans"` → plan-queue
- `"context"` → plan

If key doesn't match any known type:
```
Error: cannot detect file type.
  Expected one of these top-level keys:
    "specs"    → spec-queue
    "plans"    → plan-queue
    "context"  → plan
  Found: "widgets"
  Hint: use --type to specify the file type explicitly.
```

### Type override mismatch

If `--type spec-queue` but top-level key is `"plans"`:
```
Error: --type spec-queue expects top-level key "specs", found "plans".
  Hint: did you mean --type plan-queue?
```

### Success output

```
Detected: spec-queue (top-level key: "specs")

Validated: spec-queue.json — 3 entries, no errors.
```

For plan:
```
Detected: plan (top-level key: "context")

Validated: plan.json — 12 items, no errors.
```

### Validation logic

Reuse existing validation functions:
- `state.ValidateSpecQueue(data)`
- `state.ValidatePlanQueue(data)`
- `state.ValidatePlanJSON(data, filepath.Dir(filePath))`

For plan.json, `baseDir` is the directory containing the file (for ref path resolution).

### Error output

```
Detected: spec-queue (top-level key: "specs")

Error: validation failed with 2 errors:
  1. specs[2].depends_on[0]: references "Snapshot Diffing" but no spec with that name exists
  2. specs[4]: missing required field "planning_sources"
```

### Exit codes
- 0: validation passes
- 1: any error (file not found, invalid JSON, validation failure)

### Read-only
Does not log (status/eval/validate are read-only per activity-logging spec).

## add-queue-item command

See specifying.md for full details.

Cobra command: `forgectl add-queue-item`

Flags:
- `--name` (required)
- `--domain` (required at DONE, optional elsewhere)
- `--topic` (required)
- `--file` (required)
- `--source` (optional, repeatable via StringArrayVar)

State validation: load state, check phase==specifying (error: "add-queue-item is only valid in the specifying phase (current phase: <phase>)."), check current state is one of DRAFT, CROSS_REFERENCE_REVIEW, DONE, RECONCILE_REVIEW. Return error otherwise.

Input validation:
- `--file` must point to an existing file on disk. Error: "file <path> does not exist. add-queue-item registers specs that have already been written. Create the spec file first, then register it."
- `--name` must be unique across queue and completed specs. Error if duplicate.
- `domain_path` is derived from `--file` (two levels up from spec file, e.g. `optimizer/specs/foo.md` -> `domain_path = optimizer/`).

Domain inference: at DRAFT, use `current_specs[0].Domain`; at CROSS_REFERENCE_REVIEW, use the domain from the active cross-reference state.

## set-roots command

Cobra command: `forgectl set-roots`

Usage: `forgectl set-roots [--domain <domain>] <path> [<path>...]`

Flags:
- `--domain` (required at DONE, optional elsewhere)

Args: one or more positional directory paths.

State validation: load state, check phase==specifying (error: "set-roots is only valid in the specifying phase (current phase: <phase>)."), check current state is CROSS_REFERENCE_REVIEW or DONE.

Input validation:
- At least one positional path argument is required. Error if none provided.
- The domain must have at least one completed spec before accepting set-roots. Error if not.

Implementation:
```go
if s.Specifying.Domains == nil {
    s.Specifying.Domains = make(map[string]DomainMeta)
}
s.Specifying.Domains[domain] = DomainMeta{CodeSearchRoots: paths}
```

Output: print confirmation of roots set for domain.
