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
		Config:         makeTestConfig(2, 1, 3),
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
		Config:         makeTestConfig(2, 1, 3),
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

func makeTestConfig(batchSize, minRounds, maxRounds int) Config {
	cfg := DefaultConfig()
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
		Phase:      PhaseSpecifying,
		State:      StateOrient,
		Config:     makeTestConfig(2, 1, 3),
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
	s := newSpecifyingState(1)
	s.Config.General.EnableCommits = true
	advanceToEvaluate(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)

	err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, "")
	if err == nil {
		t.Error("expected error for missing --message with PASS when enable_commits is true")
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
	// CROSS_REFERENCE_EVAL must reject advance without verdict or eval-report.
	s := newSpecifyingState(1)
	advanceToAccept(t, s)
	Advance(s, AdvanceInput{}, "")  // ACCEPT → CROSS_REFERENCE
	Advance(s, AdvanceInput{}, "")  // CROSS_REFERENCE → CROSS_REFERENCE_EVAL

	// Missing both.
	err := Advance(s, AdvanceInput{}, "")
	if err == nil {
		t.Error("expected error for missing --verdict in CROSS_REFERENCE_EVAL")
	}

	// Verdict present but missing eval-report.
	err = Advance(s, AdvanceInput{Verdict: "PASS"}, "")
	if err == nil {
		t.Error("expected error for missing --eval-report in CROSS_REFERENCE_EVAL")
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
	// RECONCILE_EVAL must reject advance without --eval-report.
	s := newSpecifyingState(1)
	advanceToDone(t, s)
	Advance(s, AdvanceInput{}, "") // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "") // RECONCILE → RECONCILE_EVAL

	// Missing eval-report.
	err := Advance(s, AdvanceInput{Verdict: "PASS"}, "")
	if err == nil {
		t.Error("expected error for missing --eval-report in RECONCILE_EVAL")
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
	// --message required at RECONCILE_EVAL PASS when enable_commits=true.
	s := newSpecifyingState(1)
	s.Config.General.EnableCommits = true
	advanceToDone(t, s)

	dir := t.TempDir()
	evalFile := filepath.Join(dir, "re-eval.md")
	os.WriteFile(evalFile, []byte("reconcile eval"), 0644)

	Advance(s, AdvanceInput{}, "") // DONE → RECONCILE
	Advance(s, AdvanceInput{}, "") // RECONCILE → RECONCILE_EVAL

	// PASS without --message should fail when enable_commits=true.
	err := Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, "")
	if err == nil {
		t.Error("expected error for missing --message when enable_commits=true")
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
			{Name: "Plan1", Domain: "test", File: "plan.json", Specs: []string{"spec.md"}, SpecCommits: []string{}, CodeSearchRoots: []string{"test/"}},
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

func newPlanningState() *ForgeState {
	return &ForgeState{
		Phase:  PhasePlanning,
		State:  StateOrient,
		Config: makeTestConfig(2, 1, 3),
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
			Completed: []interface{}{},
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
				Ref:         "notes/config.md",
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
		Phase:  PhaseImplementing,
		State:  StateOrient,
		Config: makeTestConfig(batchSize, 1, 3),
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
