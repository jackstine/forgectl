package cmd

import (
	"fmt"
	"os"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var handoffCmd = &cobra.Command{
	Use:   "handoff <file> [<file>...]",
	Short: "Register sub-agent artifacts for the current evaluator round",
	Long: "The sub-agent's artifact-return command — the counterpart to eval. " +
		"Registers each generated file (QA report, QA step list, e2e report) so the " +
		"scaffold surfaces them on the Review: line for engineer review before the verdict. " +
		"Valid only in ui_implementing EVALUATE, QA_TEST, and E2E_VERIFY.",
	Args: cobra.MinimumNArgs(1),
	RunE: runHandoff,
}

func init() {
	rootCmd.AddCommand(handoffCmd)
}

func runHandoff(cmd *cobra.Command, args []string) error {
	_, stateDir, _, err := resolveSession()
	if err != nil {
		return err
	}
	s, err := state.Load(stateDir)
	if err != nil {
		return err
	}

	// State-gated: hand-off is valid only in the three ui_implementing evaluator
	// states — the same states as eval (invariant 13).
	validStates := map[state.StateName]bool{
		state.StateEvaluate:  true,
		state.StateQATest:    true,
		state.StateE2EVerify: true,
	}
	if s.Phase != state.PhaseUIImplementing || !validStates[s.State] {
		return fmt.Errorf("forgectl handoff is only valid in ui_implementing EVALUATE, QA_TEST, or E2E_VERIFY (current: %s %s)", s.Phase, s.State)
	}

	if s.UIImplementing == nil || s.UIImplementing.CurrentBatch == nil {
		return fmt.Errorf("no active batch to register artifacts for")
	}

	// Verify every named file exists before registering any — a missing file
	// rejects the whole hand-off and registers nothing.
	for _, path := range args {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("file not found: %s", path)
		}
	}

	// Latest hand-off wins — replace any artifacts registered earlier in this round.
	s.UIImplementing.CurrentBatch.HandedOffArtifacts = args

	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "Registered %d artifact(s) for review:\n", len(args))
	for _, path := range args {
		fmt.Fprintf(out, "  %s\n", path)
	}
	return nil
}
