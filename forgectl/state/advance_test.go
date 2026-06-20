package state

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
			{Name: "Plan1", Domain: "test", File: "plan.json", Specs: []string{}, SpecCommits: []string{}, CodeSearchRoots: []string{}},
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

func makeTestConfig(batchSize, minRounds, maxRounds int) ForgeConfig {
	cfg := DefaultForgeConfig()
	cfg.Implementing.Batch = batchSize
	cfg.Implementing.Eval.MinRounds = minRounds
	cfg.Implementing.Eval.MaxRounds = maxRounds
	cfg.Specifying.Eval.MinRounds = minRounds
	cfg.Specifying.Eval.MaxRounds = maxRounds
	cfg.Planning.Eval.MinRounds = minRounds
	cfg.Planning.Eval.MaxRounds = maxRounds
	return cfg
}

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

func TestSpecifyingPassMessageNotRequiredWithoutEnableCommits(t *testing.T) {
	// enable_commits defaults to false — message should not be required.
	s := newSpecifyingState(1)
	advanceToEvaluate(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, "")
	if err != nil {
		t.Errorf("expected no error without enable_commits, got: %v", err)
	}
}

func TestSpecifyingPassRequiresMessageWhenEnableCommits(t *testing.T) {
	// Per spec, --message is required at COMPLETE (not EVALUATE) when enable_commits=true.
	// At EVALUATE, PASS with eval-report should succeed and advance to ACCEPT.
	s := newSpecifyingState(1)
	s.Config.General.EnableCommits = true
	s.Config.Specifying.Eval.EnableEvalOutput = true
	advanceToEvaluate(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, "")
	if err != nil {
		t.Errorf("EVALUATE PASS with eval-report should succeed: %v", err)
	}
	if s.State != StateAccept {
		t.Errorf("expected ACCEPT after PASS at min_rounds, got %s", s.State)
	}
}

func TestSpecifyingAcceptGoesToCrossReference(t *testing.T) {
	s := newSpecifyingState(1)
	advanceToAccept(t, s)

	// ACCEPT → CROSS_REFERENCE (domain done, queue empty)
	Advance(s, AdvanceInput{}, "")
	if s.State != StateCrossReference {
		t.Fatalf("expected CROSS_REFERENCE, got %s", s.State)
	}
}

// --- Batch Processing Tests ---

func newSpecifyingStateWithSpecs(specs []SpecQueueEntry) *ForgeState {
	return &ForgeState{
		Phase:      PhaseSpecifying,
		State:      StateOrient,
		Config:     makeTestConfig(2, 1, 3),
		Specifying: NewSpecifyingState(specs),
	}
}

func TestBatchSelectionSameDomain(t *testing.T) {
	// With batch_size=2 and 3 same-domain specs, ORIENT selects first 2.
	specs := []SpecQueueEntry{
		{Name: "A", Domain: "test", Topic: "t", File: "a.md"},
		{Name: "B", Domain: "test", Topic: "t", File: "b.md"},
		{Name: "C", Domain: "test", Topic: "t", File: "c.md"},
	}
	s := newSpecifyingStateWithSpecs(specs)
	s.Config.Specifying.Batch = 2

	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateSelect {
		t.Fatalf("expected SELECT, got %s", s.State)
	}
	if len(s.Specifying.CurrentSpecs) != 2 {
		t.Errorf("expected 2 specs in batch, got %d", len(s.Specifying.CurrentSpecs))
	}
	if len(s.Specifying.Queue) != 1 {
		t.Errorf("expected 1 spec remaining in queue, got %d", len(s.Specifying.Queue))
	}
	if s.Specifying.BatchNumber != 1 {
		t.Errorf("expected BatchNumber=1, got %d", s.Specifying.BatchNumber)
	}
}

func TestBatchSelectionStopsAtDomainBoundary(t *testing.T) {
	// Batch stops at domain boundary — specs from "other" stay in queue.
	specs := []SpecQueueEntry{
		{Name: "A", Domain: "test", Topic: "t", File: "a.md"},
		{Name: "B", Domain: "other", Topic: "t", File: "b.md"},
		{Name: "C", Domain: "test", Topic: "t", File: "c.md"},
	}
	s := newSpecifyingStateWithSpecs(specs)
	s.Config.Specifying.Batch = 3

	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}
	if len(s.Specifying.CurrentSpecs) != 1 {
		t.Errorf("expected 1 spec in batch (contiguous boundary), got %d", len(s.Specifying.CurrentSpecs))
	}
	if s.Specifying.CurrentSpecs[0].Name != "A" {
		t.Errorf("expected spec A in batch, got %s", s.Specifying.CurrentSpecs[0].Name)
	}
	if len(s.Specifying.Queue) != 2 {
		t.Errorf("expected 2 specs in queue, got %d", len(s.Specifying.Queue))
	}
}

func TestBatchEvalRecordAppliedToAllSpecs(t *testing.T) {
	// EVALUATE applies eval record to ALL specs in batch.
	specs := []SpecQueueEntry{
		{Name: "A", Domain: "test", Topic: "t", File: "a.md"},
		{Name: "B", Domain: "test", Topic: "t", File: "b.md"},
	}
	s := newSpecifyingStateWithSpecs(specs)
	s.Config.Specifying.Batch = 2

	advanceToEvaluate(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	if err := Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: evalFile}, ""); err != nil {
		t.Fatal(err)
	}

	for i, cs := range s.Specifying.CurrentSpecs {
		if len(cs.Evals) != 1 {
			t.Errorf("spec[%d] expected 1 eval record, got %d", i, len(cs.Evals))
		}
	}
}

func TestBatchAcceptMovesAllSpecsToCompleted(t *testing.T) {
	// ACCEPT moves all current_specs to completed.
	specs := []SpecQueueEntry{
		{Name: "A", Domain: "test", Topic: "t", File: "a.md"},
		{Name: "B", Domain: "test", Topic: "t", File: "b.md"},
	}
	s := newSpecifyingStateWithSpecs(specs)
	s.Config.Specifying.Batch = 2

	advanceToAccept(t, s)
	if len(s.Specifying.CurrentSpecs) != 2 {
		t.Fatalf("expected 2 specs in batch before accept advance")
	}

	// ACCEPT → CROSS_REFERENCE
	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}
	if len(s.Specifying.Completed) != 2 {
		t.Errorf("expected 2 completed specs, got %d", len(s.Specifying.Completed))
	}
	if s.Specifying.CurrentSpecs != nil {
		t.Errorf("expected current_specs to be nil after accept")
	}
	for i, c := range s.Specifying.Completed {
		if c.BatchNumber != 1 {
			t.Errorf("completed[%d] expected BatchNumber=1, got %d", i, c.BatchNumber)
		}
	}
}

func TestBatchAcceptSameDomainRemainingGoesToOrient(t *testing.T) {
	// When same domain has more queued specs, ACCEPT transitions to ORIENT.
	specs := []SpecQueueEntry{
		{Name: "A", Domain: "test", Topic: "t", File: "a.md"},
		{Name: "B", Domain: "test", Topic: "t", File: "b.md"},
	}
	s := newSpecifyingStateWithSpecs(specs)
	s.Config.Specifying.Batch = 1 // batch of 1, so B stays queued

	advanceToAccept(t, s)
	// ACCEPT → ORIENT (same domain still in queue)
	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateOrient {
		t.Errorf("expected ORIENT (same domain in queue), got %s", s.State)
	}
}

func TestCrossReferenceFlow(t *testing.T) {
	// Full CROSS_REFERENCE → CROSS_REFERENCE_EVAL → CROSS_REFERENCE_REVIEW → DONE.
	s := newSpecifyingState(1)
	advanceToAccept(t, s)

	dir := t.TempDir()
	crEvalFile := filepath.Join(dir, "cr-eval.md")
	os.WriteFile(crEvalFile, []byte("cross-ref eval"), 0644)

	// ACCEPT → CROSS_REFERENCE
	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateCrossReference {
		t.Fatalf("expected CROSS_REFERENCE, got %s", s.State)
	}

	// CROSS_REFERENCE → CROSS_REFERENCE_EVAL
	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateCrossReferenceEval {
		t.Fatalf("expected CROSS_REFERENCE_EVAL, got %s", s.State)
	}

	// CROSS_REFERENCE_EVAL PASS → CROSS_REFERENCE_REVIEW
	if err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: crEvalFile}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateCrossReferenceReview {
		t.Fatalf("expected CROSS_REFERENCE_REVIEW, got %s", s.State)
	}

	// CROSS_REFERENCE_REVIEW → DONE (queue empty)
	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateDone {
		t.Errorf("expected DONE, got %s", s.State)
	}
}

func TestCrossReferenceEvalRequiresVerdictAndReport(t *testing.T) {
	// CROSS_REFERENCE_EVAL must reject advance without verdict.
	// eval-report is required only when enable_eval_output=true.
	s := newSpecifyingState(1)
	s.Config.Specifying.Eval.EnableEvalOutput = true
	advanceToAccept(t, s)
	Advance(s, AdvanceInput{}, "")  // ACCEPT → CROSS_REFERENCE
	Advance(s, AdvanceInput{}, "")  // CROSS_REFERENCE → CROSS_REFERENCE_EVAL

	// Missing both.
	err := Advance(s, AdvanceInput{}, "")
	if err == nil {
		t.Error("expected error for missing --verdict in CROSS_REFERENCE_EVAL")
	}

	// Verdict present but missing eval-report (enable_eval_output=true).
	err = Advance(s, AdvanceInput{Verdict: "PASS"}, "")
	if err == nil {
		t.Error("expected error for missing --eval-report in CROSS_REFERENCE_EVAL when enable_eval_output=true")
	}
}

func TestCrossReferenceFailBelowMaxGoesBackToRef(t *testing.T) {
	// CROSS_REFERENCE_EVAL FAIL below max_rounds returns to CROSS_REFERENCE.
	s := newSpecifyingState(1)
	s.Config.Specifying.CrossReference.MaxRounds = 2
	advanceToAccept(t, s)

	dir := t.TempDir()
	crEvalFile := filepath.Join(dir, "cr-eval.md")
	os.WriteFile(crEvalFile, []byte("cross-ref eval"), 0644)

	Advance(s, AdvanceInput{}, "") // ACCEPT → CROSS_REFERENCE
	Advance(s, AdvanceInput{}, "") // CROSS_REFERENCE → CROSS_REFERENCE_EVAL (round 1)

	// FAIL at round 1 (below max=2) → back to CROSS_REFERENCE
	if err := Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: crEvalFile}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateCrossReference {
		t.Errorf("expected CROSS_REFERENCE after FAIL below max, got %s", s.State)
	}
}

func TestCrossReferenceEvalPassAtRound1GoesToReview(t *testing.T) {
	// CROSS_REFERENCE_EVAL PASS at round 1 with min_rounds=1 transitions to CROSS_REFERENCE_REVIEW.
	s := newSpecifyingState(1)
	s.Config.Specifying.CrossReference.MinRounds = 1
	advanceToAccept(t, s)

	dir := t.TempDir()
	crEvalFile := filepath.Join(dir, "cr-eval.md")
	os.WriteFile(crEvalFile, []byte("cross-ref eval"), 0644)

	Advance(s, AdvanceInput{}, "") // ACCEPT → CROSS_REFERENCE
	Advance(s, AdvanceInput{}, "") // CROSS_REFERENCE → CROSS_REFERENCE_EVAL (round 1)

	if err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: crEvalFile}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateCrossReferenceReview {
		t.Errorf("expected CROSS_REFERENCE_REVIEW, got %s", s.State)
	}
}

func TestCrossReferenceEvalPassBelowMinRoundsLoopsBack(t *testing.T) {
	// CROSS_REFERENCE_EVAL PASS below min_rounds loops back to CROSS_REFERENCE.
	s := newSpecifyingState(1)
	s.Config.Specifying.CrossReference.MinRounds = 2
	advanceToAccept(t, s)

	dir := t.TempDir()
	crEvalFile := filepath.Join(dir, "cr-eval.md")
	os.WriteFile(crEvalFile, []byte("cross-ref eval"), 0644)

	Advance(s, AdvanceInput{}, "") // ACCEPT → CROSS_REFERENCE
	Advance(s, AdvanceInput{}, "") // CROSS_REFERENCE → CROSS_REFERENCE_EVAL (round 1)

	// PASS at round 1 with min_rounds=2 → not enough, loop back.
	if err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: crEvalFile}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateCrossReference {
		t.Errorf("expected CROSS_REFERENCE (below min rounds), got %s", s.State)
	}
}

func TestCrossReferenceEvalPassRound2SkipsReview(t *testing.T) {
	// CROSS_REFERENCE_EVAL PASS at round>1 with min_rounds met skips CROSS_REFERENCE_REVIEW,
	// going directly to DONE (queue empty) or ORIENT (queue non-empty).
	s := newSpecifyingState(1)
	s.Config.Specifying.CrossReference.MinRounds = 1
	s.Config.Specifying.CrossReference.MaxRounds = 3
	advanceToAccept(t, s)

	dir := t.TempDir()
	crEvalFile := filepath.Join(dir, "cr-eval.md")
	os.WriteFile(crEvalFile, []byte("cross-ref eval"), 0644)

	Advance(s, AdvanceInput{}, "")                                             // ACCEPT → CROSS_REFERENCE
	Advance(s, AdvanceInput{}, "")                                             // CROSS_REFERENCE → CROSS_REFERENCE_EVAL (round 1)
	Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: crEvalFile}, "")     // FAIL at round 1 → CROSS_REFERENCE
	Advance(s, AdvanceInput{}, "")                                             // CROSS_REFERENCE → CROSS_REFERENCE_EVAL (round 2)

	// PASS at round 2 (round>1, min_rounds met) → skip review, go to DONE (queue empty).
	if err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: crEvalFile}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateDone {
		t.Errorf("expected DONE (skipped review at round>1), got %s", s.State)
	}
}

func TestCrossReferenceEvalForcedAtRound1GoesToReview(t *testing.T) {
	// CROSS_REFERENCE_EVAL FAIL at max_rounds (forced) on round 1 enters CROSS_REFERENCE_REVIEW.
	s := newSpecifyingState(1)
	s.Config.Specifying.CrossReference.MaxRounds = 1
	advanceToAccept(t, s)

	dir := t.TempDir()
	crEvalFile := filepath.Join(dir, "cr-eval.md")
	os.WriteFile(crEvalFile, []byte("cross-ref eval"), 0644)

	Advance(s, AdvanceInput{}, "") // ACCEPT → CROSS_REFERENCE
	Advance(s, AdvanceInput{}, "") // CROSS_REFERENCE → CROSS_REFERENCE_EVAL (round 1)

	// FAIL at round 1 with max_rounds=1 → forced accept, round==1 → CROSS_REFERENCE_REVIEW.
	if err := Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: crEvalFile}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateCrossReferenceReview {
		t.Errorf("expected CROSS_REFERENCE_REVIEW (forced at round 1), got %s", s.State)
	}
}

func TestCrossReferenceReviewEmptyQueueToDone(t *testing.T) {
	// CROSS_REFERENCE_REVIEW with empty queue advances to DONE.
	s := newSpecifyingState(1)
	advanceToAccept(t, s)

	dir := t.TempDir()
	crEvalFile := filepath.Join(dir, "cr-eval.md")
	os.WriteFile(crEvalFile, []byte("cross-ref eval"), 0644)

	Advance(s, AdvanceInput{}, "")                                          // ACCEPT → CROSS_REFERENCE
	Advance(s, AdvanceInput{}, "")                                          // CROSS_REFERENCE → CROSS_REFERENCE_EVAL
	Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: crEvalFile}, "")  // → CROSS_REFERENCE_REVIEW

	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateDone {
		t.Errorf("expected DONE (empty queue), got %s", s.State)
	}
}

func TestCrossReferenceReviewOutputUserReviewTrue(t *testing.T) {
	// CROSS_REFERENCE_REVIEW output shows STOP when user_review=true.
	s := newSpecifyingState(1)
	s.Config.Specifying.CrossReference.UserReview = true
	advanceToAccept(t, s)

	dir := t.TempDir()
	crEvalFile := filepath.Join(dir, "cr-eval.md")
	os.WriteFile(crEvalFile, []byte("cross-ref eval"), 0644)

	Advance(s, AdvanceInput{}, "")
	Advance(s, AdvanceInput{}, "")
	Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: crEvalFile}, "") // → CROSS_REFERENCE_REVIEW

	var buf bytes.Buffer
	PrintAdvanceOutput(&buf, s, "")
	out := buf.String()
	if !strings.Contains(out, "STOP") {
		t.Errorf("expected 'STOP' in CROSS_REFERENCE_REVIEW output when user_review=true, got:\n%s", out)
	}
}

func TestCrossReferenceReviewOutputUserReviewFalse(t *testing.T) {
	// CROSS_REFERENCE_REVIEW output shows 'Domain cross-reference complete' when user_review=false.
	s := newSpecifyingState(1)
	s.Config.Specifying.CrossReference.UserReview = false
	advanceToAccept(t, s)

	dir := t.TempDir()
	crEvalFile := filepath.Join(dir, "cr-eval.md")
	os.WriteFile(crEvalFile, []byte("cross-ref eval"), 0644)

	Advance(s, AdvanceInput{}, "")
	Advance(s, AdvanceInput{}, "")
	Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: crEvalFile}, "") // → CROSS_REFERENCE_REVIEW

	var buf bytes.Buffer
	PrintAdvanceOutput(&buf, s, "")
	out := buf.String()
	if !strings.Contains(out, "Domain cross-reference complete") {
		t.Errorf("expected 'Domain cross-reference complete' in output when user_review=false, got:\n%s", out)
	}
}

func TestLastDomainCrossReferenceReviewToDone(t *testing.T) {
	// After the last domain's CROSS_REFERENCE_REVIEW, advancing transitions to DONE.
	s := newSpecifyingState(1)
	// Queue is empty after accept (single spec, single domain).
	advanceToAccept(t, s)
	if len(s.Specifying.Queue) != 0 {
		t.Skip("test requires empty queue after accept")
	}

	dir := t.TempDir()
	crEvalFile := filepath.Join(dir, "cr-eval.md")
	os.WriteFile(crEvalFile, []byte("cross-ref eval"), 0644)

	Advance(s, AdvanceInput{}, "")
	Advance(s, AdvanceInput{}, "")
	Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: crEvalFile}, "") // → CROSS_REFERENCE_REVIEW

	if s.State != StateCrossReferenceReview {
		t.Fatalf("expected CROSS_REFERENCE_REVIEW, got %s", s.State)
	}

	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateDone {
		t.Errorf("expected DONE after last domain's CROSS_REFERENCE_REVIEW, got %s", s.State)
	}
}

func TestSpecifyingDoneToReconcile(t *testing.T) {
	s := newSpecifyingState(1)
	advanceToDone(t, s)

	// DONE → RECONCILE
	Advance(s, AdvanceInput{}, "")
	if s.State != StateReconcile {
		t.Errorf("expected RECONCILE, got %s", s.State)
	}
}

func TestReconcileFlowPass(t *testing.T) {
	// RECONCILE_EVAL PASS at round 1 (min_rounds=0) → RECONCILE_REVIEW → COMPLETE (empty queue).
	s := newSpecifyingState(1)
	advanceToDone(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "reconcile-eval.md")
	os.WriteFile(evalFile, []byte("reconcile eval"), 0644)

	Advance(s, AdvanceInput{}, "") // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "") // RECONCILE → RECONCILE_EVAL
	if s.State != StateReconcileEval {
		t.Fatalf("expected RECONCILE_EVAL, got %s", s.State)
	}

	// PASS at round 1 with min_rounds=0 → RECONCILE_REVIEW.
	if err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateReconcileReview {
		t.Fatalf("expected RECONCILE_REVIEW, got %s", s.State)
	}

	// RECONCILE_REVIEW with empty queue → COMPLETE.
	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateComplete {
		t.Errorf("expected COMPLETE, got %s", s.State)
	}
}

func TestReconcileFlowFailThenFix(t *testing.T) {
	// FAIL below max_rounds → RECONCILE; then PASS at round 2 → COMPLETE (skips REVIEW).
	s := newSpecifyingState(1)
	s.Config.Specifying.Reconciliation.MaxRounds = 3
	advanceToDone(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "reconcile-eval.md")
	os.WriteFile(evalFile, []byte("reconcile eval"), 0644)

	Advance(s, AdvanceInput{}, "") // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "") // RECONCILE → RECONCILE_EVAL (round 1)

	// FAIL at round 1 (below max=3) → RECONCILE.
	if err := Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: evalFile}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateReconcile {
		t.Fatalf("expected RECONCILE after FAIL below max, got %s", s.State)
	}

	Advance(s, AdvanceInput{}, "") // RECONCILE → RECONCILE_EVAL (round 2)

	// PASS at round 2 (round>1) → COMPLETE (skips REVIEW).
	if err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateComplete {
		t.Errorf("expected COMPLETE (round>1 skips review), got %s", s.State)
	}
}

func TestReconcileEvalRequiresEvalReport(t *testing.T) {
	// RECONCILE_EVAL must reject advance without --eval-report when enable_eval_output=true.
	s := newSpecifyingState(1)
	s.Config.Specifying.Eval.EnableEvalOutput = true
	advanceToDone(t, s)
	Advance(s, AdvanceInput{}, "") // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "") // RECONCILE → RECONCILE_EVAL

	// Missing eval-report with enable_eval_output=true.
	err := Advance(s, AdvanceInput{Verdict: "PASS"}, "")
	if err == nil {
		t.Error("expected error for missing --eval-report in RECONCILE_EVAL when enable_eval_output=true")
	}
}

func TestReconcileEvalRequiresVerdict(t *testing.T) {
	// RECONCILE_EVAL must reject advance without --verdict.
	s := newSpecifyingState(1)
	advanceToDone(t, s)
	Advance(s, AdvanceInput{}, "") // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "") // RECONCILE → RECONCILE_EVAL

	err := Advance(s, AdvanceInput{}, "")
	if err == nil {
		t.Error("expected error for missing --verdict in RECONCILE_EVAL")
	}
}

func TestReconcileEvalPassAtRound1GoesToReview(t *testing.T) {
	// PASS at round 1 with min_rounds=0 → RECONCILE_REVIEW.
	s := newSpecifyingState(1)
	advanceToDone(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "re-eval.md")
	os.WriteFile(evalFile, []byte("reconcile eval"), 0644)

	Advance(s, AdvanceInput{}, "") // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "") // RECONCILE → RECONCILE_EVAL (round 1)

	if err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateReconcileReview {
		t.Errorf("expected RECONCILE_REVIEW at round 1, got %s", s.State)
	}
}

func TestReconcileEvalPassBelowMinRoundsLoopsBack(t *testing.T) {
	// PASS below min_rounds loops back to RECONCILE.
	s := newSpecifyingState(1)
	s.Config.Specifying.Reconciliation.MinRounds = 2
	advanceToDone(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "re-eval.md")
	os.WriteFile(evalFile, []byte("reconcile eval"), 0644)

	Advance(s, AdvanceInput{}, "") // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "") // RECONCILE → RECONCILE_EVAL (round 1)

	// PASS at round 1 with min_rounds=2 → not enough, loop back.
	if err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateReconcile {
		t.Errorf("expected RECONCILE (below min rounds), got %s", s.State)
	}
}

func TestReconcileEvalPassAtRound2SkipsReview(t *testing.T) {
	// PASS at round>1 with min_rounds met skips RECONCILE_REVIEW → COMPLETE.
	s := newSpecifyingState(1)
	s.Config.Specifying.Reconciliation.MaxRounds = 3
	advanceToDone(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "re-eval.md")
	os.WriteFile(evalFile, []byte("reconcile eval"), 0644)

	Advance(s, AdvanceInput{}, "")                                        // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "")                                        // RECONCILE → RECONCILE_EVAL (round 1)
	Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: evalFile}, "")  // FAIL → RECONCILE
	Advance(s, AdvanceInput{}, "")                                        // RECONCILE → RECONCILE_EVAL (round 2)

	// PASS at round 2 → skip REVIEW → COMPLETE.
	if err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateComplete {
		t.Errorf("expected COMPLETE (skipped review at round>1), got %s", s.State)
	}
}

func TestReconcileEvalFailBelowMaxLoopsBack(t *testing.T) {
	// FAIL below max_rounds loops back to RECONCILE.
	s := newSpecifyingState(1)
	s.Config.Specifying.Reconciliation.MaxRounds = 2
	advanceToDone(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "re-eval.md")
	os.WriteFile(evalFile, []byte("reconcile eval"), 0644)

	Advance(s, AdvanceInput{}, "") // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "") // RECONCILE → RECONCILE_EVAL (round 1)

	if err := Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: evalFile}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateReconcile {
		t.Errorf("expected RECONCILE after FAIL below max, got %s", s.State)
	}
}

func TestReconcileEvalForcedAtRound1GoesToReview(t *testing.T) {
	// FAIL at max_rounds (forced) at round 1 → RECONCILE_REVIEW.
	s := newSpecifyingState(1)
	s.Config.Specifying.Reconciliation.MaxRounds = 1
	advanceToDone(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "re-eval.md")
	os.WriteFile(evalFile, []byte("reconcile eval"), 0644)

	Advance(s, AdvanceInput{}, "") // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "") // RECONCILE → RECONCILE_EVAL (round 1)

	// FAIL at round 1 with max_rounds=1 → forced → RECONCILE_REVIEW.
	if err := Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: evalFile}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateReconcileReview {
		t.Errorf("expected RECONCILE_REVIEW (forced at round 1), got %s", s.State)
	}
}

func TestReconcileEvalForcedAtRound2GoesToComplete(t *testing.T) {
	// FAIL at max_rounds (forced) at round>1 → COMPLETE (skips REVIEW).
	s := newSpecifyingState(1)
	s.Config.Specifying.Reconciliation.MaxRounds = 2
	advanceToDone(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "re-eval.md")
	os.WriteFile(evalFile, []byte("reconcile eval"), 0644)

	Advance(s, AdvanceInput{}, "")                                        // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "")                                        // RECONCILE → RECONCILE_EVAL (round 1)
	Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: evalFile}, "")  // FAIL at round 1 (below max=2) → RECONCILE
	Advance(s, AdvanceInput{}, "")                                        // RECONCILE → RECONCILE_EVAL (round 2)

	// FAIL at round 2 with max_rounds=2 → forced → round>1 → COMPLETE.
	if err := Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: evalFile}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateComplete {
		t.Errorf("expected COMPLETE (forced at round>1), got %s", s.State)
	}
}

func TestReconcileReviewEmptyQueueToComplete(t *testing.T) {
	// RECONCILE_REVIEW with empty queue → COMPLETE.
	s := newSpecifyingState(1)
	advanceToDone(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "re-eval.md")
	os.WriteFile(evalFile, []byte("reconcile eval"), 0644)

	Advance(s, AdvanceInput{}, "")
	Advance(s, AdvanceInput{}, "")
	Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, "") // → RECONCILE_REVIEW

	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateComplete {
		t.Errorf("expected COMPLETE (empty queue), got %s", s.State)
	}
}

func TestReconcileReviewNonEmptyQueueToDone(t *testing.T) {
	// RECONCILE_REVIEW with non-empty queue → DONE (re-enter for new specs).
	s := newSpecifyingState(1)
	advanceToDone(t, s)

	// Add a spec to the queue.
	s.Specifying.Queue = append(s.Specifying.Queue, SpecQueueEntry{
		Name: "New Spec", Domain: "test", Topic: "t", File: "test/specs/new.md",
	})

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "re-eval.md")
	os.WriteFile(evalFile, []byte("reconcile eval"), 0644)

	Advance(s, AdvanceInput{}, "")
	Advance(s, AdvanceInput{}, "")
	Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, "") // → RECONCILE_REVIEW

	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}
	if s.State != StateDone {
		t.Errorf("expected DONE (non-empty queue re-enters DONE), got %s", s.State)
	}
}

func TestReconcileEvalMessageRequiredWhenEnableCommits(t *testing.T) {
	// Per spec, --message is required at COMPLETE (not RECONCILE_EVAL) when enable_commits=true.
	// RECONCILE_EVAL PASS should succeed without --message.
	s := newSpecifyingState(1)
	s.Config.General.EnableCommits = true
	s.Config.Specifying.Eval.EnableEvalOutput = true
	advanceToDone(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "re-eval.md")
	os.WriteFile(evalFile, []byte("reconcile eval"), 0644)

	Advance(s, AdvanceInput{}, "") // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "") // RECONCILE → RECONCILE_EVAL

	// PASS with eval-report should succeed (--message not required at RECONCILE_EVAL).
	err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, "")
	if err != nil {
		t.Errorf("RECONCILE_EVAL PASS should not require --message: %v", err)
	}
}

func TestReconcileEvalMessageNotRequiredWithoutEnableCommits(t *testing.T) {
	// --message NOT required at RECONCILE_EVAL PASS when enable_commits=false (default).
	s := newSpecifyingState(1)
	s.Config.General.EnableCommits = false
	advanceToDone(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "re-eval.md")
	os.WriteFile(evalFile, []byte("reconcile eval"), 0644)

	Advance(s, AdvanceInput{}, "") // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "") // RECONCILE → RECONCILE_EVAL

	// PASS without --message should succeed when enable_commits=false.
	err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, "")
	if err != nil {
		t.Errorf("expected no error without --message when enable_commits=false: %v", err)
	}
}

func TestReconcileReviewOutputUserReviewTrue(t *testing.T) {
	// RECONCILE_REVIEW output shows STOP when user_review=true.
	s := newSpecifyingState(1)
	s.Config.Specifying.Reconciliation.UserReview = true
	advanceToDone(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "re-eval.md")
	os.WriteFile(evalFile, []byte("reconcile eval"), 0644)

	Advance(s, AdvanceInput{}, "")
	Advance(s, AdvanceInput{}, "")
	Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, "") // → RECONCILE_REVIEW

	var buf bytes.Buffer
	PrintAdvanceOutput(&buf, s, "")
	out := buf.String()
	if !strings.Contains(out, "STOP") {
		t.Errorf("expected 'STOP' in RECONCILE_REVIEW output when user_review=true, got:\n%s", out)
	}
}

func TestReconcileReviewOutputUserReviewFalse(t *testing.T) {
	// RECONCILE_REVIEW output shows 'Reconciliation review complete' when user_review=false.
	s := newSpecifyingState(1)
	s.Config.Specifying.Reconciliation.UserReview = false
	advanceToDone(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "re-eval.md")
	os.WriteFile(evalFile, []byte("reconcile eval"), 0644)

	Advance(s, AdvanceInput{}, "")
	Advance(s, AdvanceInput{}, "")
	Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, "") // → RECONCILE_REVIEW

	var buf bytes.Buffer
	PrintAdvanceOutput(&buf, s, "")
	out := buf.String()
	if !strings.Contains(out, "Reconciliation review complete") {
		t.Errorf("expected 'Reconciliation review complete' when user_review=false, got:\n%s", out)
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
			{Name: "Plan1", Domain: "test", File: "plan.json", Specs: []string{"spec.md"}, SpecCommits: []string{}, CodeSearchRoots: []string{"test/"}},
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
			{Name: "Plan1", Domain: "test", File: "plan.json", Specs: []string{}, SpecCommits: []string{}, CodeSearchRoots: []string{}},
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
			{Name: "Override Plan", Domain: "override", File: "override/plan.json", Specs: []string{"s.md"}, CodeSearchRoots: []string{"override/"}},
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
		Domains: map[string]DomainMeta{"mydom": {CodeSearchRoots: []string{"mydom/src/", "mydom/pkg/"}}},
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
			{Name: "Test Plan", Domain: "test", File: "test/plan.json", Specs: []string{"spec.md"}, CodeSearchRoots: []string{"test/"}},
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

// TestEvalReportProseValueProducesPathHint verifies the eval-report-contract
// guardrail: when --eval-report is given report prose (whitespace, no path
// separator) instead of a path, the error hints that a file path is expected.
func TestEvalReportProseValueProducesPathHint(t *testing.T) {
	s := newSpecifyingState(1)
	s.Config.Specifying.Eval.EnableEvalOutput = true
	advanceToEvaluate(t, s)

	prose := "Batch 4 Round 1 FAIL: data_coverage_pct not divided by 100"
	err := Advance(s, AdvanceInput{Verdict: "FAIL", EvalReport: prose}, "")
	if err == nil {
		t.Fatal("expected error for prose eval report value")
	}
	if !strings.Contains(err.Error(), "the file path the eval sub-agent wrote, not the report text") {
		t.Errorf("expected path-vs-text hint, got: %v", err)
	}
}

// TestLooksLikeReportProse covers the prose/path heuristic: prose has whitespace
// and no separator; real paths (even with spaces) contain a separator.
func TestLooksLikeReportProse(t *testing.T) {
	cases := []struct {
		v    string
		want bool
	}{
		{"Batch 4 FAIL: not divided by 100", true},
		{"optimizer/specs/.eval/batch-1-r1.md", false},
		{"report.md", false},
		{"my dir/report.md", false}, // spaces but has separator → a path
		{"singletoken", false},
	}
	for _, c := range cases {
		if got := looksLikeReportProse(c.v); got != c.want {
			t.Errorf("looksLikeReportProse(%q) = %v, want %v", c.v, got, c.want)
		}
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

func TestPlanningAcceptNoMessageRequiredWithoutEnableCommits(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithDir(dir)
	// enable_commits defaults to false
	advancePlanningToAccept(t, s, dir)

	err := Advance(s, AdvanceInput{}, dir) // no --message
	if err != nil {
		t.Errorf("expected no error in planning ACCEPT without enable_commits, got: %v", err)
	}
	if s.State != StatePhaseShift {
		t.Errorf("expected PHASE_SHIFT, got %s", s.State)
	}
}

func TestPlanningAcceptRequiresMessageWhenEnableCommits(t *testing.T) {
	dir := t.TempDir()
	s := newPlanningStateWithDir(dir)
	s.Config.General.EnableCommits = true
	advancePlanningToAccept(t, s, dir)

	err := Advance(s, AdvanceInput{}, dir) // no --message
	if err == nil {
		t.Error("expected error in planning ACCEPT when enable_commits=true and no --message")
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
		{Name: "Plan2", Domain: "test2", File: "impl2/plan.json", Specs: []string{"spec2.md"}, CodeSearchRoots: []string{"test2/"}},
	}
	return s
}

// newImplementingStateWithPlanningQueue creates an implementing state with plans in Planning.Queue (interleaved mode).
func newImplementingStateWithPlanningQueue(dir string, numItems, batchSize int) *ForgeState {
	s := newImplementingState(dir, numItems, batchSize)
	// Preserve Planning.CurrentPlan, just add to the Queue.
	s.Planning.Queue = []PlanQueueEntry{
		{Name: "Next Plan", Domain: "next", File: "next/plan.json", Specs: []string{"s.md"}, CodeSearchRoots: []string{"next/"}},
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

func TestFirstRoundImplementRequiresMessageWhenEnableCommits(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)
	s.Config.General.EnableCommits = true

	Advance(s, AdvanceInput{}, dir) // ORIENT → IMPLEMENT

	err := Advance(s, AdvanceInput{}, dir) // no --message
	if err == nil {
		t.Error("expected error for missing --message in first-round IMPLEMENT when enable_commits=true")
	}
}

func TestFirstRoundImplementNoMessageRequiredWithoutEnableCommits(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)
	// enable_commits defaults to false

	Advance(s, AdvanceInput{}, dir) // ORIENT → IMPLEMENT

	err := Advance(s, AdvanceInput{}, dir) // no --message — should succeed
	if err != nil {
		t.Errorf("expected no error without enable_commits, got: %v", err)
	}
}

// captureStderr runs fn with os.Stderr redirected to a pipe and returns what was written.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// TestEvaluateReportModeRequiresEvalReport verifies that in report mode, advancing
// from implementing EVALUATE with --verdict but no --eval-report is rejected.
func TestEvaluateReportModeRequiresEvalReport(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)
	s.Config.Implementing.Eval.EvalMode = "report"

	advanceImplToEvaluate(t, s, dir)

	err := Advance(s, AdvanceInput{Verdict: "PASS"}, dir)
	if err == nil {
		t.Fatal("expected error for missing --eval-report in report mode")
	}
	if !strings.Contains(err.Error(), "--eval-report is required in EVALUATE state") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestEvaluateReportModeRejectsMissingFile verifies that in report mode, an --eval-report
// pointing at a non-existent file errors naming the path.
func TestEvaluateReportModeRejectsMissingFile(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)
	s.Config.Implementing.Eval.EvalMode = "report"

	advanceImplToEvaluate(t, s, dir)

	missing := filepath.Join(dir, "does-not-exist.md")
	err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: missing}, dir)
	if err == nil {
		t.Fatal("expected error for non-existent --eval-report file")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Errorf("error should name the path %q, got: %v", missing, err)
	}
}

// TestEvaluateConversationalModeIgnoresEvalReport verifies that in a non-report mode,
// a supplied --eval-report is accepted-but-ignored and the advance proceeds. The
// ignore warning is emitted by the cmd layer, not the state transition, so the state
// layer stays silent here (this is what keeps the warning from being printed twice).
func TestEvaluateConversationalModeIgnoresEvalReport(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)
	// EvalMode unset + EnableEvalOutput false → resolves to conversational.

	advanceImplToEvaluate(t, s, dir)

	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	var err error
	stderr := captureStderr(t, func() {
		err = Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, dir)
	})
	if err != nil {
		t.Fatalf("advance should proceed in conversational mode: %v", err)
	}
	if s.State != StateCommit {
		t.Errorf("expected COMMIT, got %s", s.State)
	}
	if strings.Contains(stderr, "--eval-report is ignored") {
		t.Errorf("state layer must not print the ignore warning (cmd layer owns it), got stderr: %q", stderr)
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

func TestCommitNoMessageRequiredWithoutEnableCommits(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)
	// enable_commits defaults to false

	advanceImplToCommit(t, s, dir)

	err := Advance(s, AdvanceInput{}, dir) // no --message
	if err != nil {
		t.Errorf("expected no error in COMMIT without enable_commits, got: %v", err)
	}
}

func TestCommitRequiresMessageWhenEnableCommits(t *testing.T) {
	dir := t.TempDir()
	initTestGitRepo(t, dir)
	s := newImplementingState(dir, 1, 1)
	s.Config.General.EnableCommits = true

	advanceImplToCommit(t, s, dir)

	err := Advance(s, AdvanceInput{}, dir) // no --message
	if err == nil {
		t.Error("expected error in COMMIT when enable_commits=true and no --message")
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

	dir := t.TempDir()
	crEvalFile := filepath.Join(dir, "cr-eval.md")
	os.WriteFile(crEvalFile, []byte("cross-ref eval"), 0644)

	Advance(s, AdvanceInput{}, "")                                             // ACCEPT → CROSS_REFERENCE
	Advance(s, AdvanceInput{}, "")                                             // CROSS_REFERENCE → CROSS_REFERENCE_EVAL
	Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: crEvalFile}, "")      // CROSS_REFERENCE_EVAL → CROSS_REFERENCE_REVIEW
	Advance(s, AdvanceInput{}, "")                                             // CROSS_REFERENCE_REVIEW → DONE (queue empty)
}

func advanceToComplete(t *testing.T, s *ForgeState) {
	t.Helper()
	dir := t.TempDir()
	reEvalFile := filepath.Join(dir, "reconcile-eval.md")
	os.WriteFile(reEvalFile, []byte("reconcile eval"), 0644)

	advanceToDone(t, s)
	Advance(s, AdvanceInput{}, "")                                                // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "")                                                // RECONCILE → RECONCILE_EVAL
	Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: reEvalFile}, "")        // RECONCILE_EVAL PASS → RECONCILE_REVIEW
	Advance(s, AdvanceInput{}, "")                                                // RECONCILE_REVIEW → COMPLETE (empty queue)
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
	// Create domain directory so scoped git add has something to stage.
	os.MkdirAll(filepath.Join(dir, "test"), 0755)
	os.WriteFile(filepath.Join(dir, "test", "main.go"), []byte("package main"), 0644)

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
		Implementing: &ImplementingState{
			CurrentPlanDomain: "test",
		},
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

// writeREQueue writes content to the fixed reverse engineering queue path.
func writeREQueue(t *testing.T, dir, content string) {
	t.Helper()
	p := filepath.Join(dir, ".forgectl", "state")
	if err := os.MkdirAll(p, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, "reverse-engineering-queue.json"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func newREState(t *testing.T, dir string, domains []string, state StateName) *ForgeState {
	t.Helper()
	return &ForgeState{
		Phase:              PhaseReverseEngineering,
		State:              state,
		StartedAtPhase:     PhaseReverseEngineering,
		ReverseEngineering: NewReverseEngineeringState("auth refactor", domains),
	}
}

// Functional: the full per-domain analysis loop across two domains —
// ORIENT→SURVEY→GAP_ANALYSIS→DECOMPOSE→QUEUE, QUEUE advancing to the next
// domain, then into the execution loop after the last domain.
func TestREAdvanceDomainLoop(t *testing.T) {
	dir := buildREProject(t, map[string][]string{
		"optimizer": {"src"},
		"api":       {"handlers"},
	})
	s := newREState(t, dir, []string{"optimizer", "api"}, StateOrient)

	for _, want := range []StateName{StateSurvey, StateGapAnalysis, StateDecompose, StateQueue} {
		if err := Advance(s, AdvanceInput{}, dir); err != nil {
			t.Fatalf("advance toward %s: %v", want, err)
		}
		if s.State != want {
			t.Fatalf("state = %s, want %s", s.State, want)
		}
	}
	if s.ReverseEngineering.DomainIndex != 1 {
		t.Fatalf("domain index = %d, want 1 at first QUEUE", s.ReverseEngineering.DomainIndex)
	}

	// QUEUE for domain 1 (first advance): write a valid queue, expect transition
	// to SURVEY for domain 2 with the hash + parsed queue recorded.
	optEntry := `{"name":"Opt","domain":"optimizer","topic":"t","file":"specs/opt.md","action":"create","code_search_roots":["src/"],"depends_on":[]}`
	writeREQueue(t, dir, `{"specs":[`+optEntry+`]}`)
	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("QUEUE domain 1: %v", err)
	}
	if s.State != StateSurvey {
		t.Fatalf("after QUEUE domain 1, state = %s, want SURVEY", s.State)
	}
	if s.ReverseEngineering.DomainIndex != 2 {
		t.Errorf("domain index = %d, want 2", s.ReverseEngineering.DomainIndex)
	}
	if s.ReverseEngineering.QueueContentHash == "" {
		t.Error("queue content hash should be recorded after first QUEUE advance")
	}
	if s.ReverseEngineering.QueueFilePath == "" {
		t.Error("queue file path should be recorded")
	}
	if len(s.ReverseEngineering.Queue) != 1 {
		t.Errorf("parsed queue len = %d, want 1", len(s.ReverseEngineering.Queue))
	}

	// Walk domain 2 to QUEUE.
	for _, want := range []StateName{StateGapAnalysis, StateDecompose, StateQueue} {
		if err := Advance(s, AdvanceInput{}, dir); err != nil {
			t.Fatalf("domain 2 advance toward %s: %v", want, err)
		}
		if s.State != want {
			t.Fatalf("domain 2 state = %s, want %s", s.State, want)
		}
	}

	// Unchanged file → rejected (hash already stored).
	if err := Advance(s, AdvanceInput{}, dir); err == nil {
		t.Fatal("expected unchanged-queue error on domain 2 QUEUE")
	}

	// Update the file with both domains' entries → transition into execution loop.
	apiEntry := `{"name":"Api","domain":"api","topic":"t","file":"specs/api.md","action":"create","code_search_roots":["handlers/"],"depends_on":["Opt"]}`
	writeREQueue(t, dir, `{"specs":[`+optEntry+`,`+apiEntry+`]}`)
	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("QUEUE domain 2 (updated): %v", err)
	}
	if s.State != StateExecuteReverseEngineer {
		t.Fatalf("after last QUEUE, state = %s, want EXECUTE_REVERSE_ENGINEER", s.State)
	}
	if s.ReverseEngineering.ExecuteItemIndex != 1 {
		t.Errorf("execute item index = %d, want 1", s.ReverseEngineering.ExecuteItemIndex)
	}
	if len(s.ReverseEngineering.Queue) != 2 {
		t.Errorf("final queue len = %d, want 2", len(s.ReverseEngineering.Queue))
	}
}

// Rejection: QUEUE rejects a --file flag, a missing queue file, and an invalid
// queue (nonexistent code_search_roots).
func TestREQueueRejections(t *testing.T) {
	dir := buildREProject(t, map[string][]string{"optimizer": {"src"}})

	// --file flag rejected.
	s := newREState(t, dir, []string{"optimizer"}, StateQueue)
	if err := Advance(s, AdvanceInput{File: "somewhere.json"}, dir); err == nil {
		t.Error("expected error when --file is supplied in QUEUE")
	} else if !strings.Contains(err.Error(), "takes no --file flag in QUEUE") {
		t.Errorf("unexpected --file error: %v", err)
	}

	// Missing queue file.
	s = newREState(t, dir, []string{"optimizer"}, StateQueue)
	if err := Advance(s, AdvanceInput{}, dir); err == nil {
		t.Error("expected error when queue file is missing")
	} else if !strings.Contains(err.Error(), "not found at expected path") {
		t.Errorf("unexpected missing-file error: %v", err)
	}

	// Invalid queue: code_search_roots directory does not exist.
	s = newREState(t, dir, []string{"optimizer"}, StateQueue)
	writeREQueue(t, dir, `{"specs":[{"name":"X","domain":"optimizer","topic":"t","file":"specs/x.md","action":"create","code_search_roots":["ghost/"],"depends_on":[]}]}`)
	err := Advance(s, AdvanceInput{}, dir)
	if err == nil {
		t.Fatal("expected validation error for nonexistent code_search_roots")
	}
	if _, ok := err.(*ValidationError); !ok {
		t.Errorf("expected *ValidationError, got %T: %v", err, err)
	}
	if s.State != StateQueue {
		t.Errorf("state should stay QUEUE on validation failure, got %s", s.State)
	}
}

// Edge case: on a subsequent QUEUE advance an unchanged file is rejected and the
// state stays at QUEUE so the user can update and retry.
func TestREQueueUnchangedHash(t *testing.T) {
	dir := buildREProject(t, map[string][]string{"optimizer": {"src"}})
	content := `{"specs":[{"name":"X","domain":"optimizer","topic":"t","file":"specs/x.md","action":"create","code_search_roots":["src/"],"depends_on":[]}]}`
	writeREQueue(t, dir, content)

	s := newREState(t, dir, []string{"optimizer"}, StateQueue)
	// Pre-set the stored hash to simulate a prior domain's advance.
	s.ReverseEngineering.QueueContentHash = HashBytes([]byte(content))

	err := Advance(s, AdvanceInput{}, dir)
	if err == nil {
		t.Fatal("expected unchanged-queue error")
	}
	if !strings.Contains(err.Error(), "Queue file has not changed") {
		t.Errorf("unexpected error: %v", err)
	}
	if s.State != StateQueue {
		t.Errorf("state should stay QUEUE, got %s", s.State)
	}
}

// reExecState builds an RE state positioned in the execution loop.
func reExecState(domains []string, queue []REQueueEntry, st StateName, idx int) *ForgeState {
	re := NewReverseEngineeringState("auth refactor", domains)
	re.Queue = queue
	re.ExecuteItemIndex = idx
	return &ForgeState{
		Phase:              PhaseReverseEngineering,
		State:              st,
		StartedAtPhase:     PhaseReverseEngineering,
		ReverseEngineering: re,
	}
}

func twoItemQueue() []REQueueEntry {
	return []REQueueEntry{
		{Name: "One", Domain: "optimizer", Topic: "t", File: "specs/one.md", Action: "create", CodeSearchRoots: []string{"src/"}, DependsOn: []string{}},
		{Name: "Two", Domain: "api", Topic: "t", File: "specs/two.md", Action: "update", CodeSearchRoots: []string{"handlers/"}, DependsOn: []string{}},
	}
}

// Functional: EXECUTE advances to POST for the same item (index unchanged).
func TestREExecuteToPost(t *testing.T) {
	dir := buildREProject(t, map[string][]string{"optimizer": {"src"}, "api": {"handlers"}})
	s := reExecState([]string{"optimizer", "api"}, twoItemQueue(), StateExecuteReverseEngineer, 1)

	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("EXECUTE→POST: %v", err)
	}
	if s.State != StatePostReverseEngineer {
		t.Errorf("state = %s, want POST_REVERSE_ENGINEER", s.State)
	}
	if s.ReverseEngineering.ExecuteItemIndex != 1 {
		t.Errorf("execute item index = %d, want 1 (unchanged)", s.ReverseEngineering.ExecuteItemIndex)
	}
}

// Functional: POST advances to EXECUTE for the next item, incrementing the index
// and creating that item's domain specs directory.
func TestREPostToNextExecute(t *testing.T) {
	dir := buildREProject(t, map[string][]string{"optimizer": {"src"}, "api": {"handlers"}})
	s := reExecState([]string{"optimizer", "api"}, twoItemQueue(), StatePostReverseEngineer, 1)

	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("POST→EXECUTE: %v", err)
	}
	if s.State != StateExecuteReverseEngineer {
		t.Errorf("state = %s, want EXECUTE_REVERSE_ENGINEER", s.State)
	}
	if s.ReverseEngineering.ExecuteItemIndex != 2 {
		t.Errorf("execute item index = %d, want 2", s.ReverseEngineering.ExecuteItemIndex)
	}
	// Item 2's domain specs dir must now exist.
	if info, err := os.Stat(filepath.Join(dir, "api", "specs")); err != nil || !info.IsDir() {
		t.Errorf("api/specs directory should have been created: %v", err)
	}
}

// Functional: the full loop over all items ends in RECONCILE for domain 1, round 1.
func TestREExecuteLoopToReconcile(t *testing.T) {
	dir := buildREProject(t, map[string][]string{"optimizer": {"src"}, "api": {"handlers"}})
	s := reExecState([]string{"optimizer", "api"}, twoItemQueue(), StateExecuteReverseEngineer, 1)

	// item1: EXECUTE→POST→EXECUTE(item2)→POST→RECONCILE
	seq := []StateName{
		StatePostReverseEngineer,    // EXECUTE item1 → POST
		StateExecuteReverseEngineer, // POST item1 → EXECUTE item2
		StatePostReverseEngineer,    // EXECUTE item2 → POST
		StateReconcile,              // POST item2 (last) → RECONCILE
	}
	for i, want := range seq {
		if err := Advance(s, AdvanceInput{}, dir); err != nil {
			t.Fatalf("loop step %d: %v", i, err)
		}
		if s.State != want {
			t.Fatalf("loop step %d: state = %s, want %s", i, s.State, want)
		}
	}
	if s.ReverseEngineering.DomainIndex != 1 || s.ReverseEngineering.ReconcileRound != 1 {
		t.Errorf("reconcile entry: domainIndex=%d round=%d, want 1/1",
			s.ReverseEngineering.DomainIndex, s.ReverseEngineering.ReconcileRound)
	}
}

// Rejection: an empty queue at EXECUTE entry errors and stays in EXECUTE.
func TestREExecuteEmptyQueueRejected(t *testing.T) {
	dir := buildREProject(t, map[string][]string{"optimizer": {"src"}})
	s := reExecState([]string{"optimizer"}, []REQueueEntry{}, StateExecuteReverseEngineer, 1)

	err := Advance(s, AdvanceInput{}, dir)
	if err == nil || !strings.Contains(err.Error(), "Queue contains zero entries. Nothing to execute.") {
		t.Fatalf("expected zero-entries error, got: %v", err)
	}
	if s.State != StateExecuteReverseEngineer {
		t.Errorf("state should stay EXECUTE, got %s", s.State)
	}
}

// Rejection: a specs-dir creation failure surfaces an error naming the path.
func TestREExecuteMkdirFailure(t *testing.T) {
	dir := buildREProject(t, map[string][]string{"optimizer": {"src"}, "api": {"handlers"}})
	// Block api/specs by placing a regular file where the directory must go.
	if err := os.WriteFile(filepath.Join(dir, "api", "specs"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	s := reExecState([]string{"optimizer", "api"}, twoItemQueue(), StatePostReverseEngineer, 1)

	err := Advance(s, AdvanceInput{}, dir)
	if err == nil {
		t.Fatal("expected mkdir failure advancing to item 2 (api)")
	}
	if !strings.Contains(err.Error(), filepath.Join(dir, "api", "specs")) {
		t.Errorf("error should name the specs path, got: %v", err)
	}
}

// Edge case: depends_on is ignored — the loop walks stored order even when an
// earlier item depends on a later one.
func TestREExecuteIgnoresDependsOn(t *testing.T) {
	dir := buildREProject(t, map[string][]string{"optimizer": {"src"}, "api": {"handlers"}})
	q := twoItemQueue()
	q[0].DependsOn = []string{"Two"} // item 1 depends on item 2 — must NOT reorder
	s := reExecState([]string{"optimizer", "api"}, q, StateExecuteReverseEngineer, 1)

	// EXECUTE item1 → POST → EXECUTE item2 : index must go 1 → 2 in stored order.
	if err := Advance(s, AdvanceInput{}, dir); err != nil { // → POST
		t.Fatal(err)
	}
	if err := Advance(s, AdvanceInput{}, dir); err != nil { // → EXECUTE item2
		t.Fatal(err)
	}
	if s.ReverseEngineering.ExecuteItemIndex != 2 {
		t.Errorf("index = %d, want 2 (stored order, deps ignored)", s.ReverseEngineering.ExecuteItemIndex)
	}
}

// Edge case: items that produced no file (and a code_search_roots dir deleted
// after QUEUE) take no failure path — the loop still completes.
func TestREExecuteNoFilesWritten(t *testing.T) {
	dir := buildREProject(t, map[string][]string{"optimizer": {"src"}, "api": {"handlers"}})
	// Delete a code_search_roots directory that QUEUE had validated.
	if err := os.RemoveAll(filepath.Join(dir, "optimizer", "src")); err != nil {
		t.Fatal(err)
	}
	s := reExecState([]string{"optimizer", "api"}, twoItemQueue(), StateExecuteReverseEngineer, 1)

	// Never write any spec file; the loop must still reach RECONCILE.
	for i := 0; i < 4; i++ {
		if err := Advance(s, AdvanceInput{}, dir); err != nil {
			t.Fatalf("advance %d should not fail on missing files: %v", i, err)
		}
	}
	if s.State != StateReconcile {
		t.Errorf("state = %s, want RECONCILE after a file-less loop", s.State)
	}
}

// reReconcileState builds an RE state positioned in the reconcile loop with the
// given reconcile config locked in.
func reReconcileState(domains []string, st StateName, domainIndex, round, min, max int, colleague bool) *ForgeState {
	re := NewReverseEngineeringState("auth refactor", domains)
	re.DomainIndex = domainIndex
	re.ReconcileRound = round
	re.ColleagueReview = colleague
	s := &ForgeState{
		Phase:              PhaseReverseEngineering,
		State:              st,
		StartedAtPhase:     PhaseReverseEngineering,
		ReverseEngineering: re,
	}
	s.Config.ReverseEngineering.Reconcile.MinRounds = min
	s.Config.ReverseEngineering.Reconcile.MaxRounds = max
	s.Config.ReverseEngineering.Reconcile.ColleagueReview = colleague
	return s
}

// Functional: RECONCILE→RECONCILE_EVAL, a PASS at/above min rounds (colleague
// review disabled) advances to RECONCILE_ADVANCE, which on the last domain
// reaches DONE. The verdict is recorded against the domain's reconcile history.
func TestREReconcilePassToDone(t *testing.T) {
	dir := t.TempDir()
	s := reReconcileState([]string{"optimizer"}, StateReconcile, 1, 1, 1, 3, false)

	// RECONCILE → RECONCILE_EVAL (no flags).
	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("RECONCILE→EVAL: %v", err)
	}
	if s.State != StateReconcileEval {
		t.Fatalf("state = %s, want RECONCILE_EVAL", s.State)
	}

	// PASS at round 1 (min 1) → RECONCILE_ADVANCE (colleague disabled).
	if err := Advance(s, AdvanceInput{Verdict: "PASS"}, dir); err != nil {
		t.Fatalf("EVAL PASS: %v", err)
	}
	if s.State != StateReconcileAdvance {
		t.Fatalf("state = %s, want RECONCILE_ADVANCE", s.State)
	}
	rec := s.ReverseEngineering.DomainReconcile["optimizer"]
	if rec == nil || len(rec.Evals) != 1 || rec.Evals[0].Verdict != "PASS" || rec.Evals[0].Round != 1 {
		t.Fatalf("domain reconcile history not recorded: %+v", rec)
	}

	// RECONCILE_ADVANCE on the last domain → DONE.
	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("RECONCILE_ADVANCE→DONE: %v", err)
	}
	if s.State != StateDone {
		t.Fatalf("state = %s, want DONE", s.State)
	}
}

// Functional: RECONCILE_ADVANCE with a domain remaining moves to RECONCILE for
// the next domain, resetting the round to 1 and bumping the domain index.
func TestREReconcileAdvanceNextDomain(t *testing.T) {
	dir := t.TempDir()
	s := reReconcileState([]string{"optimizer", "api"}, StateReconcileAdvance, 1, 2, 1, 3, false)

	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("RECONCILE_ADVANCE→next domain: %v", err)
	}
	if s.State != StateReconcile {
		t.Fatalf("state = %s, want RECONCILE", s.State)
	}
	if s.ReverseEngineering.DomainIndex != 2 {
		t.Errorf("domain index = %d, want 2", s.ReverseEngineering.DomainIndex)
	}
	if s.ReverseEngineering.ReconcileRound != 1 {
		t.Errorf("round = %d, want 1 (reset for new domain)", s.ReverseEngineering.ReconcileRound)
	}
}

// Edge case: a FAIL below max loops back to RECONCILE incrementing the round;
// a FAIL at max rounds is force-accepted to RECONCILE_ADVANCE. Both verdicts are
// recorded.
func TestREReconcileEvalFailLoopsThenForces(t *testing.T) {
	dir := t.TempDir()
	s := reReconcileState([]string{"optimizer"}, StateReconcileEval, 1, 1, 1, 2, false)

	// Round 1 FAIL (max 2) → back to RECONCILE, round 2.
	if err := Advance(s, AdvanceInput{Verdict: "FAIL"}, dir); err != nil {
		t.Fatalf("EVAL FAIL round 1: %v", err)
	}
	if s.State != StateReconcile {
		t.Fatalf("state = %s, want RECONCILE after FAIL below max", s.State)
	}
	if s.ReverseEngineering.ReconcileRound != 2 {
		t.Fatalf("round = %d, want 2", s.ReverseEngineering.ReconcileRound)
	}

	// RECONCILE → EVAL again.
	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("RECONCILE→EVAL round 2: %v", err)
	}
	// Round 2 FAIL (>= max 2) → forced to RECONCILE_ADVANCE.
	if err := Advance(s, AdvanceInput{Verdict: "FAIL"}, dir); err != nil {
		t.Fatalf("EVAL FAIL round 2: %v", err)
	}
	if s.State != StateReconcileAdvance {
		t.Fatalf("state = %s, want RECONCILE_ADVANCE (forced at max)", s.State)
	}
	rec := s.ReverseEngineering.DomainReconcile["optimizer"]
	if rec == nil || len(rec.Evals) != 2 {
		t.Fatalf("want 2 recorded evals, got %+v", rec)
	}
}

// Edge case: a PASS below min rounds loops back to RECONCILE (round++), and with
// colleague review enabled a terminal verdict gates through COLLEAGUE_REVIEW
// before RECONCILE_ADVANCE.
func TestREReconcileMinRoundsAndColleagueGate(t *testing.T) {
	dir := t.TempDir()
	// min 2, colleague enabled.
	s := reReconcileState([]string{"optimizer"}, StateReconcileEval, 1, 1, 2, 3, true)

	// PASS at round 1 < min 2 → loop back to RECONCILE, round 2.
	if err := Advance(s, AdvanceInput{Verdict: "PASS"}, dir); err != nil {
		t.Fatalf("EVAL PASS round 1: %v", err)
	}
	if s.State != StateReconcile || s.ReverseEngineering.ReconcileRound != 2 {
		t.Fatalf("want RECONCILE round 2, got %s round %d", s.State, s.ReverseEngineering.ReconcileRound)
	}

	// RECONCILE → EVAL.
	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("RECONCILE→EVAL: %v", err)
	}
	// PASS at round 2 >= min 2, colleague enabled → COLLEAGUE_REVIEW.
	if err := Advance(s, AdvanceInput{Verdict: "PASS"}, dir); err != nil {
		t.Fatalf("EVAL PASS round 2: %v", err)
	}
	if s.State != StateColleagueReview {
		t.Fatalf("state = %s, want COLLEAGUE_REVIEW", s.State)
	}
	// COLLEAGUE_REVIEW → RECONCILE_ADVANCE.
	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("COLLEAGUE_REVIEW→RECONCILE_ADVANCE: %v", err)
	}
	if s.State != StateReconcileAdvance {
		t.Fatalf("state = %s, want RECONCILE_ADVANCE", s.State)
	}
}

// Rejection: RECONCILE_EVAL requires a PASS/FAIL verdict.
func TestREReconcileEvalRequiresVerdict(t *testing.T) {
	dir := t.TempDir()
	s := reReconcileState([]string{"optimizer"}, StateReconcileEval, 1, 1, 1, 3, false)

	if err := Advance(s, AdvanceInput{}, dir); err == nil {
		t.Error("expected error when --verdict is missing")
	} else if !strings.Contains(err.Error(), "verdict is required") {
		t.Errorf("unexpected error: %v", err)
	}

	s = reReconcileState([]string{"optimizer"}, StateReconcileEval, 1, 1, 1, 3, false)
	if err := Advance(s, AdvanceInput{Verdict: "MAYBE"}, dir); err == nil {
		t.Error("expected error for an invalid verdict")
	} else if !strings.Contains(err.Error(), "must be PASS or FAIL") {
		t.Errorf("unexpected error: %v", err)
	}
}

// Edge case: ReverseEngineeringDomainGaps reports only the current domain's
// queue entries whose target spec files are absent on disk, resolving against
// the absolute project root.
func TestREReconcileDomainGaps(t *testing.T) {
	dir := buildREProject(t, map[string][]string{"optimizer": {"specs"}, "api": {"specs"}})
	re := NewReverseEngineeringState("auth refactor", []string{"optimizer", "api"})
	re.DomainIndex = 1
	re.Queue = []REQueueEntry{
		{Name: "Present", Domain: "optimizer", File: "specs/present.md", Action: "create"},
		{Name: "Missing", Domain: "optimizer", File: "specs/missing.md", Action: "create"},
		{Name: "Other", Domain: "api", File: "specs/other.md", Action: "create"},
	}
	// Only the "present" file exists.
	if err := os.WriteFile(filepath.Join(dir, "optimizer", "specs", "present.md"), []byte("# spec"), 0644); err != nil {
		t.Fatal(err)
	}

	gaps := ReverseEngineeringDomainGaps(re, dir)
	if len(gaps) != 1 || gaps[0] != "specs/missing.md" {
		t.Fatalf("gaps = %v, want [specs/missing.md] (api entry excluded, present excluded)", gaps)
	}
}
