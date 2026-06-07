package cmd

import (
	"fmt"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var evalCmd = &cobra.Command{
	Use:   "eval",
	Short: "Output full evaluation context for the sub-agent",
	Long:  "Only valid in EVALUATE, RECONCILE_EVAL, and CROSS_REFERENCE_EVAL states.",
	RunE:  runEval,
}

func init() {
	rootCmd.AddCommand(evalCmd)
}

func runEval(cmd *cobra.Command, args []string) error {
	projectRoot, stateDir, _, err := resolveSession()
	if err != nil {
		return err
	}
	s, err := state.Load(stateDir)
	if err != nil {
		return err
	}

	switch {
	case s.Phase == state.PhaseSpecifying && s.State == state.StateReconcileEval:
		return state.PrintReconcileEvalOutput(cmd.OutOrStdout(), s)
	case s.Phase == state.PhaseSpecifying && s.State == state.StateCrossReferenceEval:
		return state.PrintCrossRefEvalOutput(cmd.OutOrStdout(), s)
	case s.Phase == state.PhaseSpecifying && s.State == state.StateEvaluate:
		return state.PrintSpecEvalOutput(cmd.OutOrStdout(), s, projectRoot)
	case s.Phase == state.PhasePlanning || s.Phase == state.PhaseImplementing:
		return state.PrintEvalOutput(cmd.OutOrStdout(), s, projectRoot)
	case s.Phase == state.PhaseReverseEngineering && s.State == state.StateReconcileEval:
		return state.PrintReverseEngineeringEvalOutput(cmd.OutOrStdout(), s)
	case s.Phase == state.PhaseReverseEngineering:
		return fmt.Errorf("forgectl eval is only available during RECONCILE_EVAL.")
	case s.Phase == state.PhaseUIImplementing && s.State == state.StateEvaluate:
		return state.PrintEvalOutput(cmd.OutOrStdout(), s, projectRoot)
	case s.Phase == state.PhaseUIImplementing && s.State == state.StateQATest:
		return state.PrintUIQAEvalOutput(cmd.OutOrStdout(), s, projectRoot)
	case s.Phase == state.PhaseUIImplementing && s.State == state.StateE2EVerify:
		return state.PrintUIE2EEvalOutput(cmd.OutOrStdout(), s, projectRoot)
	case s.Phase == state.PhaseUIImplementing:
		return fmt.Errorf("eval is only valid in ui_implementing EVALUATE, QA_TEST, or E2E_VERIFY (current: %s %s)", s.Phase, s.State)
	default:
		return fmt.Errorf("eval is only valid in EVALUATE, RECONCILE_EVAL, or CROSS_REFERENCE_EVAL state (current: %s)", s.State)
	}
}
