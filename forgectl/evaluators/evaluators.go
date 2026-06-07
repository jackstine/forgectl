package evaluators

import _ "embed"

// SpecEval contains the spec evaluation prompt.
//
//go:embed spec-eval.md
var SpecEval string

// PlanEval contains the plan evaluation prompt.
//
//go:embed plan-eval.md
var PlanEval string

// ImplEval contains the implementation evaluation prompt.
//
//go:embed impl-eval.md
var ImplEval string

// ReconcileEval contains the reconciliation evaluation prompt.
//
//go:embed reconcile-eval.md
var ReconcileEval string

// CrossRefEval contains the cross-reference evaluation prompt.
//
//go:embed cross-reference-eval.md
var CrossRefEval string

// UIQAEval contains the UI QA evaluation prompt.
//
//go:embed ui-qa-eval.md
var UIQAEval string

// UIE2EEval contains the UI e2e verification prompt.
//
//go:embed ui-e2e-eval.md
var UIE2EEval string
