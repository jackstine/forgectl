//go:build integration

package integration

import (
	"strings"
	"testing"
)

// This file is coverage item §4.2: a hand-authored corpus of input artifacts —
// the kind an agent or operator would produce — written to the spec, never
// emitted by forgectl. Each fixture is run through the real `forgectl validate`
// command. Because the corpus is independent of forgectl's own writer, this
// tests the reader against the documented contract, not against itself.
//
// For every artifact there is one valid variant forgectl must accept and a set
// of one-violation-each invalid variants it must reject, each with the exact
// documented signal. The fixtures are inline string constants (rather than
// testdata/ files) so each violation sits next to the behavior it probes; the
// independence property — not produced by forgectl — is what matters, and inline
// hand-authored JSON has it.

// --- spec-queue corpus ---

const specQueueValid = `{
  "specs": [
    {"name":"Alpha","domain":"core","topic":"alpha topic","file":"core/specs/alpha.md","planning_sources":["docs/a.md"],"depends_on":[]},
    {"name":"Beta","domain":"core","topic":"beta topic","file":"core/specs/beta.md","planning_sources":[],"depends_on":["Alpha"]}
  ]
}`

// missing the required "topic" field on specs[0].
const specQueueMissingField = `{
  "specs": [
    {"name":"Alpha","domain":"core","file":"core/specs/alpha.md","planning_sources":[],"depends_on":[]}
  ]
}`

// an unexpected field "priority" on specs[0].
const specQueueExtraField = `{
  "specs": [
    {"name":"Alpha","domain":"core","topic":"t","file":"core/specs/alpha.md","planning_sources":[],"depends_on":[],"priority":1}
  ]
}`

const specQueueEmptyArray = `{"specs": []}`

// --- plan-queue corpus ---

const planQueueValid = `{
  "plans": [
    {"name":"Core Plan","domain":"core","file":"core/plan.json","specs":["core/specs/a.md"],"spec_commits":["abc1234"],"code_search_roots":["core/"]}
  ]
}`

const planQueueKindUI = `{
  "plans": [
    {"name":"Portal Plan","domain":"portal","file":"portal/plan.json","specs":[],"spec_commits":[],"code_search_roots":["portal/"],"kind":"ui"}
  ]
}`

const planQueueKindCode = `{
  "plans": [
    {"name":"Core Plan","domain":"core","file":"core/plan.json","specs":[],"spec_commits":[],"code_search_roots":["core/"],"kind":"code"}
  ]
}`

// kind set to a value outside {code, ui}.
const planQueueBadKind = `{
  "plans": [
    {"name":"X","domain":"core","file":"core/plan.json","specs":[],"spec_commits":[],"code_search_roots":["core/"],"kind":"frontend"}
  ]
}`

// missing the required "code_search_roots" field.
const planQueueMissingField = `{
  "plans": [
    {"name":"X","domain":"core","file":"core/plan.json","specs":[],"spec_commits":[]}
  ]
}`

// --- plan.json corpus ---

const planValid = `{
  "context": {"domain":"core","module":"Core"},
  "layers": [
    {"id":"L0","name":"Foundation","items":["a"]},
    {"id":"L1","name":"Wiring","items":["b"]}
  ],
  "items": [
    {"id":"a","name":"Item A","description":"first","depends_on":[],"tests":[{"category":"functional","description":"works","passes":false}]},
    {"id":"b","name":"Item B","description":"second","depends_on":["a"],"tests":[{"category":"rejection","description":"rejects bad input","passes":false}]}
  ]
}`

// b depends on a non-existent item "ghost".
const planDanglingDep = `{
  "context": {"domain":"core","module":"Core"},
  "layers": [{"id":"L0","name":"L0","items":["a","b"]}],
  "items": [
    {"id":"a","name":"A","description":"d","depends_on":[],"tests":[]},
    {"id":"b","name":"B","description":"d","depends_on":["ghost"],"tests":[]}
  ]
}`

// a→b→a cycle.
const planCycle = `{
  "context": {"domain":"core","module":"Core"},
  "layers": [{"id":"L0","name":"L0","items":["a","b"]}],
  "items": [
    {"id":"a","name":"A","description":"d","depends_on":["b"],"tests":[]},
    {"id":"b","name":"B","description":"d","depends_on":["a"],"tests":[]}
  ]
}`

// item in layer L0 depends on item in the later layer L1.
const planLaterLayerDep = `{
  "context": {"domain":"core","module":"Core"},
  "layers": [
    {"id":"L0","name":"L0","items":["a"]},
    {"id":"L1","name":"L1","items":["b"]}
  ],
  "items": [
    {"id":"a","name":"A","description":"d","depends_on":["b"],"tests":[]},
    {"id":"b","name":"B","description":"d","depends_on":[],"tests":[]}
  ]
}`

// a test with a category outside {functional, rejection, edge_case}.
const planBadTestCategory = `{
  "context": {"domain":"core","module":"Core"},
  "layers": [{"id":"L0","name":"L0","items":["a"]}],
  "items": [
    {"id":"a","name":"A","description":"d","depends_on":[],"tests":[{"category":"smoke","description":"x","passes":false}]}
  ]
}`

// item "b" exists but is not listed in any layer.
const planItemNoLayer = `{
  "context": {"domain":"core","module":"Core"},
  "layers": [{"id":"L0","name":"L0","items":["a"]}],
  "items": [
    {"id":"a","name":"A","description":"d","depends_on":[],"tests":[]},
    {"id":"b","name":"B","description":"d","depends_on":[],"tests":[]}
  ]
}`

// context.module missing.
const planMissingModule = `{
  "context": {"domain":"core"},
  "layers": [{"id":"L0","name":"L0","items":["a"]}],
  "items": [
    {"id":"a","name":"A","description":"d","depends_on":[],"tests":[]}
  ]
}`

// validateCase is one corpus fixture and its expected outcome through `validate`.
type validateCase struct {
	name     string
	file     string // filename to write under the project root
	content  string
	wantExit int    // 0 accept, 1 reject
	wantSub  string // substring required somewhere in the output when rejecting
}

func runValidateCases(t *testing.T, cases []validateCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := NewProject(t)
			p.WriteFile(tc.file, tc.content)
			res := p.forge("validate", tc.file)
			if res.Exit != tc.wantExit {
				t.Fatalf("validate %s: exit = %d, want %d\nstdout:\n%s\nstderr:\n%s",
					tc.file, res.Exit, tc.wantExit, res.Stdout, res.Stderr)
			}
			if tc.wantExit != 0 {
				// Failures must exit to stderr per K4; the human-readable detail
				// is on stdout. Require the documented cue anywhere in the output.
				if !strings.Contains(res.Out(), tc.wantSub) {
					t.Errorf("validate %s: output missing %q\n%s", tc.file, tc.wantSub, res.Out())
				}
				if strings.TrimSpace(res.Stderr) == "" {
					t.Errorf("validate %s: rejection wrote nothing to stderr", tc.file)
				}
			}
		})
	}
}

// TestValidateCorpusSpecQueue runs the spec-queue corpus through validate (§4.2).
func TestValidateCorpusSpecQueue(t *testing.T) {
	runValidateCases(t, []validateCase{
		{"valid", "sq.json", specQueueValid, 0, ""},
		{"missing_field", "sq.json", specQueueMissingField, 1, `missing required field "topic"`},
		{"extra_field", "sq.json", specQueueExtraField, 1, `unexpected field "priority"`},
		{"empty_array", "sq.json", specQueueEmptyArray, 1, `"specs" array must not be empty`},
	})
}

// TestValidateCorpusPlanQueue runs the plan-queue corpus through validate (§4.2),
// including the kind routing field's accept/reject set.
func TestValidateCorpusPlanQueue(t *testing.T) {
	runValidateCases(t, []validateCase{
		{"valid", "pq.json", planQueueValid, 0, ""},
		{"kind_ui", "pq.json", planQueueKindUI, 0, ""},
		{"kind_code", "pq.json", planQueueKindCode, 0, ""},
		{"bad_kind", "pq.json", planQueueBadKind, 1, `invalid kind "frontend"`},
		{"missing_field", "pq.json", planQueueMissingField, 1, `missing required field "code_search_roots"`},
	})
}

// TestValidateCorpusPlan runs the plan.json corpus through validate (§4.2),
// hitting each graph/layer/test-schema rule with a single-violation fixture.
func TestValidateCorpusPlan(t *testing.T) {
	runValidateCases(t, []validateCase{
		{"valid", "plan.json", planValid, 0, ""},
		{"dangling_dep", "plan.json", planDanglingDep, 1, `depends on non-existent item "ghost"`},
		{"cycle", "plan.json", planCycle, 1, "dependency cycle detected"},
		{"later_layer_dep", "plan.json", planLaterLayerDep, 1, "which is a later layer"},
		{"bad_test_category", "plan.json", planBadTestCategory, 1, "invalid category"},
		{"item_no_layer", "plan.json", planItemNoLayer, 1, `item "b" not assigned to any layer`},
		{"missing_module", "plan.json", planMissingModule, 1, "context.module must be a non-empty string"},
	})
}

// TestValidateSurface covers the standalone validate CLI surface (§K1): type
// auto-detection by top-level key, the --type mismatch hint, and the
// undetectable / not-found error paths — all without a .forgectl/ present.
func TestValidateSurface(t *testing.T) {
	t.Run("autodetect_announces_type", func(t *testing.T) {
		p := NewProject(t)
		p.WriteFile("sq.json", specQueueValid)
		res := p.mustForge("validate", "sq.json")
		if !strings.Contains(res.Stdout, `Detected: spec-queue (top-level key: "specs")`) {
			t.Errorf("auto-detect banner missing:\n%s", res.Stdout)
		}
	})

	t.Run("type_mismatch_hints", func(t *testing.T) {
		p := NewProject(t)
		p.WriteFile("pq.json", planQueueValid) // top-level "plans"
		res := p.forge("validate", "--type", "spec-queue", "pq.json")
		if res.Exit == 0 {
			t.Fatal("type mismatch should fail")
		}
		if !strings.Contains(res.Out(), "did you mean --type plan-queue") {
			t.Errorf("missing did-you-mean hint:\n%s", res.Out())
		}
	})

	t.Run("undetectable_type", func(t *testing.T) {
		p := NewProject(t)
		p.WriteFile("x.json", `{"unknown": 1}`)
		res := p.forge("validate", "x.json")
		if res.Exit == 0 {
			t.Fatal("undetectable type should fail")
		}
		if !strings.Contains(res.Out(), "cannot detect file type") {
			t.Errorf("missing cannot-detect message:\n%s", res.Out())
		}
	})

	t.Run("file_not_found", func(t *testing.T) {
		p := NewProject(t)
		res := p.forge("validate", "nope.json")
		if res.Exit == 0 {
			t.Fatal("missing file should fail")
		}
		if !strings.Contains(res.Out(), "file not found") {
			t.Errorf("missing file-not-found message:\n%s", res.Out())
		}
	})
}
