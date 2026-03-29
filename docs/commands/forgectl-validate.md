# forgectl validate

Validate JSON files used in spec-driven development workflows without requiring an active session.

## Signature

```
forgectl validate [--type <type>] <file_path>
```

## Description

The `validate` command validates JSON files that forgectl consumes:
- **spec-queue.json** — queue of specifications to draft
- **plan-queue.json** — queue of implementation plans
- **plan.json** — implementation plan with context and items

No active session is required. The command does not read or modify `.forgectl/` state files.

## Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `<file_path>` | Yes | Path to the JSON file to validate (absolute or relative) |

## Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--type <type>` | string | auto-detected | Override auto-detection of file type. Valid values: `spec-queue`, `plan-queue`, `plan` |

## Auto-Detection

When `--type` is not provided, forgectl detects the file type by examining the top-level JSON key:

| Top-level key | Detected type | File |
|---|---|---|
| `"specs"` | spec-queue | spec-queue.json |
| `"plans"` | plan-queue | plan-queue.json |
| `"context"` | plan | plan.json |

If the file has an unexpected top-level key, validation fails with a list of expected keys.

## Validation Logic

The `validate` command uses the same validation logic as `init` and phase transitions:

### spec-queue validation

- JSON parse — valid JSON syntax
- Top-level field — exactly `"specs"` required
- Array format — `specs[]` is a non-empty array
- Entry schema — each spec has exactly 6 fields: `name`, `domain`, `topic`, `file`, `planning_sources`, `depends_on`
- Name uniqueness — no duplicate `name` values
- Dependency references — all `depends_on` values reference a `name` in another entry
- No circular dependencies in the `depends_on` graph

See: [schemas/spec-queue.md](../schemas/spec-queue.md)

### plan-queue validation

- JSON parse — valid JSON syntax
- Top-level field — exactly `"plans"` required
- Array format — `plans[]` is a non-empty array
- Entry schema — each plan has exactly 6 fields: `name`, `domain`, `file`, `specs`, `spec_commits`, `code_search_roots`
- No extra fields allowed

See: [schemas/plan-queue.md](../schemas/plan-queue.md)

### plan.json validation

12 validation checks:

1. JSON parse — valid JSON
2. Top-level fields — only `context`, `refs`, `layers`, `items`
3. Context fields — `domain` and `module` required, non-empty
4. Refs exist — every `refs[].path` resolves to an existing file
5. Item schema — every item has `id`, `name`, `description`, `depends_on`, `tests`
6. Item ID uniqueness — no duplicate IDs
7. Layer coverage — every item in exactly one layer; every layer item ID exists
8. Layer ordering — items only depend on items in equal or earlier layers
9. DAG validity — no cycles in `depends_on` graph
10. Test schema — every test has `category` and `description`; category is `functional`, `rejection`, or `edge_case`
11. Test categories — at least one test per item
12. Notes files — every `ref` in items resolves to an existing file

See: [schemas/plan-json.md](../schemas/plan-json.md)

## Exit Codes

| Code | Condition |
|------|-----------|
| `0` | Validation passed |
| `1` | Validation failed (schema error, auto-detection failure, type mismatch, or file I/O error) |

## Example Outputs

### Success

```
Detected: spec-queue (top-level key: "specs")
Validated: spec-queue.json — 3 entries, no errors.
```

### Validation Failure

```
Detected: spec-queue (top-level key: "specs")

Error: validation failed with 2 errors:
  1. specs[2].depends_on[0]: references "Snapshot Diffing" but no spec with that name exists
  2. specs[4]: missing required field "planning_sources"
```

### Auto-Detection Failure

```
Error: cannot detect file type.
  Expected one of these top-level keys:
    "specs"    → spec-queue (used in specifying phase)
    "plans"    → plan-queue (used in planning phase)
    "context"  → plan.json  (used in planning/implementing phases)
  Found: "metadata"
  Hint: use --type to specify the file type explicitly.
```

### Type Mismatch Error

```
Error: --type plan-queue expects top-level key "plans", found "specs".
  Hint: did you mean --type spec-queue?
```

## Usage Examples

### Validate with auto-detection

```bash
forgectl validate spec-queue.json
```

Output on success:
```
Detected: spec-queue (top-level key: "specs")
Validated: spec-queue.json — 3 entries, no errors.
```

### Validate with explicit type

```bash
forgectl validate --type plan-queue plans.json
```

### Validate plan with schema checks

```bash
forgectl validate implementation-plan.json
```

The command will validate all 12 checks, including:
- File references (refs must exist)
- Item dependencies (DAG check for cycles)
- Test coverage (at least one test per item)
- Layer constraints (items only depend on earlier layers)

### Validate before using with `init`

```bash
# Check the spec-queue before initializing
forgectl validate spec-queue.json
if [ $? -eq 0 ]; then
  forgectl init --from spec-queue.json --phase specifying
fi
```

## Session Requirements

- **Active session**: Not required
- **.forgectl/ directory**: Not required
- **Config file**: Not required

The `validate` command is independent of session state and can be run in any directory.

## Cross-References

- [json-file-catalog.md](../json-file-catalog.md) — complete catalog of JSON files
- [schemas/spec-queue.md](../schemas/spec-queue.md) — spec-queue schema details
- [schemas/plan-queue.md](../schemas/plan-queue.md) — plan-queue schema details
- [schemas/plan-json.md](../schemas/plan-json.md) — plan.json schema details
