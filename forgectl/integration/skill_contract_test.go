//go:build integration

package integration

// §4.6 Skill/binary contract tests.
//
// This file verifies two orthogonal contracts:
//
//  1. Every `forgectl <subcommand>` that appears in a skill Markdown file
//     exists in the built binary (TestSkillContractSubcommands). If a skill
//     documents a command that was renamed or removed, this test fails.
//
//  2. The output cue strings that skills instruct sub-agents to read
//     (Action:, Round:, --- REPORT OUTPUT ---) actually appear in the
//     binary's output at the states the skills reference them
//     (TestSkillContractOutputCues).
//
// Neither test couples to internal types: subcommands are extracted from
// the Markdown sources and probed with --help; cue strings are sought in
// the raw CLI output captured by the harness.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"forgectl/state"
)

// skillsRel is the path to the skills/ directory relative to this package.
// Go test sets the working directory to the package dir (forgectl/integration/),
// so two levels up reaches the project root where skills/ lives.
const skillsRel = "../../skills"

// forgectlSubcmdRe matches the first token after "forgectl " inside backtick
// invocations such as:
//
//	`forgectl advance --verdict ...`
//	`forgectl init --phase specifying`
//	`forgectl set-roots <path>`
//
// The capture group is the subcommand name, which may contain hyphens
// (e.g. "add-queue-item", "set-roots").  Placeholder tokens like <command>
// start with "<" which is not a \w character, so they never match.
var forgectlSubcmdRe = regexp.MustCompile("`forgectl\\s+(\\w[\\w-]*)(?:[\\s`])")

// TestSkillContractSubcommands walks every Markdown file under skills/,
// extracts each `forgectl <subcommand>` invocation, and asserts that the
// subcommand is present in the built binary.
//
// Detection is done by running `forgectl <subcommand> --help`.  Cobra always
// processes --help before calling RunE, so this never requires a live session
// or a config file.  An unknown subcommand causes cobra to exit non-zero with
// "unknown command" on stderr; a known command exits 0 regardless of state.
func TestSkillContractSubcommands(t *testing.T) {
	// Collect all Markdown files under skills/.
	var mdFiles []string
	err := filepath.Walk(skillsRel, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".md") {
			mdFiles = append(mdFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking skills dir %q: %v", skillsRel, err)
	}
	if len(mdFiles) == 0 {
		t.Fatalf("no .md files found under %q — check that skillsRel is correct", skillsRel)
	}

	// Extract unique subcommands, recording the first file that mentions each.
	subcmds := map[string]string{} // subcommand → first source path
	for _, path := range mdFiles {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		for _, m := range forgectlSubcmdRe.FindAllSubmatch(data, -1) {
			sub := string(m[1])
			if sub == "" {
				continue
			}
			if _, seen := subcmds[sub]; !seen {
				subcmds[sub] = path
			}
		}
	}

	if len(subcmds) == 0 {
		t.Fatalf("no `forgectl <subcommand>` invocations found in %d skill files — regex may be wrong", len(mdFiles))
	}
	t.Logf("found %d unique subcommands across %d skill files", len(subcmds), len(mdFiles))

	// All --help invocations share one project; --help never reads or writes
	// session state, so the project needs no config or init.
	p := NewProject(t)

	for sub, origin := range subcmds {
		sub, origin := sub, origin
		t.Run(sub, func(t *testing.T) {
			t.Parallel()
			res := p.forge(sub, "--help")
			if res.Exit != 0 {
				t.Errorf("forgectl %s --help exited %d — subcommand may be missing from the binary\n"+
					"first referenced in: %s\nstdout:\n%s\nstderr:\n%s",
					sub, res.Exit, filepath.Base(origin), res.Stdout, res.Stderr)
			}
		})
	}
}

// TestSkillContractOutputCues drives forgectl through the specifying phase to
// the EVALUATE state and asserts that the output cue strings the skills
// document as landmarks for sub-agents actually appear in the binary output.
//
// Cues checked:
//
//   - "Phase:"          — present in `forgectl status` (any state)
//   - "Action:"         — present in every state's action block
//   - "Round:"          — present in EVALUATE-family output
//   - "--- REPORT OUTPUT ---" — present in `forgectl eval` output at EVALUATE
//     (the default eval_mode is "report", so the section is always rendered)
func TestSkillContractOutputCues(t *testing.T) {
	p := NewProject(t)
	// specifyingConfig (from lifecycle_specifying_test.go): batch=1, rounds 1–3,
	// commits on, unguided.  The default eval_mode stays "report" because the TOML
	// does not override it; DefaultForgeConfig() initialises it to "report".
	p.WriteConfig(specifyingConfig)
	// oneSpecQueue (from lifecycle_specifying_test.go): one spec in domain "core".
	p.WriteFile("spec-queue.json", oneSpecQueue)

	// --- ORIENT: init ---
	// ORIENT is a transient setup state in specifying — it emits no Action: block
	// (there is nothing for the agent to do; the next advance moves to SELECT).
	// What status must always emit is "Phase:" in its header.
	p.mustForge("init", "--phase", "specifying", "--from", "spec-queue.json")
	p.AssertAt(state.PhaseSpecifying, state.StateOrient)

	orientStatus := p.mustForge("status")
	if !strings.Contains(orientStatus.Out(), "Phase:") {
		t.Errorf("forgectl status at ORIENT: 'Phase:' cue absent\n%s", orientStatus.Out())
	}

	// --- SELECT: check Action: ---
	// Skills say "run `forgectl status` to see what action is needed next"; every
	// non-transient state must emit "Action:" to fulfil that contract.
	selectTransition := p.mustForge("advance") // ORIENT → SELECT
	p.AssertAt(state.PhaseSpecifying, state.StateSelect)
	if !strings.Contains(selectTransition.Out(), "Action:") {
		t.Errorf("ORIENT→SELECT transition: 'Action:' cue absent\n%s", selectTransition.Out())
	}
	selectStatus := p.mustForge("status")
	if !strings.Contains(selectStatus.Out(), "Action:") {
		t.Errorf("forgectl status at SELECT: 'Action:' cue absent\n%s", selectStatus.Out())
	}

	// --- EVALUATE: advance through DRAFT ---
	p.mustForge("advance") // SELECT → DRAFT
	// Write the draft spec that the DRAFT state expects before advancing out.
	p.WriteFile("core/specs/a.md", "# Spec A\n")
	evalTransition := p.mustForge("advance") // DRAFT → EVALUATE
	p.AssertAt(state.PhaseSpecifying, state.StateEvaluate)

	// The advance output that transitions into EVALUATE must include "Round:" so
	// the human operator and the sub-agent can read the current round number.
	if !strings.Contains(evalTransition.Out(), "Round:") {
		t.Errorf("EVALUATE transition output: 'Round:' cue absent\n%s", evalTransition.Out())
	}
	// "Action:" must be present so the sub-agent knows what to do next.
	if !strings.Contains(evalTransition.Out(), "Action:") {
		t.Errorf("EVALUATE transition output: 'Action:' cue absent\n%s", evalTransition.Out())
	}

	// --- forgectl eval at EVALUATE ---
	// Skills instruct the evaluation sub-agent to run `forgectl eval` to get the
	// full evaluation context.  That output must include "--- REPORT OUTPUT ---"
	// so the sub-agent knows where to write its report.  The default eval_mode
	// is "report", so this section is rendered even without explicit config.
	evalOut := p.mustForge("eval")
	if !strings.Contains(evalOut.Out(), "--- REPORT OUTPUT ---") {
		t.Errorf("forgectl eval at EVALUATE: '--- REPORT OUTPUT ---' cue absent\n%s", evalOut.Out())
	}

	// --- forgectl status at EVALUATE ---
	// After the transition, a bare `forgectl status` must still surface "Round:"
	// and "Action:" because status embeds the same action block as advance.
	evalStatus := p.mustForge("status")
	if !strings.Contains(evalStatus.Out(), "Round:") {
		t.Errorf("forgectl status at EVALUATE: 'Round:' cue absent\n%s", evalStatus.Out())
	}
	if !strings.Contains(evalStatus.Out(), "Action:") {
		t.Errorf("forgectl status at EVALUATE: 'Action:' cue absent\n%s", evalStatus.Out())
	}
}
