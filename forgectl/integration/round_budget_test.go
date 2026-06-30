//go:build integration

package integration

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"forgectl/state"
)

// This file covers plan §3.C — round/counter/budget logic for the implementing
// and ui_implementing phases. The specifying phase's eval loop (C1/C2) is already
// covered by oracle_specifying_test.go; these tests extend that coverage to the
// implementing phase (C1/C2) and the three independent ui_implementing loops (C3).
//
// Key facts encoded here, NOT imported from advance.go:
//
//   Implementing EVALUATE: EvalRound is incremented on each entry to the state.
//   A PASS at round < min_rounds loops back to IMPLEMENT (no separate REFINE
//   state); a FAIL at round >= max_rounds force-accepts → COMMIT with items
//   "failed". EvalRound persists across re-implements; it is not reset.
//
//   ui_implementing carries THREE separate round counters (EvalRound / QARound /
//   E2ERound), each incremented only while its own loop is active. Force-accept
//   on any one loop sets a per-loop flag (CodeForceAccepted / QAForceAccepted /
//   E2EForceAccepted); COMMIT marks items "failed" if ANY flag is set.
//   QA_TEST requires the step-list file to exist on EVERY exit toward E2E_AUTHOR,
//   including force-accept exits (invariant J1).

// --- plan fixtures ---

// singleItemImplPlan is a one-item, one-layer code plan with no file entries
// (commits are off in all round-budget tests, so disk files are never staged).
func singleItemImplPlan(domain, module string) string {
	return `{
  "context": {"domain":"` + domain + `","module":"` + module + `"},
  "layers": [{"id":"L0","name":"Foundation","items":["a"]}],
  "items": [
    {"id":"a","name":"` + module + ` item","description":"only item","depends_on":[],"tests":[{"category":"functional","description":"works","passes":false}]}
  ]
}`
}

// --- assertions ---

// assertAllItemsFailed is the negative mirror of assertAllItemsPassed: every
// item in the plan must have passes="failed". Used by the force-accept tests.
func assertAllItemsFailed(t *testing.T, p *Project, planRel string) {
	t.Helper()
	var pf planItemsFile
	if err := json.Unmarshal([]byte(p.ReadFile(planRel)), &pf); err != nil {
		t.Fatalf("parsing %s: %v", planRel, err)
	}
	if len(pf.Items) == 0 {
		t.Fatalf("%s has no items", planRel)
	}
	for _, it := range pf.Items {
		if it.Passes != "failed" {
			t.Errorf("%s item %q passes = %q, want \"failed\"", planRel, it.ID, it.Passes)
		}
	}
}

// --- implementing eval loop ---

// TestImplementingMinRoundsLoops covers §C1 for the implementing phase: when
// min_rounds=2, a PASS verdict at round 1 must loop back to IMPLEMENT rather
// than advancing to COMMIT. Only at round 2 (>= min_rounds) does PASS accept,
// marking items "passed". The re-implement path resets CurrentItemIndex to 0
// but retains the EvalRound counter (no separate REFINE state, unlike specifying).
func TestImplementingMinRoundsLoops(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(`
[general]
user_guided = false
enable_commits = false
[implementing]
batch = 2
[implementing.eval]
min_rounds = 2
max_rounds = 4
`)
	p.WriteFile("core/plan.json", singleItemImplPlan("core", "Core"))

	p.mustForge("init", "--phase", "implementing", "--from", "core/plan.json")
	p.AssertAt(state.PhaseImplementing, state.StateOrient)

	// ORIENT → IMPLEMENT.
	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateImplement)

	// IMPLEMENT → EVALUATE (EvalRound becomes 1 on entry to the state).
	eval1 := p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateEvaluate)
	if got := p.State().Implementing.CurrentBatch.EvalRound; got != 1 {
		t.Errorf("EvalRound after first EVALUATE entry = %d, want 1", got)
	}

	// C1: PASS at round 1 < min_rounds 2 — must loop back to IMPLEMENT, not COMMIT.
	p.PassEval(eval1, "PASS")
	p.AssertAt(state.PhaseImplementing, state.StateImplement)

	// IMPLEMENT (re-implement) → EVALUATE (EvalRound becomes 2).
	eval2 := p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateEvaluate)
	if got := p.State().Implementing.CurrentBatch.EvalRound; got != 2 {
		t.Errorf("EvalRound after second EVALUATE entry = %d, want 2", got)
	}

	// PASS at round 2 >= min_rounds 2 → COMMIT; items marked "passed".
	p.PassEval(eval2, "PASS")
	p.AssertAt(state.PhaseImplementing, state.StateCommit)

	p.mustForge("advance") // COMMIT → DONE
	final := p.forge("advance") // DONE → session complete
	if !strings.Contains(final.Out(), "session complete") {
		t.Errorf("expected session-complete signal:\n%s", final.Out())
	}

	assertAllItemsPassed(t, p, "core/plan.json")
}

// TestImplementingForceAccept covers §C2 for the implementing phase: max_rounds
// reached on FAIL → force-accept. The item is marked "failed" and the phase
// advances to COMMIT rather than looping indefinitely.
func TestImplementingForceAccept(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(`
[general]
user_guided = false
enable_commits = false
[implementing]
batch = 2
[implementing.eval]
min_rounds = 1
max_rounds = 2
`)
	p.WriteFile("core/plan.json", singleItemImplPlan("core", "Core"))

	p.mustForge("init", "--phase", "implementing", "--from", "core/plan.json")

	// ORIENT → IMPLEMENT → EVALUATE (EvalRound 1).
	p.mustForge("advance")
	eval1 := p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateEvaluate)

	// FAIL at round 1 < max_rounds 2 → re-implement (not force-accept yet).
	p.PassEval(eval1, "FAIL")
	p.AssertAt(state.PhaseImplementing, state.StateImplement)

	// IMPLEMENT (re-implement) → EVALUATE (EvalRound 2).
	eval2 := p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateEvaluate)

	// C2: FAIL at round 2 >= max_rounds 2 → force-accept → COMMIT with items failed.
	p.PassEval(eval2, "FAIL")
	p.AssertAt(state.PhaseImplementing, state.StateCommit)

	p.mustForge("advance") // COMMIT → DONE
	final := p.forge("advance") // DONE → session complete
	if !strings.Contains(final.Out(), "session complete") {
		t.Errorf("expected session-complete signal:\n%s", final.Out())
	}

	assertAllItemsFailed(t, p, "core/plan.json")
}

// --- ui_implementing three-loop budget ---

// uiBudgetConfig builds a TOML config for a ui_implementing test with
// configurable per-loop max_rounds. All loops use min_rounds=1 so the only
// variable is when force-accept fires.
func uiBudgetConfig(evalMax, qaMax, e2eMax int) string {
	return fmt.Sprintf(`
[general]
user_guided = false
enable_commits = false
[ui_implementing]
batch = 1
[ui_implementing.app]
launch_command = "echo run"
url = "http://localhost:5173"
[ui_implementing.eval]
min_rounds = 1
max_rounds = %d
[ui_implementing.qa]
min_rounds = 1
max_rounds = %d
[ui_implementing.e2e]
min_rounds = 1
max_rounds = %d
test_command = "echo e2e"
test_dir = "e2e"
`, evalMax, qaMax, e2eMax)
}

// uiEvalStep advances from a ui_implementing state that requires a verdict and
// an eval-report (EVALUATE or E2E_VERIFY). The ui phase prints a "<path>"
// placeholder rather than a concrete path in those states, so any existing file
// satisfies --eval-report; this helper writes a stub at the caller-chosen path.
func uiEvalStep(p *Project, verdict, stubPath string) Result {
	p.t.Helper()
	p.WriteReport(stubPath)
	return p.mustForge("advance", "--verdict", verdict, "--eval-report", stubPath)
}

// uiQAStep advances from ui_implementing QA_TEST. It extracts the step-list
// path from prevOut (the output of the advance that produced QA_TEST), writes a
// minimal valid step list there (invariant J1 — required on every exit toward
// E2E_AUTHOR, including force-accept), then advances with the given verdict.
func uiQAStep(p *Project, prevOut, verdict, evalStubPath string) Result {
	p.t.Helper()
	stepsPath := extractMatch(p.t, stepsPathRe, prevOut, "QA steps path")
	p.writeQASteps(stepsPath)
	p.WriteReport(evalStubPath)
	return p.mustForge("advance", "--verdict", verdict, "--eval-report", evalStubPath)
}

// TestUIEvalForceAccept covers §C3 for the code-eval loop: eval max_rounds=1,
// FAIL at eval round 1 → CodeForceAccepted=true. The QA and e2e loops still run
// (all-PASS) but at COMMIT the item is "failed" because any force-accept flag
// is set. This shows the eval counter is independent of the QA/e2e counters.
func TestUIEvalForceAccept(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(uiBudgetConfig(1, 3, 3)) // eval max=1; qa/e2e max=3
	p.WriteFile("portal/plan.json", uiSinglePlan("portal", "Portal"))

	p.mustForge("init", "--phase", "ui_implementing", "--from", "portal/plan.json")
	p.AssertAt(state.PhaseUIImplementing, state.StateOrient)

	// ORIENT → IMPLEMENT (EvalRound is 0 before entering EVALUATE).
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateImplement)

	// IMPLEMENT → EVALUATE (EvalRound becomes 1 at this transition).
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateEvaluate)

	// C3: eval FAIL at round 1 >= eval max_rounds 1 → CodeForceAccepted=true,
	// transition to QA_TEST. The output of this advance shows the QA_TEST block,
	// which includes the Steps: path (needed by uiQAStep below).
	qaOut := uiEvalStep(p, "FAIL", "ui-stubs/eval.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateQATest)
	if !p.State().UIImplementing.CurrentBatch.CodeForceAccepted {
		t.Error("CodeForceAccepted not set after eval force-accept")
	}

	// QA_TEST: write step list, PASS (QARound 1 >= min=1 → E2E_AUTHOR).
	uiQAStep(p, qaOut.Out(), "PASS", "ui-stubs/qa.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateE2EAuthor)

	// E2E_AUTHOR → E2E_VERIFY (E2ERound becomes 1).
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateE2EVerify)

	// E2E_VERIFY PASS (E2ERound 1 >= min=1 → COMMIT).
	uiEvalStep(p, "PASS", "ui-stubs/e2e.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateCommit)

	// COMMIT → DONE → session complete.
	p.mustForge("advance")
	final := p.forge("advance")
	if !strings.Contains(final.Out(), "session complete") {
		t.Errorf("expected session-complete signal:\n%s", final.Out())
	}

	// Item must be "failed" because CodeForceAccepted was set, even though
	// QA and e2e both passed.
	assertAllItemsFailed(t, p, "portal/plan.json")
}

// TestUIQAForceAccept covers §C3 for the QA loop: qa max_rounds=1, FAIL at QA
// round 1 → QAForceAccepted=true. The eval loop passes cleanly (no
// CodeForceAccepted); the e2e loop still runs after the QA force-accept; at
// COMMIT the item is "failed" due to QAForceAccepted.
func TestUIQAForceAccept(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(uiBudgetConfig(3, 1, 3)) // qa max=1; eval/e2e max=3
	p.WriteFile("portal/plan.json", uiSinglePlan("portal", "Portal"))

	p.mustForge("init", "--phase", "ui_implementing", "--from", "portal/plan.json")

	// ORIENT → IMPLEMENT → EVALUATE.
	p.mustForge("advance")
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateEvaluate)

	// Eval PASS (round 1 >= min=1 → QA_TEST). Output includes QA Steps: path.
	qaOut := uiEvalStep(p, "PASS", "ui-stubs/eval.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateQATest)

	// C3: QA FAIL at round 1 >= qa max_rounds 1 → QAForceAccepted=true, E2E_AUTHOR.
	// The step list must exist even on a force-accept exit (invariant J1).
	uiQAStep(p, qaOut.Out(), "FAIL", "ui-stubs/qa.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateE2EAuthor)
	if !p.State().UIImplementing.CurrentBatch.QAForceAccepted {
		t.Error("QAForceAccepted not set after QA force-accept")
	}

	// E2E_AUTHOR → E2E_VERIFY → COMMIT (e2e round 1 >= min=1, PASS).
	p.mustForge("advance")
	uiEvalStep(p, "PASS", "ui-stubs/e2e.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateCommit)

	p.mustForge("advance") // COMMIT → DONE
	final := p.forge("advance")
	if !strings.Contains(final.Out(), "session complete") {
		t.Errorf("expected session-complete signal:\n%s", final.Out())
	}

	assertAllItemsFailed(t, p, "portal/plan.json")
}

// TestUIE2EForceAccept covers §C3 for the e2e loop: e2e max_rounds=1, FAIL at
// e2e round 1 → E2EForceAccepted=true → COMMIT with item "failed". The eval
// and QA loops both pass cleanly (CodeForceAccepted and QAForceAccepted stay
// false), proving the three budgets are completely independent.
func TestUIE2EForceAccept(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(uiBudgetConfig(3, 3, 1)) // e2e max=1; eval/qa max=3
	p.WriteFile("portal/plan.json", uiSinglePlan("portal", "Portal"))

	p.mustForge("init", "--phase", "ui_implementing", "--from", "portal/plan.json")

	// ORIENT → IMPLEMENT → EVALUATE.
	p.mustForge("advance")
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateEvaluate)

	// Eval PASS → QA_TEST.
	qaOut := uiEvalStep(p, "PASS", "ui-stubs/eval.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateQATest)

	// QA PASS → E2E_AUTHOR.
	uiQAStep(p, qaOut.Out(), "PASS", "ui-stubs/qa.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateE2EAuthor)

	// E2E_AUTHOR → E2E_VERIFY (E2ERound becomes 1).
	p.mustForge("advance")
	p.AssertAt(state.PhaseUIImplementing, state.StateE2EVerify)

	// C3: e2e FAIL at round 1 >= e2e max_rounds 1 → E2EForceAccepted=true → COMMIT.
	uiEvalStep(p, "FAIL", "ui-stubs/e2e.md")
	p.AssertAt(state.PhaseUIImplementing, state.StateCommit)
	batch := p.State().UIImplementing.CurrentBatch
	// Verify independence: only the e2e flag is set; the other two are clear.
	if batch.CodeForceAccepted {
		t.Error("CodeForceAccepted unexpectedly set (eval passed cleanly)")
	}
	if batch.QAForceAccepted {
		t.Error("QAForceAccepted unexpectedly set (QA passed cleanly)")
	}
	if !batch.E2EForceAccepted {
		t.Error("E2EForceAccepted not set after e2e force-accept")
	}

	p.mustForge("advance") // COMMIT → DONE
	finalE2E := p.forge("advance")
	if !strings.Contains(finalE2E.Out(), "session complete") {
		t.Errorf("expected session-complete signal:\n%s", finalE2E.Out())
	}

	assertAllItemsFailed(t, p, "portal/plan.json")
}

// TestC4EvalRoundResetPerBatch covers §C4: the EvalRound counter must reset to
// 0 at the start of each new implementing batch (new ORIENT), and must persist
// correctly across EVALUATE ↔ IMPLEMENT loops within the same batch.
//
// The test drives two batches (batch=1, two-layer plan, L0 then L1). Within the
// first batch it exercises a re-implement cycle (min_rounds=2 forces a second
// EVALUATE) and checks the counter increments. Then after COMMIT it checks the
// counter resets at the second ORIENT.
func TestC4EvalRoundResetPerBatch(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(`
[general]
user_guided = false
enable_commits = false
[implementing]
batch = 1
[implementing.eval]
min_rounds = 2
max_rounds = 4
`)
	// Two-layer plan: L0 (item a) and L1 (item b, depends on a). With batch=1
	// each layer is its own batch, giving us two consecutive ORIENT→COMMIT cycles.
	const twoLayerEvalPlan = `{
  "context":{"domain":"core","module":"Core"},
  "layers":[
    {"id":"L0","name":"L0","items":["a"]},
    {"id":"L1","name":"L1","items":["b"]}
  ],
  "items":[
    {"id":"a","name":"A","description":"a","depends_on":[],"tests":[]},
    {"id":"b","name":"B","description":"b","depends_on":["a"],"tests":[]}
  ]
}`
	p.WriteFile("core/plan.json", twoLayerEvalPlan)

	p.mustForge("init", "--phase", "implementing", "--from", "core/plan.json")
	p.AssertAt(state.PhaseImplementing, state.StateOrient)

	// --- Batch 1 (L0) ---

	// ORIENT → IMPLEMENT (EvalRound is 0 before the first EVALUATE entry).
	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateImplement)

	// IMPLEMENT → EVALUATE (EvalRound becomes 1 on entry).
	eval1 := p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateEvaluate)
	if got := p.State().Implementing.CurrentBatch.EvalRound; got != 1 {
		t.Errorf("batch 1: EvalRound after first EVALUATE entry = %d, want 1", got)
	}

	// PASS at round 1 < min_rounds 2 → loops back to IMPLEMENT (not COMMIT yet).
	p.PassEval(eval1, "PASS")
	p.AssertAt(state.PhaseImplementing, state.StateImplement)

	// IMPLEMENT (re-implement) → EVALUATE (EvalRound becomes 2 — persisted across re-implement).
	eval2 := p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateEvaluate)
	if got := p.State().Implementing.CurrentBatch.EvalRound; got != 2 {
		t.Errorf("batch 1: EvalRound after second EVALUATE entry = %d, want 2", got)
	}

	// PASS at round 2 >= min_rounds 2 → COMMIT.
	p.PassEval(eval2, "PASS")
	p.AssertAt(state.PhaseImplementing, state.StateCommit)

	// COMMIT → ORIENT (batch 2, L1).
	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateOrient)

	// --- Batch 2 (L1): EvalRound must have reset to 0 ---

	// ORIENT → IMPLEMENT.
	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateImplement)

	// IMPLEMENT → EVALUATE (EvalRound should be 1, not 3 — it reset at the new batch).
	eval3 := p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateEvaluate)
	if got := p.State().Implementing.CurrentBatch.EvalRound; got != 1 {
		t.Errorf("batch 2: EvalRound after first EVALUATE entry = %d, want 1 (counter should have reset)", got)
	}

	// PASS at round 1 < min_rounds 2 → re-implement once more (validate reset held).
	p.PassEval(eval3, "PASS")
	p.AssertAt(state.PhaseImplementing, state.StateImplement)

	eval4 := p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateEvaluate)
	if got := p.State().Implementing.CurrentBatch.EvalRound; got != 2 {
		t.Errorf("batch 2: EvalRound after second EVALUATE entry = %d, want 2", got)
	}

	// PASS at round 2 → COMMIT → DONE → session complete.
	p.PassEval(eval4, "PASS")
	p.AssertAt(state.PhaseImplementing, state.StateCommit)

	p.mustForge("advance") // COMMIT → DONE
	finalRes := p.forge("advance")
	if !strings.Contains(finalRes.Out(), "session complete") {
		t.Errorf("expected session-complete signal:\n%s", finalRes.Out())
	}

	assertAllItemsPassed(t, p, "core/plan.json")
}
