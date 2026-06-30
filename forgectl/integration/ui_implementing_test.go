//go:build integration

package integration

import (
	"strings"
	"testing"

	"forgectl/state"
)

// This file covers plan invariants J1–J3 for the ui_implementing phase:
// the QA step-list file contract (J1/J2) and the handoff command (J3).

// --- J1: QA step list must exist to exit QA_TEST → E2E_AUTHOR ---

// TestJ1QAStepListRequired pins invariant J1: advance from QA_TEST toward
// E2E_AUTHOR must be rejected when the QA step list is absent. The step list
// path is emitted by forgectl in the QA_TEST action block ("Steps: <path>");
// the test extracts it, withholds the file, and asserts that advance fails
// with an error naming the missing path and that the state stays at QA_TEST.
func TestJ1QAStepListRequired(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(uiBudgetConfig(3, 3, 3))
	p.WriteFile("portal/plan.json", uiSinglePlan("portal", "Portal"))

	p.mustForge("init", "--phase", "ui_implementing", "--from", "portal/plan.json")
	p.AssertAt(state.PhaseUIImplementing, state.StateOrient)

	// ORIENT → IMPLEMENT → EVALUATE
	p.mustForge("advance")
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateEvaluate)

	// EVALUATE PASS → QA_TEST. The output contains the "Steps: <path>" line that
	// names where the QA evaluator must write the e2e step list.
	qaOut := uiEvalStep(p, "PASS", "stubs/eval.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateQATest)

	// Extract the step list path the phase demands.
	stepsPath := extractMatch(t, stepsPathRe, qaOut.Out(), "QA steps path")

	// J1: write the eval-report stub but deliberately leave the step list absent.
	p.WriteReport("stubs/qa.md")

	res := p.forge("advance", "--verdict", "PASS", "--eval-report", "stubs/qa.md")
	if res.Exit == 0 {
		t.Fatal("advance from QA_TEST without step list succeeded; expected failure")
	}
	// The error must name the missing step-list path so the operator knows exactly
	// which file to write.
	if !strings.Contains(res.Out(), stepsPath) {
		t.Errorf("error output does not name missing step-list path %q:\nstdout:\n%s\nstderr:\n%s",
			stepsPath, res.Stdout, res.Stderr)
	}
	// State must remain at QA_TEST — the failed advance must not mutate it.
	p.AssertAt(state.PhaseUIImplementing, state.StateQATest)
}

// --- J2: Empty step list passes e2e vacuously ---

// TestJ2EmptyStepListPassesVacuously pins that a zero-scenario step list is
// accepted by QA_TEST (with a warning) and results in an E2E loop that passes
// vacuously — forgectl advances through E2E_AUTHOR → E2E_VERIFY → COMMIT
// without error and the item ends "passed" (no force-accept).
func TestJ2EmptyStepListPassesVacuously(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(uiBudgetConfig(3, 3, 3))
	p.WriteFile("portal/plan.json", uiSinglePlan("portal", "Portal"))

	p.mustForge("init", "--phase", "ui_implementing", "--from", "portal/plan.json")
	p.AssertAt(state.PhaseUIImplementing, state.StateOrient)

	// ORIENT → IMPLEMENT → EVALUATE → QA_TEST
	p.mustForge("advance")
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateEvaluate)

	qaOut := uiEvalStep(p, "PASS", "stubs/eval.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateQATest)

	// Extract the step list path and write it with zero scenarios.
	stepsPath := extractMatch(t, stepsPathRe, qaOut.Out(), "QA steps path")
	p.WriteFile(stepsPath, `{"batch":1,"round":1,"scenarios":[]}`)

	// Advance from QA_TEST: forgectl warns about 0 scenarios but must exit 0.
	p.WriteReport("stubs/qa.md")
	p.mustForge("advance", "--verdict", "PASS", "--eval-report", "stubs/qa.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateE2EAuthor)

	// E2E_AUTHOR → E2E_VERIFY (the e2e runner has zero scenarios to execute,
	// so its test command exits 0 trivially).
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateE2EVerify)

	// E2E_VERIFY PASS → COMMIT
	uiEvalStep(p, "PASS", "stubs/e2e.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateCommit)

	// COMMIT → DONE → session complete
	p.mustForge("advance")
	final := p.forge("advance")
	if !strings.Contains(final.Out(), "session complete") {
		t.Errorf("expected session-complete signal:\n%s", final.Out())
	}

	// Item must be "passed": no force-accept occurred (all three loops passed).
	assertAllItemsPassed(t, p, "portal/plan.json")
}

// --- J3: handoff registers files ---

// TestJ3HandoffRegistersFiles pins the handoff command's three contracts:
//
//  1. A successful handoff registers the file and surfaces it under "Review:"
//     in the next status output; state is unchanged.
//  2. A second handoff replaces (latest-wins) the first: status shows the new
//     file only.
//  3. A handoff naming a nonexistent file exits non-zero and registers nothing.
func TestJ3HandoffRegistersFiles(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(uiBudgetConfig(3, 3, 3))
	p.WriteFile("portal/plan.json", uiSinglePlan("portal", "Portal"))

	p.mustForge("init", "--phase", "ui_implementing", "--from", "portal/plan.json")
	// Drive to EVALUATE — handoff is valid in EVALUATE, QA_TEST, and E2E_VERIFY.
	p.mustForge("advance") // ORIENT → IMPLEMENT
	p.mustForge("advance") // IMPLEMENT → EVALUATE
	p.AssertAt(state.PhaseUIImplementing, state.StateEvaluate)

	// --- Contract 1: handoff registers the file and state is unchanged ---

	report1 := p.Path("report1.md")
	p.WriteFile("report1.md", "# report 1\n")
	p.mustForge("handoff", report1)

	// State must still be EVALUATE — handoff must not transition anything.
	p.AssertAt(state.PhaseUIImplementing, state.StateEvaluate)

	status1 := p.mustForge("status")
	if !strings.Contains(status1.Stdout, "Review:") {
		t.Error("'Review:' not found in status output after first handoff")
	}
	if !strings.Contains(status1.Stdout, "report1.md") {
		t.Errorf("report1.md not found in status output after first handoff:\n%s", status1.Stdout)
	}

	// --- Contract 2: latest-wins — second handoff replaces the first ---

	report2 := p.Path("report2.md")
	p.WriteFile("report2.md", "# report 2\n")
	p.mustForge("handoff", report2)

	status2 := p.mustForge("status")
	if strings.Contains(status2.Stdout, "report1.md") {
		t.Errorf("report1.md still shown in status after report2.md replaced it:\n%s", status2.Stdout)
	}
	if !strings.Contains(status2.Stdout, "report2.md") {
		t.Errorf("report2.md not found in status after second handoff:\n%s", status2.Stdout)
	}

	// --- Contract 3: nonexistent file is rejected ---

	res := p.forge("handoff", "/nonexistent/file.md")
	if res.Exit == 0 {
		t.Error("handoff with nonexistent file succeeded; expected non-zero exit")
	}

	// State must still be at EVALUATE after all of the above.
	p.AssertAt(state.PhaseUIImplementing, state.StateEvaluate)
}
