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
| `domain` | string | **yes** | Domain grouping (typically a top-level project directory). Must match a configured domain name when `[[domains]]` is configured in `.forgectl/config`. |
| `topic` | string | **yes** | One-sentence topic of concern. Describes the single responsibility this spec addresses. |
| `file` | string | **yes** | Target file path relative to project root. Must follow the convention `<domain>/specs/<kebab-name>.md`. When domains are configured, the path must start with the domain's configured `path` + `/specs/`. |
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
- When `[[domains]]` is configured: each entry's `domain` must match a configured domain name. Unconfigured domains are rejected.
- When `[[domains]]` is configured: each entry's `file` path must start with the domain's configured `path` + `/specs/`.
- When no `[[domains]]` section exists: domain resolution falls back to deriving from file paths.

---

## Domain Validation

The optional `[[domains]]` section in `.forgectl/config` declares known domains:

```toml
[[domains]]
name = "launcher"
path = "launcher"

[[domains]]
name = "protocols"
path = "protocols"
```

When domains are configured:

- Every spec queue entry's `domain` field must match a configured `name`.
- The spec's `file` path must start with the matching domain's `path` + `/specs/`. For example, domain `launcher` with path `launcher` requires `file` to start with `launcher/specs/`.
- Domain paths must not be nested (e.g., `domains/users` and `domains/users/employees` is rejected at init time with: `Domain paths must not be nested: <path1> is a prefix of <path2>.`).

When domains are not configured: domain resolution is derived from file paths, and no domain validation is applied.

---

## Dynamic Queue Management

During the specifying phase, specs can be added to the queue dynamically using the `forgectl add-queue-item` command. This command is valid in the following states:

- **DRAFT** — Domain is inferred from the current domain
- **CROSS_REFERENCE_REVIEW** — Domain is inferred from the current domain
- **DONE** — `--domain` flag is required
- **RECONCILE_REVIEW** — Domain is inferred from the current domain

### add-queue-item Command Flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--name` | string | **yes** | Display name for the spec. Must be unique. |
| `--domain` | string | **yes** (DONE only) | Domain grouping. Inferred from current domain at DRAFT, CROSS_REFERENCE_REVIEW, and RECONCILE_REVIEW states. |
| `--topic` | string | **yes** | One-sentence topic of concern describing the single responsibility this spec addresses. |
| `--file` | string | **yes** | Target file path relative to project root. Convention: `<domain>/specs/<kebab-name>.md`. When domains are configured, must start with the domain's `path` + `/specs/`. |
| `--source` | string[] | optional (repeatable) | Paths to planning documents this spec is derived from, relative to project root. Can be specified multiple times. |

### Domain Resolution at add-queue-item

When `--domain` is not required (DRAFT, CROSS_REFERENCE_REVIEW, RECONCILE_REVIEW), the domain is inferred from the currently active spec batch's domain. `--domain` can be provided as an optional override at these states.

When domains are configured in `.forgectl/config`, the provided or inferred domain must match a configured domain name, and the `--file` path must start with the matching domain's `path` + `/specs/`.

---

## Dynamic Queue Management

During the specifying phase, specs can be added to the queue dynamically using the `forgectl add-queue-item` command. This command is valid in the following states:

- **DRAFT** — Domain is inferred from the current domain
- **CROSS_REFERENCE_REVIEW** — Domain is inferred from the current domain
- **DONE** — `--domain` flag is required

### add-queue-item Command Flags

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--name` | string | **yes** | Display name for the spec. Must be unique. |
| `--domain` | string | **yes** (DONE only) | Domain grouping. Inferred from current domain at DRAFT and CROSS_REFERENCE_REVIEW states. |
| `--topic` | string | **yes** | One-sentence topic of concern describing the single responsibility this spec addresses. |
| `--file` | string | **yes** | Target file path relative to project root. Convention: `<domain>/specs/<kebab-name>.md` |
| `--source` | string[] | optional (repeatable) | Paths to planning documents this spec is derived from, relative to project root. Can be specified multiple times. |

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
        ".forge_workspace/planning/launcher/config-loading.md"
      ],
      "depends_on": []
    },
    {
      "name": "Launching System Processes",
      "domain": "launcher",
      "topic": "The launcher spawns and health-checks detached system processes",
      "file": "launcher/specs/launching-system-processes.md",
      "planning_sources": [
        ".forge_workspace/planning/launcher/process-lifecycle.md"
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
