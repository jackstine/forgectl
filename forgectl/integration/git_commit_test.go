//go:build integration

package integration

import (
	"strings"
	"testing"

	"forgectl/state"
)

// driveSpecifyingToComplete drives the specifying phase from ORIENT through
// every step up to (and including) COMPLETE, stopping there so the caller can
// test the COMPLETE→PHASE_SHIFT advance under varying conditions (commits on,
// commits off, clean working tree). The caller must have run
// `init --phase specifying --from <queue>` before calling. Follows the
// oneSpecQueue convention: one spec in the "core" domain, file at "specs/a.md".
func driveSpecifyingToComplete(p *Project) {
	p.t.Helper()
	p.RunScript([]Step{
		advStep("orient→select", state.PhaseSpecifying, state.StateSelect),
		advStep("select→draft", state.PhaseSpecifying, state.StateDraft),
		{
			Label: "draft→evaluate",
			Setup: func(p *Project, _ Result) { p.WriteFile("specs/a.md", "# Spec A\n") },
			Args:  func(*Project, Result) []string { return []string{"advance"} },
			WantPhase: state.PhaseSpecifying,
			WantState: state.StateEvaluate,
		},
		evalPassStep("evaluate→accept", "PASS", state.PhaseSpecifying, state.StateAccept),
		advStep("accept→cross_reference", state.PhaseSpecifying, state.StateCrossReference),
		advStep("cross_reference→xref_eval", state.PhaseSpecifying, state.StateCrossReferenceEval),
		evalPassStep("xref_eval→xref_review", "PASS", state.PhaseSpecifying, state.StateCrossReferenceReview),
		advStep("xref_review→done", state.PhaseSpecifying, state.StateDone),
		advStep("done→reconcile", state.PhaseSpecifying, state.StateReconcile),
		advStep("reconcile→recon_eval", state.PhaseSpecifying, state.StateReconcileEval),
		evalPassStep("recon_eval→recon_review", "PASS", state.PhaseSpecifying, state.StateReconcileReview),
		advStep("recon_review→complete", state.PhaseSpecifying, state.StateComplete),
	})
}

// TestF2CommitsRequiresMessage verifies that with enable_commits:true, calling
// advance at the specifying COMPLETE state without --message exits non-zero,
// emits an explanatory error, and leaves the persisted state unchanged. A
// subsequent advance with --message must succeed and reach PHASE_SHIFT.
func TestF2CommitsRequiresMessage(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(specifyingConfig) // enable_commits = true
	p.WriteFile("spec-queue.json", oneSpecQueue)

	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	driveSpecifyingToComplete(p)
	p.AssertAt(state.PhaseSpecifying, state.StateComplete)

	// advance without --message must fail with a clear error.
	fail := p.forge("advance")
	if fail.Exit == 0 {
		t.Errorf("COMPLETE advance without --message exited 0, want non-zero")
	}
	if !strings.Contains(fail.Out(), "message is required") {
		t.Errorf("missing-message error did not explain itself:\n%s", fail.Out())
	}
	// State must not have changed — the failed call is a no-op.
	p.AssertAt(state.PhaseSpecifying, state.StateComplete)

	// advance with --message must succeed and reach PHASE_SHIFT.
	p.mustForge("advance", "--message", "specs: core batch")
	p.AssertAt(state.PhaseSpecifying, state.StatePhaseShift)
}

// TestF3NothingToCommit verifies that with enable_commits:true, reaching the
// specifying COMPLETE commit point with a clean working tree (all changes
// already committed by the agent) causes forgectl to emit a "nothing to commit"
// notice but still transition to PHASE_SHIFT normally (exit 0).
func TestF3NothingToCommit(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(specifyingConfig) // enable_commits = true
	p.WriteFile("spec-queue.json", oneSpecQueue)

	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	driveSpecifyingToComplete(p)
	p.AssertAt(state.PhaseSpecifying, state.StateComplete)

	// Pre-commit specs/a.md (and any other pending changes) so the working tree
	// is clean when the COMPLETE advance attempts its auto-commit.
	p.git("add", "-A")
	p.git("commit", "-m", "pre-stage")
	preLog := p.GitLog()

	// COMPLETE with --message but nothing new to commit → notice, transitions anyway.
	res := p.mustForge("advance", "--message", "clean")
	if !strings.Contains(res.Out(), "nothing to commit") {
		t.Errorf("expected 'nothing to commit' notice in combined output:\n%s", res.Out())
	}
	p.AssertAt(state.PhaseSpecifying, state.StatePhaseShift)

	// No new commit should have been created by forgectl — history length unchanged.
	postLog := p.GitLog()
	if len(postLog) != len(preLog) {
		t.Errorf("git log has %d commit(s) after COMPLETE, want %d (no new commit from forgectl)", len(postLog), len(preLog))
	}
}

// TestF5CommitsOffSuppressesGit verifies that with enable_commits:false, driving
// through the specifying COMPLETE commit point creates no git commits, requires
// no --message flag, and still transitions to PHASE_SHIFT normally.
func TestF5CommitsOffSuppressesGit(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(specifyingCommitsOffConfig) // enable_commits = false
	p.WriteFile("spec-queue.json", oneSpecQueue)

	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	driveSpecifyingToComplete(p)
	p.AssertAt(state.PhaseSpecifying, state.StateComplete)

	// COMPLETE with commits off: no --message required, no git commit created.
	p.mustForge("advance")
	p.AssertAt(state.PhaseSpecifying, state.StatePhaseShift)

	// Git history must be empty — forgectl made no commits.
	if log := p.GitLog(); len(log) != 0 {
		t.Errorf("git log = %v, want empty (commits suppressed by enable_commits:false)", log)
	}
}
