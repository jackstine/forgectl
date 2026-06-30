//go:build integration

package integration

// §4.7 Property / fuzz inputs.
//
// This file exercises the validate.go functions with:
//
//  1. Random byte sequences via testing/quick, asserting the functions never
//     panic (TestFuzzValidate*NeverPanics).  A panic on arbitrary input would
//     be a latent crash waiting to happen in production.
//
//  2. Valid-by-construction inputs via the same fixture constants the
//     lifecycle tests use, asserting zero validation errors
//     (TestFuzzValid*Accepted).
//
//  3. Deliberately invalid inputs (cycles, dangling deps, empty JSON) that
//     must produce at least one error without panicking
//     (TestFuzzPlanJSON*Rejected, TestFuzzValidateNeverPanicsOnCornerInputs).
//
// The package uses the `integration` build tag, so these run alongside the
// other black-box tests:
//
//	go test -tags=integration ./integration/...
//
// Go native fuzzing (testing.F) is intentionally avoided here: fuzz tests
// in the same package as ordinary tests require extra build infrastructure
// and have no seed-corpus advantage for the byte-level properties tested.
// testing/quick provides equivalent random coverage without the ceremony.

import (
	"encoding/json"
	"testing"
	"testing/quick"

	"forgectl/state"
)

// --- Never-panic property tests ---

// TestFuzzValidateSpecQueueNeverPanics generates random byte sequences and
// runs them through state.ValidateSpecQueue.  The function must return a
// (possibly non-empty) []string on every input; it must never panic.
func TestFuzzValidateSpecQueueNeverPanics(t *testing.T) {
	f := func(data []byte) bool {
		defer func() { recover() }() // absorb panics so quick records the failure
		_ = state.ValidateSpecQueue(data)
		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Errorf("ValidateSpecQueue panicked on a random input: %v", err)
	}
}

// TestFuzzValidatePlanQueueNeverPanics is the plan-queue equivalent.
func TestFuzzValidatePlanQueueNeverPanics(t *testing.T) {
	f := func(data []byte) bool {
		defer func() { recover() }()
		_ = state.ValidatePlanQueue(data)
		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Errorf("ValidatePlanQueue panicked on a random input: %v", err)
	}
}

// TestFuzzValidatePlanJSONNeverPanics runs random byte sequences through
// state.ValidatePlanJSON with a fixed baseDir of ".".  The baseDir is used
// only for ref-file existence checks; missing files produce errors, not panics.
func TestFuzzValidatePlanJSONNeverPanics(t *testing.T) {
	f := func(data []byte) bool {
		defer func() { recover() }()
		_ = state.ValidatePlanJSON(data, ".")
		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Errorf("ValidatePlanJSON panicked on a random input: %v", err)
	}
}

// --- Valid-by-construction acceptance tests ---

// TestFuzzValidSpecQueueAccepted asserts that the canonical one-spec queue
// (the same constant used in the specifying lifecycle tests) passes validation
// with no errors.  This pins the schema contract: if validate.go tightens a
// rule that breaks valid inputs, this test fails before any lifecycle test does.
func TestFuzzValidSpecQueueAccepted(t *testing.T) {
	// oneSpecQueue is defined in lifecycle_specifying_test.go.
	errs := state.ValidateSpecQueue([]byte(oneSpecQueue))
	if len(errs) != 0 {
		t.Errorf("ValidateSpecQueue rejected a valid spec queue: %v", errs)
	}
}

// TestFuzzValidPlanQueueAccepted asserts that the three-domain plan queue used
// by the pipeline tests is accepted with no errors.
func TestFuzzValidPlanQueueAccepted(t *testing.T) {
	// threeDomainQueue is defined in pipeline_multidomain_test.go.
	errs := state.ValidatePlanQueue([]byte(threeDomainQueue))
	if len(errs) != 0 {
		t.Errorf("ValidatePlanQueue rejected a valid plan queue: %v", errs)
	}
}

// TestFuzzValidPlanJSONAccepted asserts that the two-layer code plan fixture
// (used by the pipeline tests) is accepted with no errors when resolved
// against "." — item refs are empty so the file-existence check is a no-op.
func TestFuzzValidPlanJSONAccepted(t *testing.T) {
	// twoLayerCodePlan is defined in pipeline_multidomain_test.go.
	errs := state.ValidatePlanJSON([]byte(twoLayerCodePlan("core", "Core")), ".")
	if len(errs) != 0 {
		t.Errorf("ValidatePlanJSON rejected a valid plan.json: %v", errs)
	}
}

// --- Deliberate-invalidity rejection tests ---

// TestFuzzPlanJSONWithCycleRejected constructs a plan.json whose two items
// form a direct dependency cycle (A → B, B → A) and asserts that
// ValidatePlanJSON returns at least one error.  Neither item should cause a
// panic during cycle detection (DFS over a two-node cycle).
func TestFuzzPlanJSONWithCycleRejected(t *testing.T) {
	plan := map[string]any{
		"context": map[string]any{"domain": "core", "module": "Core"},
		"layers": []any{
			map[string]any{"id": "L0", "name": "Foundation", "items": []any{"a", "b"}},
		},
		"items": []any{
			map[string]any{
				"id": "a", "name": "Item A", "description": "first",
				"depends_on": []any{"b"},
				"tests": []any{
					map[string]any{"category": "functional", "description": "works"},
				},
			},
			map[string]any{
				"id": "b", "name": "Item B", "description": "second",
				"depends_on": []any{"a"},
				"tests": []any{
					map[string]any{"category": "functional", "description": "works"},
				},
			},
		},
	}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshalling cyclic plan: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ValidatePlanJSON panicked on a cyclic plan: %v", r)
		}
	}()

	errs := state.ValidatePlanJSON(data, ".")
	if len(errs) == 0 {
		t.Error("ValidatePlanJSON accepted a plan with a dependency cycle; want at least one error")
	}
	for _, e := range errs {
		if e == "" {
			t.Error("ValidatePlanJSON returned an empty error string")
		}
	}
}

// TestFuzzPlanJSONDanglingDepsRejected constructs a plan.json where an item
// depends on a non-existent item ID and asserts that ValidatePlanJSON rejects
// it with at least one error.
func TestFuzzPlanJSONDanglingDepsRejected(t *testing.T) {
	plan := map[string]any{
		"context": map[string]any{"domain": "core", "module": "Core"},
		"layers": []any{
			map[string]any{"id": "L0", "name": "Foundation", "items": []any{"a"}},
		},
		"items": []any{
			map[string]any{
				"id": "a", "name": "Item A", "description": "first",
				"depends_on": []any{"nonexistent"},
				"tests": []any{
					map[string]any{"category": "functional", "description": "works"},
				},
			},
		},
	}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshalling plan with dangling dep: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("ValidatePlanJSON panicked on a plan with a dangling dep: %v", r)
		}
	}()

	errs := state.ValidatePlanJSON(data, ".")
	if len(errs) == 0 {
		t.Error("ValidatePlanJSON accepted a plan with a dangling dependency; want at least one error")
	}
}

// TestFuzzValidateNeverPanicsOnCornerInputs exercises a set of corner-case
// byte sequences — nil, empty, "null", "{}", "[]", and non-UTF-8 garbage —
// against every validate function.  The only assertion is no panic; these
// inputs are all invalid, so returning non-empty error slices is expected and
// acceptable.
func TestFuzzValidateNeverPanicsOnCornerInputs(t *testing.T) {
	inputs := []struct {
		name  string
		input []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"json-null", []byte("null")},
		{"empty-object", []byte("{}")},
		{"empty-array", []byte("[]")},
		{"non-utf8", []byte("\xff\xfe\x00")},
		{"truncated-json", []byte(`{"specs":[{"name"`)},
		{"array-at-root", []byte(`[1,2,3]`)},
	}

	funcs := []struct {
		name string
		fn   func([]byte) []string
	}{
		{"ValidateSpecQueue", state.ValidateSpecQueue},
		{"ValidatePlanQueue", state.ValidatePlanQueue},
		{"ValidatePlanJSON", func(b []byte) []string { return state.ValidatePlanJSON(b, ".") }},
	}

	for _, fn := range funcs {
		for _, inp := range inputs {
			fn, inp := fn, inp
			t.Run(fn.name+"/"+inp.name, func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("panicked on input %q: %v", inp.input, r)
					}
				}()
				_ = fn.fn(inp.input)
			})
		}
	}
}
