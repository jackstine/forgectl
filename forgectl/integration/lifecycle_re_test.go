//go:build integration

package integration

import (
	"strings"
	"testing"

	"forgectl/state"
)

const reConfig = `
[general]
user_guided = false
enable_commits = false
[reverse_engineering.reconcile]
min_rounds = 1
max_rounds = 3
`

const reInitInput = `{"concept":"test concept","domains":["core"]}`

// reQueueContent is the reverse-engineering queue written by the agent at the
// QUEUE state. code_search_roots uses "." so it resolves to core/ under the
// project root — the test creates that directory before advancing from QUEUE.
const reQueueContent = `{
  "specs": [
    {
      "name": "Spec A",
      "domain": "core",
      "topic": "core topic",
      "file": "specs/a.md",
      "action": "create",
      "code_search_roots": ["."],
      "depends_on": []
    }
  ]
}`

// TestA5ReverseEngineeringFullWalk drives the reverse_engineering phase from
// init to DONE along the happy path: one domain ("core"), one queue entry,
// colleague_review=false, all reconcile verdicts PASS.
//
// State sequence:
//
//	ORIENT → SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE
//	→ EXECUTE_REVERSE_ENGINEER → POST_REVERSE_ENGINEER
//	→ RECONCILE → RECONCILE_EVAL → RECONCILE_ADVANCE → DONE
func TestA5ReverseEngineeringFullWalk(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(reConfig)
	p.WriteFile("re-input.json", reInitInput)

	// init --phase reverse_engineering --from re-input.json → ORIENT
	p.mustForge("init", "--phase", "reverse_engineering", "--from", "re-input.json")
	p.AssertAt(state.PhaseReverseEngineering, state.StateOrient)

	// Verify the RE state was initialised correctly.
	s := p.State()
	if s.ReverseEngineering == nil {
		t.Fatal("ReverseEngineering state is nil after init")
	}
	if len(s.ReverseEngineering.Domains) != 1 || s.ReverseEngineering.Domains[0] != "core" {
		t.Errorf("RE Domains = %v, want [core]", s.ReverseEngineering.Domains)
	}
	if s.ReverseEngineering.DomainCount != 1 {
		t.Errorf("RE DomainCount = %d, want 1", s.ReverseEngineering.DomainCount)
	}
	if s.ReverseEngineering.ColleagueReview {
		t.Error("RE ColleagueReview = true, want false")
	}

	// ORIENT → SURVEY (DomainIndex locked to 1)
	p.mustForge("advance")
	p.AssertAt(state.PhaseReverseEngineering, state.StateSurvey)

	s = p.State()
	if s.ReverseEngineering.DomainIndex != 1 {
		t.Errorf("DomainIndex after ORIENT = %d, want 1", s.ReverseEngineering.DomainIndex)
	}

	// SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE
	p.mustForge("advance")
	p.AssertAt(state.PhaseReverseEngineering, state.StateGapAnalysis)
	p.mustForge("advance")
	p.AssertAt(state.PhaseReverseEngineering, state.StateDecompose)
	p.mustForge("advance")
	p.AssertAt(state.PhaseReverseEngineering, state.StateQueue)

	// At QUEUE: create the core/ directory so code_search_roots validation passes,
	// then write the fixed-path queue file and advance.
	p.WriteFile("core/.keep", "")
	p.WriteFile(".forgectl/state/reverse-engineering-queue.json", reQueueContent)

	// QUEUE with one domain → DomainIndex(1) is NOT < DomainCount(1), so the
	// loop skips the per-domain iteration and goes directly to EXECUTE.
	p.mustForge("advance")
	p.AssertAt(state.PhaseReverseEngineering, state.StateExecuteReverseEngineer)

	// Verify the queue was parsed and stored, and ExecuteItemIndex is primed.
	s = p.State()
	if len(s.ReverseEngineering.Queue) != 1 {
		t.Fatalf("RE Queue len = %d, want 1", len(s.ReverseEngineering.Queue))
	}
	entry := s.ReverseEngineering.Queue[0]
	if entry.Name != "Spec A" || entry.Domain != "core" || entry.Topic != "core topic" {
		t.Errorf("RE Queue[0] = %+v, want {Name:Spec A, Domain:core, Topic:core topic}", entry)
	}
	if s.ReverseEngineering.ExecuteItemIndex != 1 {
		t.Errorf("ExecuteItemIndex = %d, want 1", s.ReverseEngineering.ExecuteItemIndex)
	}
	// QUEUE→EXECUTE transition calls ensureItemSpecsDir, creating core/specs/.
	if !p.Exists("core/specs") {
		t.Error("core/specs/ directory was not created by ensureItemSpecsDir")
	}

	// EXECUTE_REVERSE_ENGINEER → POST_REVERSE_ENGINEER (processes item at index 0)
	p.mustForge("advance")
	p.AssertAt(state.PhaseReverseEngineering, state.StatePostReverseEngineer)

	// POST_REVERSE_ENGINEER: ExecuteItemIndex(1) == len(Queue)(1) — last item done.
	// Sets DomainIndex=1, ReconcileRound=1, transitions to RECONCILE.
	p.mustForge("advance")
	p.AssertAt(state.PhaseReverseEngineering, state.StateReconcile)

	s = p.State()
	if s.ReverseEngineering.ReconcileRound != 1 {
		t.Errorf("ReconcileRound = %d, want 1", s.ReverseEngineering.ReconcileRound)
	}
	if s.ReverseEngineering.DomainIndex != 1 {
		t.Errorf("DomainIndex at RECONCILE start = %d, want 1", s.ReverseEngineering.DomainIndex)
	}

	// RECONCILE → RECONCILE_EVAL
	reconOut := p.mustForge("advance")
	p.AssertAt(state.PhaseReverseEngineering, state.StateReconcileEval)

	// The RECONCILE_EVAL output must surface the round counter and domain name.
	if !strings.Contains(reconOut.Stdout, "Round: 1/3") {
		t.Errorf("RECONCILE_EVAL output missing round indicator:\n%s", reconOut.Stdout)
	}
	if !strings.Contains(reconOut.Stdout, "core") {
		t.Errorf("RECONCILE_EVAL output missing domain name:\n%s", reconOut.Stdout)
	}

	// RECONCILE_EVAL --verdict PASS: ReconcileRound(1) >= MinRounds(1) → passed.
	// ColleagueReview=false → skip COLLEAGUE_REVIEW, go to RECONCILE_ADVANCE.
	// --eval-report is optional in RE RECONCILE_EVAL; omit it for the happy path.
	p.mustForge("advance", "--verdict", "PASS")
	p.AssertAt(state.PhaseReverseEngineering, state.StateReconcileAdvance)

	// Verify the verdict was recorded in the per-domain reconcile map.
	s = p.State()
	if s.ReverseEngineering.DomainReconcile == nil {
		t.Fatal("DomainReconcile map is nil after RECONCILE_EVAL")
	}
	coreRec := s.ReverseEngineering.DomainReconcile["core"]
	if coreRec == nil {
		t.Fatal("DomainReconcile[\"core\"] is nil")
	}
	if len(coreRec.Evals) != 1 {
		t.Fatalf("DomainReconcile[\"core\"].Evals len = %d, want 1", len(coreRec.Evals))
	}
	if coreRec.Evals[0].Verdict != "PASS" {
		t.Errorf("DomainReconcile[\"core\"].Evals[0].Verdict = %q, want PASS", coreRec.Evals[0].Verdict)
	}

	// RECONCILE_ADVANCE: DomainIndex(1) is NOT < DomainCount(1) → DONE.
	advOut := p.mustForge("advance")
	p.AssertAt(state.PhaseReverseEngineering, state.StateDone)

	// The RECONCILE_ADVANCE output should surface the terminal "DONE" indicator.
	if !strings.Contains(advOut.Stdout, "DONE") {
		t.Errorf("RECONCILE_ADVANCE output missing DONE indicator:\n%s", advOut.Stdout)
	}
}
