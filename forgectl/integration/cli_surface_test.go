//go:build integration

package integration

import (
	"strings"
	"testing"

	"forgectl/state"
)

// This file covers plan §4 — CLI surface contracts for commands that modify
// session state (K3) and the invariant that every error path surfaces its
// message on stderr with a non-zero exit code (K4).

// --- K3: add-queue-item / set-roots / add-domain guards ---

// minSpecifyingConfig is a minimal config sufficient for specifying phase
// sessions used by the K3 guard tests. Batch and round limits default to the
// compiled-in values; user_guided=false keeps the tests non-interactive.
const minSpecifyingConfig = `
[general]
user_guided = false
enable_commits = false
`

// minPlanQueue is a single-domain plan queue, the minimum input for
// init --phase planning in the K3 set-roots and add-domain guard tests.
const minPlanQueue = `{
  "plans": [
    {"name":"Core Plan","domain":"core","file":"core/plan.json","specs":[],"spec_commits":[],"code_search_roots":["core/"],"kind":"code"}
  ]
}`

// TestK3AddQueueItemGuards asserts the documented guard conditions for the
// add-queue-item command:
//
//   - No active session → error.
//   - Wrong phase (implementing) → error naming the specifying phase.
//   - Specifying but wrong state (ORIENT) → error.
//   - Valid state (DRAFT) with a nonexistent --file → error.
//   - Duplicate name (same spec already in queue) → error.
func TestK3AddQueueItemGuards(t *testing.T) {
	// Guard: no active session — resolveSession finds no .forgectl directory.
	fresh := NewProject(t)
	res := fresh.forge("add-queue-item", "--name", "X", "--topic", "t", "--file", "x.md")
	if res.Exit == 0 {
		t.Error("add-queue-item with no session: expected non-zero exit")
	}

	// Guard: wrong phase. init implementing, then call add-queue-item.
	pImpl := NewProject(t)
	pImpl.WriteConfig(`
[general]
user_guided = false
enable_commits = false
[implementing]
batch = 2
[implementing.eval]
min_rounds = 1
max_rounds = 3
`)
	pImpl.WriteFile("core/plan.json", singleItemImplPlan("core", "Core"))
	pImpl.mustForge("init", "--phase", "implementing", "--from", "core/plan.json")
	res = pImpl.forge("add-queue-item", "--name", "X", "--topic", "t", "--file", "x.md")
	if res.Exit == 0 {
		t.Error("add-queue-item in implementing phase: expected non-zero exit")
	}
	if !strings.Contains(res.Out(), "specifying") {
		t.Errorf("add-queue-item wrong-phase error should mention specifying:\n%s", res.Out())
	}

	// For the remaining guards, drive a specifying session.
	p := NewProject(t)
	p.WriteConfig(minSpecifyingConfig)
	p.WriteFile("spec-queue.json", oneSpecQueue)
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	p.AssertAt(state.PhaseSpecifying, state.StateOrient)

	// Guard: valid phase but wrong state (ORIENT is not in the allowed set).
	res = p.forge("add-queue-item", "--name", "X", "--topic", "t", "--file", "specs/x.md")
	if res.Exit == 0 {
		t.Error("add-queue-item at ORIENT: expected non-zero exit")
	}

	// Drive to DRAFT where add-queue-item is valid.
	p.mustForge("advance") // ORIENT → SELECT
	p.mustForge("advance") // SELECT → DRAFT
	p.AssertAt(state.PhaseSpecifying, state.StateDraft)

	// Guard: file does not exist.
	res = p.forge("add-queue-item", "--name", "New Spec", "--topic", "new topic", "--file", "specs/nonexistent.md")
	if res.Exit == 0 {
		t.Error("add-queue-item with nonexistent file: expected non-zero exit")
	}
	if !strings.Contains(res.Out(), "does not exist") {
		t.Errorf("add-queue-item file-not-found error should mention 'does not exist':\n%s", res.Out())
	}

	// Guard: duplicate name. Add "New Spec" once (succeeds), then again (fails).
	p.WriteFile("specs/new.md", "# New Spec\n")
	p.mustForge("add-queue-item", "--name", "New Spec", "--topic", "new topic", "--file", p.Path("specs/new.md"))

	res = p.forge("add-queue-item", "--name", "New Spec", "--topic", "new topic 2", "--file", p.Path("specs/new.md"))
	if res.Exit == 0 {
		t.Error("add-queue-item duplicate name: expected non-zero exit")
	}
	if !strings.Contains(res.Out(), "already exists") {
		t.Errorf("add-queue-item duplicate error should mention 'already exists':\n%s", res.Out())
	}
}

// TestK3SetRootsGuards asserts the documented guard conditions for the
// set-roots command:
//
//   - Wrong phase (planning) → error naming the specifying phase.
//   - Specifying but wrong state (ORIENT) → error naming the state.
func TestK3SetRootsGuards(t *testing.T) {
	// Guard: wrong phase. init planning, then call set-roots.
	pPlan := NewProject(t)
	pPlan.WriteConfig(minSpecifyingConfig)
	pPlan.WriteFile("plan-queue.json", minPlanQueue)
	pPlan.mustForge("init", "--phase", "planning", "--from", "plan-queue.json")
	res := pPlan.forge("set-roots", "core/")
	if res.Exit == 0 {
		t.Error("set-roots in planning phase: expected non-zero exit")
	}
	if !strings.Contains(res.Out(), "specifying") {
		t.Errorf("set-roots wrong-phase error should mention specifying:\n%s", res.Out())
	}

	// Guard: right phase but wrong state. set-roots is only valid at
	// CROSS_REFERENCE_REVIEW and DONE; ORIENT must be rejected.
	p := NewProject(t)
	p.WriteConfig(minSpecifyingConfig)
	p.WriteFile("spec-queue.json", oneSpecQueue)
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	p.AssertAt(state.PhaseSpecifying, state.StateOrient)

	res = p.forge("set-roots", "core/")
	if res.Exit == 0 {
		t.Error("set-roots at ORIENT: expected non-zero exit")
	}
	if !strings.Contains(res.Out(), "ORIENT") {
		t.Errorf("set-roots wrong-state error should mention the current state ORIENT:\n%s", res.Out())
	}
}

// TestK3SetCommitHashesGuards asserts the documented guard conditions for the
// set-commit-hashes command:
//
//   - Wrong phase (planning) → error naming the specifying phase.
//   - Specifying but wrong state (ORIENT) → error naming the state.
func TestK3SetCommitHashesGuards(t *testing.T) {
	// Guard: wrong phase. init planning, then call set-commit-hashes.
	pPlan := NewProject(t)
	pPlan.WriteConfig(minSpecifyingConfig)
	pPlan.WriteFile("plan-queue.json", minPlanQueue)
	pPlan.mustForge("init", "--phase", "planning", "--from", "plan-queue.json")
	res := pPlan.forge("set-commit-hashes", "abc1234")
	if res.Exit == 0 {
		t.Error("set-commit-hashes in planning phase: expected non-zero exit")
	}
	if !strings.Contains(res.Out(), "specifying") {
		t.Errorf("set-commit-hashes wrong-phase error should mention specifying:\n%s", res.Out())
	}

	// Guard: right phase but wrong state. set-commit-hashes is only valid at
	// CROSS_REFERENCE_REVIEW and DONE; ORIENT must be rejected.
	p := NewProject(t)
	p.WriteConfig(minSpecifyingConfig)
	p.WriteFile("spec-queue.json", oneSpecQueue)
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	p.AssertAt(state.PhaseSpecifying, state.StateOrient)

	res = p.forge("set-commit-hashes", "abc1234")
	if res.Exit == 0 {
		t.Error("set-commit-hashes at ORIENT: expected non-zero exit")
	}
	if !strings.Contains(res.Out(), "ORIENT") {
		t.Errorf("set-commit-hashes wrong-state error should mention the current state ORIENT:\n%s", res.Out())
	}
}

// TestK3AddDomainGuards asserts the documented guard condition for the
// add-domain command: it is only valid during the reverse_engineering QUEUE
// state. Calling it from any other phase produces a recognizable error.
func TestK3AddDomainGuards(t *testing.T) {
	// Guard: wrong phase. A specifying session is sufficient to trigger the
	// phase/state guard — add-domain requires reverse_engineering QUEUE.
	p := NewProject(t)
	p.WriteConfig(minSpecifyingConfig)
	p.WriteFile("spec-queue.json", oneSpecQueue)
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")

	res := p.forge("add-domain", "core")
	if res.Exit == 0 {
		t.Error("add-domain in specifying phase: expected non-zero exit")
	}
	// The error must mention the QUEUE state so the operator understands the
	// context requirement.
	if !strings.Contains(res.Out(), "QUEUE") {
		t.Errorf("add-domain wrong-phase/state error should mention QUEUE:\n%s", res.Out())
	}

	// Guard: no active session.
	fresh := NewProject(t)
	res = fresh.forge("add-domain", "core")
	if res.Exit == 0 {
		t.Error("add-domain with no session: expected non-zero exit")
	}
}

// --- K4: Every error path exits non-zero to stderr ---

// TestK4ErrorPathsExitNonZeroToStderr enumerates four documented error paths
// and asserts that each one (a) exits non-zero and (b) writes the error
// description to stderr rather than stdout.
//
// Paths tested:
//
//  1. Unknown command — cobra's unknown-command error.
//  2. validate with a nonexistent file — file-not-found before any output.
//  3. advance --verdict INVALID_VALUE at EVALUATE — verdict validation in the
//     state machine.
//  4. advance without --verdict at EVALUATE — missing-verdict check in the
//     state machine.
func TestK4ErrorPathsExitNonZeroToStderr(t *testing.T) {
	p := NewProject(t)

	// Path 1: unknown command.
	res := p.forge("unknowncmd")
	if res.Exit == 0 {
		t.Error("unknown command: expected non-zero exit")
	}
	if strings.TrimSpace(res.Stderr) == "" {
		t.Errorf("unknown command: expected error on stderr, got nothing\nstdout:\n%s", res.Stdout)
	}

	// Path 2: validate a nonexistent file.
	res = p.forge("validate", "/nonexistent/file.json")
	if res.Exit == 0 {
		t.Error("validate nonexistent file: expected non-zero exit")
	}
	if strings.TrimSpace(res.Stderr) == "" {
		t.Errorf("validate nonexistent: expected error on stderr, got nothing\nstdout:\n%s", res.Stdout)
	}

	// Paths 3 and 4 require a session at EVALUATE. Use implementing phase for
	// a straightforward ORIENT → IMPLEMENT → EVALUATE walk.
	p2 := NewProject(t)
	p2.WriteConfig(`
[general]
user_guided = false
enable_commits = false
[implementing]
batch = 2
[implementing.eval]
min_rounds = 1
max_rounds = 3
`)
	p2.WriteFile("core/plan.json", singleItemImplPlan("core", "Core"))
	p2.mustForge("init", "--phase", "implementing", "--from", "core/plan.json")
	p2.mustForge("advance") // ORIENT → IMPLEMENT
	p2.mustForge("advance") // IMPLEMENT → EVALUATE
	p2.AssertAt(state.PhaseImplementing, state.StateEvaluate)

	// Path 3: invalid verdict value at EVALUATE.
	p2.WriteReport("stubs/report.md")
	res = p2.forge("advance", "--verdict", "INVALID_VALUE", "--eval-report", "stubs/report.md")
	if res.Exit == 0 {
		t.Error("invalid verdict: expected non-zero exit")
	}
	if strings.TrimSpace(res.Stderr) == "" {
		t.Errorf("invalid verdict: expected error on stderr, got nothing\nstdout:\n%s", res.Stdout)
	}

	// Path 4: missing --verdict at EVALUATE (advance with no flags).
	res = p2.forge("advance")
	if res.Exit == 0 {
		t.Error("missing verdict: expected non-zero exit")
	}
	if strings.TrimSpace(res.Stderr) == "" {
		t.Errorf("missing verdict: expected error on stderr, got nothing\nstdout:\n%s", res.Stdout)
	}

	// Both failed advances must have left the state at EVALUATE.
	p2.AssertAt(state.PhaseImplementing, state.StateEvaluate)
}
