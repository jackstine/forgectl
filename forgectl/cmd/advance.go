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
	projectRoot, stateDir, _, err := resolveSession()
	if err != nil {
		return err
	}
	s, err := state.Load(stateDir)
	if err != nil {
		return err
	}

	out := cmd.OutOrStdout()

	// Validate context-dependent flag constraints and print warnings.
	if err2 := validateAdvanceFlags(s); err2 != nil {
		return err2
	}
	printAdvanceWarnings(out, s)

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

	// Snapshot state before transition for logging.
	prevState := string(s.State)
	prevPhase := string(s.Phase)

	// Attach logger so state transitions can write phase-specific detail.
	s.Logger = state.NewLogger(s.Config.Logs, s.StartedAtPhase, s.SessionID)

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

	// Activity logging. RE phase writes its own log entries inside state.Advance
	// (with richer domain/round context), so skip the generic cmd-level entry for that phase.
	if prevPhase != string(state.PhaseReverseEngineering) {
		detail := buildAdvanceDetail(in)
		logger := state.NewLogger(s.Config.Logs, s.StartedAtPhase, s.SessionID)
		logger.Write(state.LogEntry{
			TS:        state.LogNow(),
			Cmd:       "advance",
			Phase:     prevPhase,
			PrevState: prevState,
			State:     string(s.State),
			Detail:    detail,
		})
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

// buildAdvanceDetail builds the log detail map from advance flags.
func buildAdvanceDetail(in state.AdvanceInput) map[string]interface{} {
	detail := map[string]interface{}{}
	if in.Verdict != "" {
		detail["verdict"] = in.Verdict
	}
	if in.EvalReport != "" {
		detail["eval_report"] = in.EvalReport
	}
	return detail
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
	// --file valid in specifying DRAFT or reverse_engineering QUEUE.
	if advanceFile != "" {
		specifyingDraft := s.Phase == state.PhaseSpecifying && s.State == state.StateDraft
		reQueue := s.Phase == state.PhaseReverseEngineering && s.State == state.StateQueue
		if !specifyingDraft && !reQueue {
			return fmt.Errorf("--file is only valid in specifying DRAFT or reverse_engineering QUEUE state (current: %s %s)", s.Phase, s.State)
		}
	}

	// --verdict only valid in eval states.
	if advanceVerdict != "" {
		validStates := map[state.StateName]bool{
			state.StateEvaluate:           true,
			state.StateReconcileEval:      true,
			state.StateCrossReferenceEval: true,
		}
		if !validStates[s.State] {
			return fmt.Errorf("--verdict is only valid in EVALUATE, RECONCILE_EVAL, or CROSS_REFERENCE_EVAL state (current: %s)", s.State)
		}
	}

	return nil
}

// printAdvanceWarnings prints warnings about flags that will be ignored due to config settings.
func printAdvanceWarnings(w interface{ Write([]byte) (int, error) }, s *state.ForgeState) {
	evalStates := map[state.StateName]bool{
		state.StateEvaluate:      true,
		state.StateReconcileEval: true,
	}

	// Warn if --eval-report provided but eval output is disabled.
	if advanceEvalReport != "" && evalStates[s.State] {
		var enabled bool
		switch s.Phase {
		case state.PhaseSpecifying:
			enabled = s.Config.Specifying.Eval.EnableEvalOutput
		case state.PhasePlanning:
			enabled = s.Config.Planning.Eval.EnableEvalOutput
		case state.PhaseImplementing:
			enabled = s.Config.Implementing.Eval.EnableEvalOutput
		}
		if !enabled {
			fmt.Fprintf(w, "warning: ignoring --eval-report: eval output is not enabled\n")
		}
	}

	// Warn if --message provided but commits are disabled.
	if advanceMessage != "" && !s.Config.General.EnableCommits {
		commitStates := map[state.StateName]bool{
			state.StateComplete:  true,
			state.StateAccept:    true,
			state.StateImplement: true,
			state.StateCommit:    true,
		}
		if commitStates[s.State] {
			fmt.Fprintf(w, "warning: ignoring --message: commits are not enabled\n")
		}
	}
}
