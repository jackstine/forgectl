package evaluators

import (
	"strings"
	"testing"
)

// Functional: the UI evaluator prompts are embedded at build time and carry
// their expected H1 headings. A wrong //go:embed filename fails the build; this
// test guards against an embed pointing at the wrong (but existing) file.
func TestUIEvaluatorPromptsEmbedded(t *testing.T) {
	cases := []struct {
		name    string
		content string
		heading string
	}{
		{"UIQAEval", UIQAEval, "# UI QA Evaluation Prompt"},
		{"UIE2EEval", UIE2EEval, "# UI E2E Verification Prompt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if strings.TrimSpace(tc.content) == "" {
				t.Fatalf("%s is empty", tc.name)
			}
			if !strings.Contains(tc.content, tc.heading) {
				t.Errorf("%s missing heading %q", tc.name, tc.heading)
			}
		})
	}
}

// Functional: GauntletEval is embedded and non-empty at runtime, and its content
// carries the adversarial-mutating contract — review the code, edit it directly,
// and emit no verdict word — that generated workflow scripts bake into the
// evaluator agent's instruction.
func TestGauntletEvalEmbedded(t *testing.T) {
	if strings.TrimSpace(GauntletEval) == "" {
		t.Fatal("GauntletEval is empty; the embedded prompt is not accessible at runtime")
	}

	lower := strings.ToLower(GauntletEval)

	// It must instruct adversarial review that mutates the code directly.
	for _, want := range []string{"adversarial", "edit"} {
		if !strings.Contains(lower, want) {
			t.Errorf("GauntletEval missing adversarial/mutating instruction: expected to mention %q", want)
		}
	}

	// It must forbid emitting a verdict word — convergence is detected via git, not
	// a spoken PASS/FAIL.
	if !strings.Contains(lower, "verdict") {
		t.Error("GauntletEval must address verdicts (it should forbid emitting one)")
	}
	if !strings.Contains(GauntletEval, "PASS") || !strings.Contains(GauntletEval, "FAIL") {
		t.Error("GauntletEval should explicitly name PASS/FAIL as the verdict words it must not emit")
	}
}
