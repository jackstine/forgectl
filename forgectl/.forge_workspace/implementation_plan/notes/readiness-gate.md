# Notes — Planning Readiness Gate (new subsystem)

Reference for `forgectl/specs/planning-readiness-gate.md`. Nothing in this note describes existing code unless stated — the gate is entirely absent today.

## Confirmed absences

- No `preflight` command. `grep -rn "preflight" forgectl/` matches **only** the spec file.
- No workspace inspection. `Paths.WorkspaceDir` (`types.go:204`, TOML key at `config.go:155`, default `.forge_workspace` at `types.go:377`) is read into config and then never used to build or walk a path. The one place a workspace path is constructed is `advance.go:1601`, and it **hardcodes the literal** `"/.forge_workspace/implementation_plan/plan.json"` rather than consulting `cfg.Paths.WorkspaceDir`.
- No gate hook in `init`.

## Domain → workspace path resolution

`DomainConfig` (`types.go:196`) has `Name` and `Path`; the list lives at `ForgeConfig.Domains` (`types.go:225`), populated from `[[domains]]` by `config.go:252-256`. Domains are **optional** — this project's own `.forgectl/config` declares none.

The workspace path for a domain is `<domain-path>/<workspace_dir>/`, where `<domain-path>` is the configured `[[domains]]` path for that name, and falls back to the domain name itself when no domain is configured (the convention the rest of the codebase already relies on — see `advance.go:1601`, which concatenates the bare domain name).

`ValidateConfig` already guarantees no domain path is a prefix of another (`config.go:603-615`), so a domain never resolves to a workspace nested inside another domain's workspace.

A domain name may itself contain a path separator (spec §Edge Cases: `protocols/ws1`). It resolves to `protocols/ws1/<workspace_dir>/` and that nested directory is the one inspected — so the resolution must join path segments rather than treat the name as a single flat directory component.

## Cleanliness rule (spec §Workspace Cleanliness)

Exactly two outcomes, no third:

- **Clean** — the directory does not exist, **or** it exists and contains no regular files at any depth. A tree of empty subdirectories is clean.
- **Dirty** — any regular file at any depth, *including dotfiles and placeholders* (`.gitkeep` counts as dirty). Also dirty when the directory exists but cannot be traversed; the OS error detail is retained.

`filepath.WalkDir` is the natural fit — it reports directory-read errors through the walk callback, which is what lets a permission failure surface as dirty-with-error rather than as a panic or a false clean. Early-exit with `filepath.SkipAll` on the first regular file found. Symlinks are not followed by `WalkDir`, and a symlink is not a regular file (`d.Type().IsRegular()` is false), so a dangling symlink cannot fabricate an error.

## Data model (spec §Data Models)

```
Domain Workspace Status:
  domain          string        // name from the plan queue entry
  workspace_path  string        // project-root-relative <domain>/<workspace_dir>/
  clean           bool
  error           string|null   // set when it exists but cannot be traversed
```

The verdict is `ready: true` iff every status has `clean: true`.

## Gated vs. not gated

**Gated — the three cold-start entries:**

1. `init --phase planning --from <plan-queue.json>` — domains from the `--from` queue.
2. The `generate_planning_queue → planning` phase shift — domains from the generated queue, or from a `--from` override.
3. The `specifying → generate_planning_queue` shift when `--from` is supplied, which skips generate_planning_queue and lands directly in planning ORIENT.

**Deliberately NOT gated — intra-session domain-boundary re-entries into planning ORIENT:**

- `planning → planning` (`advance.go:1463`)
- `implementing → planning` and `ui_implementing → planning` (`advance.go:1509`)

The spec's rationale (§Edge Cases): the cold start already inspected every domain in the cycle's queue and found each clean; between then and the boundary the scaffold writes only to the domain being planned. Sibling domains now holding a finished `plan.json` are expected continuation, not stale cruft. Re-gating would false-block every multi-domain run. **Gating these is the primary regression risk of this work.**

## Where the three enforcement points live

| Entry | Location | Notes |
| --- | --- | --- |
| `init --phase planning` | `cmd/init.go:143-169` | The plan queue is parsed into `state.PlanQueueInput` at line 150ish; the gate runs after that parse and **before** any state is written. `init` saves state at `cmd/init.go:286`, so aborting anywhere before that leaves no state file. |
| generate_planning_queue → planning | `advance.go:1357` | Reads `s.GeneratePlanningQueue.PlanQueueFile`, honours a `--from` override, validates, then populates the queue. |
| specifying → generate_planning_queue `--from` skip | `advance.go:1325-1355` | The `--from` branch is what lands directly in planning. |

On a blocked verdict the operation aborts **before any mutation**: `init` writes no state file; a phase shift performs no transition and the session stays at its pre-planning phase. Exit code 1.

## Output format (spec §`preflight` output — reproduce exactly)

Ready:

```
Planning readiness: READY
Inspected 3 domain workspaces — all clean.
```

Exit 0.

Blocked:

```
Planning readiness: BLOCKED
The following domain workspaces contain prior-cycle artifacts:
  - protocols   protocols/.forge_workspace/
  - launcher    launcher/.forge_workspace/
Run the workspace close-out procedure for each domain to archive and clear it,
then re-run preflight.
```

Exit 1. The enforcement points print the same dirty-domain list and remediation.

## `preflight` command shape

`preflight [--from <plan-queue.json>]`. Non-mutating: writes no state file, creates no directories, deletes nothing.

Queue resolution:

| Invocation | Source |
| --- | --- |
| `preflight --from <file>` | the plan queue at `<file>` |
| `preflight` (no `--from`) | the pending plan queue recorded in the active session's generate_planning_queue state |

`GeneratePlanningQueueState.PlanQueueFile` (`types.go:476`) holds that path — it is set at `advance.go:1351` as a project-root-relative path.

Rejections:

- `--from` unreadable or not a valid plan queue → error with the path and the parse/validation detail, exit 1. `ValidatePlanQueue(data []byte) []string` (`validate.go:68`) is the existing validator and already checks the `kind` field (`validate.go:118-127`).
- No `--from`, and no active session or no pending plan queue → exact message:
  `No plan queue to check. Pass --from <plan-queue.json> or run from a session with a generated plan queue.`
  Exit 1.

## Command wiring pattern

Follow `cmd/validate.go`: package-level flag var, a `&cobra.Command{Use, Short, Args, RunE}`, and an `init()` that registers flags and calls `rootCmd.AddCommand`. `resolveSession()` (`cmd/root.go:33`) returns `(projectRoot, stateDir, cfg, err)` and is the standard way to reach config — needed here for `cfg.Paths.WorkspaceDir` and `cfg.Domains`. `state.Exists(stateDir)` / `state.Load(stateDir)` gate the no-`--from` path.

Write user-facing output to `cmd.OutOrStdout()` — the cmd tests capture it that way. Errors returned from `RunE` reach stderr with exit 1 via `Execute()` (`cmd/root.go:22-27`), which is what the K4 integration invariant asserts.

Note `preflight` must **not** require an existing session when `--from` is given — the operator runs it before `init`. It does still need a project root to resolve `workspace_dir`; `state.Scaffold(cwd)` (`scaffold.go:44`) is the non-destructive way `init` obtains one, and reusing it keeps `preflight` working in a project that has never been initialized.

## Logging (spec §Observability — reproduce exactly)

Three levels, all at the cold-start planning-entry points only (never at the intra-session domain-boundary re-entries, since the gate itself does not run there):

| Level | Content |
| --- | --- |
| INFO | Planning readiness evaluated: the number of domains inspected and the verdict (ready/blocked). |
| ERROR | Planning entry blocked by the gate: the list of dirty domains and their workspace paths. Emitted in addition to the INFO entry, only when blocked. |
| DEBUG | Per-domain: the inspected workspace path and whether it was found clean or dirty. One entry per inspected domain. |

Use the existing logging API — `state.Logger`, built with `NewLogger(cfg LogsConfig, phase PhaseName, sessionID string) *Logger` (`forgectl/state/logger.go:33`), and `Write(LogEntry)` (`logger.go:57`). `LogEntry` carries `TS`, `Cmd`, `Phase`, `PrevState`, `State`, and a free-form `Detail map[string]interface{}` — the domain count, verdict, dirty-domain list, and per-domain path/clean-dirty result all belong in `Detail`, following the shape `buildAdvanceDetail` already uses for advance's own log entries.

Follow the wiring pattern at `cmd/advance.go:86-124`: construct the logger with `state.NewLogger(s.Config.Logs, s.StartedAtPhase, s.SessionID)` (or the equivalent phase/session values available at each entry point — `init` and the phase-shift advances each have their own config and session context) and call `.Write(...)` with a populated `LogEntry`. No new logging mechanism, file, or sink is introduced — this is the same JSONL log every other command writes to.

The important detail that makes "`preflight` run outside any session emits its verdict to stdout only, no log file written" fall out naturally rather than needing a special case: `NewLogger` returns a no-op `Logger{enabled: false}` whenever `sessionID == ""` (`logger.go:34`). `preflight`'s `--from` path has no session at all, and its no-`--from` path requires an active session to resolve a pending plan queue in the first place — so in the one case where `preflight` really does run session-less, `sessionID` is empty, `NewLogger` returns a disabled logger, and every `Write` call is silently skipped. No conditional logic is needed in the gate or in `preflight` to suppress logging outside a session — it is a direct consequence of reusing the existing no-op behavior.
