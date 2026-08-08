# Notes — Derived Docs Sync

Per `CLAUDE.md`, `docs/diagrams/` and `docs/schemas/` are **derived** — they follow the specs and code. Source of truth: `forgectl/state/types.go`, `forgectl/state/validate.go`, `forgectl/specs/`.

## Already current — do not re-edit

- **`docs/auto-committing.md`** — rewritten ahead of the code. It already documents per-item commits every round (`:15-16`, `:59`), the inline batch-terminal commit (`:17-18`, `:60`), the message-synthesis algorithm (`:30-35`), and that COMMIT is skipped when `enable_commits: true` (`:26`). It is the **reference** for this work, not a target.
- **`docs/schemas/plan-queue.md`** — already documents the `kind` field (`:31`) with a `kind: "ui"` example (`:64-69`).

## Needs updating

### Diagrams (`docs/diagrams/`, plus rendered `docs/diagrams/html/`)

| File | Why |
| --- | --- |
| `03-implementing-phase.txt` | terminal EVALUATE now branches on `enable_commits`: inline commit → ORIENT/DONE, or COMMIT when commits are off. Direct-mode EVALUATE→EVALUATE self-loop. |
| `10-ui-implementing-phase.txt` | same two changes at the code-eval loop and terminal E2E_VERIFY. Note `:17` already *mentions* the direct-mode self-loop, so the diagram is ahead of the code here. |
| `06-state-machine-complete.txt` | unified view inherits both changes. |
| `07-evaluation-loop.txt` | depicts the eval loop and `eval_mode`; the direct-mode re-entry target changes. |
| `04-cli-commands.txt` | new `preflight` command in the command tree. |
| `00-full-lifecycle.txt` | the readiness gate as a precondition on cold-start entry into planning. |

The `html/` directory holds rendered counterparts (`implementing.html`, `implementing-schematic.html`, `ui-implementing.html`, `ui-implementing-schematic.html`, `cli-commands.html`, …) — the prior commit `1df6b74` updated `.txt` and `.html` together, so both forms move as a pair.

### Schemas (`docs/schemas/`)

`forge-state.md` — the largest schema doc, describes the state-file structure. Only touch it if this work adds or changes a state field. The gate is stateless and stores nothing, so the expected diff is small or empty; verify rather than assume.

### Other docs

- `docs/configurations.md` — config reference. No new config keys are introduced (the gate reads the existing `paths.workspace_dir`), but the `enable_commits` and `eval_mode` entries describe behavior that changes.
- `docs/default-config.toml` — the canonical commented config. **Embedded verbatim** via `//go:embed` into `state/default-config.toml` (`scaffold.go:15`), and `TestEmbeddedTemplateMatchesDocs` guards the two against drift. Edit one, edit both, or the test fails. No new keys are expected here.
- `docs/json-file-catalog.md` — lists JSON file types; the gate reads plan queues but introduces no new JSON artifact.

## Skills

`CLAUDE.md` permits a phase's skill to be drafted alongside its plan once implementation planning has begun. `skills/workspace_closeout/SKILL.md` already exists and is what the gate's remediation text directs operators to — the spec names it as the sole definition of the close-out procedure (`planning-readiness-gate.md:38`). Confirm the remediation wording the gate prints matches what that skill actually does; the scaffold only *queries and enforces* readiness — it never writes an archive and never deletes a workspace.
