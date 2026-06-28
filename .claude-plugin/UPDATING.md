# Updating the Claude Code Plugin

This document explains what to update and where to look when the plugin's skills or commands change.

---

## Files to Update

```
.claude-plugin/
├── plugin.json       — Plugin manifest: version, metadata, explicit command paths
└── marketplace.json  — Marketplace entry: explicit skills list
```

---

## When to Update

| Change | Update |
|--------|--------|
| New skill added to `skills/` | `marketplace.json` → `skills` array |
| Skill removed | `marketplace.json` → `skills` array |
| New command file added in a subdirectory of `commands/` | `plugin.json` → `commands` array (if new subdirectory) |
| New command file added to an existing subdirectory | Nothing — auto-discovered |
| Any structural change | Bump `version` in `plugin.json` |

---

## Skills

Skills live in `skills/<name>/SKILL.md`. A directory is only a skill if it contains a `SKILL.md` file — shared resource directories (like `skills/shared/`) are not skills and must not be listed.

### Where to register: `marketplace.json`

The `skills` array in the plugin entry inside `marketplace.json` is the authoritative list. Add or remove entries here when skills change.

```json
"skills": [
  "./skills/frontend_specs",
  "./skills/implement_from_specs",
  "./skills/implementation",
  "./skills/implementation_planning",
  "./skills/planner",
  "./skills/reverse_engineering",
  "./skills/specs"
]
```

Paths are relative to the plugin root (the repo root, since `"source": "./"` in the plugin entry).

Skills are also auto-discovered by Claude Code from `skills/` — the explicit list is for marketplace discoverability.

---

## Commands

Commands live in `commands/<namespace>/<command-name>.md`. Claude Code auto-discovers flat `.md` files in `commands/` but does **not** recursively scan subdirectories by default.

### Where to register: `plugin.json`

The `commands` array in `plugin.json` tells Claude Code which subdirectories to scan. Each entry is a subdirectory path relative to the plugin root.

```json
"commands": [
  "./commands/meta",
  "./commands/q",
  "./commands/review"
]
```

- Adding a new command file to an **existing** subdirectory: no update needed.
- Adding a **new** subdirectory under `commands/`: add it to this array.

---

## Version

Bump `version` in `plugin.json` whenever skills or commands change. Follow semver:

- **patch** (0.1.0 → 0.1.1): command content edits, minor fixes
- **minor** (0.1.0 → 0.2.0): new skill or command added
- **major** (0.1.0 → 2.0.0): breaking structural change

---

## Current Inventory

### Skills (7)

| Skill | Path |
|-------|------|
| `frontend_specs` | `skills/frontend_specs/SKILL.md` |
| `implement_from_specs` | `skills/implement_from_specs/SKILL.md` |
| `implementation` | `skills/implementation/SKILL.md` |
| `implementation_planning` | `skills/implementation_planning/SKILL.md` |
| `planner` | `skills/planner/SKILL.md` |
| `reverse_engineering` | `skills/reverse_engineering/SKILL.md` |
| `specs` | `skills/specs/SKILL.md` |

### Commands (6)

| Command | Path |
|---------|------|
| `gen-prompt-from-discussion` | `commands/meta/gen-prompt-from-discussion.md` |
| `handoff` | `commands/meta/handoff.md` |
| `prompt-structure` | `commands/meta/prompt-structure.md` |
| `start-discussion-to-gen-prompt` | `commands/meta/start-discussion-to-gen-prompt.md` |
| `n_number_of_problems` | `commands/q/n_number_of_problems.md` |
| `review_with_fresh_eyes` | `commands/review/review_with_fresh_eyes.md` |
