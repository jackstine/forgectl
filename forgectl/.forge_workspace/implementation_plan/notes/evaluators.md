# Notes: Evaluators Package

## Embedding pattern

All evaluator prompts are in `evaluators/evaluators.go` using `//go:embed`:

```go
package evaluators

import _ "embed"

//go:embed spec-eval.md
var SpecEval string

//go:embed plan-eval.md
var PlanEval string

// ... etc
```

To add `GauntletEval`, add a new `.md` file and a new embedded var in the same file:

```go
//go:embed gauntlet-eval.md
var GauntletEval string
```

The variable is then accessible as `evaluators.GauntletEval` from the `cmd` package.

## Existing evaluators

| Variable | File | Purpose |
|---|---|---|
| `SpecEval` | spec-eval.md | Spec quality evaluation |
| `PlanEval` | plan-eval.md | Plan evaluation (11 dimensions) |
| `ImplEval` | impl-eval.md | Implementation evaluation |
| `ReconcileEval` | reconcile-eval.md | Spec reconciliation |
| `CrossRefEval` | cross-reference-eval.md | Cross-reference check |
| `UIQAEval` | ui-qa-eval.md | UI QA evaluation |
| `UIE2EEval` | ui-e2e-eval.md | UI end-to-end evaluation |

## What gauntlet-eval.md must contain

The gauntlet adversarial evaluator prompt instructs a code-review agent to:

1. **Be adversarial** — assume the primary implementation has deficiencies. Do not merely report them; correct them directly by editing code files.
2. **Focus on the batch's test criteria** — each item's `tests` field defines acceptance criteria. The evaluator must verify every test criterion and fix code that fails any of them.
3. **Mutate directly** — the evaluator's output is a modified working tree, not a report. It reads code, identifies problems, and applies edits.
4. **Whole batch** — the evaluator always receives all items in the batch and all their context (description, steps, files, specs, tests). It must consider the full batch's requirements.
5. **No verdict word** — the gauntlet uses git (change detection), not a verdict. The evaluator does not emit PASS/FAIL. It either changes code or doesn't.
6. **Correct, don't comment** — fix the code, not add comments about what's wrong.

The prompt content should be terse instructions suitable for inclusion in a baked agent prompt. It works as the system instruction for an agent whose job is to adversarially review and mutate a batch of implementation work.
