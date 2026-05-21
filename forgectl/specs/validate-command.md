# Validate Command

## Topic of Concern
> The scaffold validates JSON input files against their expected schemas, with auto-detection from top-level keys.

## Context

Forgectl consumes three types of JSON input files across its lifecycle phases: spec queues, plan queues, and implementation plans. Each has a distinct schema with specific validation rules. The `validate` command runs these validations independently of session state — no active session is required.

The command auto-detects the file type by inspecting the top-level JSON key. A `--type` flag allows explicit override when auto-detection is insufficient or when the user wants to be explicit.

## Depends On
- **session-init** — reuses the same validation functions used during `init`.

## Integration Points

| Spec | Relationship |
|------|-------------|
| session-init | Same validation logic used at `init` and phase shifts; `validate` exposes it as a standalone command |
| plan-production | Same 12-point plan.json validation used at the VALIDATE state |
| reverse-engineering | Init input and reverse engineering queue schemas validated using the same validation logic |

---

## Interface

### Inputs

```
forgectl validate [--type <type>] <file_path>
```

| Flag | Required | Description |
|------|----------|-------------|
| `<file_path>` | yes | Path to the JSON file to validate |
| `--type` | no | Explicit file type: `spec-queue`, `plan-queue`, or `plan`. Overrides auto-detection. |

### Outputs

**Successful validation:**

```
Detected: spec-queue (top-level key: "specs")

Validated: spec-queue.json — 3 entries, no errors.
```

**Validation failure (auto-detected):**

```
Detected: spec-queue (top-level key: "specs")

Error: validation failed with 2 errors:
  1. specs[2].depends_on[0]: references "Snapshot Diffing" but no spec with that name exists
  2. specs[4]: missing required field "planning_sources"
```

**Auto-detection failure:**

```
Error: cannot detect file type.
  Expected one of these top-level keys:
    "specs"    → spec-queue (used in specifying phase)
    "plans"    → plan-queue (used in planning phase)
    "context"  → plan.json  (used in planning/implementing phases)
  Found: "widgets"
  Hint: use --type to specify the file type explicitly.
```

**Invalid JSON:**

```
Error: invalid JSON at line 14, column 3: unexpected comma.
```

**Type override mismatch:**

```
Error: --type spec-queue expects top-level key "specs", found "plans".
  Hint: did you mean --type plan-queue?
```

**Type override, validation failure:**

```
Validated: plan.json

Error: validation failed with 3 errors:
  1. refs[1].path: "notes/auth-flow.md" does not exist (resolved from plan.json directory)
  2. items: duplicate ID "config.load" (items[2] and items[5])
  3. layers[1].items: "daemon.io" depends on "daemon.types" which is in a later layer (L2)
```

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| No file path argument | Error: "usage: forgectl validate [--type <type>] <file_path>". Exit code 1. | File path is required |
| File does not exist | Error naming the path. Exit code 1. | File must exist to validate |
| File is not valid JSON | Error with line/column. Exit code 1. | JSON parse is the first check |
| Auto-detection fails and no `--type` | Error listing expected keys and found key. Exit code 1. | Cannot determine which schema to apply |
| `--type` value is not `spec-queue`, `plan-queue`, or `plan` | Error listing valid types. Exit code 1. | Unknown type |
| `--type` provided but top-level key doesn't match | Error with hint. Exit code 1. | Type mismatch prevents silent miscategorization |

---

## Behavior

### Auto-Detection

The command inspects the top-level keys of the parsed JSON to determine the file type:

| Top-level key | Detected type | Validation applied |
|---------------|--------------|-------------------|
| `"specs"` | spec-queue | Spec queue validation (field count, required fields, depends_on refs, no cycles) |
| `"plans"` | plan-queue | Plan queue validation (field count, required fields) |
| `"context"` | plan | Plan validation (12-point check: JSON parse, top-level fields, context, refs exist, item schema, ID uniqueness, layer coverage, layer ordering, DAG validity, test schema, test categories, notes files) |

If no recognized top-level key is found and `--type` is not provided, the command prints the auto-detection failure message and exits.

### Type Override

When `--type` is provided:

1. Parse the JSON.
2. Check that the top-level key matches the expected key for the specified type. If not, print a mismatch error with a hint.
3. Run the validation for the specified type.

### Validation

The same validation functions used by `init`, phase shifts, and the planning VALIDATE state are invoked. No separate validation logic exists — `validate` is a standalone entry point into the existing validators.

### No Session Required

The `validate` command does not read or require `forgectl-state.json`. It does not require `.forgectl/` to exist. It operates purely on the input file.

### Path Resolution

For plan.json validation, relative paths in `refs[].path` and `items[].refs` are resolved relative to the directory containing the validated file — i.e., `<domain>/<workspace_dir>/implementation_plan/` (same behavior as during `init` and phase shifts). Paths in `items[].files` and `items[].specs` are resolved relative to the project root.

---

## Invariants

1. **Same validators.** `validate` uses the identical validation functions as `init`, phase shifts, and the planning VALIDATE state. No separate code paths.
2. **No session dependency.** The command works without an active session or `.forgectl/` directory.
3. **Auto-detection is deterministic.** The same file always maps to the same detected type.
4. **Type override is strict.** When `--type` is provided, the top-level key must match. No fallback to auto-detection.

---

## Edge Cases

- **Scenario:** File has multiple recognized top-level keys (e.g., both `"specs"` and `"plans"`).
  - **Expected:** Auto-detection uses the first recognized key found. `--type` override is recommended.
  - **Rationale:** Ambiguous files are likely malformed; the hint to use `--type` guides the user.

- **Scenario:** File is valid JSON but empty object `{}`.
  - **Expected:** Auto-detection failure (no recognized top-level key). Error with hint to use `--type`.
  - **Rationale:** Empty objects have no top-level key to match against.

- **Scenario:** plan.json with `refs[].path` pointing to non-existent file.
  - **Expected:** Validation error naming the path and noting it was resolved from the plan.json directory.
  - **Rationale:** Path resolution follows the same rules as runtime validation.

- **Scenario:** `--type plan` used on a spec-queue file.
  - **Expected:** Error: "--type plan expects top-level key "context", found "specs". Hint: did you mean --type spec-queue?"
  - **Rationale:** Strict type checking prevents miscategorized validation.

---

## Testing Criteria

### Auto-detect spec-queue
- **Verifies:** Auto-detection from "specs" key.
- **Given:** Valid spec-queue.json file.
- **When:** `forgectl validate spec-queue.json`
- **Then:** "Detected: spec-queue". Validation passes. Exit code 0.

### Auto-detect plan-queue
- **Verifies:** Auto-detection from "plans" key.
- **Given:** Valid plan-queue.json file.
- **When:** `forgectl validate plan-queue.json`
- **Then:** "Detected: plan-queue". Validation passes. Exit code 0.

### Auto-detect plan
- **Verifies:** Auto-detection from "context" key.
- **Given:** Valid plan.json file.
- **When:** `forgectl validate plan.json`
- **Then:** "Detected: plan". Validation passes. Exit code 0.

### Auto-detection failure
- **Verifies:** Unrecognized top-level key.
- **Given:** JSON file with top-level key "widgets".
- **When:** `forgectl validate widgets.json`
- **Then:** Error listing expected keys and found key. Hint to use --type. Exit code 1.

### Invalid JSON
- **Verifies:** Parse error with location.
- **Given:** File with invalid JSON.
- **When:** `forgectl validate broken.json`
- **Then:** Error with line/column. Exit code 1.

### Type override success
- **Verifies:** Explicit type bypasses auto-detection.
- **Given:** Valid plan.json file.
- **When:** `forgectl validate --type plan plan.json`
- **Then:** Validation passes. Exit code 0.

### Type override mismatch
- **Verifies:** Top-level key must match specified type.
- **Given:** plan-queue.json file (top-level key "plans").
- **When:** `forgectl validate --type spec-queue plan-queue.json`
- **Then:** Error: expects "specs", found "plans". Hint. Exit code 1.

### Validation errors reported
- **Verifies:** Schema violations enumerated.
- **Given:** spec-queue.json with duplicate names and missing field.
- **When:** `forgectl validate spec-queue.json`
- **Then:** Error listing each violation with index and field path. Exit code 1.

### No session required
- **Verifies:** Works without .forgectl/ directory.
- **Given:** No .forgectl/ in directory tree. Valid spec-queue.json.
- **When:** `forgectl validate spec-queue.json`
- **Then:** Validation passes. Exit code 0.

### Plan path resolution for refs
- **Verifies:** Relative paths in refs resolved from plan.json directory.
- **Given:** plan.json in `launcher/.forge_workspace/implementation_plan/` with `refs[0].path: "notes/auth.md"`. File exists at `launcher/.forge_workspace/implementation_plan/notes/auth.md`.
- **When:** `forgectl validate launcher/.forge_workspace/implementation_plan/plan.json`
- **Then:** Validation passes. Path resolved from plan.json directory.

---

## Implements
- Standalone JSON validation command for all forgectl input file types
- Auto-detection from top-level JSON keys with `--type` override
- Same validation logic as init, phase shifts, and planning VALIDATE state
- No session dependency
