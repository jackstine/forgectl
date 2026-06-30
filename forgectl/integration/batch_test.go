//go:build integration

package integration

import (
	"testing"

	"forgectl/state"
)

// This file covers plan §D batch-selection contracts:
//
//   D1 (TestD1BatchNeverMixesDomains): SELECT batches must never cross domain
//   boundaries — every spec in a batch must share the same domain.
//
//   D2 (TestD2ImplementingUnblockedItemsOnly): the implementing phase must
//   respect layer gating — items in a deeper layer cannot appear in a batch
//   until all items in the preceding layer are terminal (passed).
//
//   D3 (TestD3ItemOrderPreserved): items within a layer must be presented in
//   exactly the order they are declared in the plan's layer definition.

// ─── D1 ─────────────────────────────────────────────────────────────────────

// d1Config uses batch=2 with commits off. Four interleaved specs (core, api,
// core, api) mean any cross-domain grouping would produce a mixed batch.
const d1Config = `
[specifying]
batch = 2
[specifying.eval]
min_rounds = 1
max_rounds = 3
[general]
user_guided = false
enable_commits = false
`

// d1SpecQueue has four specs interleaved across two domains. A correct
// implementation groups by domain so each batch of 2 is all-core or all-api.
const d1SpecQueue = `{"specs":[
  {"name":"Core Spec 1","domain":"core","topic":"core a","file":"specs/core1.md","planning_sources":[],"depends_on":[]},
  {"name":"Api Spec 1", "domain":"api", "topic":"api a", "file":"specs/api1.md", "planning_sources":[],"depends_on":[]},
  {"name":"Core Spec 2","domain":"core","topic":"core b","file":"specs/core2.md","planning_sources":[],"depends_on":[]},
  {"name":"Api Spec 2", "domain":"api", "topic":"api b", "file":"specs/api2.md", "planning_sources":[],"depends_on":[]}
]}`

// TestD1BatchNeverMixesDomains verifies that each specifying SELECT batch
// contains specs from exactly one domain. With batch=2 and four specs
// interleaved as core/api/core/api, a correct implementation groups by domain
// (all-core then all-api, or vice-versa) rather than mixing across domains.
func TestD1BatchNeverMixesDomains(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(d1Config)
	p.WriteFile("spec-queue.json", d1SpecQueue)

	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	p.AssertAt(state.PhaseSpecifying, state.StateOrient)

	batchDomains := driveSpecifyingRecordingBatches(p, t)

	if len(batchDomains) == 0 {
		t.Fatal("no batches were recorded during specifying run")
	}

	for i, domains := range batchDomains {
		if len(domains) == 0 {
			t.Errorf("batch %d contained no specs", i+1)
			continue
		}
		first := domains[0]
		for _, d := range domains[1:] {
			if d != first {
				t.Errorf("batch %d mixes domains: %v (want all %q)", i+1, domains, first)
				break
			}
		}
	}
}

// driveSpecifyingRecordingBatches drives the specifying phase from its current
// state to PHASE_SHIFT, supplying all required side effects (stub spec files at
// DRAFT, PASS verdicts + stub reports at each eval state). At every SELECT it
// records the domains of the chosen specs and returns that slice. Commits are
// assumed to be disabled (COMPLETE requires no --message).
func driveSpecifyingRecordingBatches(p *Project, t *testing.T) [][]string {
	t.Helper()
	var batchDomains [][]string
	var prevRes Result

	for steps := 0; steps < 150; steps++ {
		s := p.State()

		switch s.State {
		case state.StatePhaseShift:
			return batchDomains

		case state.StateSelect:
			// Record the domain of each spec the binary chose for this batch.
			var domains []string
			for _, spec := range s.Specifying.CurrentSpecs {
				domains = append(domains, spec.Domain)
			}
			batchDomains = append(batchDomains, domains)
			prevRes = p.mustForge("advance")

		case state.StateDraft:
			// CurrentSpecs is still set from SELECT; write a stub file for each.
			for _, spec := range s.Specifying.CurrentSpecs {
				p.WriteFile(spec.File, "# "+spec.Name+"\n")
			}
			// Advance DRAFT → EVALUATE; the result carries the eval-report path.
			prevRes = p.mustForge("advance")

		case state.StateEvaluate:
			// prevRes is the output from advancing into EVALUATE (from DRAFT).
			prevRes = p.PassEval(prevRes, "PASS")

		case state.StateCrossReferenceEval:
			// prevRes is the output from advancing into CROSS_REFERENCE_EVAL.
			prevRes = p.PassEval(prevRes, "PASS")

		case state.StateReconcileEval:
			// prevRes is the output from advancing into RECONCILE_EVAL.
			prevRes = p.PassEval(prevRes, "PASS")

		default:
			// ORIENT, ACCEPT, CROSS_REFERENCE, CROSS_REFERENCE_REVIEW, DONE,
			// RECONCILE, RECONCILE_REVIEW, COMPLETE — all take a plain advance.
			prevRes = p.mustForge("advance")
		}
	}

	t.Fatal("specifying phase did not reach PHASE_SHIFT within step limit")
	return nil
}

// ─── D2 ─────────────────────────────────────────────────────────────────────

// d2d3Config is shared by D2 and D3: batch=10 (fits an entire layer in one
// batch), commits off, 1–3 eval rounds.
const d2d3Config = `
[general]
user_guided = false
enable_commits = false
[implementing]
batch = 10
[implementing.eval]
min_rounds = 1
max_rounds = 3
`

// d2Plan returns a two-layer plan: L0 holds items "a" and "b" (no deps), L1
// holds item "c" which depends on both "a" and "b". With batch=10 the whole L0
// fits in a single batch; L1 must not start until L0 is terminal.
func d2Plan() string {
	return `{
  "context": {"domain":"core","module":"Core"},
  "layers": [
    {"id":"L0","name":"Foundation","items":["a","b"]},
    {"id":"L1","name":"Wiring",    "items":["c"]}
  ],
  "items": [
    {"id":"a","name":"A","description":"a","depends_on":[],        "tests":[]},
    {"id":"b","name":"B","description":"b","depends_on":[],        "tests":[]},
    {"id":"c","name":"C","description":"c","depends_on":["a","b"], "tests":[]}
  ]
}`
}

// TestD2ImplementingUnblockedItemsOnly verifies that the implementing phase
// respects layer gating: item "c" (in L1, depending on "a" and "b") must not
// appear in any batch before L0 has completed. With batch=10, L0's entire
// batch is [a, b]; only after COMMIT does L1 become eligible, presenting [c].
func TestD2ImplementingUnblockedItemsOnly(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(d2d3Config)
	p.WriteFile("core/plan.json", d2Plan())

	p.mustForge("init", "--phase", "implementing", "--from", "core/plan.json")
	p.AssertAt(state.PhaseImplementing, state.StateOrient)

	// ORIENT → first IMPLEMENT (L0 batch selected).
	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateImplement)

	// L0 batch must contain a and b, and must NOT contain c.
	l0 := p.State().Implementing.CurrentBatch
	if l0 == nil {
		t.Fatal("CurrentBatch is nil at L0 IMPLEMENT")
	}
	if containsStr(l0.Items, "c") {
		t.Errorf("L0 batch contains blocked item 'c' before L0 is terminal: items=%v", l0.Items)
	}
	if !containsStr(l0.Items, "a") || !containsStr(l0.Items, "b") {
		t.Errorf("L0 batch does not contain both 'a' and 'b': items=%v", l0.Items)
	}

	// Drive L0 through all IMPLEMENT steps → EVALUATE → COMMIT, arriving at L1 ORIENT.
	driveImplBatchToNextState(p, t)
	p.AssertAt(state.PhaseImplementing, state.StateOrient)

	// L1 ORIENT → IMPLEMENT (only after L0 is complete).
	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateImplement)

	// L1 batch must contain c and must NOT still carry a or b.
	l1 := p.State().Implementing.CurrentBatch
	if l1 == nil {
		t.Fatal("CurrentBatch is nil at L1 IMPLEMENT")
	}
	if !containsStr(l1.Items, "c") {
		t.Errorf("L1 batch missing 'c' after L0 completed: items=%v", l1.Items)
	}
	if containsStr(l1.Items, "a") || containsStr(l1.Items, "b") {
		t.Errorf("L1 batch still contains L0 items: items=%v", l1.Items)
	}
}

// driveImplBatchToNextState drives the implementing phase from the current
// IMPLEMENT state through any remaining IMPLEMENT steps, then EVALUATE and
// COMMIT, and the advance past COMMIT — leaving the project at the next ORIENT
// (if another layer remains) or DONE (if the plan is complete). It uses plain
// advances for IMPLEMENT and COMMIT (commits off) and PassEval for EVALUATE.
func driveImplBatchToNextState(p *Project, t *testing.T) {
	t.Helper()
	var prevRes Result

	for steps := 0; steps < 30; steps++ {
		s := p.State()
		switch s.State {
		case state.StateOrient, state.StateDone:
			// Passed COMMIT and arrived at the next state.
			return
		case state.StateEvaluate:
			// prevRes captured the output of advancing into EVALUATE.
			prevRes = p.PassEval(prevRes, "PASS")
		default:
			// IMPLEMENT (current or subsequent item) and COMMIT both use plain advance.
			prevRes = p.mustForge("advance")
		}
	}

	t.Fatal("implementing batch did not reach next ORIENT or DONE within step limit")
}

// ─── D3 ─────────────────────────────────────────────────────────────────────

// d3Plan returns a single-layer plan with three items declared in order x→y→z.
// With batch=10 all three land in one batch; the Items slice must preserve
// declaration order.
func d3Plan() string {
	return `{
  "context": {"domain":"core","module":"Core"},
  "layers": [{"id":"L0","name":"L0","items":["x","y","z"]}],
  "items": [
    {"id":"x","name":"X","description":"x","depends_on":[],"tests":[]},
    {"id":"y","name":"Y","description":"y","depends_on":[],"tests":[]},
    {"id":"z","name":"Z","description":"z","depends_on":[],"tests":[]}
  ]
}`
}

// TestD3ItemOrderPreserved verifies that the implementing phase presents items
// within a layer in exactly the order they are declared in the plan. With
// batch=10, all three items (x, y, z) fit in one batch; successive IMPLEMENT
// advances must step through them in declaration order (x first, z last).
func TestD3ItemOrderPreserved(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(d2d3Config)
	p.WriteFile("core/plan.json", d3Plan())

	p.mustForge("init", "--phase", "implementing", "--from", "core/plan.json")
	p.AssertAt(state.PhaseImplementing, state.StateOrient)

	// ORIENT → first IMPLEMENT.
	p.mustForge("advance")
	p.AssertAt(state.PhaseImplementing, state.StateImplement)

	// The full Items slice must equal the declared order.
	wantOrder := []string{"x", "y", "z"}
	batch := p.State().Implementing.CurrentBatch
	if batch == nil {
		t.Fatal("CurrentBatch is nil at first IMPLEMENT")
	}
	if !equalStrings(batch.Items, wantOrder) {
		t.Fatalf("batch Items = %v, want %v (declaration order)", batch.Items, wantOrder)
	}

	// Walk through every IMPLEMENT step; at each one verify the current-item
	// index and identity match the declared order before advancing.
	for wantIdx, wantID := range wantOrder {
		s := p.State()
		if s.State != state.StateImplement {
			// Batch exhausted earlier than expected (binary advanced to EVALUATE).
			t.Errorf("step %d: expected IMPLEMENT, got %s", wantIdx, s.State)
			break
		}
		b := s.Implementing.CurrentBatch
		if b.CurrentItemIndex != wantIdx {
			t.Errorf("step %d: CurrentItemIndex = %d, want %d", wantIdx, b.CurrentItemIndex, wantIdx)
		}
		if got := b.Items[b.CurrentItemIndex]; got != wantID {
			t.Errorf("step %d: current item = %q, want %q", wantIdx, got, wantID)
		}
		p.mustForge("advance")
	}
}

// ─── shared helpers ──────────────────────────────────────────────────────────

// containsStr reports whether ss contains the exact string s.
func containsStr(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}
