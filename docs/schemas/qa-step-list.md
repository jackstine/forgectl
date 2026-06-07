# QA Step List Schema (`batch-N-steps.json`)

> Produced by the **QA_TEST** state and consumed by **E2E_AUTHOR** — the contract between the QA placement loop and the e2e loop.
> Written to: `<plan-dir>/qa/batch-<N>-steps.json`, where `<plan-dir>` is the directory containing the active plan.json (e.g. `portal/.forge_workspace/implementation_plan/qa/batch-1-steps.json`).

The QA sub-agent derives this list of end-to-end scenarios from exercising the batch's running UI through the Playwright MCP. The file is **rewritten on every QA_TEST round** (the latest round overwrites) and is **independent of `eval_mode`** — it is always produced, even in `conversational` mode, because the e2e loop depends on it.

The scaffold reads this file only to confirm its presence and to count the `scenarios` array at the QA → e2e boundary; it does not interpret scenario contents.

---

## Root

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `batch` | integer | **yes** | Batch number the scenarios belong to |
| `round` | integer | **yes** | The QA round that produced this list |
| `scenarios` | QAScenario[] | **yes** | The derived e2e scenarios. May be empty `[]` when the batch has no e2e-worthy flow (the e2e loop then short-circuits at its minimum rounds). |

---

## QAScenario

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `id` | string | **yes** | Stable identifier for the scenario, unique within the list |
| `name` | string | **yes** | Human-readable scenario title |
| `preconditions` | string[] | no | Application state assumed before the steps run |
| `steps` | string[] | **yes** | Ordered user actions to perform, one action per entry |
| `expected` | string[] | **yes** | Observable results that must hold after the steps |
| `priority` | string | no | One of `critical`, `normal`, `low`; absent means `normal` |

---

## Example

```json
{
  "batch": 1,
  "round": 1,
  "scenarios": [
    {
      "id": "shell-nav-toggle",
      "name": "Sidebar collapses and expands",
      "preconditions": ["App loaded at the dashboard route"],
      "steps": [
        "Click the sidebar toggle in the top bar",
        "Click the sidebar toggle again"
      ],
      "expected": [
        "After the first click the sidebar is collapsed",
        "After the second click the sidebar is expanded"
      ],
      "priority": "normal"
    }
  ]
}
```

An empty list is valid:

```json
{ "batch": 2, "round": 1, "scenarios": [] }
```

---

## Source

- Spec: `forgectl/specs/ui-batch-implementation.md` (§ Data Models — QA Step List / QA Scenario)
- Produced by: the QA_TEST sub-agent (driven by `evaluators/ui-qa-eval.md`), registered with `forgectl handoff`
- Consumed by: the E2E_AUTHOR state and the QA → e2e boundary scenario count in `forgectl/state/advance.go`
- Path construction: `qaStepListPath` in `forgectl/state/output.go` joins the active plan.json's directory (`currentPlanDir`) with `qa/batch-<N>-steps.json`
