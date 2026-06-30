//go:build integration

package integration

import (
	"strings"
	"testing"

	"forgectl/state"
)

// planningConfigCommitsOff is the standard single-plan config used by the
// planning lifecycle tests that don't exercise the git side-channel.
const planningConfigCommitsOff = `
[general]
user_guided = false
enable_commits = false
[planning.eval]
min_rounds = 1
max_rounds = 3
`

// planningConfigCommitsOn is the config variant for tests that verify the
// planning ACCEPT commit and the --message requirement.
const planningConfigCommitsOn = `
[general]
user_guided = false
enable_commits = true
[planning.eval]
min_rounds = 1
max_rounds = 3
`

// onePlanQueue is a single-entry plan queue pointing at core/plan.json with
// kind:"code", so PHASE_SHIFT routes to implementing.
const onePlanQueue = `{"plans":[{"name":"Core Plan","domain":"core","file":"core/plan.json","specs":[],"spec_commits":[],"code_search_roots":["core/"],"kind":"code"}]}`

// minimalPlanJSON is the smallest valid plan.json accepted by ValidatePlanJSON.
const minimalPlanJSON = `{
  "context": {"domain":"core","module":"Core"},
  "layers": [{"id":"L0","name":"Foundation","items":["a"]}],
  "items": [
    {"id":"a","name":"Core A","description":"first item","depends_on":[],"tests":[{"category":"functional","description":"works","passes":false}]}
  ]
}`

// drivePlanningPrologue advances from ORIENT through DRAFT (five advances) and
// then one more advance from DRAFT to EVALUATE, returning the EVALUATE result so
// the caller can call PassEval. It assumes plan.json already exists on disk.
func drivePlanningPrologue(p *Project) Result {
	p.t.Helper()
	p.mustForge("advance") // ORIENT → STUDY_SPECS
	p.mustForge("advance") // STUDY_SPECS → STUDY_CODE
	p.mustForge("advance") // STUDY_CODE → STUDY_PACKAGES
	p.mustForge("advance") // STUDY_PACKAGES → REVIEW
	p.mustForge("advance") // REVIEW → DRAFT
	return p.mustForge("advance") // DRAFT → EVALUATE
}

// TestA2PlanningFullWalkCommitsOff drives the complete planning lifecycle for a
// single "code" kind plan from init through PHASE_SHIFT with commits off.
// Asserts each step's persisted phase/state, that the plan lands in
// Planning.Completed, that PhaseShift.To is "implementing", and that the
// PHASE_SHIFT output contains "planning → implementing".
func TestA2PlanningFullWalkCommitsOff(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(planningConfigCommitsOff)
	p.WriteFile("plan-queue.json", onePlanQueue)
	p.WriteFile("core/plan.json", minimalPlanJSON)

	// init → ORIENT
	p.mustForge("init", "--phase", "planning", "--from", "plan-queue.json")
	p.AssertAt(state.PhasePlanning, state.StateOrient)

	// Prologue: ORIENT → STUDY_SPECS → STUDY_CODE → STUDY_PACKAGES → REVIEW →
	// DRAFT → EVALUATE. plan.json is already present so DRAFT advances cleanly.
	p.mustForge("advance")
	p.AssertAt(state.PhasePlanning, state.StateStudySpecs)
	p.mustForge("advance")
	p.AssertAt(state.PhasePlanning, state.StateStudyCode)
	p.mustForge("advance")
	p.AssertAt(state.PhasePlanning, state.StateStudyPackages)
	p.mustForge("advance")
	p.AssertAt(state.PhasePlanning, state.StateReview)
	p.mustForge("advance")
	p.AssertAt(state.PhasePlanning, state.StateDraft)

	// DRAFT → EVALUATE (plan.json present and valid).
	evalRes := p.mustForge("advance")
	p.AssertAt(state.PhasePlanning, state.StateEvaluate)

	// EVALUATE PASS (round 1 >= min_rounds 1) → ACCEPT.
	p.PassEval(evalRes, "PASS")
	p.AssertAt(state.PhasePlanning, state.StateAccept)

	// ACCEPT (commits off, no --message needed) → PHASE_SHIFT.
	shiftRes := p.mustForge("advance")
	p.AssertAt(state.PhasePlanning, state.StatePhaseShift)

	// PHASE_SHIFT output must announce the routing from planning to implementing.
	if !strings.Contains(shiftRes.Out(), "planning → implementing") {
		t.Errorf("PHASE_SHIFT output missing 'planning → implementing':\n%s", shiftRes.Out())
	}

	// Plan must appear in Planning.Completed with the correct name and domain.
	s := p.State()
	if len(s.Planning.Completed) != 1 {
		t.Fatalf("Planning.Completed len = %d, want 1", len(s.Planning.Completed))
	}
	done := s.Planning.Completed[0]
	if done.Name != "Core Plan" {
		t.Errorf("completed plan Name = %q, want %q", done.Name, "Core Plan")
	}
	if done.Domain != "core" {
		t.Errorf("completed plan Domain = %q, want %q", done.Domain, "core")
	}

	// PhaseShift info must record the correct destination.
	if s.PhaseShift == nil {
		t.Fatal("PhaseShift is nil after PHASE_SHIFT")
	}
	if s.PhaseShift.To != state.PhaseImplementing {
		t.Errorf("PhaseShift.To = %q, want %q", s.PhaseShift.To, state.PhaseImplementing)
	}
}

// TestA2PlanningAcceptRequiresMessage asserts that advancing at ACCEPT when
// enable_commits is true without providing --message exits non-zero and leaves
// the persisted state unchanged at ACCEPT.
func TestA2PlanningAcceptRequiresMessage(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(planningConfigCommitsOn)
	p.WriteFile("plan-queue.json", onePlanQueue)
	p.WriteFile("core/plan.json", minimalPlanJSON)

	p.mustForge("init", "--phase", "planning", "--from", "plan-queue.json")

	// Drive through prologue then pass EVALUATE to reach ACCEPT.
	evalRes := drivePlanningPrologue(p)
	p.AssertAt(state.PhasePlanning, state.StateEvaluate)
	p.PassEval(evalRes, "PASS")
	p.AssertAt(state.PhasePlanning, state.StateAccept)

	// Advance at ACCEPT without --message must be rejected.
	fail := p.forge("advance")
	if fail.Exit == 0 {
		t.Errorf("advance at ACCEPT without --message succeeded, want non-zero exit")
	}
	if !strings.Contains(fail.Out(), "message is required") {
		t.Errorf("expected 'message is required' in error output, got:\n%s", fail.Out())
	}

	// State must remain at ACCEPT — the failed advance must not mutate it.
	p.AssertAt(state.PhasePlanning, state.StateAccept)
}

// TestA2PlanningCommitAtAccept verifies that advancing at ACCEPT with
// enable_commits:true and --message "plan: core" produces a PHASE_SHIFT and
// exactly one git commit whose subject matches the message.
func TestA2PlanningCommitAtAccept(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(planningConfigCommitsOn)
	p.WriteFile("plan-queue.json", onePlanQueue)
	p.WriteFile("core/plan.json", minimalPlanJSON)
	// The default planning commit strategy is "strict", which stages plan.json
	// and the adjacent notes/ directory. notes/ must be non-empty for git to
	// include it; a stub file is sufficient.
	p.WriteFile("core/notes/study.md", "# notes\n")

	p.mustForge("init", "--phase", "planning", "--from", "plan-queue.json")

	// Drive through prologue and EVALUATE to reach ACCEPT.
	evalRes := drivePlanningPrologue(p)
	p.AssertAt(state.PhasePlanning, state.StateEvaluate)
	p.PassEval(evalRes, "PASS")
	p.AssertAt(state.PhasePlanning, state.StateAccept)

	// ACCEPT with --message → commit + PHASE_SHIFT.
	p.mustForge("advance", "--message", "plan: core")
	p.AssertAt(state.PhasePlanning, state.StatePhaseShift)

	// Exactly one commit, carrying our message.
	log := p.GitLog()
	if len(log) != 1 {
		t.Fatalf("git log has %d commits, want 1:\n%v", len(log), log)
	}
	if log[0] != "plan: core" {
		t.Errorf("commit subject = %q, want %q", log[0], "plan: core")
	}
}

// TestA2PlanningValidateStateOnMissingFile drives to DRAFT without writing
// plan.json. The advance from DRAFT must exit non-zero and persist state at
// VALIDATE. Once plan.json is written, advancing again from VALIDATE must
// succeed and land at EVALUATE.
func TestA2PlanningValidateStateOnMissingFile(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(planningConfigCommitsOff)
	p.WriteFile("plan-queue.json", onePlanQueue)
	// Deliberately omit core/plan.json.

	p.mustForge("init", "--phase", "planning", "--from", "plan-queue.json")

	// Drive through the prologue up to (but not past) DRAFT.
	p.mustForge("advance") // → STUDY_SPECS
	p.mustForge("advance") // → STUDY_CODE
	p.mustForge("advance") // → STUDY_PACKAGES
	p.mustForge("advance") // → REVIEW
	p.mustForge("advance") // → DRAFT
	p.AssertAt(state.PhasePlanning, state.StateDraft)

	// Advance from DRAFT without plan.json → should fail. The binary returns a
	// regular (non-ValidationError) error for a missing file, so the cmd layer
	// does not persist the in-memory state mutation; state stays at DRAFT on disk.
	fail := p.forge("advance")
	if fail.Exit == 0 {
		t.Errorf("advance from DRAFT without plan.json succeeded, want non-zero exit")
	}
	p.AssertAt(state.PhasePlanning, state.StateDraft)

	// Now supply the plan file and retry from DRAFT → EVALUATE.
	p.WriteFile("core/plan.json", minimalPlanJSON)

	p.mustForge("advance")
	p.AssertAt(state.PhasePlanning, state.StateEvaluate)
}
