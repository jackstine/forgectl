//go:build integration

package integration

import (
	"strings"
	"testing"

	"forgectl/state"
)

// specifyingConfig is the standard single-domain config used by the specifying
// lifecycle tests: batch 1, a 1–3 round eval budget, commits on, unguided.
const specifyingConfig = `
[specifying]
batch = 1
[specifying.eval]
min_rounds = 1
max_rounds = 3
[general]
enable_commits = true
user_guided = false
`

// oneSpecQueue is a one-entry spec queue in the "core" domain.
const oneSpecQueue = `{"specs":[{"name":"Spec A","domain":"core","topic":"topic a","file":"specs/a.md","planning_sources":[],"depends_on":[]}]}`

// TestSmokeBuiltBinaryVersion exercises the actually built binary's simplest
// path: --version with no session. It proves TestMain's build produced a
// runnable artifact before any heavier test relies on it.
func TestSmokeBuiltBinaryVersion(t *testing.T) {
	p := NewProject(t)
	res := p.mustForge("--version")
	if !strings.Contains(res.Stdout, "forgectl version") {
		t.Errorf("--version stdout = %q, want it to contain %q", res.Stdout, "forgectl version")
	}
	// --version must not require or create a session, and must not log (H1).
	if p.StateExists() {
		t.Error("--version created a state file")
	}
	if got := p.LogFiles(); len(got) != 0 {
		t.Errorf("--version wrote activity logs: %v", got)
	}
}

// TestA1SpecifyingFullWalk drives the specifying phase from init to PHASE_SHIFT
// through every state, asserting the trifecta — CLI output/exit, persisted
// state, and external side effects (git commit + hash, session archive,
// activity log) — at the points that matter. This is coverage gap A1.
func TestA1SpecifyingFullWalk(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(specifyingConfig)
	p.WriteFile("spec-queue.json", oneSpecQueue)

	// init --phase specifying --from spec-queue.json → ORIENT. (Specifying ORIENT
	// is a transient setup state, so init prints no action block; the persisted
	// state is the assertion that matters.)
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	p.AssertAt(state.PhaseSpecifying, state.StateOrient)

	// init is one of only two commands that log; it should have created exactly
	// one session log file named <phase>-<sessionid[:8]>.jsonl (H1/H2).
	logs := p.LogFiles()
	if len(logs) != 1 {
		t.Fatalf("after init: log files = %v, want exactly one", logs)
	}
	sid := p.State().SessionID
	wantLog := "specifying-" + sid[:8] + ".jsonl"
	if logs[0] != wantLog {
		t.Errorf("log file = %q, want %q", logs[0], wantLog)
	}

	// ORIENT → SELECT → DRAFT
	p.mustForge("advance")
	p.AssertAt(state.PhaseSpecifying, state.StateSelect)
	p.mustForge("advance")
	p.AssertAt(state.PhaseSpecifying, state.StateDraft)

	// The agent drafts the spec, then advances to evaluation.
	p.WriteFile("specs/a.md", "# Spec A\n")
	eval := p.mustForge("advance")
	p.AssertAt(state.PhaseSpecifying, state.StateEvaluate)
	if !strings.Contains(eval.Stdout, "Round:   1/3") {
		t.Errorf("EVALUATE output missing round 1/3:\n%s", eval.Stdout)
	}

	// EVALUATE PASS at round >= min_rounds → ACCEPT.
	p.PassEval(eval, "PASS")
	p.AssertAt(state.PhaseSpecifying, state.StateAccept)

	// ACCEPT (queue empty) → per-domain CROSS_REFERENCE loop.
	p.mustForge("advance")
	p.AssertAt(state.PhaseSpecifying, state.StateCrossReference)
	xrefEval := p.mustForge("advance")
	p.AssertAt(state.PhaseSpecifying, state.StateCrossReferenceEval)
	p.PassEval(xrefEval, "PASS")
	p.AssertAt(state.PhaseSpecifying, state.StateCrossReferenceReview)

	// CROSS_REFERENCE_REVIEW → DONE → RECONCILE loop.
	p.mustForge("advance")
	p.AssertAt(state.PhaseSpecifying, state.StateDone)
	p.mustForge("advance")
	p.AssertAt(state.PhaseSpecifying, state.StateReconcile)
	reconEval := p.mustForge("advance")
	p.AssertAt(state.PhaseSpecifying, state.StateReconcileEval)
	p.PassEval(reconEval, "PASS")
	p.AssertAt(state.PhaseSpecifying, state.StateReconcileReview)

	// RECONCILE_REVIEW → COMPLETE.
	p.mustForge("advance")
	p.AssertAt(state.PhaseSpecifying, state.StateComplete)

	// F2/L1: at COMPLETE with commits enabled, a missing --message is an error
	// that does NOT mutate state.
	fail := p.forge("advance")
	if fail.Exit == 0 {
		t.Errorf("COMPLETE advance without --message succeeded, want failure")
	}
	if !strings.Contains(fail.Out(), "message is required") {
		t.Errorf("missing-message error didn't explain itself:\n%s", fail.Out())
	}
	p.AssertAt(state.PhaseSpecifying, state.StateComplete) // unchanged

	// COMPLETE --message → PHASE_SHIFT, committing at this single point.
	shift := p.mustForge("advance", "--message", "specs: core batch")
	p.AssertAt(state.PhaseSpecifying, state.StatePhaseShift)
	if !strings.Contains(shift.Stdout, "specifying → generate_planning_queue") {
		t.Errorf("PHASE_SHIFT output missing transition line:\n%s", shift.Stdout)
	}

	// --- External side effects (the third side-channel) ---

	// Exactly one commit, with our message, staging the spec.
	log := p.GitLog()
	if len(log) != 1 || log[0] != "specs: core batch" {
		t.Fatalf("git log = %v, want one commit 'specs: core batch'", log)
	}
	if stat := p.GitShowStat(); !strings.Contains(stat, "specs/a.md") {
		t.Errorf("commit did not stage specs/a.md:\n%s", stat)
	}

	// The commit hash is registered on the completed spec.
	s := p.State()
	if len(s.Specifying.Completed) != 1 {
		t.Fatalf("completed specs = %d, want 1", len(s.Specifying.Completed))
	}
	done := s.Specifying.Completed[0]
	if done.Name != "Spec A" || done.Domain != "core" {
		t.Errorf("completed spec = %+v, want Spec A/core", done)
	}
	if len(done.CommitHashes) == 0 {
		t.Error("completed spec has no commit_hashes recorded")
	} else if full := strings.TrimSpace(p.git("rev-parse", "HEAD")); !strings.HasPrefix(full, done.CommitHashes[0]) && done.CommitHashes[0] != full {
		t.Errorf("recorded commit hash %q is not HEAD %q", done.CommitHashes[0], full)
	}

	// Session archived at the terminal PHASE_SHIFT (E4).
	if !p.Exists(".forgectl/state/sessions") {
		t.Error("no sessions/ archive directory created at terminal state")
	}

	// The activity log stayed the same file across the whole walk (H2) and now
	// records the init plus every advance.
	logs = p.LogFiles()
	if len(logs) != 1 || logs[0] != wantLog {
		t.Fatalf("log files after walk = %v, want still just %q", logs, wantLog)
	}
	entries := p.LogEntries(logs[0])
	var cmds []string
	for _, e := range entries {
		cmds = append(cmds, e["cmd"].(string))
	}
	if cmds[0] != "init" {
		t.Errorf("first log entry cmd = %q, want init", cmds[0])
	}
	advances := 0
	for _, c := range cmds {
		if c == "advance" {
			advances++
		}
	}
	if advances == 0 {
		t.Error("no advance entries logged")
	}
}
