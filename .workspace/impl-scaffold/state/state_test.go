package state

import (
	"testing"
)

func twoPlansQueue() []QueuePlan {
	return []QueuePlan{
		{
			ID:              1,
			Name:            "Protocol Implementation",
			Domain:          "protocols",
			Topic:           "WS1 and WS2 message contracts",
			File:            "protocols/.workspace/implementation_plan/PLAN.md",
			Specs:           []string{"protocols/ws1/specs/ws1.md", "protocols/ws2/specs/ws2.md"},
			CodeSearchRoots: []string{"api/", "optimizer/"},
		},
		{
			ID:              2,
			Name:            "API Server",
			Domain:          "api",
			Topic:           "API server infrastructure",
			File:            "api/.workspace/implementation_plan/PLAN.md",
			Specs:           []string{"api/specs/server.md"},
			CodeSearchRoots: []string{"api/"},
		},
	}
}

func newTestState() *ScaffoldState {
	return NewState(1, 3, 3, false, twoPlansQueue())
}

func TestNewState(t *testing.T) {
	s := newTestState()
	if s.State != ORIENT {
		t.Fatalf("expected ORIENT, got %s", s.State)
	}
	if len(s.Queue) != 2 {
		t.Fatalf("expected 2 in queue, got %d", len(s.Queue))
	}
	if s.CurrentPlan != nil {
		t.Fatal("expected nil current plan")
	}
	if s.SubAgents != 3 {
		t.Fatalf("expected sub_agents 3, got %d", s.SubAgents)
	}
}

func TestInitSubAgentsConfig(t *testing.T) {
	s := NewState(1, 3, 5, false, twoPlansQueue())
	if s.SubAgents != 5 {
		t.Fatalf("expected sub_agents 5, got %d", s.SubAgents)
	}
	if s.MinRounds != 1 {
		t.Fatalf("expected min_rounds 1, got %d", s.MinRounds)
	}
	if s.MaxRounds != 3 {
		t.Fatalf("expected max_rounds 3, got %d", s.MaxRounds)
	}
}

func TestFullLifecyclePass(t *testing.T) {
	s := newTestState()

	// ORIENT → STUDY_SPECS
	if err := Advance(s, AdvanceInput{}); err != nil {
		t.Fatal(err)
	}
	if s.State != STUDY_SPECS {
		t.Fatalf("expected STUDY_SPECS, got %s", s.State)
	}
	if s.CurrentPlan == nil {
		t.Fatal("expected current plan to be set")
	}
	if s.CurrentPlan.Name != "Protocol Implementation" {
		t.Fatalf("expected Protocol Implementation, got %s", s.CurrentPlan.Name)
	}

	// STUDY_SPECS → STUDY_CODE
	if err := Advance(s, AdvanceInput{Notes: "WS1 and WS2 use type discriminator"}); err != nil {
		t.Fatal(err)
	}
	if s.State != STUDY_CODE {
		t.Fatalf("expected STUDY_CODE, got %s", s.State)
	}
	if s.CurrentPlan.Study.SpecsNotes != "WS1 and WS2 use type discriminator" {
		t.Fatalf("specs notes not recorded")
	}

	// STUDY_CODE → STUDY_PACKAGES
	if err := Advance(s, AdvanceInput{Notes: "Translator exists, WS2 client missing"}); err != nil {
		t.Fatal(err)
	}
	if s.State != STUDY_PACKAGES {
		t.Fatalf("expected STUDY_PACKAGES, got %s", s.State)
	}
	if s.CurrentPlan.Study.CodeNotes != "Translator exists, WS2 client missing" {
		t.Fatalf("code notes not recorded")
	}

	// STUDY_PACKAGES → SELECT
	if err := Advance(s, AdvanceInput{Notes: "coder/websocket for Go"}); err != nil {
		t.Fatal(err)
	}
	if s.State != SELECT {
		t.Fatalf("expected SELECT, got %s", s.State)
	}
	if s.CurrentPlan.Study.PackagesNotes != "coder/websocket for Go" {
		t.Fatalf("packages notes not recorded")
	}

	// SELECT → DRAFT
	if err := Advance(s, AdvanceInput{}); err != nil {
		t.Fatal(err)
	}
	if s.State != DRAFT {
		t.Fatalf("expected DRAFT, got %s", s.State)
	}

	// DRAFT → EVALUATE
	if err := Advance(s, AdvanceInput{}); err != nil {
		t.Fatal(err)
	}
	if s.State != EVALUATE {
		t.Fatalf("expected EVALUATE, got %s", s.State)
	}
	if s.CurrentPlan.Round != 1 {
		t.Fatalf("expected round 1, got %d", s.CurrentPlan.Round)
	}

	// EVALUATE PASS → ACCEPT
	if err := Advance(s, AdvanceInput{Verdict: "PASS", Message: "Add protocol impl plan"}); err != nil {
		t.Fatal(err)
	}
	if s.State != ACCEPT {
		t.Fatalf("expected ACCEPT, got %s", s.State)
	}

	// ACCEPT → ORIENT (queue has more)
	if err := Advance(s, AdvanceInput{}); err != nil {
		t.Fatal(err)
	}
	if s.State != ORIENT {
		t.Fatalf("expected ORIENT, got %s", s.State)
	}
	if len(s.Completed) != 1 {
		t.Fatalf("expected 1 completed, got %d", len(s.Completed))
	}
	if s.Completed[0].Study.SpecsNotes != "WS1 and WS2 use type discriminator" {
		t.Fatal("study notes not carried to completed")
	}
	if len(s.Completed[0].Evals) != 1 {
		t.Fatal("evals not carried to completed")
	}
}

func TestFullLifecycleWithFailAndRefine(t *testing.T) {
	plans := []QueuePlan{twoPlansQueue()[0]}
	s := NewState(1, 2, 3, false, plans)

	// ORIENT → STUDY_SPECS → STUDY_CODE → STUDY_PACKAGES → SELECT → DRAFT → EVALUATE
	Advance(s, AdvanceInput{})                           // → STUDY_SPECS
	Advance(s, AdvanceInput{})                           // → STUDY_CODE
	Advance(s, AdvanceInput{})                           // → STUDY_PACKAGES
	Advance(s, AdvanceInput{})                           // → SELECT
	Advance(s, AdvanceInput{})                           // → DRAFT
	Advance(s, AdvanceInput{})                           // → EVALUATE

	// EVALUATE FAIL → REFINE
	err := Advance(s, AdvanceInput{
		Verdict:      "FAIL",
		Deficiencies: []string{"Completeness", "Traceability"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if s.State != REFINE {
		t.Fatalf("expected REFINE, got %s", s.State)
	}
	if len(s.CurrentPlan.Evals) != 1 {
		t.Fatalf("expected 1 eval, got %d", len(s.CurrentPlan.Evals))
	}
	if s.CurrentPlan.Evals[0].Verdict != "FAIL" {
		t.Fatal("expected FAIL verdict")
	}
	if len(s.CurrentPlan.Evals[0].Deficiencies) != 2 {
		t.Fatal("deficiencies not recorded")
	}

	// REFINE → EVALUATE
	err = Advance(s, AdvanceInput{Fixed: "Added acceptance criteria"})
	if err != nil {
		t.Fatal(err)
	}
	if s.State != EVALUATE {
		t.Fatalf("expected EVALUATE, got %s", s.State)
	}
	if s.CurrentPlan.Round != 2 {
		t.Fatalf("expected round 2, got %d", s.CurrentPlan.Round)
	}
	if s.CurrentPlan.Evals[0].Fixed != "Added acceptance criteria" {
		t.Fatal("fixed not recorded on last eval")
	}

	// EVALUATE PASS → ACCEPT
	Advance(s, AdvanceInput{Verdict: "PASS", Message: "commit"})
	if s.State != ACCEPT {
		t.Fatalf("expected ACCEPT, got %s", s.State)
	}

	// ACCEPT → DONE (single plan)
	Advance(s, AdvanceInput{})
	if s.State != DONE {
		t.Fatalf("expected DONE, got %s", s.State)
	}
	if len(s.Completed) != 1 {
		t.Fatalf("expected 1 completed, got %d", len(s.Completed))
	}
	if len(s.Completed[0].Evals) != 2 {
		t.Fatal("expected 2 evals in completed")
	}
	if s.Completed[0].RoundsTaken != 2 {
		t.Fatalf("expected 2 rounds taken, got %d", s.Completed[0].RoundsTaken)
	}
}

func TestStudyNotesEmptyWhenOmitted(t *testing.T) {
	s := newTestState()
	Advance(s, AdvanceInput{}) // → STUDY_SPECS
	Advance(s, AdvanceInput{}) // → STUDY_CODE (no notes)

	if s.CurrentPlan.Study.SpecsNotes != "" {
		t.Fatal("expected empty specs notes")
	}

	Advance(s, AdvanceInput{}) // → STUDY_PACKAGES (no notes)
	if s.CurrentPlan.Study.CodeNotes != "" {
		t.Fatal("expected empty code notes")
	}
}

func TestStudyPhasesSequential(t *testing.T) {
	s := newTestState()

	expected := []Phase{STUDY_SPECS, STUDY_CODE, STUDY_PACKAGES, SELECT, DRAFT}
	for _, exp := range expected {
		if err := Advance(s, AdvanceInput{}); err != nil {
			t.Fatalf("advance to %s: %v", exp, err)
		}
		if s.State != exp {
			t.Fatalf("expected %s, got %s", exp, s.State)
		}
	}
}

func TestEvaluateRequiresVerdict(t *testing.T) {
	s := newTestState()
	// Advance to EVALUATE
	Advance(s, AdvanceInput{})                         // → STUDY_SPECS
	Advance(s, AdvanceInput{})                         // → STUDY_CODE
	Advance(s, AdvanceInput{})                         // → STUDY_PACKAGES
	Advance(s, AdvanceInput{})                         // → SELECT
	Advance(s, AdvanceInput{})                         // → DRAFT
	Advance(s, AdvanceInput{})                         // → EVALUATE

	err := Advance(s, AdvanceInput{})
	if err == nil {
		t.Fatal("expected error for missing verdict")
	}
}

func TestPassRequiresMessage(t *testing.T) {
	s := newTestState()
	Advance(s, AdvanceInput{}) // → STUDY_SPECS
	Advance(s, AdvanceInput{}) // → STUDY_CODE
	Advance(s, AdvanceInput{}) // → STUDY_PACKAGES
	Advance(s, AdvanceInput{}) // → SELECT
	Advance(s, AdvanceInput{}) // → DRAFT
	Advance(s, AdvanceInput{}) // → EVALUATE

	err := Advance(s, AdvanceInput{Verdict: "PASS"})
	if err == nil {
		t.Fatal("expected error for PASS without message")
	}
}

func TestVerdictRejectedOutsideEvaluate(t *testing.T) {
	s := newTestState()
	Advance(s, AdvanceInput{}) // → STUDY_SPECS

	err := Advance(s, AdvanceInput{Verdict: "PASS"})
	if err == nil {
		t.Fatal("expected error for verdict in STUDY_SPECS")
	}
}

func TestFileRejectedOutsideDraft(t *testing.T) {
	s := newTestState()
	Advance(s, AdvanceInput{}) // → STUDY_SPECS

	err := Advance(s, AdvanceInput{File: "some/path.md"})
	if err == nil {
		t.Fatal("expected error for --file in STUDY_SPECS")
	}
}

func TestFileOverrideInDraft(t *testing.T) {
	s := newTestState()
	Advance(s, AdvanceInput{}) // → STUDY_SPECS
	Advance(s, AdvanceInput{}) // → STUDY_CODE
	Advance(s, AdvanceInput{}) // → STUDY_PACKAGES
	Advance(s, AdvanceInput{}) // → SELECT
	Advance(s, AdvanceInput{}) // → DRAFT

	Advance(s, AdvanceInput{File: "new/path.md"})
	if s.CurrentPlan.File != "new/path.md" {
		t.Fatalf("expected file override, got %s", s.CurrentPlan.File)
	}
}

func TestDoneCannotAdvance(t *testing.T) {
	plans := []QueuePlan{twoPlansQueue()[0]}
	s := NewState(1, 1, 3, false, plans)

	// Fast-forward to DONE
	Advance(s, AdvanceInput{})                                         // → STUDY_SPECS
	Advance(s, AdvanceInput{})                                         // → STUDY_CODE
	Advance(s, AdvanceInput{})                                         // → STUDY_PACKAGES
	Advance(s, AdvanceInput{})                                         // → SELECT
	Advance(s, AdvanceInput{})                                         // → DRAFT
	Advance(s, AdvanceInput{})                                         // → EVALUATE
	Advance(s, AdvanceInput{Verdict: "PASS", Message: "done"})         // → ACCEPT
	Advance(s, AdvanceInput{})                                         // → DONE

	err := Advance(s, AdvanceInput{})
	if err == nil {
		t.Fatal("expected error advancing from DONE")
	}
}

func TestDeficienciesRecordedOnFail(t *testing.T) {
	s := newTestState()
	Advance(s, AdvanceInput{}) // → STUDY_SPECS
	Advance(s, AdvanceInput{}) // → STUDY_CODE
	Advance(s, AdvanceInput{}) // → STUDY_PACKAGES
	Advance(s, AdvanceInput{}) // → SELECT
	Advance(s, AdvanceInput{}) // → DRAFT
	Advance(s, AdvanceInput{}) // → EVALUATE

	Advance(s, AdvanceInput{
		Verdict:      "FAIL",
		Deficiencies: []string{"Completeness", "Traceability"},
	})

	if len(s.CurrentPlan.Evals) != 1 {
		t.Fatal("expected 1 eval")
	}
	eval := s.CurrentPlan.Evals[0]
	if eval.Verdict != "FAIL" {
		t.Fatal("expected FAIL")
	}
	if len(eval.Deficiencies) != 2 {
		t.Fatalf("expected 2 deficiencies, got %d", len(eval.Deficiencies))
	}
	if eval.Deficiencies[0] != "Completeness" || eval.Deficiencies[1] != "Traceability" {
		t.Fatal("wrong deficiencies")
	}
}

func TestFixedRecordedOnRefine(t *testing.T) {
	s := newTestState()
	Advance(s, AdvanceInput{}) // → STUDY_SPECS
	Advance(s, AdvanceInput{}) // → STUDY_CODE
	Advance(s, AdvanceInput{}) // → STUDY_PACKAGES
	Advance(s, AdvanceInput{}) // → SELECT
	Advance(s, AdvanceInput{}) // → DRAFT
	Advance(s, AdvanceInput{}) // → EVALUATE
	Advance(s, AdvanceInput{Verdict: "FAIL", Deficiencies: []string{"X"}}) // → REFINE

	Advance(s, AdvanceInput{Fixed: "Added missing sections"})
	if s.CurrentPlan.Evals[0].Fixed != "Added missing sections" {
		t.Fatal("fixed not recorded")
	}
}

func TestStudyNotesCarriedToCompleted(t *testing.T) {
	plans := []QueuePlan{twoPlansQueue()[0]}
	s := NewState(1, 1, 3, false, plans)

	Advance(s, AdvanceInput{})                                         // → STUDY_SPECS
	Advance(s, AdvanceInput{Notes: "spec notes"})                      // → STUDY_CODE
	Advance(s, AdvanceInput{Notes: "code notes"})                      // → STUDY_PACKAGES
	Advance(s, AdvanceInput{Notes: "pkg notes"})                       // → SELECT
	Advance(s, AdvanceInput{})                                         // → DRAFT
	Advance(s, AdvanceInput{})                                         // → EVALUATE
	Advance(s, AdvanceInput{Verdict: "PASS", Message: "done"})         // → ACCEPT
	Advance(s, AdvanceInput{})                                         // → DONE

	if len(s.Completed) != 1 {
		t.Fatal("expected 1 completed")
	}
	c := s.Completed[0]
	if c.Study.SpecsNotes != "spec notes" {
		t.Fatal("specs notes not carried")
	}
	if c.Study.CodeNotes != "code notes" {
		t.Fatal("code notes not carried")
	}
	if c.Study.PackagesNotes != "pkg notes" {
		t.Fatal("packages notes not carried")
	}
}

func TestEvalHistoryCarriedToCompleted(t *testing.T) {
	plans := []QueuePlan{twoPlansQueue()[0]}
	s := NewState(1, 2, 3, false, plans)

	Advance(s, AdvanceInput{}) // → STUDY_SPECS
	Advance(s, AdvanceInput{}) // → STUDY_CODE
	Advance(s, AdvanceInput{}) // → STUDY_PACKAGES
	Advance(s, AdvanceInput{}) // → SELECT
	Advance(s, AdvanceInput{}) // → DRAFT
	Advance(s, AdvanceInput{}) // → EVALUATE

	Advance(s, AdvanceInput{Verdict: "FAIL", Deficiencies: []string{"X"}}) // → REFINE
	Advance(s, AdvanceInput{Fixed: "fixed it"})                            // → EVALUATE
	Advance(s, AdvanceInput{Verdict: "PASS", Message: "done"})             // → ACCEPT
	Advance(s, AdvanceInput{})                                             // → DONE

	if len(s.Completed[0].Evals) != 2 {
		t.Fatalf("expected 2 evals, got %d", len(s.Completed[0].Evals))
	}
	if s.Completed[0].Evals[0].Verdict != "FAIL" {
		t.Fatal("first eval should be FAIL")
	}
	if s.Completed[0].Evals[1].Verdict != "PASS" {
		t.Fatal("second eval should be PASS")
	}
}

func TestRoundMonotonicity(t *testing.T) {
	s := newTestState()
	Advance(s, AdvanceInput{}) // → STUDY_SPECS
	Advance(s, AdvanceInput{}) // → STUDY_CODE
	Advance(s, AdvanceInput{}) // → STUDY_PACKAGES
	Advance(s, AdvanceInput{}) // → SELECT
	Advance(s, AdvanceInput{}) // → DRAFT
	Advance(s, AdvanceInput{}) // → EVALUATE (round 1)

	if s.CurrentPlan.Round != 1 {
		t.Fatalf("expected round 1, got %d", s.CurrentPlan.Round)
	}

	Advance(s, AdvanceInput{Verdict: "FAIL"}) // → REFINE
	Advance(s, AdvanceInput{})                // → EVALUATE (round 2)

	if s.CurrentPlan.Round != 2 {
		t.Fatalf("expected round 2, got %d", s.CurrentPlan.Round)
	}

	Advance(s, AdvanceInput{Verdict: "FAIL"}) // → REFINE
	Advance(s, AdvanceInput{})                // → EVALUATE (round 3)

	if s.CurrentPlan.Round != 3 {
		t.Fatalf("expected round 3, got %d", s.CurrentPlan.Round)
	}
}
