//go:build integration

package integration

import (
	"strings"
	"testing"

	"forgectl/state"
)

// This file is harness item §2.5: a tiny scripted-session DSL so full lifecycles
// read as data rather than 200-line test bodies. Each Step is one forge
// invocation plus the (phase, state, exit) it must land at; a script is a slice
// of them. Steps that need the previous step's output (the eval handshake) get
// it via the `last` argument to their Setup/Args callbacks.

// Step is one scripted forge invocation and its expected outcome.
type Step struct {
	Label string
	// Setup runs before the invocation — write a draft file, stub an eval report,
	// etc. It may read the previous step's output via last.
	Setup func(p *Project, last Result)
	// Args builds the forge arguments, optionally from the previous output (e.g.
	// to thread an extracted eval-report path). Required.
	Args func(p *Project, last Result) []string
	// WantPhase/WantState are asserted against the persisted state after the
	// invocation when non-empty.
	WantPhase state.PhaseName
	WantState state.StateName
	// WantExit is the expected exit code (0 by default).
	WantExit int
}

// RunScript executes the steps in order, asserting each one's exit code and
// resulting (phase, state). It returns the final Result.
func (p *Project) RunScript(steps []Step) Result {
	p.t.Helper()
	var last Result
	for _, s := range steps {
		label := s.Label
		if label == "" {
			label = "step"
		}
		if s.Setup != nil {
			s.Setup(p, last)
		}
		args := s.Args(p, last)
		last = p.forge(args...)
		if last.Exit != s.WantExit {
			p.t.Fatalf("%s: forge %v exit = %d, want %d\nstdout:\n%s\nstderr:\n%s",
				label, args, last.Exit, s.WantExit, last.Stdout, last.Stderr)
		}
		if s.WantPhase != "" || s.WantState != "" {
			st := p.State()
			if (s.WantPhase != "" && st.Phase != s.WantPhase) || (s.WantState != "" && st.State != s.WantState) {
				p.t.Fatalf("%s: state = %s/%s, want %s/%s", label, st.Phase, st.State, s.WantPhase, s.WantState)
			}
		}
	}
	return last
}

// advStep is a plain `advance` that expects a phase/state.
func advStep(label string, phase state.PhaseName, st state.StateName) Step {
	return Step{
		Label:     label,
		Args:      func(*Project, Result) []string { return []string{"advance"} },
		WantPhase: phase,
		WantState: st,
	}
}

// evalPassStep advances an EVALUATE-family state with a verdict, stubbing the
// report at the path the previous step's output demanded.
func evalPassStep(label, verdict string, phase state.PhaseName, st state.StateName) Step {
	var path string
	return Step{
		Label: label,
		Setup: func(p *Project, last Result) {
			path = p.ExtractReportPath(last.Out())
			p.WriteReport(path)
		},
		Args: func(*Project, Result) []string {
			return []string{"advance", "--verdict", verdict, "--eval-report", path}
		},
		WantPhase: phase,
		WantState: st,
	}
}

// TestScriptedSpecifyingHappyPath expresses the entire specifying lifecycle as a
// data script (commits off, all-PASS), demonstrating the DSL and re-verifying
// the canonical state sequence independently of the imperative A1 test.
func TestScriptedSpecifyingHappyPath(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig("[specifying]\nbatch = 1\n[specifying.eval]\nmin_rounds = 1\nmax_rounds = 3\n[general]\nuser_guided = false\nenable_commits = false\n")
	p.WriteFile("spec-queue.json", oneSpecQueue)
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")

	final := p.RunScript([]Step{
		advStep("orient→select", state.PhaseSpecifying, state.StateSelect),
		{
			Label:     "select→draft (write spec)",
			Setup:     func(p *Project, _ Result) {},
			Args:      func(*Project, Result) []string { return []string{"advance"} },
			WantPhase: state.PhaseSpecifying, WantState: state.StateDraft,
		},
		{
			Label:     "draft→evaluate",
			Setup:     func(p *Project, _ Result) { p.WriteFile("specs/a.md", "# Spec A\n") },
			Args:      func(*Project, Result) []string { return []string{"advance"} },
			WantPhase: state.PhaseSpecifying, WantState: state.StateEvaluate,
		},
		evalPassStep("evaluate→accept", "PASS", state.PhaseSpecifying, state.StateAccept),
		advStep("accept→cross_reference", state.PhaseSpecifying, state.StateCrossReference),
		advStep("cross_reference→eval", state.PhaseSpecifying, state.StateCrossReferenceEval),
		evalPassStep("xref_eval→review", "PASS", state.PhaseSpecifying, state.StateCrossReferenceReview),
		advStep("xref_review→done", state.PhaseSpecifying, state.StateDone),
		advStep("done→reconcile", state.PhaseSpecifying, state.StateReconcile),
		advStep("reconcile→eval", state.PhaseSpecifying, state.StateReconcileEval),
		evalPassStep("recon_eval→review", "PASS", state.PhaseSpecifying, state.StateReconcileReview),
		advStep("recon_review→complete", state.PhaseSpecifying, state.StateComplete),
		advStep("complete→phase_shift", state.PhaseSpecifying, state.StatePhaseShift),
	})

	if !strings.Contains(final.Stdout, "specifying → generate_planning_queue") {
		t.Errorf("final PHASE_SHIFT output unexpected:\n%s", final.Stdout)
	}
}
