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
	if s.PhaseShift == nil || s.PhaseShift.From != PhaseSpecifying || s.PhaseShift.To != PhaseGeneratePlanningQueue {
		t.Error("phase shift should be specifying → generate_planning_queue")
	}
}

// --- Phase Shift Tests ---

func TestPhaseShiftSpecifyingAutoGeneratesQueue(t *testing.T) {
	dir := t.TempDir()
	s := newSpecifyingStateWithConfig(1, dir)
	advanceToComplete(t, s)
	Advance(s, AdvanceInput{}, dir) // COMPLETE → PHASE_SHIFT

	// Without --from: auto-generates plan queue, enters generate_planning_queue ORIENT.
	err := Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Phase != PhaseGeneratePlanningQueue {
		t.Errorf("expected generate_planning_queue phase, got %s", s.Phase)
	}
	if s.State != StateOrient {
		t.Errorf("expected ORIENT, got %s", s.State)
	}
	if s.GeneratePlanningQueue == nil || s.GeneratePlanningQueue.PlanQueueFile == "" {
		t.Error("expected GeneratePlanningQueue.PlanQueueFile to be set")
	}
}

func TestPhaseShiftSpecifyingWithFromSkipsGenqueue(t *testing.T) {
	dir := t.TempDir()
	s := newSpecifyingState(1)
	advanceToComplete(t, s)
	Advance(s, AdvanceInput{}, dir) // COMPLETE → PHASE_SHIFT

	// With --from: skip genqueue, go straight to planning.
	queueFile := filepath.Join(dir, "plans-queue.json")
	input := PlanQueueInput{
		Plans: []PlanQueueEntry{
			{Name: "Plan1", Domain: "test", Topic: "t", File: "plan.json", Specs: []string{"spec.md"}, CodeSearchRoots: []string{"test/"}},
		},
	}
	data, _ := json.Marshal(input)
	os.WriteFile(queueFile, data, 0644)

	err := Advance(s, AdvanceInput{From: queueFile}, dir)
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

func TestPhaseShiftSpecifyingWithInvalidFromRejected(t *testing.T) {
	dir := t.TempDir()
	s := newSpecifyingState(1)
	advanceToComplete(t, s)
	Advance(s, AdvanceInput{}, dir) // COMPLETE → PHASE_SHIFT

	// With invalid --from: error, stays PHASE_SHIFT.
	queueFile := filepath.Join(dir, "bad-queue.json")
	os.WriteFile(queueFile, []byte(`{"plans": []}`), 0644) // empty plans — invalid

	err := Advance(s, AdvanceInput{From: queueFile}, dir)
	if err == nil {
		t.Fatal("expected validation error for empty plans")
	}
	if s.State != StatePhaseShift {
		t.Errorf("expected PHASE_SHIFT on error, got %s", s.State)
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

// --- Generate Planning Queue Phase Tests ---

func TestGenqueueOrientToRefine(t *testing.T) {
	dir := t.TempDir()
	s := newGenqueueState(dir)

	err := Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateRefine {
		t.Errorf("expected REFINE, got %s", s.State)
	}
}

func TestGenqueueRefineWithInvalidQueueStaysRefine(t *testing.T) {
	dir := t.TempDir()
	s := newGenqueueState(dir)
	Advance(s, AdvanceInput{}, dir) // → REFINE

	// Write invalid plan-queue.json.
	queuePath := filepath.Join(dir, s.GeneratePlanningQueue.PlanQueueFile)
	os.MkdirAll(filepath.Dir(queuePath), 0755)
	os.WriteFile(queuePath, []byte(`{"plans": []}`), 0644)

	err := Advance(s, AdvanceInput{}, dir)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if s.State != StateRefine {
		t.Errorf("expected REFINE on validation failure, got %s", s.State)
	}
}

func TestGenqueueRefineWithValidQueueToPhaseShift(t *testing.T) {
	dir := t.TempDir()
	s := newGenqueueState(dir)
	Advance(s, AdvanceInput{}, dir) // → REFINE

	// Write valid plan-queue.json.
	writeValidPlanQueue(t, dir, s.GeneratePlanningQueue.PlanQueueFile)

	err := Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StatePhaseShift {
		t.Errorf("expected PHASE_SHIFT, got %s", s.State)
	}
	if s.PhaseShift == nil || s.PhaseShift.From != PhaseGeneratePlanningQueue || s.PhaseShift.To != PhasePlanning {
		t.Error("phase shift should be generate_planning_queue → planning")
	}
}

func TestGenqueuePhaseShiftToPlanningWithoutFrom(t *testing.T) {
	dir := t.TempDir()
	s := newGenqueueState(dir)
	Advance(s, AdvanceInput{}, dir)                       // → REFINE
	writeValidPlanQueue(t, dir, s.GeneratePlanningQueue.PlanQueueFile)
	Advance(s, AdvanceInput{}, dir) // → PHASE_SHIFT

	err := Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Phase != PhasePlanning {
		t.Errorf("expected planning phase, got %s", s.Phase)
	}
	if s.State != StateOrient {
		t.Errorf("expected ORIENT, got %s", s.State)
	}
	if s.Planning == nil || s.Planning.CurrentPlan == nil {
		t.Error("expected planning state to be populated")
	}
}

func TestGenqueuePhaseShiftToPlanningWithFromOverride(t *testing.T) {
	dir := t.TempDir()
	s := newGenqueueState(dir)
	Advance(s, AdvanceInput{}, dir) // → REFINE
	writeValidPlanQueue(t, dir, s.GeneratePlanningQueue.PlanQueueFile)
	Advance(s, AdvanceInput{}, dir) // → PHASE_SHIFT

	// Override with a different plan queue.
	overrideFile := filepath.Join(dir, "override-queue.json")
	input := PlanQueueInput{
		Plans: []PlanQueueEntry{
			{Name: "Override Plan", Domain: "override", Topic: "override", File: "override/plan.json", Specs: []string{"s.md"}, CodeSearchRoots: []string{"override/"}},
		},
	}
	data, _ := json.Marshal(input)
	os.WriteFile(overrideFile, data, 0644)

	err := Advance(s, AdvanceInput{From: overrideFile}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Phase != PhasePlanning {
		t.Errorf("expected planning phase, got %s", s.Phase)
	}
	if s.Planning == nil || s.Planning.CurrentPlan == nil || s.Planning.CurrentPlan.Domain != "override" {
		t.Errorf("expected override domain in planning, got %v", s.Planning)
	}
}

func TestAutoGeneratePlanQueueGroupsByDomain(t *testing.T) {
	dir := t.TempDir()
	s := newSpecifyingStateWithConfig(0, dir)
	s.Specifying = &SpecifyingState{
		Completed: []CompletedSpec{
			{ID: 1, Name: "Spec1", Domain: "alpha", File: "alpha/specs/a.md"},
			{ID: 2, Name: "Spec2", Domain: "beta", File: "beta/specs/b.md"},
			{ID: 3, Name: "Spec3", Domain: "alpha", File: "alpha/specs/c.md"},
		},
		Queue: []SpecQueueEntry{},
	}

	outPath, err := autoGeneratePlanQueue(s, dir)
	if err != nil {
		t.Fatal(err)
	}
	if outPath == "" {
		t.Fatal("expected non-empty output path")
	}

	data, err := os.ReadFile(filepath.Join(dir, outPath))
	if err != nil {
		t.Fatal(err)
	}
	var result PlanQueueInput
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}

	if len(result.Plans) != 2 {
		t.Fatalf("expected 2 domain plans, got %d", len(result.Plans))
	}
	// Order: alpha first (first appearance), beta second.
	if result.Plans[0].Domain != "alpha" {
		t.Errorf("expected first domain alpha, got %s", result.Plans[0].Domain)
	}
	if result.Plans[1].Domain != "beta" {
		t.Errorf("expected second domain beta, got %s", result.Plans[1].Domain)
	}
	// Alpha should have both its specs.
	if len(result.Plans[0].Specs) != 2 {
		t.Errorf("expected 2 specs for alpha, got %d", len(result.Plans[0].Specs))
	}
}

func TestAutoGenerateUsesSetRoots(t *testing.T) {
	dir := t.TempDir()
	s := newSpecifyingStateWithConfig(0, dir)
	s.Specifying = &SpecifyingState{
		Completed: []CompletedSpec{
			{ID: 1, Name: "Spec1", Domain: "mydom", File: "mydom/specs/a.md"},
		},
		Queue:       []SpecQueueEntry{},
		DomainRoots: map[string][]string{"mydom": {"mydom/src/", "mydom/pkg/"}},
	}

	outPath, err := autoGeneratePlanQueue(s, dir)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, outPath))
	var result PlanQueueInput
	json.Unmarshal(data, &result)

	if len(result.Plans[0].CodeSearchRoots) != 2 || result.Plans[0].CodeSearchRoots[0] != "mydom/src/" {
		t.Errorf("expected configured roots, got %v", result.Plans[0].CodeSearchRoots)
	}
}

// newGenqueueState creates a state already in generate_planning_queue ORIENT.
func newGenqueueState(dir string) *ForgeState {
	s := newSpecifyingStateWithConfig(1, dir)
	// Set up a plan queue file path (not yet written).
	s.Phase = PhaseGeneratePlanningQueue
	s.State = StateOrient
	s.GeneratePlanningQueue = &GeneratePlanningQueueState{
		PlanQueueFile: ".forgectl/state/plan-queue.json",
	}
	return s
}

// writeValidPlanQueue writes a valid plan-queue.json to the state path.
func writeValidPlanQueue(t *testing.T, dir, relPath string) {
	t.Helper()
	fullPath := filepath.Join(dir, relPath)
	os.MkdirAll(filepath.Dir(fullPath), 0755)
	input := PlanQueueInput{
		Plans: []PlanQueueEntry{
			{Name: "Test Plan", Domain: "test", Topic: "t", File: "test/plan.json", Specs: []string{"spec.md"}, CodeSearchRoots: []string{"test/"}},
		},
	}
	data, _ := json.Marshal(input)
	os.WriteFile(fullPath, data, 0644)
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
	s.Config.Specifying.Eval.EnableEvalOutput = true // require eval report
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

func TestPlanningSelfReviewEnabledValidateToSelfReviewToEvaluate(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithDir(dir)
	s.Config.Planning.SelfReview = true

	advancePlanningToDraft(t, s, "")

	// Create invalid plan: DRAFT → VALIDATE.
	planPath := filepath.Join(dir, s.Planning.CurrentPlan.File)
	os.MkdirAll(filepath.Dir(planPath), 0755)
	os.WriteFile(planPath, []byte(`{"items": []}`), 0644)
	Advance(s, AdvanceInput{}, dir)
	if s.State != StateValidate {
		t.Fatalf("expected VALIDATE, got %s", s.State)
	}

	// Fix plan: VALIDATE → SELF_REVIEW.
	createValidPlan(t, dir, s.Planning.CurrentPlan.File)
	err := Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateSelfReview {
		t.Fatalf("expected SELF_REVIEW, got %s", s.State)
	}

	// SELF_REVIEW → EVALUATE.
	err = Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateEvaluate {
		t.Errorf("expected EVALUATE, got %s", s.State)
	}
}

func TestPlanningSelfReviewDisabledValidateToEvaluate(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithDir(dir)
	s.Config.Planning.SelfReview = false

	advancePlanningToDraft(t, s, "")

	// Create invalid plan: DRAFT → VALIDATE.
	planPath := filepath.Join(dir, s.Planning.CurrentPlan.File)
	os.MkdirAll(filepath.Dir(planPath), 0755)
	os.WriteFile(planPath, []byte(`{"items": []}`), 0644)
	Advance(s, AdvanceInput{}, dir)

	// Fix plan: VALIDATE → EVALUATE (skips SELF_REVIEW).
	createValidPlan(t, dir, s.Planning.CurrentPlan.File)
	err := Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StateEvaluate {
		t.Errorf("expected EVALUATE, got %s", s.State)
	}
}

func TestPlanningSelfReviewInvalidPlanEntersValidate(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithDir(dir)
	s.Config.Planning.SelfReview = true

	advancePlanningToDraft(t, s, "")
	createValidPlan(t, dir, s.Planning.CurrentPlan.File)

	// DRAFT → SELF_REVIEW (valid plan, self_review=true).
	Advance(s, AdvanceInput{}, dir)
	if s.State != StateSelfReview {
		t.Fatalf("expected SELF_REVIEW, got %s", s.State)
	}

	// Agent invalidates plan.json during review.
	planPath := filepath.Join(dir, s.Planning.CurrentPlan.File)
	os.WriteFile(planPath, []byte(`{"items": []}`), 0644)

	// SELF_REVIEW → VALIDATE (invalid plan).
	err := Advance(s, AdvanceInput{}, dir)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if s.State != StateValidate {
		t.Errorf("expected VALIDATE, got %s", s.State)
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

// --- Multi-Plan Phase Transition Tests ---

func TestPlanningAcceptInterleavedGoesToImplementing(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithDir(dir)
	// Default: PlanAllBeforeImplementing=false.
	advancePlanningToAccept(t, s, dir)

	err := Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StatePhaseShift {
		t.Fatalf("expected PHASE_SHIFT, got %s", s.State)
	}
	if s.PhaseShift == nil || s.PhaseShift.From != PhasePlanning || s.PhaseShift.To != PhaseImplementing {
		t.Errorf("expected planning→implementing, got %v", s.PhaseShift)
	}
}

func TestPlanningAcceptAllFirstWithQueueGoesToPlanningPlanning(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithTwoPlans(dir)
	s.Config.Planning.PlanAllBeforeImplementing = true
	advancePlanningToAccept(t, s, dir) // accepts plan1

	err := Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StatePhaseShift {
		t.Fatalf("expected PHASE_SHIFT, got %s", s.State)
	}
	if s.PhaseShift == nil || s.PhaseShift.From != PhasePlanning || s.PhaseShift.To != PhasePlanning {
		t.Errorf("expected planning→planning, got %v", s.PhaseShift)
	}
	if s.Planning.CurrentPlan == nil || s.Planning.CurrentPlan.Name != "Plan2" {
		t.Errorf("expected Plan2 as current, got %v", s.Planning.CurrentPlan)
	}
	if len(s.Planning.Completed) != 1 || s.Planning.Completed[0].Domain != "test" {
		t.Errorf("expected 1 completed plan (test domain), got %v", s.Planning.Completed)
	}
}

func TestPlanningAcceptAllFirstLastPlanGoesToImplementing(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithDir(dir) // single plan, no queue
	s.Config.Planning.PlanAllBeforeImplementing = true
	advancePlanningToAccept(t, s, dir)

	err := Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.State != StatePhaseShift {
		t.Fatalf("expected PHASE_SHIFT, got %s", s.State)
	}
	if s.PhaseShift == nil || s.PhaseShift.From != PhasePlanning || s.PhaseShift.To != PhaseImplementing {
		t.Errorf("expected planning→implementing, got %v", s.PhaseShift)
	}
	if len(s.Planning.Completed) != 1 {
		t.Errorf("expected 1 completed plan, got %d", len(s.Planning.Completed))
	}
}

func TestPhaseShiftPlanningToPlanningResetsRound(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithTwoPlans(dir)
	s.Config.Planning.PlanAllBeforeImplementing = true
	advancePlanningToAccept(t, s, dir)
	Advance(s, AdvanceInput{}, dir) // ACCEPT → PHASE_SHIFT(planning→planning)

	// Advance through phase shift.
	err := Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Phase != PhasePlanning {
		t.Errorf("expected planning phase, got %s", s.Phase)
	}
	if s.State != StateOrient {
		t.Errorf("expected ORIENT, got %s", s.State)
	}
	if s.Planning.Round != 0 {
		t.Errorf("expected round reset to 0, got %d", s.Planning.Round)
	}
}

func TestPhaseShiftPlanningToImplementingSetsCurrentPlanFile(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithDir(dir)
	advancePlanningToAccept(t, s, dir)
	Advance(s, AdvanceInput{}, dir) // ACCEPT → PHASE_SHIFT(planning→implementing)

	// Set up the plan.json.
	createValidPlan(t, dir, "impl/plan.json")

	err := Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Phase != PhaseImplementing {
		t.Errorf("expected implementing phase, got %s", s.Phase)
	}
	if s.Implementing == nil || s.Implementing.CurrentPlanFile != "impl/plan.json" {
		t.Errorf("expected CurrentPlanFile=impl/plan.json, got %v", s.Implementing)
	}
}

func TestImplementingDoneInterleavedWithRemainingPlansToPlanning(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingStateWithPlanningQueue(dir, 1, 1)

	// Complete the implementing phase.
	advanceImplementingToCommit(t, s, dir)
	Advance(s, AdvanceInput{}, dir) // COMMIT → ORIENT
	Advance(s, AdvanceInput{}, dir) // ORIENT → DONE

	if s.State != StatePhaseShift {
		t.Fatalf("expected PHASE_SHIFT after DONE with plans in queue, got %s", s.State)
	}
	if s.PhaseShift == nil || s.PhaseShift.From != PhaseImplementing || s.PhaseShift.To != PhasePlanning {
		t.Errorf("expected implementing→planning, got %v", s.PhaseShift)
	}
}

func TestPhaseShiftImplementingToPlanningPopsPlan(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingStateWithPlanningQueue(dir, 1, 1)

	// Reach DONE with plans remaining.
	advanceImplementingToCommit(t, s, dir)
	Advance(s, AdvanceInput{}, dir) // COMMIT → ORIENT
	Advance(s, AdvanceInput{}, dir) // ORIENT → DONE (now PHASE_SHIFT)

	// Advance through phase shift.
	err := Advance(s, AdvanceInput{}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.Phase != PhasePlanning {
		t.Errorf("expected planning phase, got %s", s.Phase)
	}
	if s.State != StateOrient {
		t.Errorf("expected ORIENT, got %s", s.State)
	}
	if s.Planning.CurrentPlan == nil {
		t.Error("expected Planning.CurrentPlan to be set")
	}
}

func TestImplementingDoneAllFirstWithRemainingToImplementing(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingStateWithPlanQueue(dir, 1, 1)
	s.Config.Planning.PlanAllBeforeImplementing = true

	// Complete implementing phase.
	advanceImplementingToCommit(t, s, dir)
	Advance(s, AdvanceInput{}, dir) // COMMIT → ORIENT
	Advance(s, AdvanceInput{}, dir) // ORIENT → DONE (→ PHASE_SHIFT)

	if s.State != StatePhaseShift {
		t.Fatalf("expected PHASE_SHIFT, got %s", s.State)
	}
	if s.PhaseShift == nil || s.PhaseShift.From != PhaseImplementing || s.PhaseShift.To != PhaseImplementing {
		t.Errorf("expected implementing→implementing, got %v", s.PhaseShift)
	}
}

func TestPlanningDoneRejectsFlags(t *testing.T) {
	s := newPlanningState()
	s.State = StateDone

	err := Advance(s, AdvanceInput{Verdict: "PASS"}, "")
	if err == nil {
		t.Error("expected error for flags in planning DONE state")
	}
	if err != nil && err.Error() != "DONE is a pass-through state. No flags accepted." {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestImplementingDoneNoPlansIsTerminal(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)
	// No planning queue, PlanAllBeforeImplementing=false.

	advanceImplementingToCommit(t, s, dir)
	Advance(s, AdvanceInput{}, dir) // COMMIT → ORIENT
	Advance(s, AdvanceInput{}, dir) // ORIENT → DONE (terminal)

	// DONE with no plans should return error.
	err := Advance(s, AdvanceInput{}, dir)
	if err == nil {
		t.Error("expected terminal error from DONE with no plans")
	}
}

// newPlanningStateWithTwoPlans creates a planning state with plan1 as current and plan2 in queue.
func newPlanningStateWithTwoPlans(dir string) *ForgeState {
	s := newPlanningStateWithDir(dir) // plan1 as CurrentPlan
	s.Planning.Queue = []PlanQueueEntry{
		{Name: "Plan2", Domain: "test2", Topic: "t2", File: "impl2/plan.json", Specs: []string{"spec2.md"}, CodeSearchRoots: []string{"test2/"}},
	}
	return s
}

// newImplementingStateWithPlanningQueue creates an implementing state with plans in Planning.Queue (interleaved mode).
func newImplementingStateWithPlanningQueue(dir string, numItems, batchSize int) *ForgeState {
	s := newImplementingState(dir, numItems, batchSize)
	// Preserve Planning.CurrentPlan, just add to the Queue.
	s.Planning.Queue = []PlanQueueEntry{
		{Name: "Next Plan", Domain: "next", Topic: "t", File: "next/plan.json", Specs: []string{"s.md"}, CodeSearchRoots: []string{"next/"}},
	}
	return s
}

// newImplementingStateWithPlanQueue creates an implementing state with plans in Implementing.PlanQueue (all-first mode).
func newImplementingStateWithPlanQueue(dir string, numItems, batchSize int) *ForgeState {
	s := newImplementingState(dir, numItems, batchSize)

	// Create a valid plan.json for the next plan.
	nextPlanFile := "next/plan.json"
	notesDir := filepath.Join(dir, "next", "notes")
	os.MkdirAll(notesDir, 0755)
	os.WriteFile(filepath.Join(notesDir, "n.md"), []byte("notes"), 0644)

	nextPlan := PlanJSON{
		Context: PlanContext{Domain: "next", Module: "next-mod"},
		Layers:  []PlanLayerDef{{ID: "L0", Name: "Foundation", Items: []string{"next.1"}}},
		Items: []PlanItem{
			{ID: "next.1", Name: "Next Item", Description: "does thing", DependsOn: []string{}, Refs: []string{"notes/n.md"}, Tests: []PlanTest{{Category: "functional", Description: "works"}}},
		},
	}
	data, _ := json.Marshal(nextPlan)
	os.MkdirAll(filepath.Join(dir, "next"), 0755)
	os.WriteFile(filepath.Join(dir, nextPlanFile), data, 0644)

	s.Implementing.PlanQueue = []PlanQueueEntry{
		{Name: "Next Plan", Domain: "next", File: nextPlanFile, Specs: []string{}, CodeSearchRoots: []string{"next/"}},
	}
	return s
}

// advanceImplementingToCommit advances to the COMMIT state (all items done, eval passed).
func advanceImplementingToCommit(t *testing.T, s *ForgeState, dir string) {
	t.Helper()
	Advance(s, AdvanceInput{}, dir) // ORIENT → IMPLEMENT

	// Advance through all batch items.
	for s.State == StateImplement {
		Advance(s, AdvanceInput{}, dir)
	}

	// EVALUATE → COMMIT.
	if s.State == StateEvaluate {
		Advance(s, AdvanceInput{Verdict: "PASS"}, dir)
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
	s.Config.General.EnableCommits = true // require message when commits enabled

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

// newSpecifyingStateWithConfig creates a specifying state with paths config so auto-generation can write files.
func newSpecifyingStateWithConfig(numSpecs int, dir string) *ForgeState {
	s := newSpecifyingState(numSpecs)
	s.Config.Paths = PathsConfig{
		StateDir:     ".forgectl/state",
		WorkspaceDir: ".forge_workspace",
	}
	return s
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
