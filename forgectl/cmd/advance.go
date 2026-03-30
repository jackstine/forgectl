package cmd

import (
	"fmt"

	"forgectl/state"

	"github.com/spf13/cobra"
)

var (
	advanceVerdict    string
	advanceEvalReport string
	advanceMessage    string
	advanceFile       string
	advanceFrom       string
	advanceGuided     bool
	advanceNoGuided   bool
)

var advanceCmd = &cobra.Command{
	Use:   "advance",
	Short: "Transition from current state to next",
	RunE:  runAdvance,
}

func init() {
	advanceCmd.Flags().StringVar(&advanceVerdict, "verdict", "", "PASS or FAIL")
	advanceCmd.Flags().StringVar(&advanceEvalReport, "eval-report", "", "Path to evaluation report")
	advanceCmd.Flags().StringVar(&advanceMessage, "message", "", "Commit message or acceptance message")
	advanceCmd.Flags().StringVar(&advanceFile, "file", "", "Override file path (specifying DRAFT)")
	advanceCmd.Flags().StringVar(&advanceFrom, "from", "", "Path to queue input file (phase shift)")
	advanceCmd.Flags().BoolVar(&advanceGuided, "guided", false, "Enable guided mode")
	advanceCmd.Flags().BoolVar(&advanceNoGuided, "no-guided", false, "Disable guided mode")
	rootCmd.AddCommand(advanceCmd)
}

func runAdvance(cmd *cobra.Command, args []string) error {
	projectRoot, stateDir, err := resolveSession()
	if err != nil {
		return err
	}
	s, err := state.Load(stateDir)
	if err != nil {
		return err
	}

	// Validate context-dependent flag constraints.
	if err := validateAdvanceFlags(s); err != nil {
		return err
	}

	// Build input.
	var guided *bool
	if cmd.Flags().Changed("guided") {
		g := true
		guided = &g
	}
	if cmd.Flags().Changed("no-guided") {
		g := false
		guided = &g
	}

	in := state.AdvanceInput{
		Verdict:    advanceVerdict,
		EvalReport: advanceEvalReport,
		Message:    advanceMessage,
		File:       advanceFile,
		From:       advanceFrom,
		Guided:     guided,
	}

	out := cmd.OutOrStdout()

	err = state.Advance(s, in, projectRoot)
	if err != nil {
		// Check if it's a validation error — still save state if VALIDATE was entered.
		if ve, ok := err.(*state.ValidationError); ok {
			if err2 := state.Save(stateDir, s); err2 != nil {
				return fmt.Errorf("saving state: %w", err2)
			}
			fmt.Fprintln(out)
			state.PrintAdvanceOutput(out, s, projectRoot)
			fmt.Fprintln(out)
			fmt.Fprintf(out, "FAIL: %d errors in plan.json\n\n", len(ve.Errors))
			for _, e := range ve.Errors {
				fmt.Fprintf(out, "  %s\n", e)
			}
			return fmt.Errorf("validation failed")
		}
		return err
	}

	if err := state.Save(stateDir, s); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	// Archive session at terminal states.
	if isTerminalState(s) {
		domain := sessionDomain(s)
		if archErr := state.ArchiveSession(stateDir, domain, s); archErr != nil {
			fmt.Fprintf(out, "Warning: failed to archive session: %v\n", archErr)
		}
	}

	state.PrintAdvanceOutput(out, s, projectRoot)

	return nil
}

// isTerminalState returns true when the session has reached a terminal point
// that warrants archiving.
func isTerminalState(s *state.ForgeState) bool {
	// Implementing phase complete.
	if s.Phase == state.PhaseImplementing && s.State == state.StateDone {
		return true
	}
	// Specifying phase complete (phase shifting to planning, started at specifying).
	if s.State == state.StatePhaseShift &&
		s.PhaseShift != nil &&
		s.PhaseShift.From == state.PhaseSpecifying &&
		s.StartedAtPhase == state.PhaseSpecifying {
		return true
	}
	return false
}

// sessionDomain returns the domain name for archive file naming.
func sessionDomain(s *state.ForgeState) string {
	if s.Planning != nil && s.Planning.CurrentPlan != nil {
		return s.Planning.CurrentPlan.Domain
	}
	if s.Specifying != nil && len(s.Specifying.Completed) > 0 {
		return s.Specifying.Completed[0].Domain
	}
	return "unknown"
}

func validateAdvanceFlags(s *state.ForgeState) error {
	// --file only valid in specifying DRAFT.
	if advanceFile != "" && !(s.Phase == state.PhaseSpecifying && s.State == state.StateDraft) {
		return fmt.Errorf("--file is only valid in specifying DRAFT state (current: %s %s)", s.Phase, s.State)
	}

	// --verdict only valid in EVALUATE, RECONCILE_EVAL, RECONCILE_REVIEW.
	if advanceVerdict != "" {
		validStates := map[state.StateName]bool{
			state.StateEvaluate:        true,
			state.StateReconcileEval:   true,
			state.StateReconcileReview: true,
		}
		if !validStates[s.State] {
			return fmt.Errorf("--verdict is only valid in EVALUATE, RECONCILE_EVAL, or RECONCILE_REVIEW state (current: %s)", s.State)
		}
	}

	return nil
}
