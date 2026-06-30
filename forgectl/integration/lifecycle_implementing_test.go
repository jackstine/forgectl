//go:build integration

package integration

import (
	"strings"
	"testing"

	"forgectl/state"
)

// --- A3 Implementing lifecycle ---
//
// This file covers plan §3.A3: the implementing phase full lifecycle walk and
// layer-gating invariant. It drives the real binary from init through both
// layers of a two-layer plan, asserting the state sequence, EvalRound counter,
// per-item passes outcome, layer-gating (L1 locked until L0 is complete), and
// the session-complete signal at the terminal DONE.

// implementingConfig is the standard config for A3 tests: commits off, batch 2,
// min_rounds=1 so a single PASS exits the eval loop immediately.
const implementingConfig = `
[general]
user_guided = false
enable_commits = false
[implementing]
batch = 2
[implementing.eval]
min_rounds = 1
max_rounds = 3
`

// implementingConfigBatch1 is like implementingConfig but with batch=1 so that
// a two-item layer is served as two sequential one-item batches. This lets the
// layer-gating test observe items "a" and "b" arriving in order, and prove that
// "c" (in L1) is never mixed in while L0 is active.
const implementingConfigBatch1 = `
[general]
user_guided = false
enable_commits = false
[implementing]
batch = 1
[implementing.eval]
min_rounds = 1
max_rounds = 3
`

// threeItemTwoLayerPlan returns a plan.json with two layers:
//
//	L0 = [a, b]   (independent items)
//	L1 = [c]      (depends_on [a, b])
//
// Used by the layer-gating test to prove L1 stays locked until L0 is complete.
func threeItemTwoLayerPlan(domain, module string) string {
	return `{
  "context": {"domain":"` + domain + `","module":"` + module + `"},
  "layers": [
    {"id":"L0","name":"Foundation","items":["a","b"]},
    {"id":"L1","name":"Wiring","items":["c"]}
  ],
  "items": [
    {"id":"a","name":"` + module + ` A","description":"first","depends_on":[],"tests":[{"category":"functional","description":"works","passes":false}]},
    {"id":"b","name":"` + module + ` B","description":"second","depends_on":[],"tests":[{"category":"functional","description":"works","passes":false}]},
    {"id":"c","name":"` + module + ` C","description":"third","depends_on":["a","b"],"tests":[{"category":"rejection","description":"rejects bad","passes":false}]}
  ]
}`
}

// containsItem reports whether id is present in items. Used to assert layer
// gating without importing reflect.
func containsItem(items []string, id string) bool {
	for _, it := range items {
		if it == id {
			return true
		}
	}
	return false
}

// TestA3ImplementingFullWalkTwoLayers is coverage gap A3 for the implementing
// phase. It initialises a two-layer plan (L0:[a] → L1:[b]) and drives it from
// init to the session-complete signal, asserting:
//
//   - Every state in the canonical ORIENT→IMPLEMENT→EVALUATE→COMMIT cycle is
//     visited exactly once per layer.
//   - EvalRound is 1 after the first EVALUATE entry in each layer (min_rounds=1
//     so the eval loop exits on the first PASS).
//   - Both items are marked "passed" in plan.json after DONE.
//   - The final advance past DONE outputs "session complete" and exits non-zero.
func TestA3ImplementingFullWalkTwoLayers(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(implementingConfig)
	p.WriteFile("core/plan.json", twoLayerCodePlan("core", "Core"))

	p.mustForge("init", "--phase", "implementing", "--from", "core/plan.json")
	p.AssertAt(state.PhaseImplementing, state.StateOrient)

	// --- Layer 0: item "a" ---

	// ORIENT → IMPLEMENT (first item of L0 batch).
	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateImplement)

	// IMPLEMENT → EVALUATE (EvalRound increments to 1 on entry to the state).
	eval1 := p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateEvaluate)
	if got := p.State().Implementing.CurrentBatch.EvalRound; got != 1 {
		t.Errorf("L0 EvalRound after EVALUATE entry = %d, want 1", got)
	}

	// EVALUATE PASS at round 1 >= min_rounds 1 → COMMIT.
	p.PassEval(eval1, "PASS")
	p.AssertAt(state.PhaseImplementing, state.StateCommit)

	// COMMIT → ORIENT (L0 complete; L1 now unlocked).
	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateOrient)

	// --- Layer 1: item "b" ---

	// ORIENT → IMPLEMENT (first item of L1 batch).
	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateImplement)

	// IMPLEMENT → EVALUATE (EvalRound is 1 for the new batch; counter resets per batch).
	eval2 := p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateEvaluate)
	if got := p.State().Implementing.CurrentBatch.EvalRound; got != 1 {
		t.Errorf("L1 EvalRound after EVALUATE entry = %d, want 1", got)
	}

	// EVALUATE PASS → COMMIT (last layer; all items done).
	p.PassEval(eval2, "PASS")
	p.AssertAt(state.PhaseImplementing, state.StateCommit)

	// COMMIT → DONE (no more layers).
	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateDone)

	// DONE → session complete: forgectl exits non-zero with the signal.
	final := p.forge("advance")
	if !strings.Contains(final.Out(), "session complete") {
		t.Errorf("expected session-complete signal after DONE:\n%s", final.Out())
	}

	// Both items must be marked "passed" in the persisted plan.json.
	assertAllItemsPassed(t, p, "core/plan.json")
}

// TestA3ImplementingLayerGating pins the layer-gating invariant: items in layer
// N+1 are not exposed until every item in layer N is COMMIT-marked complete.
//
// Plan: L0=[a,b], L1=[c depends_on a,b].  batch=1 so L0 is served as two
// sequential one-item batches, making the ordering explicit:
//
//	init → ORIENT → IMPLEMENT [a] → EVALUATE → COMMIT (a done)
//	      → ORIENT → IMPLEMENT [b] → EVALUATE → COMMIT (b done, L0 complete)
//	      → ORIENT → IMPLEMENT [c] → EVALUATE → COMMIT (c done)
//	      → DONE → session complete
//
// The test asserts that CurrentBatch.Items never contains "c" while L0 is in
// progress, and that "c" appears only after both L0 items are committed.
func TestA3ImplementingLayerGating(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(implementingConfigBatch1)
	p.WriteFile("core/plan.json", threeItemTwoLayerPlan("core", "Core"))

	p.mustForge("init", "--phase", "implementing", "--from", "core/plan.json")
	p.AssertAt(state.PhaseImplementing, state.StateOrient)

	// --- L0 batch 1: item "a" ---

	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateImplement)

	// Layer gating: "c" must not appear while L0 batch 1 is active.
	batch := p.State().Implementing.CurrentBatch
	if !containsItem(batch.Items, "a") {
		t.Errorf("L0 batch 1: want item \"a\" in CurrentBatch.Items, got %v", batch.Items)
	}
	if containsItem(batch.Items, "c") {
		t.Errorf("layer gating violated: L1 item \"c\" appeared in L0 batch 1: %v", batch.Items)
	}

	eval1 := p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateEvaluate)
	p.PassEval(eval1, "PASS")
	p.AssertAt(state.PhaseImplementing, state.StateCommit)

	// COMMIT (a done, L0 still incomplete: b remains) → ORIENT for next L0 batch.
	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateOrient)

	// --- L0 batch 2: item "b" ---

	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateImplement)

	batch = p.State().Implementing.CurrentBatch
	if !containsItem(batch.Items, "b") {
		t.Errorf("L0 batch 2: want item \"b\" in CurrentBatch.Items, got %v", batch.Items)
	}
	if containsItem(batch.Items, "c") {
		t.Errorf("layer gating violated: L1 item \"c\" appeared in L0 batch 2: %v", batch.Items)
	}

	eval2 := p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateEvaluate)
	p.PassEval(eval2, "PASS")
	p.AssertAt(state.PhaseImplementing, state.StateCommit)

	// COMMIT (b done, L0 now complete) → ORIENT for L1.
	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateOrient)

	// --- L1 batch 1: item "c" (now unlocked by L0 completion) ---

	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateImplement)

	batch = p.State().Implementing.CurrentBatch
	if !containsItem(batch.Items, "c") {
		t.Errorf("L1 batch 1: want item \"c\" in CurrentBatch.Items, got %v", batch.Items)
	}

	eval3 := p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateEvaluate)
	p.PassEval(eval3, "PASS")
	p.AssertAt(state.PhaseImplementing, state.StateCommit)

	// COMMIT → DONE (all layers complete).
	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateDone)

	// DONE → session complete.
	final := p.forge("advance")
	if !strings.Contains(final.Out(), "session complete") {
		t.Errorf("expected session-complete signal after DONE:\n%s", final.Out())
	}

	// All three items must be "passed" in the persisted plan.json.
	assertAllItemsPassed(t, p, "core/plan.json")
}
