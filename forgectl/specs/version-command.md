# Version Command

## Topic of Concern
> The `--version` flag reports the current release version of forgectl.

## Context

The `--version` flag is a standard CLI convention for reporting software version information. forgectl implements this via Cobra's built-in `Version` field on the root command, which automatically handles `forgectl --version` requests without requiring explicit state initialization or file access.

The version string is maintained as a package-level variable in the root command module and bumped during releases. This provides a single source of truth for the current release version.

## Depends On
- None. Version command is a standalone utility with no state dependencies.

## Integration Points

| Scope | Relationship |
|-------|-------------|
| Cobra framework | Uses Cobra's built-in `Version` field and auto-generated help |
| Root command | Version string sourced from `cmd/root.go` package variable |
| Release process | Version variable updated during release tagging |

---

## Interface

### Inputs

#### CLI Command

| Command | Flags | Description |
|---------|-------|-------------|
| `forgectl --version` | none | Display the current forgectl version |

Also accessible via the long form `forgectl version` when built with Cobra's version subcommand behavior (automatic).

### Outputs

#### `--version` output

```
forgectl version v0.0.1
```

The output is a single line in the format: `forgectl version <version_string>`

---

## Behavior

### Version Reporting

When a user runs `forgectl --version`:

1. Cobra intercepts the flag before command execution.
2. The `Version` field on the root command is printed.
3. Program exits with code 0.

No state file is required or consulted. This command works independently of session state.

### Version String Source

The version string is defined as a package-level variable in `cmd/root.go`:

```go
var version = "v0.0.1"
```

This variable is assigned to the root command's `Version` field during initialization.

---

## Invariants

1. **No state dependency.** The version command does not read or write `forgectl-state.json`.
2. **Always available.** Version flag is available before and after session initialization.
3. **Single source of truth.** Version string is centralized in `cmd/root.go`.

---

## Edge Cases

- **Scenario:** User runs `--version` with other flags or commands.
  - **Expected:** Cobra processes `--version` first; other flags/commands are ignored.
  - **Rationale:** Version is a global flag with highest priority by default in Cobra.

- **Scenario:** User runs `forgectl version` (subcommand form, if enabled).
  - **Expected:** Same output as `--version`.
  - **Rationale:** Cobra optionally generates a `version` subcommand when `Version` is set.

---

## Testing Criteria

### Version flag outputs current version
- **Verifies:** Basic version reporting.
- **When:** `forgectl --version`
- **Then:** Output contains "v0.0.1" and exit code 0.

### Version flag works without state file
- **Verifies:** No state dependency.
- **Given:** No `forgectl-state.json` in directory.
- **When:** `forgectl --version`
- **Then:** Output succeeds with exit code 0.

### Version flag works with --dir flag
- **Verifies:** Version is independent of state location.
- **Given:** State file in subdirectory.
- **When:** `forgectl --dir subdir --version`
- **Then:** Output succeeds with exit code 0.

---

## Implements
- Version flag reporting (standard CLI convention)
- Release version tracking via package variable
