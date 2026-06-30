//go:build integration

package integration

import (
	"strings"
	"testing"

	"forgectl/state"
)

// This file is plan §4.A — lifecycle coverage for the ui_implementing phase.
// It drives one plan item through every state in the machine: the happy path
// (all three loops PASS), the UI_REFINE sub-loop (eval FAIL within budget),
// the E2E_REMEDIATE sub-loop (e2e FAIL within budget), and the force-accept
// lifecycle guarantee (any forced loop marks items "failed" regardless of the
// other loops passing cleanly).
//
// Helpers (uiEvalStep, uiQAStep, uiBudgetConfig, uiSinglePlan, stepsPathRe,
// extractMatch, assertAllItemsFailed, assertAllItemsPassed) are defined in
// round_budget_test.go and pipeline_multidomain_test.go.

// TestA4UIImplementingHappyPath drives a single plan item through the complete
// ui_implementing lifecycle with all three loops passing on the first attempt.
// The canonical sequence is:
//
//	ORIENT → IMPLEMENT → EVALUATE → QA_TEST → E2E_AUTHOR → E2E_VERIFY → COMMIT → DONE
func TestA4UIImplementingHappyPath(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(uiBudgetConfig(3, 3, 3)) // evalMax=3, qaMax=3, e2eMax=3
	p.WriteFile("portal/plan.json", uiSinglePlan("portal", "Portal"))

	p.mustForge("init", "--phase", "ui_implementing", "--from", "portal/plan.json")
	p.AssertAt(state.PhaseUIImplementing, state.StateOrient)

	// ORIENT → IMPLEMENT.
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateImplement)

	// IMPLEMENT → EVALUATE (EvalRound becomes 1 on entry to the state).
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateEvaluate)
	if got := p.State().UIImplementing.CurrentBatch.EvalRound; got != 1 {
		t.Errorf("EvalRound after EVALUATE entry = %d, want 1", got)
	}

	// EVALUATE PASS (round 1 >= min_rounds 1) → QA_TEST.
	// Output contains the Steps: path required by uiQAStep.
	qaOut := uiEvalStep(p, "PASS", "ui-stubs/eval.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateQATest)

	// QA_TEST PASS (QARound 1 >= min_rounds 1) → E2E_AUTHOR.
	uiQAStep(p, qaOut.Out(), "PASS", "ui-stubs/qa.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateE2EAuthor)

	// E2E_AUTHOR → E2E_VERIFY (E2ERound becomes 1 on entry).
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateE2EVerify)
	if got := p.State().UIImplementing.CurrentBatch.E2ERound; got != 1 {
		t.Errorf("E2ERound after E2E_VERIFY entry = %d, want 1", got)
	}

	// E2E_VERIFY PASS (round 1 >= min_rounds 1) → COMMIT.
	uiEvalStep(p, "PASS", "ui-stubs/e2e.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateCommit)

	// Verify no force-accept flags were set during the happy path.
	batch := p.State().UIImplementing.CurrentBatch
	if batch.CodeForceAccepted {
		t.Error("CodeForceAccepted unexpectedly set on happy path")
	}
	if batch.QAForceAccepted {
		t.Error("QAForceAccepted unexpectedly set on happy path")
	}
	if batch.E2EForceAccepted {
		t.Error("E2EForceAccepted unexpectedly set on happy path")
	}

	// COMMIT → DONE.
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateDone)

	// DONE → session complete (non-zero exit).
	final := p.forge("advance")
	if !strings.Contains(final.Out(), "session complete") {
		t.Errorf("expected session-complete signal:\n%s", final.Out())
	}

	assertAllItemsPassed(t, p, "portal/plan.json")
}

// TestA4UIRefineLoop drives the QA sub-loop: a QA FAIL verdict at QARound 1
// (< qa_max_rounds 3) transitions to UI_REFINE instead of force-accepting. A
// plain advance from UI_REFINE returns to QA_TEST with QARound incremented to 2.
// A QA PASS at round 2 then proceeds normally to E2E_AUTHOR → ... → DONE.
//
// The test verifies: (a) UI_REFINE was reached, (b) QARound is 2 on re-entry
// to QA_TEST, and (c) the item ends "passed" because no force-accept fired.
//
// Note: the code-eval re-loop (eval FAIL < eval_max_rounds) goes back to
// IMPLEMENT, not UI_REFINE. UI_REFINE is exclusively the QA loop's re-loop.
func TestA4UIRefineLoop(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(uiBudgetConfig(3, 3, 3)) // evalMax=3, qaMax=3, e2eMax=3
	p.WriteFile("portal/plan.json", uiSinglePlan("portal", "Portal"))

	p.mustForge("init", "--phase", "ui_implementing", "--from", "portal/plan.json")
	p.AssertAt(state.PhaseUIImplementing, state.StateOrient)

	// ORIENT → IMPLEMENT → EVALUATE (EvalRound 1).
	p.mustForge("advance")
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateEvaluate)

	// eval PASS at round 1 >= min_rounds 1 → QA_TEST (QARound 1).
	qaOut := uiEvalStep(p, "PASS", "ui-stubs/eval1.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateQATest)

	// QA FAIL at QARound 1 < qa_max_rounds 3 → UI_REFINE (not force-accept).
	uiQAStep(p, qaOut.Out(), "FAIL", "ui-stubs/qa1.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateUIRefine)

	// UI_REFINE → QA_TEST (QARound incremented to 2 on re-entry).
	refineOut := p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateQATest)
	if got := p.State().UIImplementing.CurrentBatch.QARound; got != 2 {
		t.Errorf("QARound after UI_REFINE → QA_TEST = %d, want 2", got)
	}

	// QA PASS at round 2 >= min_rounds 1 → E2E_AUTHOR.
	uiQAStep(p, refineOut.Out(), "PASS", "ui-stubs/qa2.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateE2EAuthor)

	// E2E_AUTHOR → E2E_VERIFY → COMMIT (all PASS, single round each).
	p.mustForge("advance")
	uiEvalStep(p, "PASS", "ui-stubs/e2e.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateCommit)

	// COMMIT → DONE → session complete.
	p.mustForge("advance")
	final := p.forge("advance")
	if !strings.Contains(final.Out(), "session complete") {
		t.Errorf("expected session-complete signal:\n%s", final.Out())
	}

	// Force-accept was never triggered, so the item passed.
	assertAllItemsPassed(t, p, "portal/plan.json")
}

// TestA4E2ERemediateLoop drives the e2e sub-loop: a FAIL verdict at e2e round 1
// (< max_rounds 3) transitions to E2E_REMEDIATE instead of force-accepting. A
// plain advance from E2E_REMEDIATE returns to E2E_VERIFY with E2ERound
// incremented to 2. A PASS at round 2 then proceeds to COMMIT → DONE.
//
// The test verifies: (a) E2E_REMEDIATE was reached, (b) E2ERound is 2 on
// re-entry to E2E_VERIFY, and (c) the item ends "passed".
func TestA4E2ERemediateLoop(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(uiBudgetConfig(3, 3, 3)) // e2eMax=3, min_rounds=1
	p.WriteFile("portal/plan.json", uiSinglePlan("portal", "Portal"))

	p.mustForge("init", "--phase", "ui_implementing", "--from", "portal/plan.json")
	p.AssertAt(state.PhaseUIImplementing, state.StateOrient)

	// ORIENT → IMPLEMENT → EVALUATE (EvalRound 1).
	p.mustForge("advance")
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateEvaluate)

	// Eval PASS (round 1 >= min=1) → QA_TEST.
	qaOut := uiEvalStep(p, "PASS", "ui-stubs/eval.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateQATest)

	// QA PASS (QARound 1 >= min=1) → E2E_AUTHOR.
	uiQAStep(p, qaOut.Out(), "PASS", "ui-stubs/qa.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateE2EAuthor)

	// E2E_AUTHOR → E2E_VERIFY (E2ERound 1).
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateE2EVerify)

	// FAIL at e2e round 1 < max_rounds 3 → E2E_REMEDIATE (not force-accept).
	uiEvalStep(p, "FAIL", "ui-stubs/e2e1.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateE2ERemediate)

	// E2E_REMEDIATE → E2E_VERIFY (E2ERound incremented to 2 on re-entry).
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateE2EVerify)
	if got := p.State().UIImplementing.CurrentBatch.E2ERound; got != 2 {
		t.Errorf("E2ERound after E2E_REMEDIATE → E2E_VERIFY = %d, want 2", got)
	}

	// PASS at round 2 >= min_rounds 1 → COMMIT.
	uiEvalStep(p, "PASS", "ui-stubs/e2e2.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateCommit)

	// COMMIT → DONE → session complete.
	p.mustForge("advance")
	final := p.forge("advance")
	if !strings.Contains(final.Out(), "session complete") {
		t.Errorf("expected session-complete signal:\n%s", final.Out())
	}

	// Force-accept was never triggered, so the item passed.
	assertAllItemsPassed(t, p, "portal/plan.json")
}

// TestA4ForceAcceptMarksItemFailed drives the complete ui_implementing lifecycle
// with evalMax=1 so the very first eval FAIL triggers a force-accept
// (CodeForceAccepted=true). The QA and e2e loops are driven to PASS cleanly, but
// the item must still end as "failed" at COMMIT because any force-accept flag is
// sufficient to mark the item failed — even when the remaining loops succeed.
func TestA4ForceAcceptMarksItemFailed(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(uiBudgetConfig(1, 3, 3)) // eval max=1; qa/e2e max=3
	p.WriteFile("portal/plan.json", uiSinglePlan("portal", "Portal"))

	p.mustForge("init", "--phase", "ui_implementing", "--from", "portal/plan.json")
	p.AssertAt(state.PhaseUIImplementing, state.StateOrient)

	// ORIENT → IMPLEMENT → EVALUATE (EvalRound 1).
	p.mustForge("advance")
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateEvaluate)

	// FAIL at round 1 >= eval max_rounds 1 → CodeForceAccepted=true → QA_TEST.
	// The output of this advance shows the QA_TEST block (Steps: path).
	qaOut := uiEvalStep(p, "FAIL", "ui-stubs/eval.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateQATest)
	if !p.State().UIImplementing.CurrentBatch.CodeForceAccepted {
		t.Error("CodeForceAccepted not set after eval force-accept")
	}

	// QA PASS (QARound 1 >= min=1) → E2E_AUTHOR. Step list required even on
	// force-accept exit (invariant J1).
	uiQAStep(p, qaOut.Out(), "PASS", "ui-stubs/qa.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateE2EAuthor)

	// E2E_AUTHOR → E2E_VERIFY → COMMIT (E2ERound 1 >= min=1, PASS).
	p.mustForge("advance")
	uiEvalStep(p, "PASS", "ui-stubs/e2e.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateCommit)

	// COMMIT → DONE → session complete.
	p.mustForge("advance")
	final := p.forge("advance")
	if !strings.Contains(final.Out(), "session complete") {
		t.Errorf("expected session-complete signal:\n%s", final.Out())
	}

	// Item is "failed" because CodeForceAccepted was set, despite QA and e2e
	// both passing without force-accept.
	assertAllItemsFailed(t, p, "portal/plan.json")
}
