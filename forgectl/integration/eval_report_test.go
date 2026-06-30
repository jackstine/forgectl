//go:build integration

package integration

import (
	"regexp"
	"strings"
	"testing"

	"forgectl/state"
)

// --- Configs ---

// reportModeConfig is the specifying config for report eval mode (the default).
const reportModeConfig = `
[specifying]
batch = 1
[specifying.eval]
min_rounds = 1
max_rounds = 3
eval_mode = "report"
[general]
user_guided = false
enable_commits = false
`

// directModeConfig is the specifying config for direct eval mode.
const directModeConfig = `
[specifying]
batch = 1
[specifying.eval]
min_rounds = 1
max_rounds = 3
eval_mode = "direct"
[general]
user_guided = false
enable_commits = false
`

// conversationalModeConfig is the specifying config for conversational eval mode.
const conversationalModeConfig = `
[specifying]
batch = 1
[specifying.eval]
min_rounds = 1
max_rounds = 3
eval_mode = "conversational"
[general]
user_guided = false
enable_commits = false
`

// --- Helpers ---

// driveToEvaluate drives the project from init to EVALUATE in the specifying
// phase using oneSpecQueue. Returns the Result of the DRAFT → EVALUATE advance
// step, which contains the Action block with the report path.
func driveToEvaluate(p *Project) Result {
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	p.mustForge("advance") // ORIENT → SELECT
	p.mustForge("advance") // SELECT → DRAFT
	p.WriteFile("specs/a.md", "# Spec A\n")
	return p.mustForge("advance") // DRAFT → EVALUATE
}

// evalReportOutputPathRe extracts the eval-report file path from the
// "--- REPORT OUTPUT ---" block printed by `forgectl eval` in report mode.
//
// The relevant lines in the block look like:
//
//	Write your evaluation report to this exact path (create the file — do not only
//	describe it):
//	  core/specs/.eval/batch-1-r1.md
var evalReportOutputPathRe = regexp.MustCompile(`describe it\):\n\s+(\S+)`)

// --- I1: Deterministic report path ---

// TestI1DeterministicReportPath verifies that the report file path shown in the
// Action block (emitted by advance on entering EVALUATE) is byte-identical to
// the path shown in the `--- REPORT OUTPUT ---` block emitted by `forgectl eval`.
//
// Both code paths call specEvalReportPath, so any drift between them would
// surface here.
func TestI1DeterministicReportPath(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(reportModeConfig)
	p.WriteFile("spec-queue.json", oneSpecQueue)

	// Drive to EVALUATE. advResult carries the DRAFT→EVALUATE advance output,
	// which includes the Action block with the report path.
	advResult := driveToEvaluate(p)
	p.AssertAt(state.PhaseSpecifying, state.StateEvaluate)

	// Extract the path from the Action block (printed by advance when entering
	// EVALUATE): "The sub-agent must write its report to this exact path:\n  <path>"
	actionPath := p.ExtractReportPath(advResult.Out())

	// Run `forgectl eval` — prints the full eval context including the
	// --- REPORT OUTPUT --- block with the same deterministic path.
	evalResult := p.forge("eval")
	if evalResult.Exit != 0 {
		t.Fatalf("forgectl eval exit=%d\n%s", evalResult.Exit, evalResult.Out())
	}

	// Extract the path from the REPORT OUTPUT block in the eval output.
	m := evalReportOutputPathRe.FindStringSubmatch(evalResult.Out())
	if m == nil {
		t.Fatalf("no report path found in --- REPORT OUTPUT --- block of eval output:\n%s", evalResult.Out())
	}
	reportOutputPath := m[1]

	if actionPath != reportOutputPath {
		t.Errorf("report path mismatch between Action block and REPORT OUTPUT block:\n  Action block:        %q\n  REPORT OUTPUT block: %q",
			actionPath, reportOutputPath)
	}
}

// --- I2: --eval-report path validation ---

// TestI2EvalReportPathValidation exercises the three --eval-report validation
// cases while in EVALUATE state:
//  1. Nonexistent path → non-zero exit.
//  2. Prose-looking value (no path separator, has whitespace) → non-zero exit
//     with an error hinting that --eval-report expects a file path.
//  3. Valid file path → exit 0, EvalReport recorded in state.
func TestI2EvalReportPathValidation(t *testing.T) {
	p := NewProject(t)
	p.WriteConfig(reportModeConfig)
	p.WriteFile("spec-queue.json", oneSpecQueue)

	advResult := driveToEvaluate(p)
	p.AssertAt(state.PhaseSpecifying, state.StateEvaluate)

	// --- Case 1: nonexistent path → non-zero exit. ---
	res1 := p.forge("advance", "--verdict", "PASS", "--eval-report", "/nonexistent/path.md")
	if res1.Exit == 0 {
		t.Errorf("case 1 (nonexistent path): expected non-zero exit, got 0\n%s", res1.Out())
	}
	p.AssertAt(state.PhaseSpecifying, state.StateEvaluate) // state unchanged

	// --- Case 2: prose-looking value → non-zero exit with path hint. ---
	// "some prose text" has whitespace and no path separator — looksLikeReportProse
	// returns true, so the error explains --eval-report expects a file path.
	res2 := p.forge("advance", "--verdict", "PASS", "--eval-report", "some prose text")
	if res2.Exit == 0 {
		t.Errorf("case 2 (prose value): expected non-zero exit, got 0\n%s", res2.Out())
	}
	if !strings.Contains(res2.Out(), "path") {
		t.Errorf("case 2 (prose value): error output missing 'path' hint:\n%s", res2.Out())
	}
	p.AssertAt(state.PhaseSpecifying, state.StateEvaluate) // state unchanged

	// --- Case 3: valid file → exit 0, EvalReport recorded in state. ---
	// Use the report path extracted from the EVALUATE advance output so the stub
	// lands where forgectl expects it.
	reportPath := p.ExtractReportPath(advResult.Out())
	p.WriteReport(reportPath)

	res3 := p.forge("advance", "--verdict", "PASS", "--eval-report", reportPath)
	if res3.Exit != 0 {
		t.Fatalf("case 3 (valid path): expected exit 0, got %d\n%s", res3.Exit, res3.Out())
	}
	p.AssertAt(state.PhaseSpecifying, state.StateAccept)

	// At ACCEPT, CurrentSpecs still holds the accepted spec with its eval record.
	evals := p.State().Specifying.CurrentSpecs[0].Evals
	if len(evals) == 0 {
		t.Fatal("case 3: no EvalRecords found in CurrentSpecs[0].Evals after advance to ACCEPT")
	}
	if evals[0].EvalReport != reportPath {
		t.Errorf("case 3: EvalRecord.EvalReport = %q, want %q", evals[0].EvalReport, reportPath)
	}
}

// --- I3: Non-report modes don't surface a report path ---

// TestI3NonReportModeNoReportOutput asserts that in non-report eval modes
// (conversational and direct):
//   - No eval-report file path is surfaced in the advance EVALUATE output.
//   - `advance --verdict PASS` without --eval-report exits 0.
//
// For conversational mode specifically:
//   - The `forgectl eval` output contains no `--- REPORT OUTPUT ---` block at all.
//
// For direct mode:
//   - A `--- REPORT OUTPUT ---` block may appear in `forgectl eval` output (it
//     instructs direct corrections), but it must not contain a file path.
func TestI3NonReportModeNoReportOutput(t *testing.T) {
	t.Run("conversational", func(t *testing.T) {
		p := NewProject(t)
		p.WriteConfig(conversationalModeConfig)
		p.WriteFile("spec-queue.json", oneSpecQueue)

		advResult := driveToEvaluate(p)
		p.AssertAt(state.PhaseSpecifying, state.StateEvaluate)

		// No file path pattern in the advance output.
		if reportPathRe.MatchString(advResult.Out()) {
			t.Errorf("conversational mode: unexpected report path in advance output:\n%s", advResult.Out())
		}

		// `forgectl eval` output must not contain a REPORT OUTPUT block.
		evalResult := p.forge("eval")
		if evalResult.Exit != 0 {
			t.Fatalf("forgectl eval exit=%d\n%s", evalResult.Exit, evalResult.Out())
		}
		if strings.Contains(evalResult.Out(), "--- REPORT OUTPUT ---") {
			t.Errorf("conversational mode: unexpected --- REPORT OUTPUT --- block in eval output:\n%s", evalResult.Out())
		}
		// No file path pattern in the eval output either.
		if evalReportOutputPathRe.MatchString(evalResult.Out()) {
			t.Errorf("conversational mode: unexpected report file path in eval output:\n%s", evalResult.Out())
		}

		// advance --verdict PASS (no --eval-report) must succeed.
		res := p.forge("advance", "--verdict", "PASS")
		if res.Exit != 0 {
			t.Errorf("conversational mode: advance --verdict PASS failed: exit=%d\n%s", res.Exit, res.Out())
		}
		p.AssertAt(state.PhaseSpecifying, state.StateAccept)
	})

	t.Run("direct", func(t *testing.T) {
		p := NewProject(t)
		p.WriteConfig(directModeConfig)
		p.WriteFile("spec-queue.json", oneSpecQueue)

		advResult := driveToEvaluate(p)
		p.AssertAt(state.PhaseSpecifying, state.StateEvaluate)

		// No file path pattern in the advance output — direct mode does not surface
		// a file the engineer must pass to --eval-report.
		if reportPathRe.MatchString(advResult.Out()) {
			t.Errorf("direct mode: unexpected report path in advance output:\n%s", advResult.Out())
		}

		// `forgectl eval` output may contain --- REPORT OUTPUT --- in direct mode
		// (it instructs direct corrections to the spec files), but must not contain
		// a file path that looks like an eval report destination.
		evalResult := p.forge("eval")
		if evalResult.Exit != 0 {
			t.Fatalf("forgectl eval exit=%d\n%s", evalResult.Exit, evalResult.Out())
		}
		if evalReportOutputPathRe.MatchString(evalResult.Out()) {
			t.Errorf("direct mode: unexpected report file path in eval output:\n%s", evalResult.Out())
		}

		// advance --verdict PASS (no --eval-report) must succeed.
		res := p.forge("advance", "--verdict", "PASS")
		if res.Exit != 0 {
			t.Errorf("direct mode: advance --verdict PASS failed: exit=%d\n%s", res.Exit, res.Out())
		}
		p.AssertAt(state.PhaseSpecifying, state.StateAccept)
	})
}
