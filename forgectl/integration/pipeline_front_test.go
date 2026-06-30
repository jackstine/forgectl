//go:build integration

package integration

import (
	"encoding/json"
	"strings"
	"testing"

	"forgectl/state"
)

// This file covers plan §3.B items B2 and B3 — the front of the pipeline:
// specifying → generate_planning_queue → planning.
//
//   B2 (TestGeneratePlanningQueueAutoGenerates): advancing from the specifying
//   PHASE_SHIFT without --from auto-generates .forgectl/state/plan-queue.json
//   from the completed specs (grouped by domain, one entry per domain), enters
//   generate_planning_queue, and ultimately hands off to planning.
//
//   B3 (TestSpecifyingFromOverride): advancing from the same PHASE_SHIFT with
//   --from <file> skips generate_planning_queue entirely and lands at planning
//   ORIENT directly using the provided queue file.

// specifyingCommitsOffConfig is a minimal specifying config with commits
// disabled (no --message required at COMPLETE, which simplifies the helper).
const specifyingCommitsOffConfig = `
[specifying]
batch = 1
[specifying.eval]
min_rounds = 1
max_rounds = 3
[general]
user_guided = false
enable_commits = false
`

// driveSpecifyingToPhaseShift drives the specifying phase from ORIENT to the
// terminal PHASE_SHIFT using all-PASS verdicts, with commits disabled (so
// COMPLETE needs no --message). Assumes a single-domain, single-batch run
// where the spec file lands at "specs/a.md" (matching oneSpecQueue). The
// caller must have run `init --phase specifying --from <queue>` before calling.
func driveSpecifyingToPhaseShift(p *Project) {
	p.t.Helper()
	p.RunScript([]Step{
		advStep("orient→select", state.PhaseSpecifying, state.StateSelect),
		advStep("select→draft", state.PhaseSpecifying, state.StateDraft),
		{
			Label: "draft→evaluate",
			Setup: func(p *Project, _ Result) { p.WriteFile("specs/a.md", "# stub spec\n") },
			Args:  func(*Project, Result) []string { return []string{"advance"} },
			WantPhase: state.PhaseSpecifying,
			WantState: state.StateEvaluate,
		},
		evalPassStep("evaluate→accept", "PASS", state.PhaseSpecifying, state.StateAccept),
		advStep("accept→xref", state.PhaseSpecifying, state.StateCrossReference),
		advStep("xref→xref_eval", state.PhaseSpecifying, state.StateCrossReferenceEval),
		evalPassStep("xref_eval→xref_review", "PASS", state.PhaseSpecifying, state.StateCrossReferenceReview),
		advStep("xref_review→done", state.PhaseSpecifying, state.StateDone),
		advStep("done→reconcile", state.PhaseSpecifying, state.StateReconcile),
		advStep("reconcile→recon_eval", state.PhaseSpecifying, state.StateReconcileEval),
		evalPassStep("recon_eval→recon_review", "PASS", state.PhaseSpecifying, state.StateReconcileReview),
		advStep("recon_review→complete", state.PhaseSpecifying, state.StateComplete),
		advStep("complete→phase_shift", state.PhaseSpecifying, state.StatePhaseShift),
	})
}

// overridePlanQueue is a minimal valid hand-authored plan-queue used by the B3
// override test. The name "Core Override Plan" deliberately differs from what
// autoGeneratePlanQueue would produce ("Core Implementation Plan") so that
// the test can confirm the override file — not the auto-generator — was used.
const overridePlanQueue = `{"plans":[{
  "name": "Core Override Plan",
  "domain": "core",
  "file": "core/plan.json",
  "specs": [],
  "spec_commits": [],
  "code_search_roots": ["core/"]
}]}`

// TestGeneratePlanningQueueAutoGenerates (B2) verifies that advancing from the
// specifying PHASE_SHIFT without --from triggers auto-generation of
// .forgectl/state/plan-queue.json (one entry per domain from completed specs,
// with default code_search_roots) and enters the generate_planning_queue phase.
// Driving through that phase (ORIENT → REFINE validates the file → PHASE_SHIFT
// → planning ORIENT) completes the front-of-pipeline handoff.
func TestGeneratePlanningQueueAutoGenerates(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(specifyingCommitsOffConfig)
	p.WriteFile("spec-queue.json", oneSpecQueue)

	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	driveSpecifyingToPhaseShift(p)
	p.AssertAt(state.PhaseSpecifying, state.StatePhaseShift)

	// Advance from specifying PHASE_SHIFT without --from: auto-generates
	// plan-queue.json from completed specs and enters generate_planning_queue.
	p.mustForge("advance")
	p.AssertAt(state.PhaseGeneratePlanningQueue, state.StateOrient)

	// B2: assert the generated file exists at the documented state-dir path.
	const queuePath = ".forgectl/state/plan-queue.json"
	if !p.Exists(queuePath) {
		t.Fatalf("plan-queue.json not generated at %s", queuePath)
	}

	// Parse the generated file with a local struct (not forgectl's types) so
	// the assertion is independent of how forgectl serializes PlanQueueInput.
	var pq struct {
		Plans []struct {
			Name            string   `json:"name"`
			Domain          string   `json:"domain"`
			File            string   `json:"file"`
			Specs           []string `json:"specs"`
			CodeSearchRoots []string `json:"code_search_roots"`
		} `json:"plans"`
	}
	if err := json.Unmarshal([]byte(p.ReadFile(queuePath)), &pq); err != nil {
		t.Fatalf("parsing generated plan-queue.json: %v", err)
	}

	// oneSpecQueue has one spec in the "core" domain → one plan entry.
	if len(pq.Plans) != 1 {
		t.Fatalf("generated plan-queue has %d plan(s), want 1 (one per domain)", len(pq.Plans))
	}
	entry := pq.Plans[0]

	if entry.Domain != "core" {
		t.Errorf("plan domain = %q, want \"core\"", entry.Domain)
	}
	// autoGeneratePlanQueue builds the plan file path as
	// <domain>/.forge_workspace/implementation_plan/plan.json.
	const wantFile = "core/.forge_workspace/implementation_plan/plan.json"
	if entry.File != wantFile {
		t.Errorf("plan file = %q, want %q", entry.File, wantFile)
	}
	// The completed spec's file path (from oneSpecQueue) must appear in specs.
	const specFile = "specs/a.md"
	found := false
	for _, s := range entry.Specs {
		if s == specFile {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("plan specs = %v, want to include %q", entry.Specs, specFile)
	}
	// Default code_search_roots is ["<domain>/"] when no set-roots metadata exists.
	if len(entry.CodeSearchRoots) != 1 || entry.CodeSearchRoots[0] != "core/" {
		t.Errorf("code_search_roots = %v, want [\"core/\"]", entry.CodeSearchRoots)
	}

	// Drive generate_planning_queue to its PHASE_SHIFT and then into planning.
	// ORIENT → REFINE (validates the generated file against the plan-queue schema).
	p.mustForge("advance")
	p.AssertAt(state.PhaseGeneratePlanningQueue, state.StateRefine)

	// REFINE → PHASE_SHIFT (file validated, marks handoff to planning).
	p.mustForge("advance")
	p.AssertAt(state.PhaseGeneratePlanningQueue, state.StatePhaseShift)

	// PHASE_SHIFT → planning/ORIENT (loads the generated queue into planning state).
	p.mustForge("advance")
	p.AssertAt(state.PhasePlanning, state.StateOrient)

	// Confirm the planning state was populated from the generated queue.
	s := p.State()
	if s.Planning == nil || s.Planning.CurrentPlan == nil {
		t.Fatal("planning CurrentPlan is nil after generate_planning_queue handoff")
	}
	if s.Planning.CurrentPlan.Domain != "core" {
		t.Errorf("CurrentPlan.Domain = %q, want \"core\"", s.Planning.CurrentPlan.Domain)
	}
	if !strings.Contains(s.Planning.CurrentPlan.Name, "Core") {
		t.Errorf("CurrentPlan.Name = %q: want it to contain \"Core\"", s.Planning.CurrentPlan.Name)
	}
}

// TestSpecifyingFromOverride (B3) verifies that advancing from the specifying
// PHASE_SHIFT with --from skips generate_planning_queue entirely, loading the
// provided plan-queue directly and landing at planning ORIENT in one step.
func TestSpecifyingFromOverride(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(specifyingCommitsOffConfig)
	p.WriteFile("spec-queue.json", oneSpecQueue)

	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	driveSpecifyingToPhaseShift(p)
	p.AssertAt(state.PhaseSpecifying, state.StatePhaseShift)

	// Write a hand-crafted queue whose name differs from the auto-generator's
	// output so the test can confirm the override — not the auto-generator — was used.
	p.WriteFile("override-queue.json", overridePlanQueue)

	// B3: advance with --from → skip generate_planning_queue, go to planning ORIENT.
	p.mustForge("advance", "--from", "override-queue.json")
	p.AssertAt(state.PhasePlanning, state.StateOrient)

	// Confirm the override queue was used (not the auto-generated one).
	s := p.State()
	if s.Planning == nil || s.Planning.CurrentPlan == nil {
		t.Fatal("planning CurrentPlan is nil after --from override")
	}
	if s.Planning.CurrentPlan.Domain != "core" {
		t.Errorf("CurrentPlan.Domain = %q, want \"core\"", s.Planning.CurrentPlan.Domain)
	}
	// "Core Override Plan" (from overridePlanQueue) vs "Core Implementation Plan"
	// (what autoGeneratePlanQueue would produce) — the distinguishing marker.
	if !strings.Contains(s.Planning.CurrentPlan.Name, "Override") {
		t.Errorf("CurrentPlan.Name = %q: want to contain \"Override\" (the hand-crafted override queue, not the auto-generator)", s.Planning.CurrentPlan.Name)
	}
}
