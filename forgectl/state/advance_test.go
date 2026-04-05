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

// --- Reverse Engineering Phase Tests ---

func newReverseEngineeringState(domains []string) *ForgeState {
	return &ForgeState{
		Phase:              PhaseReverseEngineering,
		State:              StateOrient,
		Config:             DefaultForgeConfig(),
		ReverseEngineering: NewReverseEngineeringState("understand the codebase", domains, false),
	}
}

// validREQueueJSON returns a valid reverse engineering queue JSON for the given domain.
func validREQueueJSON(domain string) string {
	return `{"specs":[{"name":"spec1","domain":"` + domain + `","topic":"topic-1","file":"` + domain + `/specs/spec1.md","action":"create","code_search_roots":["src/"],"depends_on":[]}]}`
}

// TestReverseEngineeringOrientToSurvey verifies ORIENT sets domain index to 0 and advances to SURVEY.
func TestReverseEngineeringOrientToSurvey(t *testing.T) {
	s := newReverseEngineeringState([]string{"optimizer", "api"})

	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}

	if s.State != StateSurvey {
		t.Fatalf("expected SURVEY, got %s", s.State)
	}
	if s.ReverseEngineering.CurrentDomain != 0 {
		t.Fatalf("expected domain index 0, got %d", s.ReverseEngineering.CurrentDomain)
	}
}

// TestReverseEngineeringPreExecuteSequence verifies SURVEY → GAP_ANALYSIS → DECOMPOSE → QUEUE transitions.
func TestReverseEngineeringPreExecuteSequence(t *testing.T) {
	s := newReverseEngineeringState([]string{"optimizer"})
	s.State = StateSurvey

	steps := []StateName{StateGapAnalysis, StateDecompose, StateQueue}
	for _, want := range steps {
		if err := Advance(s, AdvanceInput{}, ""); err != nil {
			t.Fatalf("advance to %s: %v", want, err)
		}
		if s.State != want {
			t.Fatalf("expected %s, got %s", want, s.State)
		}
	}
}

// TestReverseEngineeringQueueToSurveyNextDomain verifies QUEUE → SURVEY when more domains remain.
func TestReverseEngineeringQueueToSurveyNextDomain(t *testing.T) {
	dir := t.TempDir()
	queueFile := filepath.Join(dir, "queue.json")
	os.WriteFile(queueFile, []byte(validREQueueJSON("optimizer")), 0644)

	s := newReverseEngineeringState([]string{"optimizer", "api"})
	s.State = StateQueue
	s.ReverseEngineering.CurrentDomain = 0

	// Pass dir="" to skip code_search_roots path validation.
	if err := Advance(s, AdvanceInput{File: queueFile}, ""); err != nil {
		t.Fatal(err)
	}

	if s.State != StateSurvey {
		t.Fatalf("expected SURVEY, got %s", s.State)
	}
	if s.ReverseEngineering.CurrentDomain != 1 {
		t.Fatalf("expected domain index 1, got %d", s.ReverseEngineering.CurrentDomain)
	}
}

// TestReverseEngineeringQueueToExecuteLastDomain verifies QUEUE → EXECUTE when processing the last domain.
func TestReverseEngineeringQueueToExecuteLastDomain(t *testing.T) {
	dir := t.TempDir()
	queueFile := filepath.Join(dir, "queue.json")
	initial := validREQueueJSON("optimizer")
	os.WriteFile(queueFile, []byte(initial), 0644)

	s := newReverseEngineeringState([]string{"optimizer", "api"})
	s.State = StateQueue
	s.ReverseEngineering.CurrentDomain = 1 // last domain (0-based)
	// Simulate first domain already validated: QueueFile and QueueHash set with old hash.
	s.ReverseEngineering.QueueFile = queueFile
	s.ReverseEngineering.QueueHash = "old-hash-value" // differs from actual file content

	// domains: ["optimizer", "api"] — entry domain "optimizer" is valid.
	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}

	if s.State != StateExecute {
		t.Fatalf("expected EXECUTE, got %s", s.State)
	}
}

// TestReverseEngineeringQueueFirstAdvanceRequiresFile verifies --file is required on first QUEUE advance.
func TestReverseEngineeringQueueFirstAdvanceRequiresFile(t *testing.T) {
	s := newReverseEngineeringState([]string{"optimizer"})
	s.State = StateQueue
	s.ReverseEngineering.CurrentDomain = 0

	err := Advance(s, AdvanceInput{}, "")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "Queue file path required") {
		t.Fatalf("unexpected error: %v", err)
	}
	// State must remain QUEUE.
	if s.State != StateQueue {
		t.Fatalf("expected state to remain QUEUE, got %s", s.State)
	}
}

// TestReverseEngineeringQueueFirstAdvanceStoresPathAndHash verifies that the first QUEUE advance
// stores the queue file path and content hash in state after successful validation.
func TestReverseEngineeringQueueFirstAdvanceStoresPathAndHash(t *testing.T) {
	dir := t.TempDir()
	queueFile := filepath.Join(dir, "queue.json")
	content := validREQueueJSON("optimizer")
	os.WriteFile(queueFile, []byte(content), 0644)

	s := newReverseEngineeringState([]string{"optimizer"})
	s.State = StateQueue

	if err := Advance(s, AdvanceInput{File: queueFile}, ""); err != nil {
		t.Fatal(err)
	}

	if s.ReverseEngineering.QueueFile != queueFile {
		t.Fatalf("QueueFile = %q, want %q", s.ReverseEngineering.QueueFile, queueFile)
	}
	if s.ReverseEngineering.QueueHash == "" {
		t.Fatal("QueueHash must be set after first advance")
	}
	expectedHash := computeContentHash([]byte(content))
	if s.ReverseEngineering.QueueHash != expectedHash {
		t.Fatalf("QueueHash = %q, want %q", s.ReverseEngineering.QueueHash, expectedHash)
	}
}

// TestReverseEngineeringQueueSubsequentAdvanceWithChangedFile verifies that a subsequent QUEUE
// advance with a changed file re-validates and advances state.
func TestReverseEngineeringQueueSubsequentAdvanceWithChangedFile(t *testing.T) {
	dir := t.TempDir()
	queueFile := filepath.Join(dir, "queue.json")
	initial := validREQueueJSON("optimizer")
	os.WriteFile(queueFile, []byte(initial), 0644)

	s := newReverseEngineeringState([]string{"optimizer"})
	s.State = StateQueue
	s.ReverseEngineering.QueueFile = queueFile
	s.ReverseEngineering.QueueHash = "stale-hash" // differs from file content

	if err := Advance(s, AdvanceInput{}, ""); err != nil {
		t.Fatal(err)
	}

	if s.State != StateExecute {
		t.Fatalf("expected EXECUTE, got %s", s.State)
	}
	// Hash must be updated to match file content.
	if s.ReverseEngineering.QueueHash == "stale-hash" {
		t.Fatal("QueueHash must be updated after successful subsequent advance")
	}
}

// TestReverseEngineeringQueueSubsequentAdvanceRejectsFile verifies that subsequent QUEUE
// advances reject the --file flag (path is already stored).
func TestReverseEngineeringQueueSubsequentAdvanceRejectsFile(t *testing.T) {
	dir := t.TempDir()
	queueFile := filepath.Join(dir, "queue.json")
	os.WriteFile(queueFile, []byte(validREQueueJSON("optimizer")), 0644)

	s := newReverseEngineeringState([]string{"optimizer"})
	s.State = StateQueue
	s.ReverseEngineering.QueueFile = queueFile
	s.ReverseEngineering.QueueHash = "old-hash"

	err := Advance(s, AdvanceInput{File: "/other/queue.json"}, "")
	if err == nil {
		t.Fatal("expected error when --file provided on subsequent advance")
	}
	if !strings.Contains(err.Error(), "Queue file path already set") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestReverseEngineeringQueueSubsequentAdvanceRejectsUnchangedFile verifies that a subsequent
// QUEUE advance is rejected when the file content has not changed.
func TestReverseEngineeringQueueSubsequentAdvanceRejectsUnchangedFile(t *testing.T) {
	dir := t.TempDir()
	queueFile := filepath.Join(dir, "queue.json")
	content := validREQueueJSON("optimizer")
	os.WriteFile(queueFile, []byte(content), 0644)

	s := newReverseEngineeringState([]string{"optimizer"})
	s.State = StateQueue
	s.ReverseEngineering.QueueFile = queueFile
	s.ReverseEngineering.QueueHash = computeContentHash([]byte(content)) // matches file

	err := Advance(s, AdvanceInput{}, "")
	if err == nil {
		t.Fatal("expected error when queue file has not changed")
	}
	if !strings.Contains(err.Error(), "Queue file has not changed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestReverseEngineeringQueueValidationFailureOnFirstAdvance verifies that schema validation
// errors block the first QUEUE advance and do not store the file path.
func TestReverseEngineeringQueueValidationFailureOnFirstAdvance(t *testing.T) {
	dir := t.TempDir()
	queueFile := filepath.Join(dir, "queue.json")
	// Missing required "code_search_roots" field.
	invalid := `{"specs":[{"name":"s1","domain":"optimizer","topic":"t","file":"f.md","action":"create","depends_on":[]}]}`
	os.WriteFile(queueFile, []byte(invalid), 0644)

	s := newReverseEngineeringState([]string{"optimizer"})
	s.State = StateQueue

	err := Advance(s, AdvanceInput{File: queueFile}, "")
	if err == nil {
		t.Fatal("expected validation error")
	}
	// Path must NOT be stored on validation failure.
	if s.ReverseEngineering.QueueFile != "" {
		t.Fatalf("QueueFile must not be stored on validation failure, got %q", s.ReverseEngineering.QueueFile)
	}
}

// TestReverseEngineeringQueueDomainMembershipRejection verifies that entries with unrecognized
// domains are rejected at QUEUE advance.
func TestReverseEngineeringQueueDomainMembershipRejection(t *testing.T) {
	dir := t.TempDir()
	queueFile := filepath.Join(dir, "queue.json")
	// Entry has domain "portal" which is not in initialized domains.
	content := `{"specs":[{"name":"s1","domain":"portal","topic":"t","file":"f.md","action":"create","code_search_roots":["src/"],"depends_on":[]}]}`
	os.WriteFile(queueFile, []byte(content), 0644)

	s := newReverseEngineeringState([]string{"optimizer", "api"})
	s.State = StateQueue

	err := Advance(s, AdvanceInput{File: queueFile}, "")
	if err == nil {
		t.Fatal("expected domain membership error")
	}
	// Should be a validation error mentioning the unrecognized domain.
	if _, ok := err.(*ValidationError); !ok {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
}

// --- EXECUTE State Tests ---

// setupREExecuteState creates a state at StateExecute with a valid queue file.
func setupREExecuteState(t *testing.T, dir string, specs []ReverseEngineeringQueueEntry, mode string) *ForgeState {
	t.Helper()
	domains := uniqueQueueDomains(specs)
	s := &ForgeState{
		Phase:              PhaseReverseEngineering,
		State:              StateExecute,
		Config:             DefaultForgeConfig(),
		ReverseEngineering: NewReverseEngineeringState("test concept", domains, false),
	}
	s.Config.ReverseEngineering.Mode = mode
	// Use a relative path so advanceREFromExecute joins it with dir correctly.
	s.Config.Paths.StateDir = ".forgectl/state"
	os.MkdirAll(filepath.Join(dir, ".forgectl", "state"), 0755)

	qi := ReverseEngineeringQueueInput{Specs: specs}
	queueData, _ := json.Marshal(qi)
	queueFile := filepath.Join(dir, "queue.json")
	os.WriteFile(queueFile, queueData, 0644)
	s.ReverseEngineering.QueueFile = queueFile

	return s
}

func uniqueQueueDomains(specs []ReverseEngineeringQueueEntry) []string {
	seen := make(map[string]bool)
	var domains []string
	for _, s := range specs {
		if !seen[s.Domain] {
			seen[s.Domain] = true
			domains = append(domains, s.Domain)
		}
	}
	return domains
}

func makeRESpec(name, domain string) ReverseEngineeringQueueEntry {
	return ReverseEngineeringQueueEntry{
		Name:            name,
		Domain:          domain,
		Topic:           "test topic",
		File:            domain + "/specs/" + name + ".md",
		Action:          "create",
		CodeSearchRoots: []string{"src/"},
		DependsOn:       []string{},
	}
}

// withSuccessRunner replaces pyRunner for the duration of the test, writing
// success results to execute.json before returning exit code 0.
func withSuccessRunner(t *testing.T) func() {
	t.Helper()
	old := pyRunner
	pyRunner = func(executeFilePath, dir string) (string, int) {
		data, err := os.ReadFile(executeFilePath)
		if err != nil {
			return "cannot read execute.json", 1
		}
		var ef ExecuteJSONFile
		if err := json.Unmarshal(data, &ef); err != nil {
			return "cannot parse execute.json", 1
		}
		for i := range ef.Specs {
			status := "success"
			iters := 1
			ef.Specs[i].Result = &ExecuteJSONSpecResult{Status: status, IterationsCompleted: &iters}
		}
		updated, _ := json.MarshalIndent(ef, "", "  ")
		os.WriteFile(executeFilePath, updated, 0644)
		return "", 0
	}
	return func() { pyRunner = old }
}

// withPartialFailureRunner replaces pyRunner: first entry fails, rest succeed.
func withPartialFailureRunner(t *testing.T) func() {
	t.Helper()
	old := pyRunner
	pyRunner = func(executeFilePath, dir string) (string, int) {
		data, _ := os.ReadFile(executeFilePath)
		var ef ExecuteJSONFile
		json.Unmarshal(data, &ef)
		for i := range ef.Specs {
			if i == 0 {
				errMsg := "agent timed out"
				ef.Specs[i].Result = &ExecuteJSONSpecResult{Status: "failure", Error: &errMsg}
			} else {
				iters := 1
				ef.Specs[i].Result = &ExecuteJSONSpecResult{Status: "success", IterationsCompleted: &iters}
			}
		}
		updated, _ := json.MarshalIndent(ef, "", "  ")
		os.WriteFile(executeFilePath, updated, 0644)
		return "", 0
	}
	return func() { pyRunner = old }
}

// withNonZeroExitRunner replaces pyRunner: exits non-zero and does NOT write execute.json.
func withNonZeroExitRunner(t *testing.T, stderrMsg string) func() {
	t.Helper()
	old := pyRunner
	pyRunner = func(executeFilePath, dir string) (string, int) {
		// Remove execute.json to simulate unreadable results.
		os.Remove(executeFilePath)
		return stderrMsg, 1
	}
	return func() { pyRunner = old }
}

// TestReverseEngineeringExecuteGeneratesExecuteJSON verifies that EXECUTE writes execute.json
// with the correct structure: project_root, active mode config, and all queue entries.
func TestReverseEngineeringExecuteGeneratesExecuteJSON(t *testing.T) {
	dir := t.TempDir()
	specs := []ReverseEngineeringQueueEntry{makeRESpec("auth-handler", "api")}
	s := setupREExecuteState(t, dir, specs, "self_refine")
	s.Config.ReverseEngineering.SelfRefine = &SelfRefineConfig{Rounds: 2}

	defer withSuccessRunner(t)()

	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("Advance failed: %v", err)
	}

	// Read execute.json from state dir.
	executeFilePath := filepath.Join(dir, ".forgectl", "state", "execute.json")
	data, err := os.ReadFile(executeFilePath)
	if err != nil {
		t.Fatalf("execute.json not written: %v", err)
	}

	var ef ExecuteJSONFile
	if err := json.Unmarshal(data, &ef); err != nil {
		t.Fatalf("invalid execute.json: %v", err)
	}

	if ef.ProjectRoot != dir {
		t.Errorf("project_root = %q, want %q", ef.ProjectRoot, dir)
	}
	if ef.Config.Mode != "self_refine" {
		t.Errorf("config.mode = %q, want self_refine", ef.Config.Mode)
	}
	if ef.Config.SelfRefine == nil || ef.Config.SelfRefine.Rounds != 2 {
		t.Errorf("config.self_refine not set correctly")
	}
	if ef.Config.MultiPass != nil {
		t.Errorf("inactive multi_pass should be omitted, got %+v", ef.Config.MultiPass)
	}
	if len(ef.Specs) != 1 || ef.Specs[0].Name != "auth-handler" {
		t.Errorf("unexpected specs: %+v", ef.Specs)
	}
}

// TestReverseEngineeringExecuteCreatesSpecsDirectories verifies that EXECUTE creates
// <project_root>/<domain>/specs/ directories for each unique domain before invoking the subprocess.
func TestReverseEngineeringExecuteCreatesSpecsDirectories(t *testing.T) {
	dir := t.TempDir()
	specs := []ReverseEngineeringQueueEntry{
		makeRESpec("spec-a", "api"),
		makeRESpec("spec-b", "api"), // same domain — deduped
		makeRESpec("spec-c", "billing"),
	}
	s := setupREExecuteState(t, dir, specs, "single_shot")

	defer withSuccessRunner(t)()

	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("Advance failed: %v", err)
	}

	for _, domain := range []string{"api", "billing"} {
		specsDir := filepath.Join(dir, domain, "specs")
		if _, err := os.Stat(specsDir); os.IsNotExist(err) {
			t.Errorf("specs dir not created: %s", specsDir)
		}
	}
}

// TestReverseEngineeringExecuteAllSuccessAdvancesToReconcile verifies that when all subprocess
// results are "success", state advances to RECONCILE with reconcile_domain=0 and round=1.
func TestReverseEngineeringExecuteAllSuccessAdvancesToReconcile(t *testing.T) {
	dir := t.TempDir()
	specs := []ReverseEngineeringQueueEntry{makeRESpec("spec-a", "api"), makeRESpec("spec-b", "api")}
	s := setupREExecuteState(t, dir, specs, "single_shot")

	defer withSuccessRunner(t)()

	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("Advance failed: %v", err)
	}

	if s.State != StateReconcile {
		t.Errorf("state = %s, want RECONCILE", s.State)
	}
	if s.ReverseEngineering.ReconcileDomain != 0 {
		t.Errorf("reconcile_domain = %d, want 0", s.ReverseEngineering.ReconcileDomain)
	}
	if s.ReverseEngineering.Round != 1 {
		t.Errorf("round = %d, want 1", s.ReverseEngineering.Round)
	}
}

// TestReverseEngineeringExecutePartialFailureStaysInExecute verifies that when any subprocess
// result is "failure", state stays in EXECUTE and per-entry results are written to executeOutput.
func TestReverseEngineeringExecutePartialFailureStaysInExecute(t *testing.T) {
	dir := t.TempDir()
	specs := []ReverseEngineeringQueueEntry{makeRESpec("spec-a", "api"), makeRESpec("spec-b", "api")}
	s := setupREExecuteState(t, dir, specs, "single_shot")

	defer withPartialFailureRunner(t)()

	var buf bytes.Buffer
	oldOut := executeOutput
	executeOutput = &buf
	defer func() { executeOutput = oldOut }()

	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("Advance failed: %v", err)
	}

	if s.State != StateExecute {
		t.Errorf("state = %s, want EXECUTE (partial failure should stay in EXECUTE)", s.State)
	}

	out := buf.String()
	if !strings.Contains(out, "failure") {
		t.Errorf("expected per-entry failure output, got:\n%s", out)
	}
	if !strings.Contains(out, "spec-a") {
		t.Errorf("expected failed entry name in output, got:\n%s", out)
	}
	if !strings.Contains(out, "success") {
		t.Errorf("expected successful entry in output, got:\n%s", out)
	}
}

// TestReverseEngineeringExecuteEmptyQueueRejected verifies that an empty queue returns an error
// before any subprocess invocation.
func TestReverseEngineeringExecuteEmptyQueueRejected(t *testing.T) {
	dir := t.TempDir()
	s := setupREExecuteState(t, dir, []ReverseEngineeringQueueEntry{}, "single_shot")

	subprocessCalled := false
	old := pyRunner
	pyRunner = func(_, _ string) (string, int) {
		subprocessCalled = true
		return "", 0
	}
	defer func() { pyRunner = old }()

	err := Advance(s, AdvanceInput{}, dir)
	if err == nil {
		t.Fatal("expected error for empty queue")
	}
	if !strings.Contains(err.Error(), "zero entries") {
		t.Errorf("expected 'zero entries' in error, got: %v", err)
	}
	if subprocessCalled {
		t.Error("subprocess must not be invoked for empty queue")
	}
	if s.State != StateExecute {
		t.Errorf("state should stay in EXECUTE, got %s", s.State)
	}
}

// TestReverseEngineeringExecuteSubprocessFailureOutputsStop verifies that when the subprocess
// exits non-zero and execute.json is unreadable, the STOP message is written to executeOutput.
func TestReverseEngineeringExecuteSubprocessFailureOutputsStop(t *testing.T) {
	dir := t.TempDir()
	specs := []ReverseEngineeringQueueEntry{makeRESpec("spec-a", "api")}
	s := setupREExecuteState(t, dir, specs, "single_shot")

	stderrMsg := "Traceback: KeyError 'model'"
	defer withNonZeroExitRunner(t, stderrMsg)()

	var buf bytes.Buffer
	oldOut := executeOutput
	executeOutput = &buf
	defer func() { executeOutput = oldOut }()

	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("Advance should not return error for subprocess failure, got: %v", err)
	}

	if s.State != StateExecute {
		t.Errorf("state = %s, want EXECUTE after subprocess failure", s.State)
	}
	out := buf.String()
	if !strings.Contains(out, "STOP") {
		t.Errorf("expected STOP in output, got:\n%s", out)
	}
	if !strings.Contains(out, stderrMsg) {
		t.Errorf("expected stderr in output, got:\n%s", out)
	}
}

// TestReverseEngineeringExecuteWorksFromAnyDir verifies that EXECUTE uses absolute paths
// so it works correctly regardless of the current working directory.
func TestReverseEngineeringExecuteWorksFromAnyDir(t *testing.T) {
	dir := t.TempDir()
	specs := []ReverseEngineeringQueueEntry{makeRESpec("spec-a", "api")}
	s := setupREExecuteState(t, dir, specs, "single_shot")

	// Capture the executeFilePath passed to the subprocess.
	var capturedPath string
	old := pyRunner
	pyRunner = func(executeFilePath, runDir string) (string, int) {
		capturedPath = executeFilePath
		// Write success results.
		data, _ := os.ReadFile(executeFilePath)
		var ef ExecuteJSONFile
		json.Unmarshal(data, &ef)
		iters := 1
		for i := range ef.Specs {
			ef.Specs[i].Result = &ExecuteJSONSpecResult{Status: "success", IterationsCompleted: &iters}
		}
		updated, _ := json.MarshalIndent(ef, "", "  ")
		os.WriteFile(executeFilePath, updated, 0644)
		return "", 0
	}
	defer func() { pyRunner = old }()

	if err := Advance(s, AdvanceInput{}, dir); err != nil {
		t.Fatalf("Advance failed: %v", err)
	}

	if !filepath.IsAbs(capturedPath) {
		t.Errorf("execute file path passed to subprocess is not absolute: %q", capturedPath)
	}
	if s.ReverseEngineering.ExecuteFile != capturedPath {
		t.Errorf("state.execute_file = %q, want %q", s.ReverseEngineering.ExecuteFile, capturedPath)
	}
}
