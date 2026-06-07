# Subagent Usage — `ui_implementing` Phase

Guidelines for using sub-agents during UI implementation. This phase spawns sub-agents in **three** evaluator states — EVALUATE (code), QA_TEST (UI), and E2E_VERIFY (e2e) — each with its own evaluator prompt delivered by `forgectl eval`.

## The evaluator sub-agents

In each evaluator state, the sub-agent's lifecycle is the same:

1. Run `forgectl eval` to receive full context (evaluator prompt, items, paths, app/test config).
2. Do the work for that state.
3. Write its output files to the paths named in the eval context.
4. Run `forgectl handoff <file>…` to register those files.
5. Return its verdict (PASS/FAIL) and the report path to the engineer.

| State | Sub-agent does | Hands off |
|-------|----------------|-----------|
| EVALUATE | Reads implementation, specs, refs; writes a code eval report | `<impl-eval-report>` (report mode) |
| QA_TEST | Drives the running app via the **Playwright MCP**; judges placement/controls; writes the e2e step list | `<qa-report> <step-list>` (step list alone in conversational mode) |
| E2E_VERIFY | Confirms the authored Playwright tests pass and cover the step list; writes an e2e report | `<e2e-report>` (report mode) |

The QA sub-agent **must have the Playwright MCP available** — see [qa-playwright.md](qa-playwright.md). Provide the app URL and step-list path in its prompt (they also come through `forgectl eval`).

## When to use sub-agents

- **Codebase / component search:** before implementing, confirm the component or feature doesn't already exist.
- **Evaluation:** spawn one sub-agent per evaluator state (code, QA, e2e). Do not self-evaluate.
- **Updating documents:** use a sub-agent to update the implementation log or `{domain}/CLAUDE.md`, keeping the main agent focused.
- **Build/test:** use a single sub-agent for build/test operations to avoid conflicts.

## When NOT to use sub-agents

- Trivial single-file edits the main agent can do directly.
- Stateful interactive work (e.g. live debugging).

## Principles

- Sub-agents have no memory — provide complete context: the state, the resolved paths from forgectl, the app URL, and (for QA) the Playwright MCP expectation.
- One sub-agent per concern (search, evaluate, QA, e2e-verify, update docs) rather than one doing everything.
- The sub-agent's last act is `forgectl handoff` — without it, the engineer is not told which files to review.
