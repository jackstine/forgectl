# Planning Readiness Gate

## Topic of Concern
> The scaffold refuses to start a planning cycle over any incoming domain whose workspace still holds prior-cycle artifacts.
> See: [topic-of-concern.md](topic-of-concern.md)

## Context

A domain's workspace directory (`<domain>/<workspace_dir>/`, where `workspace_dir` defaults to `.forge_workspace`) accumulates artifacts across a full lifecycle: the planning phase writes `implementation_plan/plan.json`, notes, and `implementation_plan/evals/`; the implementing and ui_implementing phases add `implementation/` logs and batch eval reports. These artifacts are correct and expected *during* a cycle.

The hazard appears when a project is reused for a second cycle. Starting a fresh plan for a domain whose workspace still contains the previous cycle's `plan.json`, evals, and implementation log silently mixes stale material into new planning: study steps read the old plan, evaluators compare against stale evals, and the resulting `plan.json` is written over — or beside — artifacts that were never reconciled or archived. Nothing signals that a prior body of work was abandoned in place.

The Planning Readiness Gate closes this gap. Before the planning phase begins, the scaffold requires the workspace of every domain the cycle is about to plan to be empty. When it is not, the scaffold refuses to enter planning and directs the operator to run the workspace close-out procedure, which archives the abandoned work to a durable ExecPlan-style document and then clears the workspace (see Integration Points → Workspace close-out procedure). Only once the workspace is empty does planning proceed.

The gate is **stateless**: its verdict depends only on the current filesystem and the list of domains in the plan queue handed to the cold-start of the planning cycle. It does not consult a marker file or a record of whether those domains were previously planned. Two facts make this sufficient. First, across sessions, a continuation plan queue carries only the domains still to be planned — domains already planned in an earlier session are absent from it and therefore never inspected. Second, within a session, the gate runs only at cold start (see the entry routes below), so domains completed later in the same cycle are never re-inspected. Either way, a completed domain's full workspace never trips the gate, and no "already-planned" exemption logic is required.

The gate guards **the start of a planning cycle** — the cold-start entry into the planning phase from outside it. There are exactly three cold-start routes, all gated identically against the plan queue that will drive planning:

1. `init --phase planning --from <plan-queue.json>`.
2. The generate_planning_queue→planning phase shift.
3. The specifying→generate_planning_queue `--from` skip, which bypasses generate_planning_queue and lands directly in planning ORIENT.

The planning phase's ORIENT state is also re-entered *within* a running cycle, once per domain, when planning advances from one domain to the next: the planning→planning domain boundary (plan-all-before-implementing mode) and the implementing→planning and ui_implementing→planning boundaries (interleaved mode). These intra-session re-entries are **not** gated. The cold-start gate already inspected every domain in the cycle's plan queue and found each clean; between then and a boundary, the scaffold writes only to the domain being planned, so sibling domains that legitimately now hold a finished `plan.json` or implementation log are the expected continuation, not stale cruft. Re-gating them would false-block exactly the multi-domain continuation the cold-start gate was designed to permit.

## Depends On
- **session-init** — `init --phase planning` is one cold-start entry into planning; the gate is evaluated there before the state file is created.
- **phase-transitions** — the generate_planning_queue→planning phase shift and the specifying→generate_planning_queue `--from` skip are the other two cold-start entries; the gate is evaluated before ORIENT is entered. phase-transitions also defines the intra-session planning-ORIENT re-entries the gate deliberately does not guard.
- **config-scaffolding** — establishes the project root and provides `paths.workspace_dir`, which names the per-domain workspace directory the gate inspects.

## Integration Points

| Adjacent concern | Relationship |
|------------------|-------------|
| session-init | `init --phase planning` evaluates the gate against the domains in the `--from` plan queue before creating the state file. A dirty workspace makes init fail; no state file is written. |
| phase-transitions | The two phase-shift cold-start entries (generate_planning_queue→planning, and the specifying→generate_planning_queue `--from` skip) evaluate the gate against the domains in the plan queue they hand to planning, before entering ORIENT. A dirty workspace blocks the shift. The intra-session planning-ORIENT re-entries phase-transitions also defines (planning→planning, implementing/ui_implementing→planning) are not gated. |
| plan-production | The planning phase's ORIENT state is entered only after the gate passes at cold start. Plan production assumes a clean workspace for every domain it plans. |
| config-scaffolding | Supplies `paths.workspace_dir` (default `.forge_workspace`) and the resolved project root against which each `<domain>/<workspace_dir>/` path is computed. |
| Workspace close-out procedure | The remediation the gate directs operators to on a blocked verdict: it archives each dirty domain's abandoned work to a durable document stored outside every workspace, clears the workspace, and confirms the result with `preflight`. It is carried out by the `workspace_closeout` agent skill; the procedure is defined there and nowhere else. The scaffold only queries readiness and enforces it — it never writes an archive and never deletes a workspace. |
| activity-logging | When logging is enabled and a session is active, the gate result is recorded as a log entry at the cold-start planning-entry point. |

---

## Data Models

### Domain Workspace Status
The per-domain result of a readiness evaluation. Produced for each domain in the incoming plan queue.

| Field | Type | Required | Nullable | Notes / Constraints |
|-------|------|----------|----------|---------------------|
| `domain` | string | yes | no | Domain name as it appears in the plan queue entry. |
| `workspace_path` | string | yes | no | Project-root-relative path `<domain-path>/<workspace_dir>/` that was inspected. |
| `clean` | boolean | yes | no | `true` when the workspace directory is absent or contains no regular files at any depth; `false` when it holds a file or could not be inspected. |
| `error` | string | no | yes | Set when the workspace exists but could not be traversed (permission or I/O error); carries the OS error detail. When set, `clean` is `false`. Null on a successful inspection. |

### Readiness Verdict
The aggregate result across all inspected domains.

| Field | Type | Required | Nullable | Notes / Constraints |
|-------|------|----------|----------|---------------------|
| `ready` | boolean | yes | no | `true` iff every Domain Workspace Status has `clean: true`. |
| `dirty_domains` | Domain Workspace Status[] | yes | no | The subset with `clean: false`; empty when `ready` is `true`. |

---

## Interface

### Inputs

#### CLI Command — `preflight`

| Command | Flags | Description |
|---------|-------|-------------|
| `preflight` | `--from <plan-queue.json>` (optional) | Report planning readiness for the domains in the given plan queue. When `--from` is omitted and an active session has a pending plan queue (produced by generate_planning_queue), that queue's domains are used instead. |

`preflight` is non-mutating: it writes no state file, creates no directories, and deletes nothing. It is the query the operator or planning agent runs as the first step of a planning cycle to decide whether close-out is required before advancing.

#### Enforcement at planning entry

The same evaluation runs, without a separate flag, at each of the three cold-start entries into the planning phase:

- `init --phase planning --from <plan-queue.json>` — the domains are the entries of the `--from` plan queue.
- The generate_planning_queue→planning phase shift — the domains are the entries of the plan queue that shift hands to planning (the generated queue, or a `--from` override).
- The specifying→generate_planning_queue `--from` skip that lands directly in planning ORIENT — the domains are the entries of the `--from` skip queue.

The evaluation does **not** run at the intra-session domain-boundary re-entries into planning ORIENT (planning→planning, implementing→planning, ui_implementing→planning); those are covered by the cold-start evaluation and are never re-gated. See Context and the "intra-session re-entry" edge case.

#### Domain source resolution

The set of domains to inspect is always "the domains the cycle is about to plan," drawn from the plan queue relevant to the entry point:

| Entry point | Domain source |
|-------------|---------------|
| `preflight --from <file>` | Plan queue at `<file>` |
| `preflight` (no `--from`) | Pending plan queue recorded in the active session's generate_planning_queue state |
| `init --phase planning --from <file>` | Plan queue at `<file>` |
| generate_planning_queue→planning phase shift | Plan queue that shift hands to planning (generated queue, or `--from` override) |
| specifying→generate_planning_queue `--from` skip | The `--from` skip queue |

For each domain, the inspected path is `<domain-path>/<workspace_dir>/`, where `<domain-path>` is the configured `[[domains]]` path for the domain name (or the domain name itself when domains are derived from file paths) and `<workspace_dir>` is `paths.workspace_dir` (default `.forge_workspace`).

### Outputs

#### `preflight` output

When ready:

```
Planning readiness: READY
Inspected 3 domain workspaces — all clean.
```

Exit code 0.

When not ready:

```
Planning readiness: BLOCKED
The following domain workspaces contain prior-cycle artifacts:
  - protocols   protocols/.forge_workspace/
  - launcher    launcher/.forge_workspace/
Run the workspace close-out procedure for each domain to archive and clear it,
then re-run preflight.
```

Exit code 1.

#### Enforcement output

At each of the three cold-start entries, a blocked verdict prints the same list of dirty domains and the same remediation instruction, then fails the operation. On a ready verdict, the entry proceeds and produces no gate-specific output beyond the normal init / phase-shift output.

### Rejection

| Condition | Signal | Rationale |
|-----------|--------|-----------|
| `init --phase planning` with one or more incoming domains whose workspace is non-empty | Error listing each dirty domain and its workspace path, plus the close-out remediation instruction. Exit code 1. No state file is created. | A planning cycle must not begin over unarchived prior-cycle artifacts. |
| generate_planning_queue→planning phase shift with one or more incoming domains whose workspace is non-empty | Error listing each dirty domain and its workspace path, plus the close-out remediation instruction. The phase shift does not occur; the session stays in generate_planning_queue. | Same hazard applies at this cold-start entry. |
| specifying→generate_planning_queue `--from` skip (landing directly in planning) with one or more incoming domains whose workspace is non-empty | Error listing each dirty domain and its workspace path, plus the close-out remediation instruction. The skip does not complete; the session stays at the specifying→generate_planning_queue shift. Exit code 1. | Same hazard applies at this cold-start entry. |
| `preflight --from` points to a file that cannot be read or is not a valid plan queue | Error with the path and the parse/validation detail. Exit code 1. | The gate cannot determine the domain set without a readable queue. |
| `preflight` with no `--from` and no active session or no pending plan queue | Error: "No plan queue to check. Pass --from <plan-queue.json> or run from a session with a generated plan queue." Exit code 1. | Readiness is meaningless without a set of domains to inspect. |

---

## Behavior

### Readiness Query

#### Preconditions
- A plan queue is resolvable from `--from` or from the active session's generate_planning_queue state.

#### Steps
1. Resolve the domain set from the plan queue (see Domain source resolution).
2. For each domain, compute `<domain-path>/<workspace_dir>/` and determine its cleanliness (see Workspace Cleanliness Determination).
3. Assemble a Readiness Verdict: `ready` is true iff every Domain Workspace Status is `clean`.
4. Print the verdict. On ready, exit 0. On blocked, print the dirty domains and the remediation instruction, exit 1.

#### Postconditions
- No files are created, modified, or deleted.
- Exit code reflects the verdict: 0 ready, 1 blocked (or 1 on an input error).

#### Error Handling
- Plan queue unreadable or invalid: error with detail, exit 1.
- No resolvable domain set: error instructing the caller to pass `--from`, exit 1.

---

### Enforcement at Planning Entry

#### Preconditions
- The operation is a cold-start entry into the planning phase: `init --phase planning`, the generate_planning_queue→planning phase shift, or the specifying→generate_planning_queue `--from` skip that lands directly in planning ORIENT.
- The operation is *not* an intra-session domain-boundary re-entry into planning ORIENT (planning→planning, implementing→planning, ui_implementing→planning); the gate does not run at those.

#### Steps
1. Resolve the incoming domain set from the relevant plan queue.
2. Evaluate readiness (identical logic to the Readiness Query).
3. If blocked: print the dirty domains and remediation instruction, abort the operation before any state mutation, and exit non-zero. For `init`, no state file is written. For a phase shift, no transition occurs and the session stays at its current phase/state.
4. If ready: proceed with the normal init / phase-shift behavior, entering the planning phase at ORIENT.

#### Postconditions
- A cold-start entry reaches planning ORIENT only when every incoming domain workspace is clean.
- On a blocked verdict, no state file is created (init) and no phase transition occurs (phase shift); the filesystem is otherwise unchanged.

#### Error Handling
- Any dirty incoming domain aborts the operation with the listed remediation; this is a rejection, not a crash.

---

### Workspace Cleanliness Determination

#### Preconditions
- A `<domain-path>/<workspace_dir>/` path has been computed for a domain.

#### Steps
1. If the workspace directory does not exist, the domain is clean.
2. If it exists, walk it recursively. If it contains no regular files at any depth (it is absent, empty, or holds only empty subdirectories), the domain is clean.
3. If it contains any regular file at any depth — including dotfiles and placeholder files — the domain is dirty.

#### Postconditions
- Each domain resolves to exactly one of clean or dirty; there is no third state.

#### Error Handling
- If the workspace directory exists but cannot be traversed (permission or I/O error), treat the evaluation as failed for that domain and report it with the OS error detail; the overall verdict is blocked so the ambiguity cannot silently pass the gate.

---

## Configuration

The gate reads the workspace directory name from configuration; it introduces no new configuration keys.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `paths.workspace_dir` | string | `.forge_workspace` | Name of the per-domain workspace directory the gate inspects, joined under each domain path. Defined by config-scaffolding. |

---

## Observability

### Logging
| Level | What is logged |
|-------|---------------|
| INFO | Planning readiness evaluated: the number of domains inspected and the verdict (ready / blocked). |
| ERROR | Planning entry blocked by the gate: the list of dirty domains and their workspace paths. |
| DEBUG | Per-domain: the inspected workspace path and whether it was found clean or dirty. |

`preflight` run outside any session (no logging context) emits its verdict to standard output only; no log file is written.

### Metrics
None.

---

## Invariants

1. **Clean cold-start guarantee.** A cold-start entry reaches the planning phase's ORIENT state only when every domain in that cycle's incoming plan queue has an empty workspace. No cold-start code path reaches planning ORIENT with a dirty incoming domain. (Intra-session domain-boundary re-entries are out of scope for this invariant; they are already covered by the cold-start check.)
2. **Query is read-only.** `preflight` never creates, modifies, or deletes any file or directory.
3. **Statelessness.** The verdict is a pure function of the incoming domain list and the current filesystem. It reads no session state beyond resolving the domain list and consults no record of whether those domains were previously planned.
4. **No partial entry.** A blocked verdict leaves the filesystem unchanged with respect to the operation: init writes no state file, and a phase shift performs no transition.
5. **Uniform cold-start enforcement.** For a given domain set and filesystem, the verdict is identical across all three cold-start entries and the `preflight` query; the same evaluation logic backs every one.

---

## Edge Cases

- **Scenario:** A new session's plan queue lists only the domains still to be planned (a cross-session continuation run); other domains in the project already have full workspaces from a previous session.
  - **Expected behavior:** The cold-start gate inspects only the queued domains and passes; the already-planned domains are never examined.
  - **Rationale:** Continuation queues carry only remaining domains. This is what makes the stateless emptiness check correct without an "already-planned" exemption.

- **Scenario:** Within one running multi-domain cycle, planning finishes domain A (its workspace now holds a full `plan.json`, and in interleaved mode an `implementation/` log) and re-enters planning ORIENT for domain B via a domain boundary (planning→planning, or implementing/ui_implementing→planning).
  - **Expected behavior:** The gate does not run at the boundary; the re-entry proceeds regardless of domain A's now-full workspace.
  - **Rationale:** The cold-start gate already inspected every domain in this cycle's queue and found each clean; the scaffold has since written only to the domain being planned. Domain A's full workspace is the expected result of the cycle, not stale cruft. Re-gating it would false-block the legitimate multi-domain continuation.

- **Scenario:** A domain in the incoming plan queue passes the cold-start inspection, and something outside the scaffold then writes into that domain's workspace before the cycle reaches it.
  - **Expected behavior:** The domain is planned over the added content. Nothing re-inspects it, and no error is raised.
  - **Rationale:** The guarantee is that every queued domain was clean *at cold start*, not that each is clean at the instant it is planned. The gate inspects the whole queue once per cycle; from then on the only writes it anticipates are the scaffold's own, into the domain currently being worked. Re-inspecting at each domain boundary would require distinguishing the cycle's own output from a foreign write, which a stateless emptiness check cannot do — it would instead false-block the multi-domain continuation. An operator who suspects a workspace changed mid-cycle re-runs `preflight` against the remaining domains; the query is available at any time and mutates nothing.

- **Scenario:** A domain's workspace directory does not exist at all.
  - **Expected behavior:** The domain is clean.
  - **Rationale:** Absence is the strongest form of empty; the phase will create the directory when it needs it.

- **Scenario:** A domain's workspace directory exists but contains only empty subdirectories (e.g., a leftover `implementation_plan/` with nothing inside).
  - **Expected behavior:** The domain is clean.
  - **Rationale:** The gate's concern is retained *content*, not directory scaffolding. No artifact means no abandoned work.

- **Scenario:** A domain's workspace contains only a placeholder file such as `.gitkeep`.
  - **Expected behavior:** The domain is dirty.
  - **Rationale:** The rule is strict — any regular file counts as content. A workspace meant to be treated as empty must contain no files. This avoids a loophole where arbitrary content is disguised as a placeholder.

- **Scenario:** The workspace was cleared by hand (or by any means other than the close-out procedure), so it is empty but no ExecPlan archive was written.
  - **Expected behavior:** The gate passes.
  - **Rationale:** The gate guarantees the workspace is clean, not that prior work was archived. Statelessness is a deliberate trade: the archival discipline lives in the close-out procedure, and the gate cannot and does not verify it. This limitation is intentional and documented so operators do not rely on the gate to enforce archival.

- **Scenario:** A domain name contains a path separator (e.g., `protocols/ws1`).
  - **Expected behavior:** The gate resolves the workspace to `protocols/ws1/<workspace_dir>/` and inspects that directory.
  - **Rationale:** A nested domain path must resolve to a real directory for the gate to inspect it.

- **Scenario:** A dirty workspace exists for a domain that is *not* in the incoming plan queue.
  - **Expected behavior:** It is ignored; the gate does not fail on it.
  - **Rationale:** The gate guards only the domains the current cycle will plan. Unrelated domains are out of scope for this entry.

---

## Testing Criteria

### preflight reports ready over clean workspaces
- **Verifies:** Readiness Query on an all-clean domain set.
- **Given:** A plan queue with domains `a` and `b`, neither of which has a `<workspace_dir>/` containing files.
- **When:** `forgectl preflight --from plan-queue.json`
- **Then:** Output reports READY; exit code 0; no files created.

### preflight reports blocked and names dirty domains
- **Verifies:** Readiness Query aggregation and output.
- **Given:** A plan queue with domains `a` and `b`; `b/.forge_workspace/implementation_plan/plan.json` exists.
- **When:** `forgectl preflight --from plan-queue.json`
- **Then:** Output reports BLOCKED, lists domain `b` with path `b/.forge_workspace/`, includes the close-out remediation instruction; exit code 1.

### init --phase planning is blocked by a dirty incoming workspace
- **Verifies:** Enforcement at the init entry point; no partial entry.
- **Given:** A plan queue whose domain `a` has a non-empty `a/.forge_workspace/`.
- **When:** `forgectl init --phase planning --from plan-queue.json`
- **Then:** Exit code 1 naming domain `a`; no state file is created.

### gpq→planning phase shift is blocked by a dirty incoming workspace
- **Verifies:** Enforcement at the generate_planning_queue→planning cold-start entry.
- **Given:** An active session in generate_planning_queue whose generated plan queue includes a domain with a non-empty workspace.
- **When:** The operator advances to shift into planning.
- **Then:** The shift does not occur; the session remains in generate_planning_queue; the dirty domain is named with the close-out instruction.

### specifying→gpq `--from` skip into planning is blocked by a dirty incoming workspace
- **Verifies:** Enforcement at the specifying→generate_planning_queue `--from` skip cold-start entry.
- **Given:** An active session at the specifying→generate_planning_queue shift; the operator supplies `--from` a plan queue whose domain `a` has a non-empty `a/.forge_workspace/`.
- **When:** The operator advances with the skip `--from`.
- **Then:** The skip does not complete; the session stays at the specifying→generate_planning_queue shift; domain `a` is named with the close-out instruction; exit code 1.

### intra-session domain-boundary re-entry is not gated
- **Verifies:** The intra-session re-entry exemption; no false-block of the multi-domain continuation.
- **Given:** A running cycle whose queue was clean at cold start; planning has completed domain `a` (whose `a/.forge_workspace/` now holds a full `plan.json`) and advances across a domain boundary to plan domain `b`.
- **When:** The boundary advance runs (planning→planning, or implementing/ui_implementing→planning).
- **Then:** Planning ORIENT for `b` is entered; the gate does not run; domain `a`'s full workspace does not block the re-entry.

### a mid-cycle write into a not-yet-planned domain does not block
- **Verifies:** The cold-start-only scope of the clean guarantee; the documented limitation.
- **Given:** A running cycle whose queue `[a, b]` was clean at cold start; while `a` is being planned, a file is written into `b/.forge_workspace/` by something other than the scaffold.
- **When:** The domain boundary advance to plan `b` runs.
- **Then:** Planning ORIENT for `b` is entered; no gate error is printed; the added file does not block the re-entry.

### planning entry succeeds once workspaces are clean
- **Verifies:** Clean cold-start guarantee end to end.
- **Given:** A previously-dirty domain whose entire `<workspace_dir>/` has been removed.
- **When:** `forgectl init --phase planning --from plan-queue.json` (or an equivalent cold-start phase shift).
- **Then:** Planning is entered at ORIENT; no gate error is printed.

### empty subdirectories count as clean
- **Verifies:** Workspace Cleanliness Determination step 2.
- **Given:** A domain whose `<workspace_dir>/implementation_plan/` exists but contains no files.
- **When:** `forgectl preflight --from plan-queue.json`
- **Then:** The domain is reported clean; verdict READY.

### a placeholder file counts as dirty
- **Verifies:** Workspace Cleanliness Determination step 3; strict-content rule.
- **Given:** A domain whose `<workspace_dir>/` contains only `.gitkeep`.
- **When:** `forgectl preflight --from plan-queue.json`
- **Then:** The domain is reported dirty; verdict BLOCKED.

### continuation queue ignores already-planned domains
- **Verifies:** Statelessness and the continuation edge case.
- **Given:** Domain `a` has a full workspace; the plan queue lists only domain `b`, whose workspace is empty.
- **When:** `forgectl preflight --from plan-queue.json`
- **Then:** Verdict READY; domain `a` is never inspected.

### preflight is read-only
- **Verifies:** Query-is-read-only invariant.
- **Given:** Any plan queue and filesystem.
- **When:** `forgectl preflight --from plan-queue.json`
- **Then:** No file or directory is created, modified, or deleted by the command.

### unreadable plan queue is rejected
- **Verifies:** Input error handling.
- **Given:** `--from` points to a missing or malformed plan queue file.
- **When:** `forgectl preflight --from bad.json`
- **Then:** Exit code 1 with the path and parse/validation detail.

---

## Implements
- The planning readiness precondition: planning is entered only over clean per-domain workspaces
- The non-mutating `preflight` readiness query command
- Enforcement of the gate at every planning entry point (`init --phase planning`, the generate_planning_queue→planning phase shift, and the specifying→generate_planning_queue `--from` skip)
- Stateless, continuation-safe emptiness evaluation scoped to the incoming plan queue's domains
- The pointer to the workspace close-out procedure as the remediation for a blocked verdict
