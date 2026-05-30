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

// reOutputState builds a reverse_engineering ForgeState with default config in
// the given state, domain index, and queue.
func reOutputState(st StateName, domains []string, domainIndex int, queue []REQueueEntry) *ForgeState {
	re := NewReverseEngineeringState("auth refactor", domains)
	re.DomainIndex = domainIndex
	re.Queue = queue
	return &ForgeState{
		Phase:              PhaseReverseEngineering,
		State:              st,
		Config:             DefaultForgeConfig(),
		StartedAtPhase:     PhaseReverseEngineering,
		ReverseEngineering: re,
	}
}

// Functional: ORIENT lists every domain with its 1-based index and the domain
// processing order.
func TestREOutputOrientListsDomainsAndOrder(t *testing.T) {
	dir := t.TempDir()
	s := reOutputState(StateOrient, []string{"optimizer", "api", "portal"}, 1, nil)

	out := outputOf(s, dir)
	if !strings.Contains(out, "Phase: reverse_engineering") {
		t.Errorf("expected RE phase header, got:\n%s", out)
	}
	if !strings.Contains(out, "optimizer (1/3)") || !strings.Contains(out, "portal (3/3)") {
		t.Errorf("expected indexed domain list, got:\n%s", out)
	}
	if !strings.Contains(out, "optimizer → api → portal") {
		t.Errorf("expected domain order arrow line, got:\n%s", out)
	}
	if !strings.Contains(out, "Advance to begin SURVEY on domain: optimizer") {
		t.Errorf("expected SURVEY advance hint, got:\n%s", out)
	}
}

// Functional: SURVEY and GAP_ANALYSIS interpolate the configured sub-agent
// counts/models/types, and GAP_ANALYSIS includes topic-of-concern rules.
func TestREOutputSurveyAndGapAnalysisConfig(t *testing.T) {
	dir := t.TempDir()

	survey := outputOf(reOutputState(StateSurvey, []string{"optimizer"}, 1, nil), dir)
	if !strings.Contains(survey, "Spawn 2 haiku explorer sub-agents") {
		t.Errorf("SURVEY should reflect configured sub-agents, got:\n%s", survey)
	}
	if !strings.Contains(survey, "optimizer/specs/") {
		t.Errorf("SURVEY should scope to the domain specs dir, got:\n%s", survey)
	}

	gap := outputOf(reOutputState(StateGapAnalysis, []string{"optimizer"}, 1, nil), dir)
	if !strings.Contains(gap, "Spawn 5 sonnet explorer sub-agents") {
		t.Errorf("GAP_ANALYSIS should reflect configured sub-agents, got:\n%s", gap)
	}
	if !strings.Contains(gap, "Must not contain \"and\" conjoining unrelated capabilities") {
		t.Errorf("GAP_ANALYSIS should include topic-of-concern rules, got:\n%s", gap)
	}
}

// Functional: EXECUTE_REVERSE_ENGINEER renders the current item's index, domain,
// spec, action, target file, topic, code_search_roots (domain-relative, with the
// root-of-core-code definition), and the configured execute sub-agents.
func TestREOutputExecuteItem(t *testing.T) {
	dir := t.TempDir()
	queue := []REQueueEntry{
		{Name: "Repository Loading", Domain: "optimizer", Topic: "loads a repo", File: "specs/repo.md", Action: "create", CodeSearchRoots: []string{"src/repo/", "src/config/"}, DependsOn: []string{}},
	}
	s := reOutputState(StateExecuteReverseEngineer, []string{"optimizer"}, 1, queue)
	s.ReverseEngineering.ExecuteItemIndex = 1

	out := outputOf(s, dir)
	for _, want := range []string{
		"EXECUTE_REVERSE_ENGINEER",
		"Item: 1/1",
		"Spec: Repository Loading  (create)",
		"Target file: optimizer/specs/repo.md",
		"Topic of concern: \"loads a repo\"",
		"- optimizer/src/repo/",
		"- optimizer/src/config/",
		"root of the core",
		"Spawn 3 haiku explorer sub-agents",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("EXECUTE output missing %q, got:\n%s", want, out)
		}
	}
}

// Functional: RECONCILE lists the current domain's specs with depends_on, and
// RECONCILE_EVAL instructs running forgectl eval with the configured sub-agents.
func TestREOutputReconcileAndEval(t *testing.T) {
	dir := t.TempDir()
	queue := []REQueueEntry{
		{Name: "A", Domain: "optimizer", File: "specs/a.md", Action: "create", DependsOn: []string{"B"}},
		{Name: "B", Domain: "optimizer", File: "specs/b.md", Action: "update", DependsOn: []string{}},
		{Name: "C", Domain: "api", File: "specs/c.md", Action: "create", DependsOn: []string{}},
	}
	rec := reOutputState(StateReconcile, []string{"optimizer", "api"}, 1, queue)
	rec.ReverseEngineering.ReconcileRound = 1
	out := outputOf(rec, dir)
	if !strings.Contains(out, "optimizer/specs/a.md  (create)") || !strings.Contains(out, "depends_on: [B]") {
		t.Errorf("RECONCILE should list domain specs with depends_on, got:\n%s", out)
	}
	if strings.Contains(out, "api/specs/c.md") {
		t.Errorf("RECONCILE should not list other-domain specs, got:\n%s", out)
	}

	ev := reOutputState(StateReconcileEval, []string{"optimizer", "api"}, 1, queue)
	ev.ReverseEngineering.ReconcileRound = 2
	evOut := outputOf(ev, dir)
	if !strings.Contains(evOut, "Round: 2/3") {
		t.Errorf("RECONCILE_EVAL should show round/max, got:\n%s", evOut)
	}
	if !strings.Contains(evOut, "forgectl eval") {
		t.Errorf("RECONCILE_EVAL should instruct running forgectl eval, got:\n%s", evOut)
	}
	if !strings.Contains(evOut, "Spawn 1 opus general-purpose sub-agents") {
		t.Errorf("RECONCILE_EVAL should reflect configured eval sub-agents, got:\n%s", evOut)
	}
	if !strings.Contains(evOut, filepath.Join("optimizer", "specs", ".eval", "reconciliation-r2.md")) {
		t.Errorf("RECONCILE_EVAL should show the report path, got:\n%s", evOut)
	}
}

// Functional: POST_REVERSE_ENGINEER emits the STOP / clear-context message and
// points to the next item (mid-loop) or to RECONCILE (last item).
func TestREOutputPostReverseEngineer(t *testing.T) {
	dir := t.TempDir()
	queue := []REQueueEntry{
		{Name: "One", Domain: "optimizer", File: "specs/one.md", Action: "create"},
		{Name: "Two", Domain: "optimizer", File: "specs/two.md", Action: "create"},
	}

	// Mid-loop: item 1 of 2 → points to the next item.
	mid := reOutputState(StatePostReverseEngineer, []string{"optimizer"}, 1, queue)
	mid.ReverseEngineering.ExecuteItemIndex = 1
	midOut := outputOf(mid, dir)
	if !strings.Contains(midOut, "STOP ensure you have created the specification that you need,") {
		t.Errorf("POST should emit the STOP message, got:\n%s", midOut)
	}
	if !strings.Contains(midOut, "clear your context window for the next iteration.") {
		t.Errorf("POST should emit the clear-context message, got:\n%s", midOut)
	}
	if !strings.Contains(midOut, "continue with item 2/2") {
		t.Errorf("POST mid-loop should point to the next item, got:\n%s", midOut)
	}

	// Last item: item 2 of 2 → points to RECONCILE.
	last := reOutputState(StatePostReverseEngineer, []string{"optimizer"}, 1, queue)
	last.ReverseEngineering.ExecuteItemIndex = 2
	lastOut := outputOf(last, dir)
	if !strings.Contains(lastOut, "proceed to RECONCILE") {
		t.Errorf("POST on last item should point to RECONCILE, got:\n%s", lastOut)
	}
}

// Edge case: SURVEY notes when the domain has no specs/ directory, and omits the
// note once the directory exists.
func TestREOutputSurveyNotesMissingSpecsDir(t *testing.T) {
	dir := t.TempDir()
	s := reOutputState(StateSurvey, []string{"optimizer"}, 1, nil)

	// No optimizer/specs/ yet — the absence must be noted.
	missing := outputOf(s, dir)
	if !strings.Contains(missing, "optimizer/specs/ does not exist") {
		t.Errorf("SURVEY should note the missing specs dir, got:\n%s", missing)
	}

	// Create the directory — the note must disappear.
	if err := os.MkdirAll(filepath.Join(dir, "optimizer", "specs"), 0755); err != nil {
		t.Fatal(err)
	}
	present := outputOf(s, dir)
	if strings.Contains(present, "does not exist") {
		t.Errorf("SURVEY should not note absence once specs/ exists, got:\n%s", present)
	}
}

// Edge case: QUEUE shows the "write" variant on the first advance (no stored
// hash) and the "add entries" variant on subsequent advances (hash present);
// RECONCILE_ADVANCE shows the next-domain vs DONE variant by domain position.
func TestREOutputQueueAndReconcileAdvanceVariants(t *testing.T) {
	dir := t.TempDir()

	// First QUEUE advance — no stored hash.
	first := reOutputState(StateQueue, []string{"optimizer", "api"}, 1, nil)
	firstOut := outputOf(first, dir)
	if !strings.Contains(firstOut, "Write the reverse engineering queue file") {
		t.Errorf("first QUEUE should show the write variant, got:\n%s", firstOut)
	}

	// Subsequent QUEUE advance — hash already recorded.
	sub := reOutputState(StateQueue, []string{"optimizer", "api"}, 2, nil)
	sub.ReverseEngineering.QueueContentHash = "deadbeef"
	subOut := outputOf(sub, dir)
	if !strings.Contains(subOut, "Add entries for domain api to the existing queue file.") {
		t.Errorf("subsequent QUEUE should show the add-entries variant, got:\n%s", subOut)
	}

	// RECONCILE_ADVANCE with a domain remaining → next domain.
	next := reOutputState(StateReconcileAdvance, []string{"optimizer", "api"}, 1, nil)
	nextOut := outputOf(next, dir)
	if !strings.Contains(nextOut, "Next: RECONCILE for domain api (2/2)") {
		t.Errorf("RECONCILE_ADVANCE should point to the next domain, got:\n%s", nextOut)
	}

	// RECONCILE_ADVANCE on the last domain → DONE.
	last := reOutputState(StateReconcileAdvance, []string{"optimizer", "api"}, 2, nil)
	lastOut := outputOf(last, dir)
	if !strings.Contains(lastOut, "All domains reconciled. Advancing to DONE.") {
		t.Errorf("RECONCILE_ADVANCE on last domain should advance to DONE, got:\n%s", lastOut)
	}
}
