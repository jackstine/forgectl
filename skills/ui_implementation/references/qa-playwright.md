# QA via the Playwright MCP

In QA_TEST the sub-agent does not read source code to judge the UI — it **drives the running application** the way a user would, through the Playwright MCP, and judges what it observes. This reference describes how.

## What the QA sub-agent receives

From `forgectl eval` in QA_TEST: the QA evaluator prompt, the batch's UI items (with their specs), the **app launch command and URL**, the **step-list target path**, and (in `report` mode) the **report target path**. The app is expected to be reachable at `ui_implementing.app.url` within `ui_implementing.app.ready_timeout_seconds`.

## The driving loop

The **accessibility snapshot is the primary judgment surface** — it is structural and deterministic, exposing each control's role, label, and order without relying on pixels. Screenshots are supplementary evidence.

1. **Navigate** — `browser_navigate(url)` to the app URL.
   - If navigation fails or the page does not load, that is a **launch defect**: record FAIL with the failure as the finding, still write a (possibly empty) step list, and hand off.
2. **Snapshot** — `browser_snapshot()` to read the accessibility tree. Judge against the items' specs:
   - Are the expected controls present, labelled correctly, and in a sensible order?
   - Is placement correct (top bar, sidebar, content region, etc.)?
   - Each element carries a `ref` — use it to act in the next steps.
3. **Read the console** — `browser_console_messages(level: error)` to catch runtime errors a snapshot won't show (failed requests, thrown exceptions, hydration errors).
4. **Exercise controls** — drive each interactive control to confirm it is wired:
   - `browser_click(target: <ref>, element: "<description>")`
   - `browser_type(target: <ref>, element: "...", text: "...")`
   - `browser_fill_form(...)`, `browser_select_option(...)`, `browser_hover(...)`, `browser_press_key(...)`
   - Re-snapshot after an action to confirm the expected state change.
5. **Capture evidence** — `browser_take_screenshot(filename: "<qa-dir>/batch-N-round-M-<label>.png")` for placement evidence; reference these in the report.
6. **Assert when needed** — `browser_evaluate(function)` to read computed state the snapshot does not surface.

## Two outputs, always

### 1. The e2e step list (always written, every mode)
Derive the step list from the paths you **actually walked**. It is a transcript of verified user flows, not invented scenarios — this is what makes the e2e loop trustworthy. Write it to the step-list path as JSON:

```json
{
  "batch": 1,
  "round": 1,
  "scenarios": [
    {
      "id": "nav-to-detail",
      "name": "Navigate from list to detail",
      "preconditions": ["app loaded at /"],
      "steps": ["click the first row in the list", "wait for the detail panel"],
      "expected": ["detail panel shows the selected item's title", "URL is /items/:id"],
      "priority": "critical"
    }
  ]
}
```

Each scenario's `steps` should map cleanly to Playwright actions and each `expected` to a Playwright assertion, because E2E_AUTHOR turns them into Playwright test files. An empty `scenarios` array is valid when the batch has no end-to-end flow (e.g. pure theme tokens) — the e2e loop then passes vacuously.

### 2. The verdict (per `eval_mode`)
- `report` — write a QA report (findings, placement judgments, screenshot references, PASS/FAIL rationale) to the report path.
- `direct` — make the placement corrections directly in the UI; the engineer reviews the unstaged diff.
- `conversational` — relay the verdict and findings verbally.

## Hand off

After writing the step list and (in `report` mode) the report, register them:

```bash
forgectl handoff <qa-report> <step-list>     # report / direct modes
forgectl handoff <step-list>                  # conversational mode (no report file)
```

This is what makes forgectl tell the engineer to review the files you produced before they record the verdict.

## Required Playwright MCP tools

`browser_navigate`, `browser_snapshot`, `browser_click`, `browser_type`, `browser_fill_form`, `browser_select_option`, `browser_hover`, `browser_press_key`, `browser_console_messages`, `browser_take_screenshot`, `browser_evaluate`. Confirm the Playwright MCP is connected before starting; an absent MCP means QA cannot run.
