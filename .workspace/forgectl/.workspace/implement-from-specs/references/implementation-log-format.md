# Implementation Log — Format Reference

This file defines the format for `{domain}/.workspace/implement-from-specs/IMPLEMENTATION_LOG.md`. If that file does not exist, create it using this format.

---

## Log Entry Format

```markdown
### <Date or Session Identifier>
- **Errors:** <None | Yes — brief primary error message>
- **All Tests Pass:** <Yes | No>
- **Notes:** <1–2 short sentences, optional>
```

## Field Definitions

| Field | Description |
|-------|-------------|
| `Errors` | `None` if clean, otherwise the primary error message (brief) |
| `All Tests Pass` | `Yes` or `No` — reflects the state after the session's work |
| `Notes` | Optional context: what was done, what's next, or why something was skipped |

## Guidelines

- Add a new entry each time you complete a unit of work (implement, fix, validate).
- Entries are appended chronologically (newest last).
- Each entry is a snapshot of the state after that session's work.

## New Log File Template

When creating `{domain}/.workspace/implement-from-specs/IMPLEMENTATION_LOG.md`, use:

```markdown
# Implementation Log

Log of implementation updates across sessions.

---

## Entries
```
