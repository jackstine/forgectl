# First Session Walkthrough

A concrete, end-to-end first run for the most common newcomer path: **a brand-new
product, starting from planning docs, going specs → plan → code.** It shows the
*shape* of a session — the commands, the hand-offs, and the rhythm. The per-phase
skills own the details of the work inside each state.

The single habit that makes all of this easy: **after every command, read the
`Action:` line and do exactly what it says.** When lost, `forgectl status`.

---

## 0. One-time setup

```bash
make install-global          # build + install the binary
forgectl --version           # confirm
cd /path/to/your/product     # your project root
forgectl init                # scaffold .forgectl/ and a default config, then exit
```

Skim `.forgectl/config`. Defaults are fine for a first run. The two settings worth
a glance now: `general.user_guided` (leave `true` — it pauses for you) and
`general.enable_commits` (leave `false` — you commit manually while learning). See
[config-cheatsheet.md](config-cheatsheet.md).

---

## 1. Specifying — planning docs become specs

You need a `spec-queue.json`. **Do not hand-author it.** Invoke the `specs` skill;
its search step reads your planning docs and produces the queue. (No planning docs
yet? Use the `planner` skill first.)

```bash
forgectl init --phase specifying --from spec-queue.json
forgectl status              # phase: specifying, state: ORIENT
```

Now drive the loop. Per spec, forgectl walks you:

```
ORIENT → SELECT → DRAFT → EVALUATE ⇄ REFINE → ACCEPT → (next spec) → DONE
```

The work in each state is the `specs` skill's job. The forgectl rhythm is:

```bash
forgectl advance                         # ORIENT → SELECT (discuss scope if guided)
forgectl advance                         # SELECT → DRAFT
# ... you write the spec file ...
forgectl advance                         # DRAFT → EVALUATE   (--file <path> to override output)
# ... spawn a sub-agent: it runs `forgectl eval`, reads the draft, writes a report ...
forgectl advance --verdict PASS --eval-report .eval/spec-r1.md
# FAIL → REFINE → back to EVALUATE; PASS (>= min_rounds) → ACCEPT → next spec
```

After the last spec, forgectl runs cross-reference and reconciliation loops to make
the spec corpus internally consistent, then reaches the specifying `PHASE_SHIFT`.

---

## 2. Generate planning queue — specs become a plan queue

From the specifying `PHASE_SHIFT`, the default advance enters
`generate_planning_queue`, which **auto-builds** `plan-queue.json` (one entry per
domain) for you to review:

```bash
forgectl advance             # specifying PHASE_SHIFT → generate_planning_queue ORIENT
forgectl advance             # ORIENT (forgectl writes the queue) → REFINE
```

At `REFINE`, open `.forgectl/state/plan-queue.json` and adjust (the
`generate_planning_queue` skill explains each field):

- Set `kind: "ui"` on any domain that needs the QA + e2e loops (default `"code"`).
- Reorder domains, rename plans, fix spec grouping, add cross-domain `code_search_roots`.

```bash
forgectl advance             # REFINE (validates the queue) → PHASE_SHIFT → planning ORIENT
```

---

## 3. Planning — specs + code become plan.json

You are now at planning `ORIENT`, one plan-queue entry loaded. Invoke the
`implementation_planning` skill. The rhythm:

```
ORIENT → STUDY_SPECS → STUDY_CODE → STUDY_PACKAGES → REVIEW → DRAFT
       → VALIDATE → (SELF_REVIEW) → EVALUATE ⇄ REFINE → ACCEPT
```

```bash
forgectl advance             # walk the study states (each prints its Action:)
# ... you draft plan.json + notes/*.md ...
forgectl advance --verdict PASS --eval-report <path>   # at EVALUATE
```

On ACCEPT, a `PHASE_SHIFT` reads the plan's `kind` and routes:
`code → implementing`, `ui → ui_implementing`.

---

## 4. Implementing — plan.json becomes production code

At implementing `ORIENT`, invoke the `implementation` skill. The loop, per batch:

```
ORIENT → IMPLEMENT(item 1..n) → EVALUATE ⇄ IMPLEMENT(round 2+) → COMMIT → (next batch) → DONE
```

```bash
forgectl advance             # ORIENT → IMPLEMENT (batch selected)
# ... implement the item completely, run its tests ...
forgectl advance             # next item, or → EVALUATE   (--message only if enable_commits)
# ... spawn evaluator sub-agent at EVALUATE ...
forgectl advance --verdict PASS --eval-report <path>
# ... at COMMIT, commit your work, add an IMPLEMENTATION_LOG.md entry ...
forgectl advance             # COMMIT → ORIENT (more) or DONE
```

If the plan was `kind: "ui"`, you are in `ui_implementing` instead: same spine, with
two extra verification loops after the code eval — **QA** (`QA_TEST ⇄ UI_REFINE`,
driven through the Playwright MCP) and **e2e** (`E2E_AUTHOR → E2E_VERIFY ⇄
E2E_REMEDIATE`, authored Playwright tests). A batch passes only when all three loops
pass. Sub-agents register their artifacts with `forgectl handoff <file>`. Use the
`ui_implementation` skill.

When implementing reaches `DONE` with plans still in the queue, a `PHASE_SHIFT`
moves to the next domain's plan (in interleaved mode, back to planning).

---

## What to remember

- The commands are always the same three: `status` (where am I), `advance` (next),
  and `eval`/`handoff` (the sub-agent side at evaluation gates).
- You never pick the next item — forgectl does, via the `Action:` line.
- Each phase has a skill. Onboarding gets you to the session; the phase skill does
  the work inside it.
</content>
