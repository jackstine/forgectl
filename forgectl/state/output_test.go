package state

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// outputOf runs PrintAdvanceOutput and returns the result as a string.
func outputOf(s *ForgeState, dir string) string {
	var buf bytes.Buffer
	PrintAdvanceOutput(&buf, s, dir)
	return buf.String()
}

// TestOutputCommitEnableCommitsShowsMessage verifies that COMMIT with enable_commits=true
// instructs the user to advance with --message.
func TestOutputCommitEnableCommitsShowsMessage(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)
	s.Config.General.EnableCommits = true
	advanceImplToCommit(t, s, dir)

	out := outputOf(s, dir)
	if !strings.Contains(out, "--message") {
		t.Errorf("expected --message in COMMIT output with enable_commits=true, got:\n%s", out)
	}
	if strings.Contains(out, "Advance to continue.") {
		t.Errorf("unexpected 'Advance to continue.' in COMMIT output with enable_commits=true, got:\n%s", out)
	}
}

// TestOutputCommitNoCommitsShowsAdvance verifies that COMMIT with enable_commits=false
// shows a simple "Advance to continue." action.
func TestOutputCommitNoCommitsShowsAdvance(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)
	s.Config.General.EnableCommits = false
	advanceImplToCommit(t, s, dir)

	out := outputOf(s, dir)
	if !strings.Contains(out, "advance to continue") {
		t.Errorf("expected 'advance to continue' in COMMIT output with enable_commits=false, got:\n%s", out)
	}
	if strings.Contains(out, "--message") {
		t.Errorf("unexpected --message in COMMIT output with enable_commits=false, got:\n%s", out)
	}
}

// TestOutputAcceptEnableCommitsShowsMessage verifies that ACCEPT with enable_commits=true
// instructs the user to advance with --message.
func TestOutputAcceptEnableCommitsShowsMessage(t *testing.T) {
	dir := t.TempDir()
	createValidPlan(t, dir, "impl/plan.json")
	s := newPlanningStateForCommit(t, dir)
	s.Config.General.EnableCommits = true
	s.Planning.Round = 1
	s.Planning.Evals = []EvalRecord{{Round: 1, Verdict: "PASS"}}
	s.State = StateAccept

	out := outputOf(s, dir)
	if !strings.Contains(out, "--message") {
		t.Errorf("expected --message in ACCEPT output with enable_commits=true, got:\n%s", out)
	}
}

// TestOutputAcceptNoCommitsShowsAdvance verifies that ACCEPT with enable_commits=false
// shows "Advance to continue.".
func TestOutputAcceptNoCommitsShowsAdvance(t *testing.T) {
	dir := t.TempDir()
	createValidPlan(t, dir, "impl/plan.json")
	s := newPlanningStateForCommit(t, dir)
	s.Config.General.EnableCommits = false
	s.Planning.Round = 1
	s.Planning.Evals = []EvalRecord{{Round: 1, Verdict: "PASS"}}
	s.State = StateAccept

	out := outputOf(s, dir)
	if !strings.Contains(out, "Advance to continue.") {
		t.Errorf("expected 'Advance to continue.' in ACCEPT output with enable_commits=false, got:\n%s", out)
	}
	if strings.Contains(out, "--message") {
		t.Errorf("unexpected --message in ACCEPT output with enable_commits=false, got:\n%s", out)
	}
}

// TestOutputImplementSpecsAndRefsMultiline verifies that IMPLEMENT output shows
// Specs:/Refs: labels with multiline formatting.
func TestOutputImplementSpecsAndRefsMultiline(t *testing.T) {
	dir := t.TempDir()

	// Build a plan with multi-spec, multi-ref item.
	planPath := filepath.Join(dir, "impl", "plan.json")
	os.MkdirAll(filepath.Dir(planPath), 0755)
	notesDir := filepath.Join(filepath.Dir(planPath), "notes")
	os.MkdirAll(notesDir, 0755)
	os.WriteFile(filepath.Join(notesDir, "a.md"), []byte("notes"), 0644)
	os.WriteFile(filepath.Join(notesDir, "b.md"), []byte("notes"), 0644)

	plan := PlanJSON{
		Context: PlanContext{Domain: "test", Module: "mod"},
		Layers:  []PlanLayerDef{{ID: "L0", Name: "Base", Items: []string{"x.item"}}},
		Items: []PlanItem{
			{
				ID:          "x.item",
				Name:        "X Item",
				Description: "desc",
				DependsOn:   []string{},
				Passes:      "pending",
				Specs:       []string{"spec-a.md#section", "spec-b.md#other"},
				Refs:        []string{"notes/a.md", "notes/b.md"},
				Tests:       []PlanTest{{Category: "functional", Description: "works"}},
			},
		},
	}
	data, _ := json.Marshal(plan)
	os.WriteFile(planPath, data, 0644)

	s := &ForgeState{
		Phase: PhaseImplementing,
		State: StateOrient,
		Config: ForgeConfig{
			Implementing: ImplementingConfig{
				Batch: 2,
				Eval:  EvalConfig{MinRounds: 1, MaxRounds: 3},
			},
		},
		Planning: &PlanningState{
			CurrentPlan: &ActivePlan{ID: 1, Name: "Test Plan", Domain: "test", File: "impl/plan.json"},
		},
		Implementing: NewImplementingState(),
	}

	// Advance to IMPLEMENT.
	Advance(s, AdvanceInput{}, dir)
	if s.State != StateImplement {
		t.Fatalf("expected IMPLEMENT, got %s", s.State)
	}

	out := outputOf(s, dir)
	if !strings.Contains(out, "Specs:   spec-a.md#section") {
		t.Errorf("expected 'Specs:   spec-a.md#section', got:\n%s", out)
	}
	if !strings.Contains(out, "         spec-b.md#other") {
		t.Errorf("expected indented second spec, got:\n%s", out)
	}
	if !strings.Contains(out, "Refs:    notes/a.md") {
		t.Errorf("expected 'Refs:    notes/a.md', got:\n%s", out)
	}
	if !strings.Contains(out, "         notes/b.md") {
		t.Errorf("expected indented second ref, got:\n%s", out)
	}
	if strings.Contains(out, "Spec:    ") {
		t.Errorf("unexpected old 'Spec:' label in output, got:\n%s", out)
	}
	if strings.Contains(out, "Ref:     ") {
		t.Errorf("unexpected old 'Ref:' label in output, got:\n%s", out)
	}
}

// TestOutputOrientNextBatchCount verifies that after a COMMIT within a layer,
// the ORIENT output shows "Next: N unblocked items in next batch".
func TestOutputOrientNextBatchCount(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 3, 1) // 3 items, batch=1
	s.Config.General.EnableCommits = false

	// Advance through first item (ORIENT→IMPLEMENT→EVALUATE→COMMIT→ORIENT).
	Advance(s, AdvanceInput{}, dir) // ORIENT→IMPLEMENT
	Advance(s, AdvanceInput{Message: "msg"}, dir) // IMPLEMENT→EVALUATE

	evalFile := filepath.Join(dir, "eval.md")
	os.WriteFile(evalFile, []byte("eval"), 0644)
	Advance(s, AdvanceInput{Verdict: "PASS", EvalReport: evalFile}, dir) // EVALUATE→COMMIT
	Advance(s, AdvanceInput{Message: "commit"}, dir)                     // COMMIT→ORIENT

	if s.State != StateOrient {
		t.Fatalf("expected ORIENT, got %s", s.State)
	}

	out := outputOf(s, dir)
	if !strings.Contains(out, "Next:") {
		t.Errorf("expected 'Next:' line in ORIENT output, got:\n%s", out)
	}
	if !strings.Contains(out, "unblocked items in next batch") {
		t.Errorf("expected 'unblocked items in next batch' in ORIENT output, got:\n%s", out)
	}
}

// TestOutputOrientFinalLayerLabel verifies that the Progress line shows "(final layer)"
// when the current layer is the last layer and all its items are terminal.
func TestOutputOrientFinalLayerLabel(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1) // 1 item, 1 layer

	// Advance to IMPLEMENT to set CurrentLayer and CurrentBatch.
	Advance(s, AdvanceInput{}, dir) // initial ORIENT → IMPLEMENT

	// Mark the item as passed in the plan file and set state to ORIENT to test output.
	plan, err := loadPlan(s, dir)
	if err != nil {
		t.Fatalf("loadPlan: %v", err)
	}
	for i := range plan.Items {
		plan.Items[i].Passes = "passed"
		plan.Items[i].Rounds = 1
	}
	if err := savePlan(s, dir, plan); err != nil {
		t.Fatalf("savePlan: %v", err)
	}
	s.State = StateOrient

	out := outputOf(s, dir)
	if !strings.Contains(out, "final layer") {
		t.Errorf("expected 'final layer' in Progress line, got:\n%s", out)
	}
}

// TestEvalOutputPlanningReportSectionWithEnableEvalOutput verifies that planning eval output
// includes '--- REPORT OUTPUT ---' when enable_eval_output is true.
func TestEvalOutputPlanningReportSectionWithEnableEvalOutput(t *testing.T) {
	dir := t.TempDir()
	createValidPlan(t, dir, "impl/plan.json")
	s := newPlanningState()
	s.Planning.CurrentPlan.File = "impl/plan.json"
	s.Planning.Round = 1
	s.Config.General.EnableEvalOutput = true
	s.State = StateEvaluate

	var buf bytes.Buffer
	if err := PrintEvalOutput(&buf, s, dir); err != nil {
		t.Fatalf("PrintEvalOutput: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "--- REPORT OUTPUT ---") {
		t.Errorf("expected '--- REPORT OUTPUT ---' in planning eval output with enable_eval_output=true, got:\n%s", out)
	}
}

// TestEvalOutputPlanningReportSectionOmittedWithoutEnableEvalOutput verifies that planning eval
// output omits '--- REPORT OUTPUT ---' when enable_eval_output is false.
func TestEvalOutputPlanningReportSectionOmittedWithoutEnableEvalOutput(t *testing.T) {
	dir := t.TempDir()
	createValidPlan(t, dir, "impl/plan.json")
	s := newPlanningState()
	s.Planning.CurrentPlan.File = "impl/plan.json"
	s.Planning.Round = 1
	s.Config.General.EnableEvalOutput = false
	s.State = StateEvaluate

	var buf bytes.Buffer
	if err := PrintEvalOutput(&buf, s, dir); err != nil {
		t.Fatalf("PrintEvalOutput: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "--- REPORT OUTPUT ---") {
		t.Errorf("unexpected '--- REPORT OUTPUT ---' in planning eval output with enable_eval_output=false, got:\n%s", out)
	}
}

// TestEvalOutputReconcileEvalContainsEvaluatorPrompt verifies that eval in RECONCILE_EVAL
// state outputs the reconcile-eval.md contents.
func TestEvalOutputReconcileEvalContainsEvaluatorPrompt(t *testing.T) {
	s := &ForgeState{
		Phase: PhaseSpecifying,
		State: StateReconcileEval,
		Config: ForgeConfig{
			Specifying: SpecifyingConfig{
				Reconciliation: ReconciliationConfig{
					MinRounds: 1,
					MaxRounds: 2,
				},
			},
		},
		Specifying: &SpecifyingState{
			Reconcile: &ReconcileState{Round: 1},
			Completed: []CompletedSpec{
				{ID: 1, Name: "spec-a.md", Domain: "optimizer", File: "optimizer/specs/spec-a.md", RoundsTaken: 1},
				{ID: 2, Name: "spec-b.md", Domain: "portal", File: "portal/specs/spec-b.md", RoundsTaken: 1},
			},
		},
	}

	var buf bytes.Buffer
	if err := PrintReconcileEvalOutput(&buf, s); err != nil {
		t.Fatalf("PrintReconcileEvalOutput: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "RECONCILIATION EVALUATION") {
		t.Errorf("expected 'RECONCILIATION EVALUATION' header, got:\n%s", out)
	}
	if !strings.Contains(out, "--- EVALUATOR INSTRUCTIONS ---") {
		t.Errorf("expected evaluator instructions section, got:\n%s", out)
	}
	if !strings.Contains(out, "Reconciliation Evaluation Prompt") {
		t.Errorf("expected reconcile-eval.md contents, got:\n%s", out)
	}
	if !strings.Contains(out, "--- DOMAINS ---") {
		t.Errorf("expected domains section, got:\n%s", out)
	}
	if !strings.Contains(out, "optimizer: 1") {
		t.Errorf("expected optimizer domain count, got:\n%s", out)
	}
	if !strings.Contains(out, "--- RECONCILIATION CONTEXT ---") {
		t.Errorf("expected reconciliation context section, got:\n%s", out)
	}
	if strings.Contains(out, "--- REPORT OUTPUT ---") {
		t.Errorf("unexpected report output section when enable_eval_output=false, got:\n%s", out)
	}
}

// TestEvalOutputCrossRefEvalContainsEvaluatorPrompt verifies that eval in CROSS_REFERENCE_EVAL
// state outputs the cross-reference-eval.md contents.
func TestEvalOutputCrossRefEvalContainsEvaluatorPrompt(t *testing.T) {
	s := &ForgeState{
		Phase: PhaseSpecifying,
		State: StateCrossReferenceEval,
		Config: ForgeConfig{
			Specifying: SpecifyingConfig{
				CrossReference: CrossRefConfig{
					MinRounds: 1,
					MaxRounds: 2,
				},
			},
		},
		Specifying: &SpecifyingState{
			CurrentDomain: "optimizer",
			CrossReference: map[string]*CrossReferenceState{
				"optimizer": {Domain: "optimizer", Round: 1},
			},
			Completed: []CompletedSpec{
				{ID: 1, Name: "spec-a.md", Domain: "optimizer", File: "optimizer/specs/spec-a.md", RoundsTaken: 1},
				{ID: 2, Name: "spec-b.md", Domain: "optimizer", File: "optimizer/specs/spec-b.md", RoundsTaken: 1},
			},
		},
	}

	var buf bytes.Buffer
	if err := PrintCrossRefEvalOutput(&buf, s); err != nil {
		t.Fatalf("PrintCrossRefEvalOutput: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "CROSS-REFERENCE EVALUATION") {
		t.Errorf("expected 'CROSS-REFERENCE EVALUATION' header, got:\n%s", out)
	}
	if !strings.Contains(out, "--- EVALUATOR INSTRUCTIONS ---") {
		t.Errorf("expected evaluator instructions section, got:\n%s", out)
	}
	if !strings.Contains(out, "Cross-Reference Evaluation Prompt") {
		t.Errorf("expected cross-reference-eval.md contents, got:\n%s", out)
	}
	if !strings.Contains(out, "--- DOMAIN ---") {
		t.Errorf("expected domain section, got:\n%s", out)
	}
	if !strings.Contains(out, "optimizer: 2") {
		t.Errorf("expected optimizer domain with 2 specs, got:\n%s", out)
	}
	if !strings.Contains(out, "--- SPECS ---") {
		t.Errorf("expected specs section, got:\n%s", out)
	}
}

// TestEvalOutputOutsideValidStatesReturnsError verifies that eval command outside
// valid states returns an error naming the current state.
func TestEvalOutputOutsideValidStatesReturnsError(t *testing.T) {
	s := &ForgeState{
		Phase: PhaseSpecifying,
		State: StateDraft,
		Config: ForgeConfig{
			Specifying: SpecifyingConfig{
				Eval: EvalConfig{MinRounds: 1, MaxRounds: 3},
			},
		},
	}

	var buf bytes.Buffer
	err := PrintEvalOutput(&buf, s, ".")
	if err == nil {
		t.Fatal("expected error when calling PrintEvalOutput in non-evaluation state")
	}
	if !strings.Contains(err.Error(), string(StateDraft)) {
		t.Errorf("expected error to mention current state %q, got: %v", StateDraft, err)
	}
}

// TestOutputDoneDomainVariantWhenPlansRemain verifies that the DONE output shows
// "Domain complete. Advance to continue to next domain." when plans remain.
// TestREEvalOutputContainsPromptAndSpecs verifies that PrintReverseEngineeringEvalOutput
// outputs the evaluator prompt, spec list with depends_on, and eval report path.
func TestREEvalOutputContainsPromptAndSpecs(t *testing.T) {
	dir := t.TempDir()

	queueFile := filepath.Join(dir, "re-queue.json")
	qi := ReverseEngineeringQueueInput{
		Specs: []ReverseEngineeringQueueEntry{
			{Name: "Auth Init", Domain: "api", File: "specs/auth-init.md", Action: "create", DependsOn: []string{}},
			{Name: "Auth Tokens", Domain: "api", File: "specs/auth-tokens.md", Action: "create", DependsOn: []string{"Auth Init"}},
		},
	}
	data, _ := json.Marshal(qi)
	os.WriteFile(queueFile, data, 0644)

	s := newREState([]string{"api"})
	s.State = StateReconcileEval
	s.ReverseEngineering.ReconcileDomain = 0
	s.ReverseEngineering.Round = 1
	s.ReverseEngineering.QueueFile = queueFile
	s.Config.ReverseEngineering.Reconcile.MaxRounds = 3

	var buf bytes.Buffer
	if err := PrintReverseEngineeringEvalOutput(&buf, s); err != nil {
		t.Fatalf("PrintReverseEngineeringEvalOutput: %v", err)
	}
	out := buf.String()

	if !strings.Contains(out, "RECONCILIATION EVALUATION ROUND 1/3") {
		t.Errorf("expected round header in eval output, got:\n%s", out)
	}
	if !strings.Contains(out, "--- EVALUATOR INSTRUCTIONS ---") {
		t.Errorf("expected evaluator instructions section, got:\n%s", out)
	}
	if !strings.Contains(out, "Reconciliation Evaluation Prompt") {
		t.Errorf("expected embedded prompt in eval output, got:\n%s", out)
	}
	if !strings.Contains(out, "api/specs/auth-init.md") {
		t.Errorf("expected spec file in eval output, got:\n%s", out)
	}
	if !strings.Contains(out, "Auth Init") {
		t.Errorf("expected depends_on reference in eval output, got:\n%s", out)
	}
	if !strings.Contains(out, "reconciliation-r1.md") {
		t.Errorf("expected eval report path in eval output, got:\n%s", out)
	}
}

// TestREEvalOutputRejectsNonReconcileEvalState verifies that PrintReverseEngineeringEvalOutput
// returns an error when called outside RECONCILE_EVAL state.
func TestREEvalOutputRejectsNonReconcileEvalState(t *testing.T) {
	s := newREState([]string{"api"})
	s.State = StateReconcile // wrong state

	var buf bytes.Buffer
	err := PrintReverseEngineeringEvalOutput(&buf, s)
	if err == nil {
		t.Fatal("expected error when calling PrintReverseEngineeringEvalOutput outside RECONCILE_EVAL")
	}
	if !strings.Contains(err.Error(), "RECONCILE_EVAL") {
		t.Errorf("expected error to mention RECONCILE_EVAL, got: %v", err)
	}
}

// --- Reverse Engineering Output Tests ---

func newREState(domains []string) *ForgeState {
	cfg := DefaultForgeConfig()
	return &ForgeState{
		Phase:  PhaseReverseEngineering,
		State:  StateOrient,
		Config: cfg,
		ReverseEngineering: &ReverseEngineeringState{
			Concept:       "understand the auth system",
			Domains:       domains,
			TotalDomains:  len(domains),
			CurrentDomain: 0,
			Round:         1,
		},
	}
}

func TestStatusREShowsModeAndRounds(t *testing.T) {
	s := newREState([]string{"api", "billing"})
	s.Config.ReverseEngineering.Mode = "self_refine"
	s.Config.ReverseEngineering.Reconcile.MinRounds = 1
	s.Config.ReverseEngineering.Reconcile.MaxRounds = 3

	var buf bytes.Buffer
	PrintStatus(&buf, s, t.TempDir(), false)
	out := buf.String()

	if !strings.Contains(out, "mode=self_refine") {
		t.Errorf("expected mode in RE status config line, got:\n%s", out)
	}
	if !strings.Contains(out, "rounds=1-3") {
		t.Errorf("expected reconcile rounds in RE status config line, got:\n%s", out)
	}
	// Should NOT show batch= for RE phase.
	if strings.Contains(out, "batch=") {
		t.Errorf("unexpected batch= in RE status config line, got:\n%s", out)
	}
}

func TestOutputREOrientShowsConceptAndDomains(t *testing.T) {
	s := newREState([]string{"api", "billing"})
	s.State = StateOrient

	out := outputOf(s, t.TempDir())
	if !strings.Contains(out, "understand the auth system") {
		t.Errorf("expected concept in ORIENT output, got:\n%s", out)
	}
	if !strings.Contains(out, "api (1/2)") {
		t.Errorf("expected domain index in ORIENT output, got:\n%s", out)
	}
	if !strings.Contains(out, "billing (2/2)") {
		t.Errorf("expected second domain in ORIENT output, got:\n%s", out)
	}
	if !strings.Contains(out, "Advance to begin SURVEY on domain: api") {
		t.Errorf("expected advance action in ORIENT output, got:\n%s", out)
	}
}

func TestOutputRESurveyShowsDomainAndSubagentConfig(t *testing.T) {
	s := newREState([]string{"api"})
	s.State = StateSurvey
	s.Config.ReverseEngineering.Survey = SubAgentConfig{Model: "haiku", Type: "explorer", Count: 3}

	out := outputOf(s, t.TempDir())
	if !strings.Contains(out, "api (1/1)") {
		t.Errorf("expected domain in SURVEY output, got:\n%s", out)
	}
	if !strings.Contains(out, "3 haiku explorer") {
		t.Errorf("expected sub-agent config in SURVEY output, got:\n%s", out)
	}
	if !strings.Contains(out, "api/specs/") {
		t.Errorf("expected specs dir in SURVEY output, got:\n%s", out)
	}
}

func TestOutputRESurveyNotesMissingSpecsDir(t *testing.T) {
	dir := t.TempDir()
	s := newREState([]string{"no-specs-domain"})
	s.State = StateSurvey
	// dir has no no-specs-domain/specs/ subdirectory

	out := outputOf(s, dir)
	if !strings.Contains(out, "does not exist") {
		t.Errorf("expected missing specs dir note in SURVEY output, got:\n%s", out)
	}
}

func TestOutputREGapAnalysisShowsDomainAndSubagentConfig(t *testing.T) {
	s := newREState([]string{"api"})
	s.State = StateGapAnalysis
	s.Config.ReverseEngineering.GapAnalysis = SubAgentConfig{Model: "sonnet", Type: "explore", Count: 5}

	out := outputOf(s, t.TempDir())
	if !strings.Contains(out, "GAP_ANALYSIS") {
		t.Errorf("expected state in GAP_ANALYSIS output, got:\n%s", out)
	}
	if !strings.Contains(out, "5 sonnet explore") {
		t.Errorf("expected sub-agent config in GAP_ANALYSIS output, got:\n%s", out)
	}
	if !strings.Contains(out, "Next: DECOMPOSE for domain api") {
		t.Errorf("expected next step in GAP_ANALYSIS output, got:\n%s", out)
	}
}

func TestOutputREDecomposeShowsDomainAndAction(t *testing.T) {
	s := newREState([]string{"payments"})
	s.State = StateDecompose

	out := outputOf(s, t.TempDir())
	if !strings.Contains(out, "DECOMPOSE") {
		t.Errorf("expected state in DECOMPOSE output, got:\n%s", out)
	}
	if !strings.Contains(out, "Synthesize findings from domain payments") {
		t.Errorf("expected action in DECOMPOSE output, got:\n%s", out)
	}
	if !strings.Contains(out, "payments") {
		t.Errorf("expected domain name in DECOMPOSE output, got:\n%s", out)
	}
}

func TestOutputREQueueFirstTimeShowsFileFlag(t *testing.T) {
	s := newREState([]string{"api"})
	s.State = StateQueue
	// QueueFile is empty — first time

	out := outputOf(s, t.TempDir())
	if !strings.Contains(out, "--file <queue.json>") {
		t.Errorf("expected --file flag instruction in first QUEUE output, got:\n%s", out)
	}
}

func TestOutputREQueueSubsequentShowsStoredPath(t *testing.T) {
	s := newREState([]string{"api", "billing"})
	s.State = StateQueue
	s.ReverseEngineering.CurrentDomain = 1
	s.ReverseEngineering.QueueFile = "/tmp/re-queue.json"

	out := outputOf(s, t.TempDir())
	if !strings.Contains(out, "/tmp/re-queue.json") {
		t.Errorf("expected stored queue path in subsequent QUEUE output, got:\n%s", out)
	}
	if strings.Contains(out, "--file <queue.json>") {
		t.Errorf("unexpected --file flag in subsequent QUEUE output (file already set), got:\n%s", out)
	}
}

func TestOutputREReconcileShowsSpecsAndDependsOn(t *testing.T) {
	dir := t.TempDir()

	// Write a queue file with entries.
	queueFile := filepath.Join(dir, "re-queue.json")
	qi := ReverseEngineeringQueueInput{
		Specs: []ReverseEngineeringQueueEntry{
			{Name: "Auth Init", Domain: "api", Topic: "auth init", File: "specs/auth-init.md", Action: "create", DependsOn: []string{}},
			{Name: "Auth Tokens", Domain: "api", Topic: "auth tokens", File: "specs/auth-tokens.md", Action: "create", DependsOn: []string{"Auth Init"}},
		},
	}
	data, _ := json.Marshal(qi)
	os.WriteFile(queueFile, data, 0644)

	s := newREState([]string{"api"})
	s.State = StateReconcile
	s.ReverseEngineering.ReconcileDomain = 0
	s.ReverseEngineering.QueueFile = queueFile

	out := outputOf(s, dir)
	if !strings.Contains(out, "RECONCILE") {
		t.Errorf("expected state in RECONCILE output, got:\n%s", out)
	}
	if !strings.Contains(out, "api/specs/auth-init.md") {
		t.Errorf("expected spec file in RECONCILE output, got:\n%s", out)
	}
	if !strings.Contains(out, "Auth Init") {
		t.Errorf("expected depends_on entry in RECONCILE output, got:\n%s", out)
	}
}

func TestOutputREReconcileEvalShowsEvalInstructions(t *testing.T) {
	s := newREState([]string{"api"})
	s.State = StateReconcileEval
	s.ReverseEngineering.ReconcileDomain = 0
	s.ReverseEngineering.Round = 1
	s.Config.ReverseEngineering.Reconcile.MaxRounds = 3
	s.Config.ReverseEngineering.Reconcile.Eval = AgentConfig{Model: "opus", Type: "general-purpose", Count: 1}

	out := outputOf(s, t.TempDir())
	if !strings.Contains(out, "RECONCILE_EVAL") {
		t.Errorf("expected state in RECONCILE_EVAL output, got:\n%s", out)
	}
	if !strings.Contains(out, "forgectl eval") {
		t.Errorf("expected forgectl eval instruction in RECONCILE_EVAL output, got:\n%s", out)
	}
	if !strings.Contains(out, "reconciliation-r1.md") {
		t.Errorf("expected eval report path in RECONCILE_EVAL output, got:\n%s", out)
	}
}

func TestOutputREExecuteShowsModeAndQueueCount(t *testing.T) {
	dir := t.TempDir()

	// Write a queue file.
	queueFile := filepath.Join(dir, "re-queue.json")
	qi := ReverseEngineeringQueueInput{
		Specs: []ReverseEngineeringQueueEntry{
			{Name: "Spec A", Domain: "api", File: "specs/a.md", Action: "create", CodeSearchRoots: []string{"src/"}},
			{Name: "Spec B", Domain: "api", File: "specs/b.md", Action: "create", CodeSearchRoots: []string{"src/"}},
			{Name: "Spec C", Domain: "api", File: "specs/c.md", Action: "update", CodeSearchRoots: []string{"src/"}},
		},
	}
	data, _ := json.Marshal(qi)
	os.WriteFile(queueFile, data, 0644)

	s := newREState([]string{"api"})
	s.State = StateExecute
	s.ReverseEngineering.QueueFile = queueFile
	s.Config.ReverseEngineering.Mode = "self_refine"

	out := outputOf(s, dir)
	if !strings.Contains(out, "EXECUTE") {
		t.Errorf("expected state in EXECUTE output, got:\n%s", out)
	}
	if !strings.Contains(out, "3 entries") {
		t.Errorf("expected queue entry count in EXECUTE output, got:\n%s", out)
	}
	if !strings.Contains(out, "self_refine") {
		t.Errorf("expected mode in EXECUTE output, got:\n%s", out)
	}
}

func TestOutputREReconcileSubsequentRoundShowsFailureContext(t *testing.T) {
	dir := t.TempDir()

	queueFile := filepath.Join(dir, "re-queue.json")
	qi := ReverseEngineeringQueueInput{
		Specs: []ReverseEngineeringQueueEntry{
			{Name: "Spec A", Domain: "api", File: "specs/a.md", Action: "create", DependsOn: []string{}},
		},
	}
	data, _ := json.Marshal(qi)
	os.WriteFile(queueFile, data, 0644)

	s := newREState([]string{"api"})
	s.State = StateReconcile
	s.ReverseEngineering.ReconcileDomain = 0
	s.ReverseEngineering.Round = 2 // subsequent round
	s.ReverseEngineering.QueueFile = queueFile

	out := outputOf(s, dir)
	if !strings.Contains(out, "evaluation failed on the previous round") {
		t.Errorf("expected failure context in subsequent RECONCILE output, got:\n%s", out)
	}
	if !strings.Contains(out, "api/specs/a.md") {
		t.Errorf("expected spec file in subsequent RECONCILE output, got:\n%s", out)
	}
}

func TestOutputREExecuteFailureRendersStopTemplate(t *testing.T) {
	var buf bytes.Buffer
	stderrText := "Traceback (most recent call last):\n  File \"reverse_engineer.py\"\nKeyError: 'model'"
	PrintExecuteFailureOutput(&buf, stderrText)
	out := buf.String()

	if !strings.Contains(out, "STOP") {
		t.Errorf("expected STOP in failure output, got:\n%s", out)
	}
	if !strings.Contains(out, "Python subprocess") {
		t.Errorf("expected Python subprocess mention in failure output, got:\n%s", out)
	}
	if !strings.Contains(out, stderrText) {
		t.Errorf("expected full stderr in failure output, got:\n%s", out)
	}
}

func TestOutputREDoneShowsSummary(t *testing.T) {
	s := newREState([]string{"api", "billing"})
	s.State = StateDone

	out := outputOf(s, t.TempDir())
	if !strings.Contains(out, "DONE") {
		t.Errorf("expected state in DONE output, got:\n%s", out)
	}
	if !strings.Contains(out, "Reverse engineering workflow complete") {
		t.Errorf("expected completion message in DONE output, got:\n%s", out)
	}
	if !strings.Contains(out, "understand the auth system") {
		t.Errorf("expected concept in DONE output, got:\n%s", out)
	}
}

func TestOutputDoneDomainVariantWhenPlansRemain(t *testing.T) {
	dir := t.TempDir()
	s := newImplementingState(dir, 1, 1)
	s.Config.General.EnableCommits = false
	// Add a plan queue entry to simulate remaining domains.
	s.Implementing.PlanQueue = []PlanQueueEntry{{Name: "Next Plan", Domain: "next", File: "next/plan.json"}}
	s.Implementing.CurrentPlanDomain = "test"
	s.State = StateDone

	out := outputOf(s, dir)
	if !strings.Contains(out, "Domain complete.") {
		t.Errorf("expected 'Domain complete.' in DONE output with plans remaining, got:\n%s", out)
	}
	if !strings.Contains(out, "Advance to continue to next domain.") {
		t.Errorf("expected 'Advance to continue to next domain.' in DONE output, got:\n%s", out)
	}
}
