# reverse-engineering-init.json Schema

> Input file for initializing the **reverse engineering phase**.
> Passed via `forgectl init --phase reverse_engineering --from reverse-engineering-init.json`

The reverse engineering phase is the inverse of specifying: specs are derived
from existing code rather than code from specs. The init file scopes the effort
with a concept and an ordered list of domains to process.

---

## Root

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `concept` | string | **yes** | A general description of the work to be performed. Used to scope which areas of the codebase and which existing specs are relevant. |
| `domains` | string[] | **yes** | Ordered list of domains to process. Each domain runs SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE sequentially; order determines processing sequence. |

No other top-level fields are allowed.

---

## Validation Rules

- `concept` must be non-empty. Rejection: `"A concept is required to scope the reverse engineering effort."`
- `domains` must be non-empty. Rejection: `"At least one domain is required."`
- `domains` must contain no duplicates. Rejection identifies the duplicate.
- All field names and values are case-sensitive.

---

## Example

```json
{
  "concept": "auth middleware refactor",
  "domains": ["optimizer", "api", "portal"]
}
```

---

## Adding Domains After Init

Domains can be appended after init using `forgectl add-domain <domain>`, but
only during the **QUEUE** state. The new domain is appended to the end of the
domain order. The SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE loop does not
re-run for an added domain — the orchestrating agent is responsible for having
performed the analysis before adding queue entries for it.

- Duplicate domain → error.
- Called outside QUEUE → error: `"forgectl add-domain is only available during the QUEUE state."`

---

## Source

- Intended behavior: `forgectl/specs/reverse-engineering.md`
- (Type definitions and validation in `forgectl/state/` are added when the
  reverse engineering phase is implemented; this schema is derived from the
  spec until then.)
