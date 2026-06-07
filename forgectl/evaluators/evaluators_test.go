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
