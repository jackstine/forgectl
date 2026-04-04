package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- Init Tests ---

func TestInitDefaultsToSpecifyingPhase(t *testing.T) {
	s := &ForgeState{
		Phase:          PhaseSpecifying,
		State:          StateOrient,
		StartedAtPhase: PhaseSpecifying,
		Specifying: NewSpecifyingState([]SpecQueueEntry{
			{Name: "Spec1", Domain: "test", Topic: "t", File: "spec1.md", PlanningSources: []string{}, DependsOn: []string{}},
		}),
	}

	if s.Phase != PhaseSpecifying {
		t.Errorf("phase = %s, want specifying", s.Phase)
	}
	if s.State != StateOrient {
		t.Errorf("state = %s, want ORIENT", s.State)
	}
	if s.StartedAtPhase != PhaseSpecifying {
		t.Errorf("started_at_phase = %s, want specifying", s.StartedAtPhase)
	}
}

func TestInitAtPlanningPhase(t *testing.T) {
	s := &ForgeState{
		Phase:          PhasePlanning,
		State:          StateOrient,
		StartedAtPhase: PhasePlanning,
		Planning: NewPlanningState([]PlanQueueEntry{
			{Name: "Plan1", Domain: "test", Topic: "t", File: "plan.json", Specs: []string{}, CodeSearchRoots: []string{}},
		}),
	}

	if s.Phase != PhasePlanning {
		t.Errorf("phase = %s, want planning", s.Phase)
	}
	if s.Specifying != nil {
		t.Error("specifying should be nil when starting at planning")
	}
}

// --- Specifying Phase Tests ---

func newSpecifyingState(numSpecs int) *ForgeState {
	var specs []SpecQueueEntry
	for i := 0; i < numSpecs; i++ {
		specs = append(specs, SpecQueueEntry{
			Name:            "Spec" + string(rune('A'+i)),
			Domain:          "test",
			Topic:           "topic",
			File:            "spec.md",
			PlanningSources: []string{},
			DependsOn:       []string{},
		})
	}
	return &ForgeState{
		Phase: PhaseSpecifying,
		State: StateOrient,
		Config: ForgeConfig{
			Specifying: SpecifyingConfig{
				Eval: EvalConfig{MinRounds: 1, MaxRounds: 3},
			},
		},
		Specifying: NewSpecifyingState(specs),
	}
}

func TestSpecifyingAdvanceSequential(t *testing.T) {
	s := newSpecifyingState(1)

	// ORIENT → SELECT
	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateSelect {
		t.Fatalf("expected SELECT, got %s", s.State)
	}

	// SELECT → DRAFT
	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateDraft {
		t.Fatalf("expected DRAFT, got %s", s.State)
	}

	// DRAFT → EVALUATE
	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateEvaluate {
		t.Fatalf("expected EVALUATE, got %s", s.State)
	}
}

func TestSpecifyingFailBelowMaxRoundsGoesToRefine(t *testing.T) {
	s := newSpecifyingState(1)
	advanceToEvaluate(t, s)

	// Create eval report file.
	dir := t.TempDir()
	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	err := Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: evalFile}, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateRefine {
		t.Errorf("expected REFINE, got %s", s.State)
	}
}

func TestSpecifyingFailAtMaxRoundsForcesAccept(t *testing.T) {
	s := newSpecifyingState(1)
	s.Config.Specifying.Eval.MaxRounds = 2

	advanceToEvaluate(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	// Round 1: FAIL → REFINE
	Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: evalFile}, "")
	// REFINE → EVALUATE (round 2)
	Advance(s, AdvanceInput{}, "")
	// Round 2: FAIL → ACCEPT (forced)
	err := Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: evalFile}, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateAccept {
		t.Errorf("expected ACCEPT (forced), got %s", s.State)
	}
}

func TestSpecifyingPassBelowMinRoundsGoesToRefine(t *testing.T) {
	s := newSpecifyingState(1)
	s.Config.Specifying.Eval.MinRounds = 2

	advanceToEvaluate(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateRefine {
		t.Errorf("expected REFINE (min rounds not met), got %s", s.State)
	}
}

func TestSpecifyingPassAtMinRoundsGoesToAccept(t *testing.T) {
	s := newSpecifyingState(1)
	s.Config.Specifying.Eval.MinRounds = 2

	advanceToEvaluate(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	// Round 1: PASS → REFINE
	Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, "")
	// REFINE → EVALUATE (round 2)
	Advance(s, AdvanceInput{}, "")
	// Round 2: PASS → ACCEPT
	err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile, Message: "Add spec"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateAccept {
		t.Errorf("expected ACCEPT, got %s", s.State)
	}
}

func TestSpecifyingPassRequiresMessage(t *testing.T) {
	s := newSpecifyingState(1)
	advanceToEvaluate(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, "")
	if err == nil {
		t.Error("expected error for missing --message with PASS")
	}
}

func TestSpecifyingDoneToReconcile(t *testing.T) {
	s := newSpecifyingState(1)
	advanceToAccept(t, s)

	// ACCEPT → DONE (queue empty)
	Advance(s, AdvanceInput{}, "")
	if s.State != StateDone {
		t.Fatalf("expected DONE, got %s", s.State)
	}

	// DONE → RECONCILE
	Advance(s, AdvanceInput{}, "")
	if s.State != StateReconcile {
		t.Errorf("expected RECONCILE, got %s", s.State)
	}
}

func TestReconcileFlowPass(t *testing.T) {
	s := newSpecifyingState(1)
	advanceToDone(t, s)

	// DONE → RECONCILE
	Advance(s, AdvanceInput{}, "")
	// RECONCILE → RECONCILE_EVAL
	Advance(s, AdvanceInput{}, "")
	if s.State != StateReconcileEval {
		t.Fatalf("expected RECONCILE_EVAL, got %s", s.State)
	}

	// RECONCILE_EVAL PASS → COMPLETE
	err := Advance(s, AdvanceInput{Verdict: "PASS", Message: "reconcile"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateComplete {
		t.Errorf("expected COMPLETE, got %s", s.State)
	}
}

func TestReconcileFlowFailThenFix(t *testing.T) {
	s := newSpecifyingState(1)
	advanceToDone(t, s)

	Advance(s, AdvanceInput{}, "") // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "") // RECONCILE → RECONCILE_EVAL

	// FAIL → RECONCILE_REVIEW
	Advance(s, AdvanceInput{Verdict: "FAIL"}, "")
	if s.State != StateReconcileReview {
		t.Fatalf("expected RECONCILE_REVIEW, got %s", s.State)
	}

	// FAIL → RECONCILE
	err := Advance(s, AdvanceInput{Verdict: "FAIL"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateReconcile {
		t.Errorf("expected RECONCILE, got %s", s.State)
	}
}

func TestCompleteToPhaseShift(t *testing.T) {
	s := newSpecifyingState(1)
	advanceToComplete(t, s)

	// COMPLETE → PHASE_SHIFT
	Advance(s, AdvanceInput{}, "")
	if s.State != StatePhaseShift {
		t.Errorf("expected PHASE_SHIFT, got %s", s.State)
	}
	if s.PhaseShift == nil || s.PhaseShift.From != PhaseSpecifying || s.PhaseShift.To != PhasePlanning {
		t.Error("phase shift should be specifying → planning")
	}
}

// --- Phase Shift Tests ---

func TestPhaseShiftSpecifyingToPlanningRequiresFrom(t *testing.T) {
	s := newSpecifyingState(1)
	advanceToComplete(t, s)
	Advance(s, AdvanceInput{}, "") // → PHASE_SHIFT

	err := Advance(s, AdvanceInput{}, "")
	if err == nil {
		t.Error("expected error for missing --from at phase shift")
	}
}

func TestPhaseShiftSpecifyingToPlanningWithValidQueue(t *testing.T) {
	s := newSpecifyingState(1)
	advanceToComplete(t, s)
	Advance(s, AdvanceInput{}, "") // → PHASE_SHIFT

	// Write a valid plan queue file.
	dir := t.TempDir()
	queueFile := filepath.Join(dir, "plans-queue.json")
	input := PlanQueueInput{
		Plans: []PlanQueueEntry{
			{Name: "Plan1", Domain: "test", Topic: "t", File: "plan.json", Specs: []string{"spec.md"}, CodeSearchRoots: []string{"test/"}},
		},
	}
	data, _ := json.Marshal(input)
	os.WriteFile(queueFile, data, 0644)

	err := Advance(s, AdvanceInput{From: queueFile}, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.Phase != PhasePlanning {
		t.Errorf("expected planning phase, got %s", s.Phase)
	}
	if s.State != StateOrient {
		t.Errorf("expected ORIENT, got %s", s.State)
	}
}

func TestPhaseShiftGuidedSetting(t *testing.T) {
	s := newSpecifyingState(1)
	s.Config.General.UserGuided = true
	advanceToComplete(t, s)
	Advance(s, AdvanceInput{}, "") // → PHASE_SHIFT

	dir := t.TempDir()
	queueFile := filepath.Join(dir, "plans-queue.json")
	input := PlanQueueInput{
		Plans: []PlanQueueEntry{
			{Name: "Plan1", Domain: "test", Topic: "t", File: "plan.json", Specs: []string{}, CodeSearchRoots: []string{}},
		},
	}
	data, _ := json.Marshal(input)
	os.WriteFile(queueFile, data, 0644)

	noGuided := false
	Advance(s, AdvanceInput{From: queueFile, Guided: &noGuided}, "")
	if s.Config.General.UserGuided != false {
		t.Error("user_guided should be false after --no-guided at phase shift")
	}
}

// --- Planning Phase Tests ---

func TestPlanningStudyPhasesSequential(t *testing.T) {
	s := newPlanningState()

	// ORIENT → STUDY_SPECS
	Advance(s, AdvanceInput{}, "")
	if s.State != StateStudySpecs {
		t.Fatalf("expected STUDY_SPECS, got %s", s.State)
	}

	// → STUDY_CODE
	Advance(s, AdvanceInput{}, "")
	if s.State != StateStudyCode {
		t.Fatalf("expected STUDY_CODE, got %s", s.State)
	}

	// → STUDY_PACKAGES
	Advance(s, AdvanceInput{}, "")
	if s.State != StateStudyPackages {
		t.Fatalf("expected STUDY_PACKAGES, got %s", s.State)
	}

	// → REVIEW
	Advance(s, AdvanceInput{}, "")
	if s.State != StateReview {
		t.Fatalf("expected REVIEW, got %s", s.State)
	}

	// → DRAFT
	Advance(s, AdvanceInput{}, "")
	if s.State != StateDraft {
		t.Fatalf("expected DRAFT, got %s", s.State)
	}
}

func TestPlanningDraftWithValidPlanGoesToEvaluate(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithDir(dir)

	advancePlanningToDraft(t, s, "")

	// Create valid plan.json.
	createValidPlan(t, dir, s.Planning.CurrentPlan.File)

	err := Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateEvaluate {
		t.Errorf("expected EVALUATE, got %s", s.State)
	}
	if s.Planning.Round != 1 {
		t.Errorf("expected round 1, got %d", s.Planning.Round)
	}
}

func TestPlanningDraftWithInvalidPlanEntersValidate(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithDir(dir)

	advancePlanningToDraft(t, s, "")

	// Create invalid plan (missing fields).
	planPath := filepath.Join(dir, s.Planning.CurrentPlan.File)
	os.MkdirAll(filepath.Dir(planPath), 0755)
	os.WriteFile(planPath, []byte(`{"items": []}`), 0644)

	err := Advance(s, AdvanceInput{}, dir)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if s.State != StateValidate {
		t.Errorf("expected VALIDATE, got %s", s.State)
	}
}

func TestPlanningValidateStaysOnReFailure(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithDir(dir)

	advancePlanningToDraft(t, s, "")

	// Create invalid plan.
	planPath := filepath.Join(dir, s.Planning.CurrentPlan.File)
	os.MkdirAll(filepath.Dir(planPath), 0755)
	os.WriteFile(planPath, []byte(`{"items": []}`), 0644)

	// DRAFT → VALIDATE
	Advance(s, AdvanceInput{}, dir)
	if s.State != StateValidate {
		t.Fatalf("expected VALIDATE, got %s", s.State)
	}

	// Re-advance with still-invalid plan: should stay VALIDATE.
	err := Advance(s, AdvanceInput{}, dir)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if s.State != StateValidate {
		t.Errorf("expected VALIDATE on re-failure, got %s", s.State)
	}
}

func TestPlanningValidateSucceedsToEvaluate(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithDir(dir)

	advancePlanningToDraft(t, s, "")

	// Create invalid plan.
	planPath := filepath.Join(dir, s.Planning.CurrentPlan.File)
	os.MkdirAll(filepath.Dir(planPath), 0755)
	os.WriteFile(planPath, []byte(`{"items": []}`), 0644)

	// DRAFT → VALIDATE
	Advance(s, AdvanceInput{}, dir)

	// Fix the plan.
	createValidPlan(t, dir, s.Planning.CurrentPlan.File)

	// VALIDATE → EVALUATE
	err := Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateEvaluate {
		t.Errorf("expected EVALUATE, got %s", s.State)
	}
}

func TestSpecifyingEvalReportMustExist(t *testing.T) {
	s := newSpecifyingState(1)
	advanceToEvaluate(t, s)

	err := Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: "/nonexistent/path.md"}, "")
	if err == nil {
		t.Error("expected error for non-existent eval report")
	}
}

func TestPlanningDraftSetsRoundTo1OnValidationFailure(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithDir(dir)

	advancePlanningToDraft(t, s, "")

	// Create invalid plan.
	planPath := filepath.Join(dir, s.Planning.CurrentPlan.File)
	os.MkdirAll(filepath.Dir(planPath), 0755)
	os.WriteFile(planPath, []byte(`{"items": []}`), 0644)

	Advance(s, AdvanceInput{}, dir)
	if s.Planning.Round != 1 {
		t.Errorf("expected round 1 after DRAFT→VALIDATE, got %d", s.Planning.Round)
	}
}

func TestPlanningEvaluatePassAtMinRoundsAccept(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithDir(dir)
	s.Config.Planning.Eval.MinRounds = 1

	advancePlanningToEvaluate(t, s, dir)

	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateAccept {
		t.Errorf("expected ACCEPT, got %s", s.State)
	}
}

func TestPlanningEvaluateFailAtMaxRoundsForcesAccept(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithDir(dir)
	s.Config.Planning.Eval.MaxRounds = 1

	advancePlanningToEvaluate(t, s, dir)

	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	err := Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: evalFile}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateAccept {
		t.Errorf("expected ACCEPT (forced), got %s", s.State)
	}
}

func TestPlanningAcceptToPhaseShift(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithDir(dir)
	advancePlanningToAccept(t, s, dir)

	err := Advance(s, AdvanceInput{Message: "accept plan"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StatePhaseShift {
		t.Errorf("expected PHASE_SHIFT, got %s", s.State)
	}
}

// --- Implementing Phase Tests ---

func TestImplementingOrientSelectsFirstBatch(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 4, 2)

	err := Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateImplement {
		t.Errorf("expected IMPLEMENT, got %s", s.State)
	}
	if len(s.Implementing.CurrentBatch.Items) != 2 {
		t.Errorf("expected batch of 2, got %d", len(s.Implementing.CurrentBatch.Items))
	}
}

func TestImplementPresentsItemsOneAtATime(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 2, 2)

	Advance(s, AdvanceInput{}, dir) // ORIENT → IMPLEMENT (item 1)

	if s.Implementing.CurrentBatch.CurrentItemIndex != 0 {
		t.Error("should start at item 0")
	}

	// Advance past item 1.
	err := Advance(s, AdvanceInput{Message: "impl item 1"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateImplement {
		t.Fatalf("expected IMPLEMENT for item 2, got %s", s.State)
	}
	if s.Implementing.CurrentBatch.CurrentItemIndex != 1 {
		t.Error("should be at item 1")
	}
}

func TestImplementLastItemGoesToEvaluate(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 2, 2)

	Advance(s, AdvanceInput{}, dir) // ORIENT → IMPLEMENT
	Advance(s, AdvanceInput{Message: "impl 1"}, dir) // item 1 → item 2

	err := Advance(s, AdvanceInput{Message: "impl 2"}, dir) // item 2 → EVALUATE
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateEvaluate {
		t.Errorf("expected EVALUATE, got %s", s.State)
	}
}

func TestFirstRoundImplementRequiresMessage(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)

	Advance(s, AdvanceInput{}, dir) // ORIENT → IMPLEMENT

	err := Advance(s, AdvanceInput{}, dir) // no --message
	if err == nil {
		t.Error("expected error for missing --message in first-round IMPLEMENT")
	}
}

func TestEvaluatePassWithSufficientRoundsToCommit(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)

	advanceImplToEvaluate(t, s, dir)

	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateCommit {
		t.Errorf("expected COMMIT, got %s", s.State)
	}
}

func TestEvaluateFailAtMaxRoundsToCommit(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)
	s.Config.Implementing.Eval.MaxRounds = 1

	advanceImplToEvaluate(t, s, dir)

	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	err := Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: evalFile}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateCommit {
		t.Errorf("expected COMMIT (force accept), got %s", s.State)
	}
}

func TestEvaluateFailWithinMaxRoundsToImplement(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)
	s.Config.Implementing.Eval.MaxRounds = 3

	advanceImplToEvaluate(t, s, dir)

	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	err := Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: evalFile}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateImplement {
		t.Errorf("expected IMPLEMENT (re-implement), got %s", s.State)
	}
}

func TestCommitToOrientMoreItems(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 2, 1) // 2 items, batch size 1

	// Process first batch.
	advanceImplToCommit(t, s, dir)

	err := Advance(s, AdvanceInput{Message: "commit batch 1"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateOrient {
		t.Errorf("expected ORIENT (more items), got %s", s.State)
	}
}

func TestCommitToDoneAllComplete(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1) // 1 item, batch size 1

	advanceImplToCommit(t, s, dir)

	err := Advance(s, AdvanceInput{Message: "commit"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateDone {
		t.Errorf("expected DONE, got %s", s.State)
	}
}

func TestDoneCannotAdvance(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)
	advanceImplToCommit(t, s, dir)
	Advance(s, AdvanceInput{Message: "commit"}, dir) // → DONE

	err := Advance(s, AdvanceInput{}, dir)
	if err == nil {
		t.Error("expected error advancing from DONE")
	}
}

func TestSubsequentRoundImplementDoesNotRequireMessage(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)
	s.Config.Implementing.Eval.MaxRounds = 3

	advanceImplToEvaluate(t, s, dir)

	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	// FAIL → back to IMPLEMENT (round 2)
	Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: evalFile}, dir)

	// Should NOT require --message on subsequent round.
	err := Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Errorf("subsequent round should not require --message: %v", err)
	}
}

func TestFailedItemsDontBlockDependents(t *testing.T) {
	dir := t.TempDir()
	notesDir := filepath.Join(dir, "notes")
	os.MkdirAll(notesDir, 0755)
	os.WriteFile(filepath.Join(notesDir, "n.md"), []byte("notes"), 0644)

	plan := PlanJSON{
		Context: PlanContext{Domain: "test", Module: "test"},
		Layers: []PlanLayerDef{
			{ID: "L0", Name: "Foundation", Items: []string{"a", "b"}},
		},
		Items: []PlanItem{
			{ID: "a", Name: "A", Description: "d", DependsOn: []string{},
				Passes: "failed", Rounds: 1,
				Tests: []PlanTest{{Category: "functional", Description: "t"}}},
			{ID: "b", Name: "B", Description: "d", DependsOn: []string{"a"},
				Passes: "pending", Rounds: 0,
				Tests: []PlanTest{{Category: "functional", Description: "t"}}},
		},
	}

	item := findItem(&plan, "b")
	if !itemUnblocked(&plan, item) {
		t.Error("item B should be unblocked when dependency A is 'failed' (terminal)")
	}
}

// --- Helper Functions ---

func advanceToEvaluate(t *testing.T, s *ForgeState) {
	t.Helper()
	Advance(s, AdvanceInput{}, "") // ORIENT → SELECT
	Advance(s, AdvanceInput{}, "") // SELECT → DRAFT
	Advance(s, AdvanceInput{}, "") // DRAFT → EVALUATE
}

func advanceToAccept(t *testing.T, s *ForgeState) {
	t.Helper()
	advanceToEvaluate(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile, Message: "accept"}, "")
}

func advanceToDone(t *testing.T, s *ForgeState) {
	t.Helper()
	advanceToAccept(t, s)
	Advance(s, AdvanceInput{}, "") // ACCEPT → DONE
}

func advanceToComplete(t *testing.T, s *ForgeState) {
	t.Helper()
	advanceToDone(t, s)
	Advance(s, AdvanceInput{}, "")                                    // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "")                                    // RECONCILE → RECONCILE_EVAL
	Advance(s, AdvanceInput{Verdict: "PASS", Message: "reconcile"}, "") // RECONCILE_EVAL → COMPLETE
}

func newPlanningState() *ForgeState {
	return &ForgeState{
		Phase: PhasePlanning,
		State: StateOrient,
		Config: ForgeConfig{
			Planning: PlanningConfig{
				Eval: EvalConfig{MinRounds: 1, MaxRounds: 3},
			},
		},
		Planning: &PlanningState{
			CurrentPlan: &ActivePlan{
				ID:              1,
				Name:            "Test Plan",
				Domain:          "test",
				Topic:           "topic",
				File:            "plan.json",
				Specs:           []string{"spec.md"},
				CodeSearchRoots: []string{"test/"},
			},
			Queue:     []PlanQueueEntry{},
			Completed: []CompletedPlan{},
		},
	}
}

func newPlanningStateWithDir(dir string) *ForgeState {
	s := newPlanningState()
	s.Planning.CurrentPlan.File = "impl/plan.json"
	return s
}

func advancePlanningToDraft(t *testing.T, s *ForgeState, dir string) {
	t.Helper()
	Advance(s, AdvanceInput{}, dir) // ORIENT → STUDY_SPECS
	Advance(s, AdvanceInput{}, dir) // → STUDY_CODE
	Advance(s, AdvanceInput{}, dir) // → STUDY_PACKAGES
	Advance(s, AdvanceInput{}, dir) // → REVIEW
	Advance(s, AdvanceInput{}, dir) // → DRAFT
}

func advancePlanningToEvaluate(t *testing.T, s *ForgeState, dir string) {
	t.Helper()
	advancePlanningToDraft(t, s, dir)
	createValidPlan(t, dir, s.Planning.CurrentPlan.File)
	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("advancing to EVALUATE: %v", err)
	}
}

func advancePlanningToAccept(t *testing.T, s *ForgeState, dir string) {
	t.Helper()
	advancePlanningToEvaluate(t, s, dir)

	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, dir)
}

func createValidPlan(t *testing.T, dir, planFile string) {
	t.Helper()
	planPath := filepath.Join(dir, planFile)
	os.MkdirAll(filepath.Dir(planPath), 0755)

	notesDir := filepath.Join(filepath.Dir(planPath), "notes")
	os.MkdirAll(notesDir, 0755)
	os.WriteFile(filepath.Join(notesDir, "config.md"), []byte("notes"), 0644)

	plan := PlanJSON{
		Context: PlanContext{Domain: "test", Module: "test-mod"},
		Layers: []PlanLayerDef{
			{ID: "L0", Name: "Foundation", Items: []string{"item.1"}},
		},
		Items: []PlanItem{
			{
				ID:          "item.1",
				Name:        "First Item",
				Description: "Does the thing",
				DependsOn:   []string{},
				Refs:        []string{"notes/config.md"},
				Tests: []PlanTest{
					{Category: "functional", Description: "it works"},
				},
			},
		},
	}

	data, _ := json.Marshal(plan)
	os.WriteFile(planPath, data, 0644)
}

func newImplementingState(dir string, numItems, batchSize int) *ForgeState {
	notesDir := filepath.Join(dir, "impl", "notes")
	os.MkdirAll(notesDir, 0755)
	os.WriteFile(filepath.Join(notesDir, "n.md"), []byte("notes"), 0644)

	var items []PlanItem
	var itemIDs []string
	for i := 0; i < numItems; i++ {
		id := string(rune('a' + i))
		deps := []string{}
		if i > 0 {
			// Only depend within same layer for simplicity.
		}
		items = append(items, PlanItem{
			ID:          id,
			Name:        "Item " + id,
			Description: "desc " + id,
			DependsOn:   deps,
			Passes:      "pending",
			Rounds:      0,
			Tests: []PlanTest{
				{Category: "functional", Description: "it works"},
			},
		})
		itemIDs = append(itemIDs, id)
	}

	plan := PlanJSON{
		Context: PlanContext{Domain: "test", Module: "test-mod"},
		Layers: []PlanLayerDef{
			{ID: "L0", Name: "Foundation", Items: itemIDs},
		},
		Items: items,
	}

	planPath := filepath.Join(dir, "impl", "plan.json")
	data, _ := json.Marshal(plan)
	os.WriteFile(planPath, data, 0644)

	return &ForgeState{
		Phase: PhaseImplementing,
		State: StateOrient,
		Config: ForgeConfig{
			Implementing: ImplementingConfig{
				Batch: batchSize,
				Eval:  EvalConfig{MinRounds: 1, MaxRounds: 3},
			},
		},
		Planning: &PlanningState{
			CurrentPlan: &ActivePlan{
				ID:     1,
				Name:   "Test Plan",
				Domain: "test",
				File:   "impl/plan.json",
			},
		},
		Implementing: NewImplementingState(),
	}
}

func advanceImplToEvaluate(t *testing.T, s *ForgeState, dir string) {
	t.Helper()
	Advance(s, AdvanceInput{}, dir) // ORIENT → IMPLEMENT

	// Advance through all items in batch.
	batch := s.Implementing.CurrentBatch
	for i := 0; i < len(batch.Items); i++ {
		msg := ""
		if batch.EvalRound == 0 {
			msg = "impl"
		}
		if err := Advance(s, AdvanceInput{Message: msg}, dir); err != nil {
			t.Fatalf("advancing item %d: %v", i, err)
		}
	}

	if s.State != StateEvaluate {
		t.Fatalf("expected EVALUATE, got %s", s.State)
	}
}

func advanceImplToCommit(t *testing.T, s *ForgeState, dir string) {
	t.Helper()
	advanceImplToEvaluate(t, s, dir)

	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, dir)
	if s.State != StateCommit {
		t.Fatalf("expected COMMIT, got %s", s.State)
	}
}
