# Notes: Evaluator Prompt Embedding

Backs the L0 evaluators-embed item. File: `forgectl/evaluators/evaluators.go`.

## Current pattern (evaluators.go, 29 lines)

`import _ "embed"` then one block per prompt: doc comment + `//go:embed <file>.md` + `var <Name> string`. Existing vars: `SpecEval`, `PlanEval`, `ImplEval`, `ReconcileEval`, `CrossRefEval`.

## What to add

The two prompt files already exist on disk (committed in `c18fcf7`) but are NOT embedded — `grep -rn "ui-qa-eval\|UIQAEval" forgectl/` returns nothing. Add after the existing vars:

```
// UIQAEval contains the UI QA evaluation prompt.
//
//go:embed ui-qa-eval.md
var UIQAEval string

// UIE2EEval contains the UI e2e verification prompt.
//
//go:embed ui-e2e-eval.md
var UIE2EEval string
```

Consumers: `state.PrintUIQAEvalOutput` / `state.PrintUIE2EEvalOutput` reference `evaluators.UIQAEval` / `evaluators.UIE2EEval` (notes/output.md).

## Verification / tests

The `evaluators` package has no test file. Coverage comes from `state/output_test.go` asserting the embedded prompt H1 appears in eval output (e.g. `UI QA Evaluation Prompt`, `UI E2E Verification Prompt`) — mirror the existing `TestEvalOutput*ContainsEvaluatorPrompt` tests (output_test.go:264, 316). A `go build` failure is the immediate signal if a `//go:embed` filename is wrong.
