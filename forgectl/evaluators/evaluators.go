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

// ReverseEngineeringReconcileEval contains the reverse engineering reconciliation evaluation prompt.
//
//go:embed reverse-engineering-reconcile-eval.md
var ReverseEngineeringReconcileEval string
